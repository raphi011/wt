package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

func newMergeCmd() *cobra.Command {
	var (
		into      string
		mark      bool
		strategy  string
		dryRun    bool
		hookNames []string
		noHook    bool
		env       []string
	)

	cmd := &cobra.Command{
		Use:     "merge [scope:]<branch> [-- git-merge-flags...]",
		Short:   "Merge a branch and mark it for pruning",
		GroupID: GroupCore,
		Long: `Merge a worktree's branch into a target branch (default branch by default).

After a successful merge, the source branch is marked in git config so that
'wt prune' can detect it — even for squash merges that git can't detect locally.

Use --mark to retroactively mark a branch that was already merged outside wt.

All arguments after -- are passed through to git merge.`,
		Example: `  wt merge feature-x                        # Merge into default branch
  wt merge feature-x --into develop         # Merge into specific branch
  wt merge myrepo:feature-x                 # Merge from specific repo
  wt merge feature-x -- --squash            # Squash merge
  wt merge feature-x -- --ff-only           # Fast-forward only
  wt merge --mark feature-x --strategy squash  # Retroactively mark as merged`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)
			cfg := config.FromContext(ctx)
			workDir := config.WorkDirFromContext(ctx)

			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("failed to load registry: %w", err)
			}

			// Parse source target
			sourceTarget, err := parseScopedTarget(reg, args[0])
			if err != nil {
				return fmt.Errorf("invalid source: %w", err)
			}

			// Determine source repo
			var sourceRepo registry.Repo
			if len(sourceTarget.Repos) > 0 {
				sourceRepo = sourceTarget.Repos[0]
			} else {
				repoPath := git.GetCurrentRepoMainPathFrom(ctx, workDir)
				if repoPath == "" {
					return fmt.Errorf("not in a git repository and no scope specified")
				}
				repo, err := reg.FindByPath(repoPath)
				if err != nil {
					return fmt.Errorf("current repo not registered: %w", err)
				}
				sourceRepo = repo
			}

			sourceBranch := sourceTarget.Branch

			// Resolve effective config
			effCfg := resolveEffectiveConfig(ctx, sourceRepo.Path)

			// Resolve target branch
			targetBranch := into
			if targetBranch == "" {
				targetBranch = git.GetDefaultBranch(ctx, sourceRepo.Path)
			}

			if sourceBranch == targetBranch {
				return fmt.Errorf("source and target branch are the same: %s", sourceBranch)
			}

			// Check if already marked
			existing, err := git.GetWtMerged(ctx, sourceRepo.Path, sourceBranch)
			if err != nil {
				return fmt.Errorf("failed to check merge status: %w", err)
			}
			if existing != "" {
				out.Printf("Branch %q already marked as merged: %s\n", sourceBranch, existing)
				return nil
			}

			// Mark-only mode
			if mark {
				return runMergeMark(ctx, out, sourceRepo.Path, sourceBranch, targetBranch, strategy, dryRun)
			}

			// Find target worktree
			worktrees, err := git.ListWorktreesFromRepo(ctx, sourceRepo.Path)
			if err != nil {
				return fmt.Errorf("failed to list worktrees: %w", err)
			}

			var targetWorktreePath string
			for _, wt := range worktrees {
				if wt.Branch == targetBranch {
					targetWorktreePath = wt.Path
					break
				}
			}
			if targetWorktreePath == "" {
				return fmt.Errorf("no worktree found for branch %q — run 'wt checkout %s' first", targetBranch, targetBranch)
			}

			// Check target worktree is clean
			clean, err := git.IsWorktreeClean(ctx, targetWorktreePath)
			if err != nil {
				return fmt.Errorf("failed to check target worktree: %w", err)
			}
			if !clean {
				return fmt.Errorf("target worktree %q has uncommitted changes — commit or stash first", targetBranch)
			}

			// Collect git passthrough args (everything after --)
			dashIdx := cmd.ArgsLenAtDash()
			var passthroughArgs []string
			if dashIdx >= 0 && dashIdx < len(args) {
				passthroughArgs = args[dashIdx:]
			}

			// Detect strategy and flags
			detectedStrategy := git.DetectMergeStrategy(passthroughArgs)
			noCommit := git.HasNoCommitFlag(passthroughArgs)
			isSquash := detectedStrategy == "squash"

			if dryRun {
				out.Printf("Would merge %q into %q (strategy: %s)\n", sourceBranch, targetBranch, detectedStrategy)
				if isSquash && !noCommit {
					out.Println("Would auto-commit after squash merge")
				}
				out.Printf("Would mark branch with: %s\n", git.FormatWtMerged(detectedStrategy, targetBranch, time.Now().UTC()))
				return nil
			}

			// Resolve config dir for hooks
			configDir, err := effCfg.GetWtDir()
			if err != nil {
				return fmt.Errorf("config dir: %w", err)
			}

			// Run before hooks
			hookEnv, err := hooks.ParseEnvWithStdin(env)
			if err != nil {
				return fmt.Errorf("failed to parse hook env: %w", err)
			}
			hookCtx := hooks.Context{
				WorktreeDir: targetWorktreePath,
				RepoDir:     sourceRepo.Path,
				Branch:      sourceBranch,
				Repo:        sourceRepo.Name,
				Trigger:     string(hooks.CommandMerge),
				Phase:       hooks.PhaseBefore,
				ConfigDir:   configDir,
				Env:         hookEnv,
				DryRun:      dryRun,
			}

			beforeMatches, err := hooks.SelectHooks(effCfg.Hooks, hookNames, noHook, hooks.CommandMerge, "", hooks.PhaseBefore)
			if err != nil {
				return fmt.Errorf("failed to select hooks: %w", err)
			}
			if err := hooks.RunBeforeHooks(ctx, beforeMatches, hookCtx, targetWorktreePath); err != nil {
				return fmt.Errorf("before hook failed: %w", err)
			}

			// Build and run git merge
			mergeArgs := []string{"merge", sourceBranch}
			mergeArgs = append(mergeArgs, passthroughArgs...)

			l.Debug("running git merge", "target", targetWorktreePath, "args", mergeArgs)

			mergeCmd := exec.CommandContext(ctx, "git", mergeArgs...)
			mergeCmd.Dir = targetWorktreePath
			mergeCmd.Stdout = os.Stdout
			mergeCmd.Stderr = os.Stderr
			mergeCmd.Stdin = os.Stdin

			if err := mergeCmd.Run(); err != nil {
				return fmt.Errorf("git merge failed: %w", err)
			}

			// Auto-commit for squash
			if isSquash && !noCommit {
				l.Debug("auto-committing squash merge")

				commitCmd := exec.CommandContext(ctx, "git", "commit", "--no-edit")
				commitCmd.Dir = targetWorktreePath
				commitCmd.Stdout = os.Stdout
				commitCmd.Stderr = os.Stderr
				commitCmd.Stdin = os.Stdin

				if err := commitCmd.Run(); err != nil {
					return fmt.Errorf("git commit after squash merge failed: %w", err)
				}
			}

			// squash + no-commit: don't mark
			if isSquash && noCommit {
				out.Printf("Warning: branch %q not marked as merged — commit manually, then run 'wt merge --mark %s'\n", sourceBranch, sourceBranch)
				return nil
			}

			// Mark as merged
			mergedValue := git.FormatWtMerged(detectedStrategy, targetBranch, time.Now().UTC())
			if err := git.SetWtMerged(ctx, sourceRepo.Path, sourceBranch, mergedValue); err != nil {
				return fmt.Errorf("merge succeeded but failed to mark branch: %w", err)
			}

			out.Printf("Merged %q into %q (strategy: %s)\n", sourceBranch, targetBranch, detectedStrategy)
			out.Printf("Branch marked for pruning — run 'wt prune' to clean up\n")

			// Run after hooks
			hookCtx.Phase = hooks.PhaseAfter
			afterMatches, err := hooks.SelectHooks(effCfg.Hooks, hookNames, noHook, hooks.CommandMerge, "", hooks.PhaseAfter)
			if err != nil {
				l.Printf("Warning: failed to select after hooks: %v\n", err)
			}
			if len(afterMatches) > 0 {
				hooks.RunAllNonFatal(ctx, afterMatches, hookCtx, targetWorktreePath)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&into, "into", "", "target branch (default: repo's default branch)")
	cmd.Flags().BoolVar(&mark, "mark", false, "mark branch as merged without running git merge")
	cmd.Flags().StringVarP(&strategy, "strategy", "s", "merge", "merge strategy to record (only with --mark)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "show what would happen without doing it")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "run specific hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "skip all hooks")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "set hook variable KEY=VALUE")
	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")

	registerMergeCompletions(cmd)

	return cmd
}

func runMergeMark(ctx context.Context, out *output.Printer, repoPath, branch, target, strategy string, dryRun bool) error {
	// Verify branch exists
	if !git.LocalBranchExists(ctx, repoPath, branch) {
		return fmt.Errorf("branch %q not found", branch)
	}

	if strategy == "" {
		strategy = "merge"
	}

	mergedValue := git.FormatWtMerged(strategy, target, time.Now().UTC())

	if dryRun {
		out.Printf("Would mark %q as merged: %s\n", branch, mergedValue)
		return nil
	}

	if err := git.SetWtMerged(ctx, repoPath, branch, mergedValue); err != nil {
		return fmt.Errorf("failed to mark branch: %w", err)
	}

	out.Printf("Marked %q as merged: %s\n", branch, mergedValue)
	return nil
}

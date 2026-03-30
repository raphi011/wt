package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newDiffCmd() *cobra.Command {
	var (
		stat     bool
		nameOnly bool
		base     string
		working  bool
		tool     string
	)

	cmd := &cobra.Command{
		Use:     "diff [[scope:]branch]",
		Short:   "Show worktree diff",
		Aliases: []string{"d"},
		GroupID: GroupUtility,
		Args:    cobra.MaximumNArgs(1),
		Long: `Show the diff of a worktree branch against its base.

By default shows what a PR would contain: changes introduced on the branch
since it diverged from the default branch (e.g. origin/main).

The argument can be:
  - branch name: finds worktree in current repo, or all repos if ambiguous
  - repo:branch: finds exact worktree in specified repo
  - label:branch: finds worktree in repos with that label

With no arguments, diffs the current worktree.`,
		Example: `  wt diff                        # Diff current worktree vs origin/main
  wt diff feature-x              # Diff feature-x worktree
  wt diff wt:feature-x           # Diff feature-x in wt repo
  wt diff --working              # Show uncommitted changes
  wt diff --stat                 # Show diffstat summary only
  wt diff --base origin/develop  # Diff against develop instead of main
  wt diff --tool delta           # Use delta as pager for this diff`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			workDir := config.WorkDirFromContext(ctx)
			l := log.FromContext(ctx)

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Resolve target worktree
			var wtPath string

			if len(args) == 0 {
				repoPath := git.GetCurrentRepoMainPathFrom(ctx, workDir)
				if repoPath == "" {
					return fmt.Errorf("not in a git repository")
				}
				wtPath = workDir
			} else {
				match, err := resolveOneWorktreeTarget(ctx, reg, args[0])
				if err != nil {
					return err
				}
				wtPath = match.Path
			}

			// Build git args
			var gitArgs []string

			if tool != "" {
				gitArgs = append(gitArgs, "-c", "core.pager="+tool)
			}

			gitArgs = append(gitArgs, "diff")

			if stat {
				gitArgs = append(gitArgs, "--stat")
			}
			if nameOnly {
				gitArgs = append(gitArgs, "--name-only")
			}

			if working {
				gitArgs = append(gitArgs, "HEAD")
			} else {
				// Determine base ref
				baseRef := base
				if baseRef == "" {
					defaultBranch := git.GetDefaultBranch(ctx, wtPath)
					baseRef = "origin/" + defaultBranch
				}
				if !git.RefExists(ctx, wtPath, baseRef) {
					return fmt.Errorf("base ref %q not found; use --base to specify a different base", baseRef)
				}
				gitArgs = append(gitArgs, baseRef+"...HEAD")
			}

			l.Debug("diff", "path", wtPath, "args", gitArgs)

			// Execute git diff with full terminal passthrough
			gitCmd := exec.CommandContext(ctx, "git", gitArgs...)
			gitCmd.Dir = wtPath
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr
			gitCmd.Stdin = os.Stdin

			if err := gitCmd.Run(); err != nil {
				// An error here likely means git itself failed (bad ref, etc.)
				// since git diff exits 0 by default regardless of differences.
				if exitErr, ok := err.(*exec.ExitError); ok {
					return fmt.Errorf("git diff failed (exit %d)", exitErr.ExitCode())
				}
				return fmt.Errorf("git diff: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&stat, "stat", false, "Show diffstat summary only")
	cmd.Flags().BoolVar(&nameOnly, "name-only", false, "Show only names of changed files")
	cmd.Flags().StringVar(&base, "base", "", "Override comparison base ref (default: origin/<default-branch>)")
	cmd.Flags().BoolVar(&working, "working", false, "Show uncommitted changes (diff against HEAD)")
	cmd.Flags().StringVarP(&tool, "tool", "t", "", "Override pager for this diff (e.g. delta, bat)")

	cmd.MarkFlagsMutuallyExclusive("base", "working")

	// Register completions
	cmd.ValidArgsFunction = completeDiffArg
	cmd.RegisterFlagCompletionFunc("base", completeBaseBranches)

	return cmd
}

// completeDiffArg provides completion for `wt diff [scope:]branch`
func completeDiffArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeScopedWorktreeArg(cmd, args, toComplete)
}

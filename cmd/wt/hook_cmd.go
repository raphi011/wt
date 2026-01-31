package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newHookCmd() *cobra.Command {
	var (
		repository []string
		labels     []string
		env        []string
		dryRun     bool
		branch     string
	)

	cmd := &cobra.Command{
		Use:               "hook <name>...",
		Short:             "Run configured hook",
		GroupID:           GroupUtility,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeHooks,
		Long: `Run one or more configured hooks.

Hooks are defined in config.toml and can use placeholders.
By default runs in current repo/worktree; use -r/-l to target other repos.`,
		Example: `  wt hook code                # Run 'code' hook in current repo
  wt hook code -r myrepo      # Run in specific repo
  wt hook code idea           # Run multiple hooks
  wt hook code -d             # Dry-run: print command without executing
  wt hook code --branch feat  # Run in specific worktree`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			hookNames := args

			// Validate all hooks exist
			var missing []string
			for _, name := range hookNames {
				if _, exists := cfg.Hooks.Hooks[name]; !exists {
					missing = append(missing, name)
				}
			}
			if len(missing) > 0 {
				var available []string
				for name := range cfg.Hooks.Hooks {
					available = append(available, name)
				}
				if len(available) == 0 {
					return fmt.Errorf("unknown hook(s) %v (no hooks configured)", missing)
				}
				return fmt.Errorf("unknown hook(s) %v (available: %v)", missing, available)
			}

			// Parse env variables
			hookEnv, err := hooks.ParseEnvWithStdin(env)
			if err != nil {
				return err
			}

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Resolve target repos
			repos, err := resolveTargetRepos(ctx, reg, repository, labels)
			if err != nil {
				return err
			}

			if len(repos) == 0 {
				return fmt.Errorf("no repos found")
			}

			l.Debug("running hooks", "hooks", hookNames, "repos", len(repos), "dryRun", dryRun)

			// If branch specified, find and run in that worktree
			if branch != "" {
				return runHooksInBranch(ctx, repos, hookNames, branch, hookEnv, dryRun)
			}

			// Run hooks in each repo
			var errs []error
			for _, repo := range repos {
				if err := runHooksInRepo(ctx, repo, hookNames, hookEnv, dryRun); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", repo.Name, err))
				}
			}

			if len(errs) > 0 {
				return fmt.Errorf("failed to run hooks in some repos:\n%w", errors.Join(errs...))
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Repository name(s)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Target repos by label")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Print command without executing")
	cmd.Flags().StringVar(&branch, "branch", "", "Target specific worktree by branch")

	// Completions
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("label", completeLabels)

	return cmd
}

// runHooksInRepo runs hooks in the repo's current worktree or main repo
func runHooksInRepo(_ context.Context, repo *registry.Repo, hookNames []string, env map[string]string, dryRun bool) error {
	// Create hook context
	hookCtx := hooks.Context{
		RepoDir: repo.Path,
		Repo:    repo.Name,
		Origin:  repo.Name,
		Trigger: "run",
		Env:     env,
		DryRun:  dryRun,
	}

	// Try to determine if we're in a worktree of this repo
	// For now, just run in the repo directory
	hookCtx.WorktreeDir = repo.Path

	return runHooksForContext(hookNames, cfg.Hooks.Hooks, hookCtx, repo.Path)
}

// runHooksInBranch runs hooks in a specific branch's worktree
func runHooksInBranch(ctx context.Context, repos []*registry.Repo, hookNames []string, branch string, env map[string]string, dryRun bool) error {
	// Find the worktree
	for _, repo := range repos {
		wtInfos, err := git.ListWorktreesFromRepo(ctx, repo.Path)
		if err != nil {
			continue
		}
		for _, wti := range wtInfos {
			if wti.Branch == branch {
				hookCtx := hooks.Context{
					WorktreeDir: wti.Path,
					RepoDir:     repo.Path,
					Branch:      branch,
					Repo:        repo.Name,
					Origin:      repo.Name,
					Trigger:     "run",
					Env:         env,
					DryRun:      dryRun,
				}
				return runHooksForContext(hookNames, cfg.Hooks.Hooks, hookCtx, repo.Path)
			}
		}
	}
	return fmt.Errorf("worktree for branch %q not found", branch)
}

// runHooksForContext runs the specified hooks in the given context
func runHooksForContext(hookNames []string, hooksMap map[string]config.Hook, hookCtx hooks.Context, _ string) error {
	// Collect matching hooks
	var toRun []config.Hook
	for _, name := range hookNames {
		if hook, ok := hooksMap[name]; ok {
			toRun = append(toRun, hook)
		}
	}

	// Run each hook
	for i, name := range hookNames {
		if err := hooks.RunSingle(name, &toRun[i], hookCtx); err != nil {
			return fmt.Errorf("hook %s: %w", name, err)
		}
	}

	return nil
}

// resolveHookContext creates a hook context from the current directory
func resolveHookContext(ctx context.Context, workDir string) (hooks.Context, error) {
	// Check if we're in a git repo or worktree
	repoPath := git.GetCurrentRepoMainPath(ctx)
	if repoPath == "" {
		return hooks.Context{}, fmt.Errorf("not in a git repository")
	}

	// Get current branch
	branch, _ := git.GetCurrentBranch(ctx, workDir)

	return hooks.Context{
		WorktreeDir: workDir,
		RepoDir:     repoPath,
		Branch:      branch,
		Repo:        filepath.Base(repoPath),
		Origin:      git.GetRepoDisplayName(repoPath),
		Trigger:     "run",
	}, nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newHookCmd() *cobra.Command {
	var (
		env    []string
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:               "hook <name>... [-- [scope:]branch...]",
		Short:             "Run configured hook",
		Aliases:           []string{"h"},
		GroupID:           GroupUtility,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeHookArg,
		Long: `Run one or more configured hooks.

Hooks are defined in config.toml and can use placeholders.
By default runs in current worktree. Use -- followed by targets to run in specific worktrees.

Target worktrees using [scope:]branch format where scope can be a repo name or label.`,
		Example: `  wt hook code                        # Run 'code' hook in current worktree
  wt hook code idea                   # Run multiple hooks
  wt hook code -- main                # Run in main worktree (all repos)
  wt hook code -- myrepo:main         # Run in specific repo's worktree
  wt hook code -- backend:main        # Run in backend label's main worktrees
  wt hook code -d                     # Dry-run: print command without executing`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			workDir := config.WorkDirFromContext(ctx)
			l := log.FromContext(ctx)

			// Split args at "--" into hook names and targets
			hookNames, targets := splitHookArgs(args, cmd.ArgsLenAtDash())

			// Parse env variables
			hookEnv, err := hooks.ParseEnvWithStdin(env)
			if err != nil {
				return err
			}

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			l.Debug("running hooks", "hooks", hookNames, "targets", targets, "dryRun", dryRun)

			// If no targets, run in current worktree
			if len(targets) == 0 {
				repo, err := findOrRegisterCurrentRepoFromContext(ctx, reg)
				if err != nil {
					return err
				}
				return runHooksInRepo(ctx, repo, hookNames, hookEnv, dryRun, workDir)
			}

			// Run hooks in specified targets
			return runHooksInTargets(ctx, reg, hookNames, targets, hookEnv, dryRun)
		},
	}

	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Print command without executing")
	cmd.RegisterFlagCompletionFunc("arg", cobra.NoFileCompletions)

	return cmd
}

// splitHookArgs splits args into hook names and worktree targets based on -- position
func splitHookArgs(args []string, dashIdx int) (hookNames, targets []string) {
	if dashIdx == -1 {
		// No --, all args are hook names
		return args, nil
	}
	return args[:dashIdx], args[dashIdx:]
}

// runHooksInRepo runs hooks in the repo's current worktree or main repo
func runHooksInRepo(ctx context.Context, repo registry.Repo, hookNames []string, env map[string]string, dryRun bool, workDir string) error {
	l := log.FromContext(ctx)

	// Resolve effective config for this repo
	resolver := config.ResolverFromContext(ctx)
	effCfg, err := resolver.ConfigForRepo(repo.Path)
	if err != nil {
		l.Printf("Warning: failed to load local config for %s: %v\n", repo.Name, err)
		effCfg = resolver.Global()
	}

	// Validate all hooks exist in effective config
	var missing []string
	for _, name := range hookNames {
		if _, exists := effCfg.Hooks.Hooks[name]; !exists {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		var available []string
		for name := range effCfg.Hooks.Hooks {
			available = append(available, name)
		}
		if len(available) == 0 {
			return fmt.Errorf("unknown hook(s) %v (no hooks configured)", missing)
		}
		return fmt.Errorf("unknown hook(s) %v (available: %v)", missing, available)
	}

	// Get current branch if in a worktree
	branch, _ := git.GetCurrentBranch(ctx, workDir)

	// Create hook context
	hookCtx := hooks.Context{
		WorktreeDir: workDir,
		RepoDir:     repo.Path,
		Branch:      branch,
		Repo:        repo.Name,
		Origin:      repo.Name,
		Trigger:     "run",
		Env:         env,
		DryRun:      dryRun,
	}

	return runHooksForContext(ctx, hookNames, effCfg.Hooks.Hooks, hookCtx, repo.Path)
}

// runHooksInTargets runs hooks in specified [scope:]branch targets
func runHooksInTargets(ctx context.Context, reg *registry.Registry, hookNames []string, targets []string, env map[string]string, dryRun bool) error {
	l := log.FromContext(ctx)
	resolver := config.ResolverFromContext(ctx)

	// Resolve all targets
	wtTargets, err := resolveWorktreeTargets(ctx, reg, targets)
	if err != nil {
		return err
	}

	// Run hooks in each target
	var errs []error
	for _, wt := range wtTargets {
		// Resolve effective config for each repo
		effCfg, resolveErr := resolver.ConfigForRepo(wt.RepoPath)
		if resolveErr != nil {
			l.Printf("Warning: failed to load local config for %s: %v\n", wt.RepoName, resolveErr)
			effCfg = resolver.Global()
		}

		// Validate hooks exist for this repo
		var missing []string
		for _, name := range hookNames {
			if _, exists := effCfg.Hooks.Hooks[name]; !exists {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			errs = append(errs, fmt.Errorf("%s:%s: unknown hook(s) %v", wt.RepoName, wt.Branch, missing))
			continue
		}

		hookCtx := hooks.Context{
			WorktreeDir: wt.Path,
			RepoDir:     wt.RepoPath,
			Branch:      wt.Branch,
			Repo:        wt.RepoName,
			Origin:      wt.RepoName,
			Trigger:     "run",
			Env:         env,
			DryRun:      dryRun,
		}
		if err := runHooksForContext(ctx, hookNames, effCfg.Hooks.Hooks, hookCtx, wt.RepoPath); err != nil {
			errs = append(errs, fmt.Errorf("%s:%s: %w", wt.RepoName, wt.Branch, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to run hooks in some worktrees:\n%w", errors.Join(errs...))
	}
	return nil
}

// runHooksForContext runs the specified hooks in the given context
func runHooksForContext(ctx context.Context, hookNames []string, hooksMap map[string]config.Hook, hookCtx hooks.Context, _ string) error {
	// Collect matching hooks
	var toRun []config.Hook
	for _, name := range hookNames {
		if hook, ok := hooksMap[name]; ok {
			toRun = append(toRun, hook)
		}
	}

	// Run each hook
	for i, name := range hookNames {
		if err := hooks.RunSingle(ctx, name, &toRun[i], hookCtx); err != nil {
			return fmt.Errorf("hook %s: %w", name, err)
		}
	}

	return nil
}

// completeHookArg provides completion for hook command arguments
func completeHookArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()

	// After --, complete worktree targets
	if slices.Contains(args, "--") {
		return completeScopedWorktreeArg(cmd, nil, toComplete)
	}

	// Check if user is typing after --
	if strings.HasPrefix(toComplete, "--") {
		return []string{"--"}, cobra.ShellCompDirectiveNoFileComp
	}

	// Use resolver to get effective hooks (includes local hooks if in a repo)
	hooksMap := getEffectiveHooksForCompletion(ctx)

	// Complete hook names
	var hookNames []string
	for name := range hooksMap {
		if strings.HasPrefix(name, toComplete) {
			hookNames = append(hookNames, name)
		}
	}

	// Also suggest -- to switch to target completion
	if toComplete == "" || strings.HasPrefix("--", toComplete) {
		hookNames = append(hookNames, "--")
	}

	return hookNames, cobra.ShellCompDirectiveNoFileComp
}

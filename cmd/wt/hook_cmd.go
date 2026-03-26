package main

import (
	"context"
	"errors"
	"fmt"
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
		Use:               "hook [[scope:]branch] <name>",
		Short:             "Run configured hook",
		Aliases:           []string{"h"},
		GroupID:           GroupUtility,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeHookArg,
		Long: `Run a configured hook manually.

Hooks are defined in config.toml with an "on" trigger and optional placeholders.
When run manually, the hook always executes as an "after" hook
({phase}=after, {trigger}=run, {action}=manual).

With one argument, runs in the current worktree.
With two arguments, the first is a [scope:]branch target and the second is the hook name.

Target worktrees using [scope:]branch format where scope can be a repo name or label.

Trigger syntax for "on" field:
  [before:|after:]trigger[:subtype]

  Triggers: checkout, prune, merge, all
  Subtypes (checkout only): create, open, pr
  Timing: before (can cancel operation), after (default)

Examples:
  on = ["checkout"]              # All checkouts (after)
  on = ["checkout:pr"]           # PR checkouts only
  on = ["before:prune"]          # Pre-prune guard (can abort)
  on = ["before:checkout:pr"]    # Before PR checkout only`,
		Example: `  wt hook code                        # Run 'code' hook in current worktree
  wt hook main code                   # Run 'code' in main worktree (all repos)
  wt hook myrepo:main code            # Run in specific repo's worktree
  wt hook backend:main code           # Run in backend label's main worktrees
  wt hook code -a prompt="do X"       # Pass custom variable
  wt hook code -d                     # Dry-run: print command without executing`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			workDir := config.WorkDirFromContext(ctx)
			l := log.FromContext(ctx)

			// Parse positional args: [[scope:]branch] <hookname>
			var hookName string
			var target string

			if len(args) == 2 {
				target = args[0]
				hookName = args[1]
			} else {
				hookName = args[0]
			}

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

			l.Debug("running hook", "hook", hookName, "target", target, "dryRun", dryRun)

			// If no target, run in current worktree
			if target == "" {
				repo, err := findOrRegisterCurrentRepoFromContext(ctx, reg)
				if err != nil {
					return err
				}
				return runHookInRepo(ctx, repo, hookName, hookEnv, dryRun, workDir)
			}

			// Run hook in specified target
			return runHookInTargets(ctx, reg, hookName, target, hookEnv, dryRun)
		},
	}

	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable (KEY=VALUE or KEY for boolean)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Print command without executing")
	cmd.RegisterFlagCompletionFunc("arg", cobra.NoFileCompletions)

	return cmd
}

// runHookInRepo runs a hook in the repo's current worktree or main repo
func runHookInRepo(ctx context.Context, repo registry.Repo, hookName string, env map[string]string, dryRun bool, workDir string) error {
	effCfg := resolveEffectiveConfig(ctx, repo.Path)

	hook, exists := effCfg.Hooks.Hooks[hookName]
	if !exists {
		return unknownHookError(hookName, effCfg.Hooks.Hooks)
	}

	branch, _ := git.GetCurrentBranch(ctx, workDir)

	hookCtx := hooks.Context{
		WorktreeDir: workDir,
		RepoDir:     repo.Path,
		Branch:      branch,
		Repo:        repo.Name,
		Origin:      repo.Name,
		Trigger:     "run",
		Action:      "manual",
		Phase:       "after",
		Env:         env,
		DryRun:      dryRun,
	}

	if err := hooks.RunSingle(ctx, hookName, &hook, hookCtx); err != nil {
		return fmt.Errorf("hook %s: %w", hookName, err)
	}
	return nil
}

// runHookInTargets runs a hook in the specified [scope:]branch target
func runHookInTargets(ctx context.Context, reg *registry.Registry, hookName string, target string, env map[string]string, dryRun bool) error {
	wtTargets, err := resolveWorktreeTargets(ctx, reg, []string{target})
	if err != nil {
		return err
	}

	var errs []error
	for _, wt := range wtTargets {
		effCfg := resolveEffectiveConfig(ctx, wt.RepoPath)

		hook, exists := effCfg.Hooks.Hooks[hookName]
		if !exists {
			errs = append(errs, fmt.Errorf("%s:%s: %w", wt.RepoName, wt.Branch, unknownHookError(hookName, effCfg.Hooks.Hooks)))
			continue
		}

		hookCtx := hooks.Context{
			WorktreeDir: wt.Path,
			RepoDir:     wt.RepoPath,
			Branch:      wt.Branch,
			Repo:        wt.RepoName,
			Origin:      wt.RepoName,
			Trigger:     "run",
			Action:      "manual",
			Phase:       "after",
			Env:         env,
			DryRun:      dryRun,
		}
		if err := hooks.RunSingle(ctx, hookName, &hook, hookCtx); err != nil {
			errs = append(errs, fmt.Errorf("%s:%s: hook %s: %w", wt.RepoName, wt.Branch, hookName, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to run hook in some worktrees:\n%w", errors.Join(errs...))
	}
	return nil
}

// unknownHookError returns a formatted error for an unknown hook name
func unknownHookError(hookName string, hooksMap map[string]config.Hook) error {
	var available []string
	for name := range hooksMap {
		available = append(available, name)
	}
	if len(available) == 0 {
		return fmt.Errorf("unknown hook %q (no hooks configured)", hookName)
	}
	return fmt.Errorf("unknown hook %q (available: %v)", hookName, available)
}

// completeHookArg provides completion for hook command arguments.
// Position 0: hook names + [scope:]branch targets (could be either).
// Position 1: hook names only (position 0 was a target).
func completeHookArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()

	if len(args) >= 2 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Get hook names for completion
	hooksMap := getEffectiveHooksForCompletion(ctx)
	var completions []string
	for name := range hooksMap {
		if strings.HasPrefix(name, toComplete) {
			completions = append(completions, name)
		}
	}

	if len(args) == 0 {
		// Position 0: could be a hook name or a [scope:]branch target
		targets, _ := completeScopedWorktreeArg(cmd, nil, toComplete)
		completions = append(completions, targets...)
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/resolve"
)

func (c *HookCmd) runHookRun(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	// Validate all hooks exist upfront
	var missing []string
	for _, name := range c.Hooks {
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
	env, err := hooks.ParseEnvWithStdin(c.Env)
	if err != nil {
		return err
	}

	// Determine targeting mode
	hasID := len(c.ID) > 0
	hasRepo := len(c.Repository) > 0 || len(c.Label) > 0

	// If no targeting specified, use current worktree
	if !hasID && !hasRepo {
		hookCtx, err := resolveHookTargetCurrentPath(ctx, workDir)
		if err != nil {
			return err
		}
		hookCtx.Env = env
		hookCtx.DryRun = c.DryRun
		return runHooksForContext(c.Hooks, cfg, hookCtx)
	}

	worktreeDir, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Mode: by ID (worktrees)
	if hasID {
		var errs []error
		for _, id := range c.ID {
			if err := runHookForID(id, c.Hooks, worktreeDir, cfg, env, c.DryRun); err != nil {
				errs = append(errs, fmt.Errorf("ID %d: %w", id, err))
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("failed to run hooks in some worktrees:\n%w", errors.Join(errs...))
		}
		return nil
	}

	// Mode: by repo/label
	return runHookForRepos(ctx, c.Repository, c.Label, c.Hooks, worktreeDir, cfg, env, c.DryRun)
}

func runHookForID(id int, hookNames []string, worktreeDir string, cfg *config.Config, env map[string]string, dryRun bool) error {
	ctx, err := resolveHookTargetByID(id, worktreeDir)
	if err != nil {
		return err
	}
	ctx.Env = env
	ctx.DryRun = dryRun
	return runHooksForContext(hookNames, cfg, ctx)
}

func runHookForRepos(ctx context.Context, repos []string, labels []string, hookNames []string, dir string, cfg *config.Config, env map[string]string, dryRun bool) error {
	scanDir, err := resolveRepoScanDir(dir, cfg)
	if err != nil {
		return err
	}

	repoPaths, errs := collectRepoPaths(ctx, repos, labels, scanDir, cfg)

	// Run hooks for each repo
	for repoPath := range repoPaths {
		repoName := git.GetRepoDisplayName(repoPath)

		ctx := hooks.ContextFromRepo(repoPath, "run", env)
		ctx.DryRun = dryRun

		if err := runHooksForContext(hookNames, cfg, ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to run hooks in some repos:\n%w", errors.Join(errs...))
	}
	return nil
}

func runHooksForContext(hookNames []string, cfg *config.Config, ctx hooks.Context) error {
	for _, name := range hookNames {
		hook := cfg.Hooks.Hooks[name]
		if err := hooks.RunSingle(name, &hook, ctx); err != nil {
			return err
		}
	}
	return nil
}

// resolveHookTargetCurrent resolves the context for the current worktree or repo.
func resolveHookTargetCurrent(ctx context.Context) (hooks.Context, error) {
	target, err := resolve.FromCurrentWorktreeOrRepo(ctx)
	if err != nil {
		return hooks.Context{}, fmt.Errorf("use -i, -r, or -l when not inside a git repo (run 'wt list' to see IDs)")
	}
	return hooks.ContextFromWorktree(target, "run", nil), nil
}

// resolveHookTargetCurrentPath resolves the context for the given path (worktree or repo).
func resolveHookTargetCurrentPath(ctx context.Context, workDir string) (hooks.Context, error) {
	target, err := resolve.FromWorktreeOrRepoPath(ctx, workDir)
	if err != nil {
		return hooks.Context{}, fmt.Errorf("use -i, -r, or -l when not inside a git repo (run 'wt list' to see IDs)")
	}
	return hooks.ContextFromWorktree(target, "run", nil), nil
}

// resolveHookTargetByID resolves the worktree context by ID.
func resolveHookTargetByID(id int, worktreeDir string) (hooks.Context, error) {
	target, err := resolve.ByID(id, worktreeDir)
	if err != nil {
		return hooks.Context{}, err
	}
	return hooks.ContextFromWorktree(target, "run", nil), nil
}

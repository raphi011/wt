package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/resolve"
)

func runHookRun(cmd *HookCmd, cfg *config.Config) error {
	// Validate all hooks exist upfront
	var missing []string
	for _, name := range cmd.Hooks {
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
	env, err := hooks.ParseEnvWithStdin(cmd.Env)
	if err != nil {
		return err
	}

	// If no IDs provided, use current worktree (optional behavior)
	if len(cmd.ID) == 0 {
		ctx, err := resolveHookTargetCurrent()
		if err != nil {
			return err
		}
		ctx.Env = env
		return runHooksForContext(cmd.Hooks, cfg, ctx)
	}

	// Execute for each ID
	var errs []error
	for _, id := range cmd.ID {
		if err := runHookForID(id, cmd.Hooks, cmd.Dir, cfg, env); err != nil {
			errs = append(errs, fmt.Errorf("ID %d: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to run hooks in some worktrees:\n%w", joinErrors(errs))
	}
	return nil
}

func runHookForID(id int, hookNames []string, dir string, cfg *config.Config, env map[string]string) error {
	ctx, err := resolveHookTargetByID(id, dir)
	if err != nil {
		return err
	}
	ctx.Env = env
	return runHooksForContext(hookNames, cfg, ctx)
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

// resolveHookTargetCurrent resolves the worktree context for the current worktree.
func resolveHookTargetCurrent() (hooks.Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return hooks.Context{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	if !git.IsWorktree(cwd) {
		return hooks.Context{}, fmt.Errorf("--id required when not inside a worktree (run 'wt list' to see IDs)")
	}

	branch, err := git.GetCurrentBranch(cwd)
	if err != nil {
		return hooks.Context{}, fmt.Errorf("failed to get current branch: %w", err)
	}

	mainRepo, err := git.GetMainRepoPath(cwd)
	if err != nil {
		return hooks.Context{}, fmt.Errorf("failed to get main repo path: %w", err)
	}

	ctx := hooks.Context{
		Path:     cwd,
		Branch:   branch,
		MainRepo: mainRepo,
		Folder:   filepath.Base(mainRepo),
		Trigger:  "run",
	}
	ctx.Repo, _ = git.GetRepoNameFrom(mainRepo)

	return ctx, nil
}

// resolveHookTargetByID resolves the worktree context by ID.
func resolveHookTargetByID(id int, dir string) (hooks.Context, error) {
	scanPath := dir
	if scanPath == "" {
		scanPath = "."
	}
	scanPath, err := filepath.Abs(scanPath)
	if err != nil {
		return hooks.Context{}, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	resolved, err := resolve.ByID(id, scanPath)
	if err != nil {
		return hooks.Context{}, err
	}

	// Validate path still exists
	if _, err := os.Stat(resolved.Path); os.IsNotExist(err) {
		return hooks.Context{}, fmt.Errorf("worktree path no longer exists: %s (run 'wt list' to refresh)", resolved.Path)
	}

	ctx := hooks.Context{
		Path:     resolved.Path,
		Branch:   resolved.Branch,
		MainRepo: resolved.MainRepo,
		Folder:   filepath.Base(resolved.MainRepo),
		Trigger:  "run",
	}
	ctx.Repo, _ = git.GetRepoNameFrom(resolved.MainRepo)

	return ctx, nil
}

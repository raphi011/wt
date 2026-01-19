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

func runHookCmd(cmd *HookCmd, cfg config.Config) error {
	switch {
	case cmd.Run != nil:
		return runHookRun(cmd.Run, cfg)
	default:
		return fmt.Errorf("no subcommand specified (try: wt hook run)")
	}
}

func runHookRun(cmd *HookRunCmd, cfg config.Config) error {
	hookName := cmd.Hook
	target := cmd.Target

	// Validate hook exists
	hook, exists := cfg.Hooks.Hooks[hookName]
	if !exists {
		// List available hooks
		var available []string
		for name := range cfg.Hooks.Hooks {
			available = append(available, name)
		}
		if len(available) == 0 {
			return fmt.Errorf("unknown hook %q (no hooks configured)", hookName)
		}
		return fmt.Errorf("unknown hook %q (available: %v)", hookName, available)
	}

	// Resolve target
	ctx, err := resolveHookTarget(target, cmd.Dir)
	if err != nil {
		return err
	}

	// Run the hook
	return hooks.RunSingle(hookName, &hook, ctx)
}

// resolveHookTarget resolves the worktree context for hook execution.
// If target is empty and inside a worktree, uses the current worktree.
// Otherwise, uses the shared resolver to look up by ID or branch name.
func resolveHookTarget(target string, dir string) (hooks.Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return hooks.Context{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	inWorktree := git.IsWorktree(cwd)

	// If no target provided and inside a worktree, use current worktree
	if target == "" {
		if !inWorktree {
			return hooks.Context{}, fmt.Errorf("target required when not inside a worktree (run 'wt list' to see IDs)")
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

	// Target provided - resolve via ID or branch
	scanPath := dir
	if scanPath == "" {
		scanPath = "."
	}
	scanPath, err = expandPath(scanPath)
	if err != nil {
		return hooks.Context{}, fmt.Errorf("failed to expand path: %w", err)
	}
	scanPath, err = filepath.Abs(scanPath)
	if err != nil {
		return hooks.Context{}, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	resolved, err := resolve.ByIDOrBranch(target, scanPath)
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

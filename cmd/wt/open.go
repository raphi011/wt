package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/resolve"
)

func runOpen(cmd *OpenCmd, cfg *config.Config) error {
	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	// Check if we're inside a git repo
	if git.IsInsideRepo() {
		// Inside repo: use branch name directly (original behavior)
		return runOpenInRepo(cmd, cfg)
	}

	// Outside repo: resolve by ID or branch name
	return runOpenOutsideRepo(cmd, cfg)
}

// runOpenInRepo handles wt open when inside a git repository
func runOpenInRepo(cmd *OpenCmd, cfg *config.Config) error {
	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fmt.Printf("Opening worktree for branch %s in %s\n", cmd.Branch, absPath)

	result, err := git.OpenWorktree(dir, cmd.Branch, cfg.WorktreeFormat)
	if err != nil {
		return err
	}

	if result.AlreadyExists {
		fmt.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		fmt.Printf("✓ Worktree created at: %s\n", result.Path)
	}

	// Set note if provided
	if cmd.Note != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		if err := git.SetBranchNote(cwd, cmd.Branch, cmd.Note); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set note: %v\n", err)
		}
	}

	// Run post-create hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandOpen)
	if err != nil {
		return err
	}

	ctx := hooks.Context{
		Path:    result.Path,
		Branch:  cmd.Branch,
		Trigger: string(hooks.CommandOpen),
	}
	ctx.Repo, _ = git.GetRepoName()
	ctx.Folder, _ = git.GetRepoFolderName()
	ctx.MainRepo, _ = git.GetMainRepoPath(result.Path)
	if ctx.MainRepo == "" {
		ctx.MainRepo, _ = filepath.Abs(".")
	}

	return hooks.RunAll(hookMatches, ctx)
}

// runOpenOutsideRepo handles wt open when outside a git repository
// Resolves the argument as either a worktree ID or branch name
func runOpenOutsideRepo(cmd *OpenCmd, cfg *config.Config) error {
	scanDir := cmd.Dir
	if scanDir == "" {
		return fmt.Errorf("directory required when outside git repo (-d flag or WT_DEFAULT_PATH)")
	}

	var err error
	scanDir, err = filepath.Abs(scanDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve target by ID or branch name
	target, err := resolve.ByIDOrBranch(cmd.Branch, scanDir)
	if err != nil {
		return err
	}

	fmt.Printf("Opening worktree for branch %s in %s\n", target.Branch, scanDir)

	// Use OpenWorktreeFrom since we have the main repo path
	result, err := git.OpenWorktreeFrom(target.MainRepo, scanDir, target.Branch, cfg.WorktreeFormat)
	if err != nil {
		return err
	}

	if result.AlreadyExists {
		fmt.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		fmt.Printf("✓ Worktree created at: %s\n", result.Path)
	}

	// Set note if provided
	if cmd.Note != "" {
		if err := git.SetBranchNote(target.MainRepo, target.Branch, cmd.Note); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set note: %v\n", err)
		}
	}

	// Run post-create hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandOpen)
	if err != nil {
		return err
	}

	ctx := hooks.Context{
		Path:     result.Path,
		Branch:   target.Branch,
		MainRepo: target.MainRepo,
		Folder:   filepath.Base(target.MainRepo),
		Trigger:  string(hooks.CommandOpen),
	}
	ctx.Repo, _ = git.GetRepoNameFrom(target.MainRepo)

	return hooks.RunAll(hookMatches, ctx)
}

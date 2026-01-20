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

func runAdd(cmd *AddCmd, cfg *config.Config) error {
	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	// Check if we're inside a git repo
	if git.IsInsideRepo() {
		if cmd.ID != 0 {
			return fmt.Errorf("--id cannot be used inside a git repo; use branch name instead")
		}
		if cmd.Branch == "" {
			return fmt.Errorf("branch name required inside git repo")
		}
		return runAddInRepo(cmd, cfg)
	}

	// Outside repo: resolve by ID
	return runAddOutsideRepo(cmd, cfg)
}

// runAddInRepo handles wt add when inside a git repository
func runAddInRepo(cmd *AddCmd, cfg *config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	var result *git.CreateWorktreeResult

	if cmd.NewBranch {
		fmt.Printf("Creating worktree for new branch %s in %s\n", cmd.Branch, absPath)
		result, err = git.AddWorktree(dir, cmd.Branch, cfg.WorktreeFormat, true)
	} else {
		fmt.Printf("Adding worktree for branch %s in %s\n", cmd.Branch, absPath)
		result, err = git.AddWorktree(dir, cmd.Branch, cfg.WorktreeFormat, false)
	}

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

	// Run post-add hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandAdd)
	if err != nil {
		return err
	}

	ctx := hooks.Context{
		Path:    result.Path,
		Branch:  cmd.Branch,
		Trigger: string(hooks.CommandAdd),
	}
	ctx.Repo, _ = git.GetRepoName()
	ctx.Folder, _ = git.GetRepoFolderName()
	ctx.MainRepo, _ = git.GetMainRepoPath(result.Path)
	if ctx.MainRepo == "" {
		ctx.MainRepo, _ = filepath.Abs(".")
	}

	return hooks.RunAll(hookMatches, ctx)
}

// runAddOutsideRepo handles wt add when outside a git repository
// Resolves the worktree by ID
func runAddOutsideRepo(cmd *AddCmd, cfg *config.Config) error {
	if cmd.NewBranch {
		return fmt.Errorf("cannot create new branch (-b) outside git repo")
	}

	if cmd.ID == 0 {
		return fmt.Errorf("--id required when outside git repo (run 'wt list' to see IDs)")
	}

	scanDir := cmd.Dir
	if scanDir == "" {
		return fmt.Errorf("directory required when outside git repo (-d flag or WT_DEFAULT_PATH)")
	}

	var err error
	scanDir, err = filepath.Abs(scanDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve target by ID
	target, err := resolve.ByID(cmd.ID, scanDir)
	if err != nil {
		return err
	}

	fmt.Printf("Adding worktree for branch %s in %s\n", target.Branch, scanDir)

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

	// Run post-add hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandAdd)
	if err != nil {
		return err
	}

	ctx := hooks.Context{
		Path:     result.Path,
		Branch:   target.Branch,
		MainRepo: target.MainRepo,
		Folder:   filepath.Base(target.MainRepo),
		Trigger:  string(hooks.CommandAdd),
	}
	ctx.Repo, _ = git.GetRepoNameFrom(target.MainRepo)

	return hooks.RunAll(hookMatches, ctx)
}

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

func runOpen(cmd *OpenCmd, cfg config.Config) error {
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
func runOpenInRepo(cmd *OpenCmd, cfg config.Config) error {
	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	basePath, err := expandPath(dir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fmt.Printf("Opening worktree for branch %s in %s\n", cmd.Branch, absPath)

	result, err := git.OpenWorktree(basePath, cmd.Branch, cfg.WorktreeFormat)
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

	// Run post-create hooks (always run for open, ignore run_on_exists config)
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, false, hooks.CommandOpen)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		// Get context for placeholder substitution
		repoName, err := git.GetRepoName()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get repo name for hook context: %v\n", err)
		}
		folderName, err := git.GetRepoFolderName()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get folder name for hook context: %v\n", err)
		}
		mainRepo, mainRepoErr := git.GetMainRepoPath(result.Path)
		if mainRepoErr != nil || mainRepo == "" {
			// Fallback to current directory (should be the main repo when opening worktrees)
			if mainRepoErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get main repo path: %v (using current directory)\n", mainRepoErr)
			}
			mainRepo, err = filepath.Abs(".")
			if err != nil {
				return fmt.Errorf("failed to determine main repo path: %w", err)
			}
		}

		ctx := hooks.Context{
			Path:     result.Path,
			Branch:   cmd.Branch,
			Repo:     repoName,
			Folder:   folderName,
			MainRepo: mainRepo,
			Trigger:  string(hooks.CommandOpen),
		}

		for _, match := range hookMatches {
			fmt.Printf("Running hook '%s'...\n", match.Name)
			if err := hooks.Run(match.Hook, ctx); err != nil {
				return fmt.Errorf("hook %q failed: %w", match.Name, err)
			}
			if match.Hook.Description != "" {
				fmt.Printf("  ✓ %s\n", match.Hook.Description)
			}
		}
	}

	return nil
}

// runOpenOutsideRepo handles wt open when outside a git repository
// Resolves the argument as either a worktree ID or branch name
func runOpenOutsideRepo(cmd *OpenCmd, cfg config.Config) error {
	scanDir := cmd.Dir
	if scanDir == "" {
		return fmt.Errorf("directory required when outside git repo (-d flag or WT_DEFAULT_PATH)")
	}

	scanDir, err := expandPath(scanDir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
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
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, false, hooks.CommandOpen)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		repoName, _ := git.GetRepoNameFrom(target.MainRepo)
		folderName := filepath.Base(target.MainRepo)

		ctx := hooks.Context{
			Path:     result.Path,
			Branch:   target.Branch,
			Repo:     repoName,
			Folder:   folderName,
			MainRepo: target.MainRepo,
			Trigger:  string(hooks.CommandOpen),
		}

		for _, match := range hookMatches {
			fmt.Printf("Running hook '%s'...\n", match.Name)
			if err := hooks.Run(match.Hook, ctx); err != nil {
				return fmt.Errorf("hook %q failed: %w", match.Name, err)
			}
			if match.Hook.Description != "" {
				fmt.Printf("  ✓ %s\n", match.Hook.Description)
			}
		}
	}

	return nil
}

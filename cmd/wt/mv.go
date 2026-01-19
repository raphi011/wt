package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
)

func runMv(cmd *MvCmd, _ *config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Validate destination directory is provided
	if cmd.Dir == "" {
		return fmt.Errorf("destination directory required: use -d flag, set WT_DEFAULT_PATH, or configure default_path in config")
	}

	// Validate worktree format
	if err := format.ValidateFormat(cmd.Format); err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Validate destination path
	destPath, err := filepath.Abs(cmd.Dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if destination directory exists
	if info, err := os.Stat(destPath); os.IsNotExist(err) {
		return fmt.Errorf("destination directory does not exist: %s", destPath)
	} else if err != nil {
		return fmt.Errorf("failed to check destination directory: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("destination is not a directory: %s", destPath)
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Scan for worktrees in current directory
	worktrees, err := git.ListWorktrees(cwd)
	if err != nil {
		return err
	}

	if len(worktrees) == 0 {
		fmt.Println("No worktrees found in current directory")
		return nil
	}

	// Track results
	var moved, skipped, failed int

	for _, wt := range worktrees {
		// Check if dirty and force not set
		if wt.IsDirty && !cmd.Force {
			fmt.Printf("⚠ Skipping %s: dirty working directory (use -f to force)\n", filepath.Base(wt.Path))
			skipped++
			continue
		}

		// Get repo info for formatting
		gitOrigin := wt.RepoName
		folderName := filepath.Base(wt.MainRepo)

		// Format new worktree name
		newName := format.FormatWorktreeName(cmd.Format, format.FormatParams{
			GitOrigin:  gitOrigin,
			BranchName: wt.Branch,
			FolderName: folderName,
		})

		newPath := filepath.Join(destPath, newName)

		// Check if already at destination with same name
		if wt.Path == newPath {
			fmt.Printf("→ Skipping %s: already at destination\n", filepath.Base(wt.Path))
			skipped++
			continue
		}

		// Check if target path already exists
		if _, err := os.Stat(newPath); err == nil {
			fmt.Printf("⚠ Skipping %s: target path already exists: %s\n", filepath.Base(wt.Path), newPath)
			skipped++
			continue
		}

		if cmd.DryRun {
			fmt.Printf("Would move: %s → %s\n", wt.Path, newPath)
			moved++
			continue
		}

		// Move the worktree
		if err := git.MoveWorktree(wt, newPath, cmd.Force); err != nil {
			fmt.Printf("✗ Failed to move %s: %v\n", filepath.Base(wt.Path), err)
			failed++
			continue
		}

		fmt.Printf("✓ Moved: %s → %s\n", wt.Path, newPath)
		moved++
	}

	// Print summary
	fmt.Println()
	if cmd.DryRun {
		fmt.Printf("Dry run: %d would be moved, %d skipped\n", moved, skipped)
	} else {
		fmt.Printf("Moved: %d, Skipped: %d, Failed: %d\n", moved, skipped, failed)
	}

	return nil
}

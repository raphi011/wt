package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
)

func runMv(cmd *MvCmd, cfg *config.Config, workDir string) error {
	// Validate worktree format
	if err := format.ValidateFormat(cmd.Format); err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Get destination from config
	dest := cfg.WorktreeDir
	if dest == "" {
		return fmt.Errorf("destination not configured: set WT_WORKTREE_DIR env var or worktree_dir in config")
	}

	// Validate destination path - resolve relative paths against workDir
	var destPath string
	if filepath.IsAbs(dest) {
		destPath = dest
	} else {
		destPath = filepath.Join(workDir, dest)
	}
	destPath = filepath.Clean(destPath)

	// Check if destination directory exists
	if info, err := os.Stat(destPath); os.IsNotExist(err) {
		return fmt.Errorf("destination directory does not exist: %s", destPath)
	} else if err != nil {
		return fmt.Errorf("failed to check destination directory: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("destination is not a directory: %s", destPath)
	}

	// Scan for worktrees in working directory (need dirty check to skip dirty worktrees)
	worktrees, err := git.ListWorktrees(workDir, true)
	if err != nil {
		return err
	}

	// Filter by repository if specified
	if len(cmd.Repository) > 0 {
		repoSet := make(map[string]bool)
		for _, r := range cmd.Repository {
			repoSet[r] = true
		}
		var filtered []git.Worktree
		for _, wt := range worktrees {
			repoName := filepath.Base(wt.MainRepo)
			if repoSet[repoName] {
				filtered = append(filtered, wt)
			}
		}
		worktrees = filtered
	}

	if len(worktrees) == 0 {
		fmt.Println("No worktrees found in current directory")
		// Continue to process repos even if no worktrees found
	}

	// Determine repo destination: repo_dir if set, otherwise worktree_dir
	repoDestPath := destPath // default to worktree_dir
	if cfg.RepoDir != "" {
		if filepath.IsAbs(cfg.RepoDir) {
			repoDestPath = cfg.RepoDir
		} else {
			repoDestPath = filepath.Join(workDir, cfg.RepoDir)
		}
		repoDestPath = filepath.Clean(repoDestPath)

		// Validate repo_dir exists
		if info, err := os.Stat(repoDestPath); os.IsNotExist(err) {
			return fmt.Errorf("repo_dir does not exist: %s", repoDestPath)
		} else if err != nil {
			return fmt.Errorf("failed to check repo_dir: %w", err)
		} else if !info.IsDir() {
			return fmt.Errorf("repo_dir is not a directory: %s", repoDestPath)
		}
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

	// Print worktree summary (only if there were worktrees to consider)
	if len(worktrees) > 0 {
		fmt.Println()
		if cmd.DryRun {
			fmt.Printf("Worktrees: %d would be moved, %d skipped\n", moved, skipped)
		} else {
			fmt.Printf("Worktrees: %d moved, %d skipped, %d failed\n", moved, skipped, failed)
		}
	}

	// Find and move main repos in workDir (not worktrees)
	repos, err := git.FindAllRepos(workDir)
	if err != nil {
		return fmt.Errorf("failed to scan for repos: %w", err)
	}

	// Filter by repository if specified
	if len(cmd.Repository) > 0 {
		repoSet := make(map[string]bool)
		for _, r := range cmd.Repository {
			repoSet[r] = true
		}
		var filtered []string
		for _, repoPath := range repos {
			repoName := filepath.Base(repoPath)
			if repoSet[repoName] {
				filtered = append(filtered, repoPath)
			}
		}
		repos = filtered
	}

	if len(repos) == 0 {
		return nil
	}

	fmt.Println()
	var repoMoved, repoSkipped, repoFailed int

	for _, repoPath := range repos {
		repoName := filepath.Base(repoPath)
		newPath := filepath.Join(repoDestPath, repoName)

		// Skip if already at destination
		if repoPath == newPath {
			fmt.Printf("→ Skipping repo %s: already at destination\n", repoName)
			repoSkipped++
			continue
		}

		// Check if target exists
		if _, err := os.Stat(newPath); err == nil {
			fmt.Printf("⚠ Skipping repo %s: target already exists\n", repoName)
			repoSkipped++
			continue
		}

		if cmd.DryRun {
			fmt.Printf("Would move repo: %s → %s\n", repoPath, newPath)
			repoMoved++
			continue
		}

		// Get worktree list before moving (to update references after)
		wtInfos, _ := git.ListWorktreesFromRepo(repoPath)

		// Move the repo
		if err := os.Rename(repoPath, newPath); err != nil {
			fmt.Printf("✗ Failed to move repo %s: %v\n", repoName, err)
			repoFailed++
			continue
		}

		// Update worktree references to point to new repo location
		for _, wti := range wtInfos {
			if wti.Path == repoPath {
				continue // skip main repo entry
			}
			// Check if worktree still exists before updating
			if _, err := os.Stat(wti.Path); os.IsNotExist(err) {
				continue
			}
			if err := git.RepairWorktree(newPath, wti.Path); err != nil {
				fmt.Printf("⚠ Warning: failed to update worktree %s: %v\n", filepath.Base(wti.Path), err)
			}
		}

		fmt.Printf("✓ Moved repo: %s → %s\n", repoPath, newPath)
		repoMoved++
	}

	// Print repo summary
	fmt.Println()
	if cmd.DryRun {
		fmt.Printf("Repos: %d would be moved, %d skipped\n", repoMoved, repoSkipped)
	} else {
		fmt.Printf("Repos: %d moved, %d skipped, %d failed\n", repoMoved, repoSkipped, repoFailed)
	}

	return nil
}

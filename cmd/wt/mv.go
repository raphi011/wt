package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
)

func (c *MvCmd) runMv(ctx context.Context) error {
	l := log.FromContext(ctx)
	out := output.FromContext(ctx)
	cfg := c.Config
	workDir := c.WorkDir
	// Validate worktree format
	if err := format.ValidateFormat(c.Format); err != nil {
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

	// Scan for worktrees in working directory
	worktrees, err := git.ListWorktrees(ctx, workDir, false)
	if err != nil {
		return err
	}

	// Filter by repository if specified
	if len(c.Repository) > 0 {
		repoSet := make(map[string]bool)
		for _, r := range c.Repository {
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
		out.Println("No worktrees found in current directory")
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

	// Find main repos in workDir first (we move repos before worktrees)
	repos, err := git.FindAllRepos(workDir)
	if err != nil {
		return fmt.Errorf("failed to scan for repos: %w", err)
	}

	// Filter repos by repository if specified
	if len(c.Repository) > 0 {
		repoSet := make(map[string]bool)
		for _, r := range c.Repository {
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

	// Find nested worktrees (worktrees inside repo directories)
	// These must be moved OUT before the repo moves, otherwise they move with the repo
	// and git worktree repair can't find them
	var nestedWorktrees []git.Worktree
	for _, repoPath := range repos {
		wtInfos, err := git.ListWorktreesFromRepo(ctx, repoPath)
		if err != nil {
			continue
		}
		for _, wti := range wtInfos {
			// Skip main repo entry
			if wti.Path == repoPath {
				continue
			}
			// Check if worktree is nested inside repo directory
			if !isNestedPath(wti.Path, repoPath) {
				continue
			}
			// Build full Worktree struct for nested worktree
			wtInfo, err := git.GetWorktreeInfo(ctx, wti.Path)
			if err != nil {
				l.Printf("⚠ Warning: failed to get info for nested worktree %s: %v\n", wti.Path, err)
				continue
			}
			nestedWorktrees = append(nestedWorktrees, *wtInfo)
		}
	}

	// Move nested worktrees first (before repos move)
	var nestedMoved, nestedSkipped, nestedFailed int
	if len(nestedWorktrees) > 0 {
		for _, wt := range nestedWorktrees {
			// Get repo info for formatting
			gitOrigin := wt.RepoName
			folderName := filepath.Base(wt.MainRepo)

			// Format new worktree name
			newName := format.FormatWorktreeName(c.Format, format.FormatParams{
				GitOrigin:  gitOrigin,
				BranchName: wt.Branch,
				FolderName: folderName,
			})

			newPath := filepath.Join(destPath, newName)

			// Check if target path already exists
			if _, err := os.Stat(newPath); err == nil {
				l.Printf("⚠ Skipping nested %s: target path already exists: %s\n", filepath.Base(wt.Path), newPath)
				nestedSkipped++
				continue
			}

			if c.DryRun {
				l.Printf("Would move nested: %s → %s\n", wt.Path, newPath)
				nestedMoved++
				continue
			}

			// Move the nested worktree out of the repo
			if err := git.MoveWorktree(ctx, wt, newPath, c.Force); err != nil {
				l.Printf("✗ Failed to move nested %s: %v\n", filepath.Base(wt.Path), err)
				nestedFailed++
				continue
			}

			l.Printf("✓ Moved nested: %s → %s\n", wt.Path, newPath)
			nestedMoved++
		}

		// Print nested worktree summary
		l.Println()
		if c.DryRun {
			l.Printf("Nested worktrees: %d would be moved, %d skipped\n", nestedMoved, nestedSkipped)
		} else {
			l.Printf("Nested worktrees: %d moved, %d skipped, %d failed\n", nestedMoved, nestedSkipped, nestedFailed)
		}
	}

	// Track repo moves: old path -> new path (needed to update worktree MainRepo references)
	repoMoves := make(map[string]string)

	// Move repos (nested worktrees already moved out)
	var repoMoved, repoSkipped, repoFailed int

	if len(repos) > 0 {
		for _, repoPath := range repos {
			repoName := filepath.Base(repoPath)
			newPath := filepath.Join(repoDestPath, repoName)

			// Skip if already at destination
			if repoPath == newPath {
				l.Printf("→ Skipping repo %s: already at destination\n", repoName)
				repoSkipped++
				continue
			}

			// Check if target exists
			if _, err := os.Stat(newPath); err == nil {
				l.Printf("⚠ Skipping repo %s: target already exists\n", repoName)
				repoSkipped++
				continue
			}

			if c.DryRun {
				l.Printf("Would move repo: %s → %s\n", repoPath, newPath)
				repoMoves[repoPath] = newPath
				repoMoved++
				continue
			}

			// Move the repo
			if err := os.Rename(repoPath, newPath); err != nil {
				l.Printf("✗ Failed to move repo %s: %v\n", repoName, err)
				repoFailed++
				continue
			}

			// Repair all worktree references to point to new repo location
			if err := git.RepairWorktreesFromRepo(ctx, newPath); err != nil {
				l.Printf("⚠ Warning: failed to repair worktrees for %s: %v\n", repoName, err)
			}

			repoMoves[repoPath] = newPath
			l.Printf("✓ Moved repo: %s → %s\n", repoPath, newPath)
			repoMoved++
		}

		// Print repo summary
		l.Println()
		if c.DryRun {
			l.Printf("Repos: %d would be moved, %d skipped\n", repoMoved, repoSkipped)
		} else {
			l.Printf("Repos: %d moved, %d skipped, %d failed\n", repoMoved, repoSkipped, repoFailed)
		}
	}

	// Update worktree MainRepo paths to reflect moved repos
	for i := range worktrees {
		if newRepo, ok := repoMoves[worktrees[i].MainRepo]; ok {
			worktrees[i].MainRepo = newRepo
		}
	}

	// Now move worktrees (repos are already at their final locations)
	var moved, skipped, failed int

	for _, wt := range worktrees {
		// Get repo info for formatting
		gitOrigin := wt.RepoName
		folderName := filepath.Base(wt.MainRepo)

		// Format new worktree name
		newName := format.FormatWorktreeName(c.Format, format.FormatParams{
			GitOrigin:  gitOrigin,
			BranchName: wt.Branch,
			FolderName: folderName,
		})

		newPath := filepath.Join(destPath, newName)

		// Check if already at destination with same name
		if wt.Path == newPath {
			l.Printf("→ Skipping %s: already at destination\n", filepath.Base(wt.Path))
			skipped++
			continue
		}

		// Check if target path already exists
		if _, err := os.Stat(newPath); err == nil {
			l.Printf("⚠ Skipping %s: target path already exists: %s\n", filepath.Base(wt.Path), newPath)
			skipped++
			continue
		}

		if c.DryRun {
			l.Printf("Would move: %s → %s\n", wt.Path, newPath)
			moved++
			continue
		}

		// Move the worktree (MainRepo already points to new repo location if repo was moved)
		if err := git.MoveWorktree(ctx, wt, newPath, c.Force); err != nil {
			l.Printf("✗ Failed to move %s: %v\n", filepath.Base(wt.Path), err)
			failed++
			continue
		}

		l.Printf("✓ Moved: %s → %s\n", wt.Path, newPath)
		moved++
	}

	// Print worktree summary (only if there were worktrees to consider)
	if len(worktrees) > 0 {
		l.Println()
		if c.DryRun {
			l.Printf("Worktrees: %d would be moved, %d skipped\n", moved, skipped)
		} else {
			l.Printf("Worktrees: %d moved, %d skipped, %d failed\n", moved, skipped, failed)
		}
	}

	return nil
}

// isNestedPath returns true if childPath is inside parentPath.
func isNestedPath(childPath, parentPath string) bool {
	// Clean paths for consistent comparison
	child := filepath.Clean(childPath)
	parent := filepath.Clean(parentPath)

	// Child must start with parent path + separator
	return len(child) > len(parent) && child[:len(parent)] == parent && child[len(parent)] == filepath.Separator
}

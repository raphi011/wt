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

	// Determine scan directory and mode based on Path argument
	scanDir := workDir
	var singleWorktree *git.Worktree
	var singleRepo string

	if c.Path != "" {
		// Resolve path relative to workDir
		var targetPath string
		if filepath.IsAbs(c.Path) {
			targetPath = c.Path
		} else {
			targetPath = filepath.Join(workDir, c.Path)
		}
		targetPath = filepath.Clean(targetPath)

		// Check if path exists
		info, err := os.Stat(targetPath)
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", targetPath)
		} else if err != nil {
			return fmt.Errorf("failed to check path: %w", err)
		}

		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", targetPath)
		}

		// Check if it's a worktree (has .git file)
		if git.IsWorktree(targetPath) {
			wtInfo, err := git.GetWorktreeInfo(ctx, targetPath)
			if err != nil {
				return fmt.Errorf("failed to get worktree info: %w", err)
			}
			singleWorktree = wtInfo
			l.Printf("Moving single worktree: %s\n", filepath.Base(targetPath))
		} else if git.IsMainRepo(targetPath) {
			// It's a main repo - move the repo and its worktrees
			singleRepo = targetPath
			l.Printf("Moving repo and its worktrees: %s\n", filepath.Base(targetPath))
		} else {
			// It's a regular directory - use as scan directory
			scanDir = targetPath
		}
	}

	// Handle single worktree mode
	if singleWorktree != nil {
		if c.Cascade {
			// Cascade: also move repo and sibling worktrees
			l.Printf("Cascade: including repo and sibling worktrees\n")
			return c.moveSingleRepo(ctx, l, singleWorktree.MainRepo, destPath, repoDestPath)
		}
		return c.moveSingleWorktree(ctx, l, singleWorktree, destPath)
	}

	// Handle single repo mode
	if singleRepo != "" {
		return c.moveSingleRepo(ctx, l, singleRepo, destPath, repoDestPath)
	}

	// Scan for worktrees in scan directory
	worktrees, err := git.ListWorktrees(ctx, scanDir, false)
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
		out.Println("No worktrees found in " + scanDir)
		// Continue to process repos even if no worktrees found
	}

	// Find main repos in scanDir first (we move repos before worktrees)
	repos, err := git.FindAllRepos(scanDir)
	if err != nil {
		return fmt.Errorf("failed to scan for repos: %w", err)
	}

	// Build repo filter set if specified
	var repoFilter map[string]bool
	if len(c.Repository) > 0 {
		repoFilter = make(map[string]bool)
		for _, r := range c.Repository {
			repoFilter[r] = true
		}
	}

	// Filter repos by repository if specified
	if repoFilter != nil {
		var filtered []string
		for _, repoPath := range repos {
			repoName := filepath.Base(repoPath)
			if repoFilter[repoName] {
				filtered = append(filtered, repoPath)
			}
		}
		repos = filtered
	}

	// Track repos we've seen to avoid duplicates when cascade adds more
	seenRepoPaths := make(map[string]bool)
	for _, repoPath := range repos {
		seenRepoPaths[repoPath] = true
	}

	// When cascade is enabled, also add repos that are OUTSIDE scanDir
	// but have worktrees IN scanDir
	if c.Cascade {
		for _, wt := range worktrees {
			// Check if this worktree's repo is already in our list
			if seenRepoPaths[wt.MainRepo] {
				continue
			}
			// Check repo filter
			if repoFilter != nil && !repoFilter[filepath.Base(wt.MainRepo)] {
				continue
			}
			// Add this external repo to be moved
			l.Printf("Cascade: including external repo %s\n", filepath.Base(wt.MainRepo))
			repos = append(repos, wt.MainRepo)
			seenRepoPaths[wt.MainRepo] = true
		}
	}

	// Find nested worktrees (worktrees inside repo directories)
	// These must be moved OUT before the repo moves, otherwise they move with the repo
	// and git worktree repair can't find them
	//
	// When cascade is enabled, also collect EXTERNAL worktrees (outside scan directory)
	// that belong to repos in the scan directory
	var nestedWorktrees []git.Worktree
	var cascadeWorktrees []git.Worktree // external worktrees found via cascade
	seenWorktreePaths := make(map[string]bool)

	// Track worktrees already in scan directory to avoid duplicates
	for _, wt := range worktrees {
		seenWorktreePaths[wt.Path] = true
	}

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
			// Skip if already in worktrees list
			if seenWorktreePaths[wti.Path] {
				continue
			}

			// Check if worktree is nested inside repo directory
			if isNestedPath(wti.Path, repoPath) {
				// Build full Worktree struct for nested worktree
				wtInfo, err := git.GetWorktreeInfo(ctx, wti.Path)
				if err != nil {
					l.Printf("⚠ Warning: failed to get info for nested worktree %s: %v\n", wti.Path, err)
					continue
				}
				nestedWorktrees = append(nestedWorktrees, *wtInfo)
				seenWorktreePaths[wti.Path] = true
			} else if c.Cascade {
				// External worktree (outside repo dir) - add if cascade enabled
				wtInfo, err := git.GetWorktreeInfo(ctx, wti.Path)
				if err != nil {
					l.Printf("⚠ Warning: failed to get info for external worktree %s: %v\n", wti.Path, err)
					continue
				}
				l.Printf("Cascade: including external worktree %s\n", filepath.Base(wti.Path))
				cascadeWorktrees = append(cascadeWorktrees, *wtInfo)
				seenWorktreePaths[wti.Path] = true
			}
		}
	}

	// Add cascade worktrees to the main worktrees list (they'll be moved after repos)
	worktrees = append(worktrees, cascadeWorktrees...)

	// Move nested worktrees first (before repos move)
	var nestedMoved, nestedFailed int
	if len(nestedWorktrees) > 0 {
		for _, wt := range nestedWorktrees {
			// Get repo info for formatting
			repoName := filepath.Base(wt.MainRepo)
			origin := wt.RepoName // RepoName is from git origin URL
			if origin == "" {
				origin = repoName
			}

			// Format new worktree name
			newName := format.FormatWorktreeName(c.Format, format.FormatParams{
				RepoName:   repoName,
				BranchName: wt.Branch,
				Origin:     origin,
			})

			newPath := filepath.Join(destPath, newName)

			// If target exists, find a unique path with numbered suffix
			originalPath := newPath
			newPath = format.UniqueWorktreePath(newPath, format.DefaultPathExists)
			if newPath != originalPath {
				l.Printf("⚠ Collision detected: %s exists, using %s instead\n", filepath.Base(originalPath), filepath.Base(newPath))
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
			l.Printf("Nested worktrees: %d would be moved\n", nestedMoved)
		} else {
			l.Printf("Nested worktrees: %d moved, %d failed\n", nestedMoved, nestedFailed)
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
		repoName := filepath.Base(wt.MainRepo)
		origin := wt.RepoName // RepoName is from git origin URL
		if origin == "" {
			origin = repoName
		}

		// Format new worktree name
		newName := format.FormatWorktreeName(c.Format, format.FormatParams{
			RepoName:   repoName,
			BranchName: wt.Branch,
			Origin:     origin,
		})

		newPath := filepath.Join(destPath, newName)

		// Check if already at destination with same name
		if wt.Path == newPath {
			l.Printf("→ Skipping %s: already at destination\n", filepath.Base(wt.Path))
			skipped++
			continue
		}

		// If target exists, find a unique path with numbered suffix
		originalPath := newPath
		newPath = format.UniqueWorktreePath(newPath, format.DefaultPathExists)
		if newPath != originalPath {
			l.Printf("⚠ Collision detected: %s exists, using %s instead\n", filepath.Base(originalPath), filepath.Base(newPath))
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

// moveSingleWorktree moves a single worktree to the destination.
func (c *MvCmd) moveSingleWorktree(ctx context.Context, l *log.Logger, wt *git.Worktree, destPath string) error {
	repoName := filepath.Base(wt.MainRepo)
	origin := wt.RepoName
	if origin == "" {
		origin = repoName
	}

	newName := format.FormatWorktreeName(c.Format, format.FormatParams{
		RepoName:   repoName,
		BranchName: wt.Branch,
		Origin:     origin,
	})

	newPath := filepath.Join(destPath, newName)

	// Check if already at destination
	if wt.Path == newPath {
		l.Printf("→ Skipping %s: already at destination\n", filepath.Base(wt.Path))
		return nil
	}

	// If target exists, find a unique path with numbered suffix
	originalPath := newPath
	newPath = format.UniqueWorktreePath(newPath, format.DefaultPathExists)
	if newPath != originalPath {
		l.Printf("⚠ Collision detected: %s exists, using %s instead\n", filepath.Base(originalPath), filepath.Base(newPath))
	}

	if c.DryRun {
		l.Printf("Would move: %s → %s\n", wt.Path, newPath)
		return nil
	}

	if err := git.MoveWorktree(ctx, *wt, newPath, c.Force); err != nil {
		return fmt.Errorf("failed to move worktree: %w", err)
	}

	l.Printf("✓ Moved: %s → %s\n", wt.Path, newPath)
	return nil
}

// moveSingleRepo moves a repo and all its worktrees to the destination.
func (c *MvCmd) moveSingleRepo(ctx context.Context, l *log.Logger, repoPath, destPath, repoDestPath string) error {
	repoName := filepath.Base(repoPath)

	// Get all worktrees for this repo
	wtInfos, err := git.ListWorktreesFromRepo(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("failed to list worktrees for repo: %w", err)
	}

	// Separate nested and external worktrees
	var nestedWorktrees, externalWorktrees []git.Worktree
	for _, wti := range wtInfos {
		if wti.Path == repoPath {
			continue // Skip main repo entry
		}
		wtInfo, err := git.GetWorktreeInfo(ctx, wti.Path)
		if err != nil {
			l.Printf("⚠ Warning: failed to get info for worktree %s: %v\n", wti.Path, err)
			continue
		}
		if isNestedPath(wti.Path, repoPath) {
			nestedWorktrees = append(nestedWorktrees, *wtInfo)
		} else {
			externalWorktrees = append(externalWorktrees, *wtInfo)
		}
	}

	// Move nested worktrees first (before repo moves)
	for _, wt := range nestedWorktrees {
		origin := wt.RepoName
		if origin == "" {
			origin = repoName
		}

		newName := format.FormatWorktreeName(c.Format, format.FormatParams{
			RepoName:   repoName,
			BranchName: wt.Branch,
			Origin:     origin,
		})

		newPath := filepath.Join(destPath, newName)
		originalPath := newPath
		newPath = format.UniqueWorktreePath(newPath, format.DefaultPathExists)
		if newPath != originalPath {
			l.Printf("⚠ Collision detected: %s exists, using %s instead\n", filepath.Base(originalPath), filepath.Base(newPath))
		}

		if c.DryRun {
			l.Printf("Would move nested: %s → %s\n", wt.Path, newPath)
			continue
		}

		if err := git.MoveWorktree(ctx, wt, newPath, c.Force); err != nil {
			l.Printf("✗ Failed to move nested %s: %v\n", filepath.Base(wt.Path), err)
			continue
		}
		l.Printf("✓ Moved nested: %s → %s\n", wt.Path, newPath)
	}

	// Move the repo
	newRepoPath := filepath.Join(repoDestPath, repoName)

	if repoPath == newRepoPath {
		l.Printf("→ Skipping repo %s: already at destination\n", repoName)
	} else if _, err := os.Stat(newRepoPath); err == nil {
		l.Printf("⚠ Skipping repo %s: target already exists\n", repoName)
	} else {
		if c.DryRun {
			l.Printf("Would move repo: %s → %s\n", repoPath, newRepoPath)
		} else {
			if err := os.Rename(repoPath, newRepoPath); err != nil {
				return fmt.Errorf("failed to move repo: %w", err)
			}
			if err := git.RepairWorktreesFromRepo(ctx, newRepoPath); err != nil {
				l.Printf("⚠ Warning: failed to repair worktrees: %v\n", err)
			}
			l.Printf("✓ Moved repo: %s → %s\n", repoPath, newRepoPath)
		}
	}

	// Move external worktrees (updating their MainRepo reference)
	for _, wt := range externalWorktrees {
		// Update MainRepo to new location if repo was moved
		if repoPath != newRepoPath {
			wt.MainRepo = newRepoPath
		}

		origin := wt.RepoName
		if origin == "" {
			origin = repoName
		}

		newName := format.FormatWorktreeName(c.Format, format.FormatParams{
			RepoName:   repoName,
			BranchName: wt.Branch,
			Origin:     origin,
		})

		newPath := filepath.Join(destPath, newName)

		if wt.Path == newPath {
			l.Printf("→ Skipping %s: already at destination\n", filepath.Base(wt.Path))
			continue
		}

		originalPath := newPath
		newPath = format.UniqueWorktreePath(newPath, format.DefaultPathExists)
		if newPath != originalPath {
			l.Printf("⚠ Collision detected: %s exists, using %s instead\n", filepath.Base(originalPath), filepath.Base(newPath))
		}

		if c.DryRun {
			l.Printf("Would move: %s → %s\n", wt.Path, newPath)
			continue
		}

		if err := git.MoveWorktree(ctx, wt, newPath, c.Force); err != nil {
			l.Printf("✗ Failed to move %s: %v\n", filepath.Base(wt.Path), err)
			continue
		}
		l.Printf("✓ Moved: %s → %s\n", wt.Path, newPath)
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

package resolve

import (
	"fmt"
	"os"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/git"
)

// Target represents a resolved worktree target
type Target struct {
	ID       int
	Branch   string
	MainRepo string
	Path     string
}

// FromCurrentWorktree resolves target from the current working directory.
// Returns error if not inside a git worktree.
func FromCurrentWorktree() (*Target, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	if !git.IsWorktree(cwd) {
		return nil, fmt.Errorf("not inside a worktree (use --id to specify target)")
	}

	branch, err := git.GetCurrentBranch(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	mainRepo, err := git.GetMainRepoPath(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get main repo path: %w", err)
	}

	return &Target{Branch: branch, MainRepo: mainRepo, Path: cwd}, nil
}

// FromCurrentWorktreeOrRepo resolves target from the current working directory.
// Works in both worktrees and main repos.
func FromCurrentWorktreeOrRepo() (*Target, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	if !git.IsInsideRepo() {
		return nil, fmt.Errorf("not inside a git repository")
	}

	branch, err := git.GetCurrentBranch(cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	var mainRepo string
	if git.IsWorktree(cwd) {
		mainRepo, err = git.GetMainRepoPath(cwd)
		if err != nil {
			return nil, fmt.Errorf("failed to get main repo path: %w", err)
		}
	} else {
		// Inside main repo, use cwd as main repo
		mainRepo = cwd
	}

	return &Target{Branch: branch, MainRepo: mainRepo, Path: cwd}, nil
}

// ByRepoName resolves a target by repository name.
// Returns target with current branch of that repo.
func ByRepoName(repoName, repoScanDir string) (*Target, error) {
	repoPath, err := git.FindRepoByName(repoScanDir, repoName)
	if err != nil {
		return nil, err
	}

	branch, err := git.GetCurrentBranch(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	return &Target{Branch: branch, MainRepo: repoPath, Path: repoPath}, nil
}

// ByID resolves a worktree target by its numeric ID only.
// Returns error if ID not found, worktree was removed, or path no longer exists.
func ByID(id int, scanDir string) (*Target, error) {
	wtCache, err := cache.Load(scanDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load cache: %w", err)
	}

	branch, path, found, removed := wtCache.GetBranchByID(id)
	if !found {
		return nil, fmt.Errorf("worktree ID %d not found (run 'wt list' to see available IDs)", id)
	}
	if removed {
		return nil, fmt.Errorf("worktree ID %d was removed", id)
	}

	// Check if path still exists on disk
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("worktree ID %d path no longer exists: %s", id, path)
	}

	mainRepo, err := git.GetMainRepoPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get main repo for worktree %d: %w", id, err)
	}

	return &Target{ID: id, Branch: branch, MainRepo: mainRepo, Path: path}, nil
}

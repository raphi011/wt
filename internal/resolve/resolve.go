package resolve

import (
	"context"
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
func FromCurrentWorktree(ctx context.Context) (*Target, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	return FromWorktreePath(ctx, cwd)
}

// FromWorktreePath resolves target from the given path.
// Returns error if the path is not inside a git worktree.
func FromWorktreePath(ctx context.Context, path string) (*Target, error) {
	if !git.IsWorktree(path) {
		return nil, fmt.Errorf("not inside a worktree (use --id to specify target)")
	}

	branch, err := git.GetCurrentBranch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	mainRepo, err := git.GetMainRepoPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get main repo path: %w", err)
	}

	return &Target{Branch: branch, MainRepo: mainRepo, Path: path}, nil
}

// FromCurrentWorktreeOrRepo resolves target from the current working directory.
// Works in both worktrees and main repos.
func FromCurrentWorktreeOrRepo(ctx context.Context) (*Target, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	return FromWorktreeOrRepoPath(ctx, cwd)
}

// FromWorktreeOrRepoPath resolves target from the given path.
// Works in both worktrees and main repos.
func FromWorktreeOrRepoPath(ctx context.Context, path string) (*Target, error) {
	if !git.IsInsideRepoPath(ctx, path) {
		return nil, fmt.Errorf("not inside a git repository")
	}

	branch, err := git.GetCurrentBranch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	var mainRepo string
	if git.IsWorktree(path) {
		mainRepo, err = git.GetMainRepoPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to get main repo path: %w", err)
		}
	} else {
		// Inside main repo, use path as main repo
		mainRepo = path
	}

	return &Target{Branch: branch, MainRepo: mainRepo, Path: path}, nil
}

// ByRepoName resolves a target by repository name.
// Returns target with current branch of that repo.
func ByRepoName(ctx context.Context, repoName, repoDir string) (*Target, error) {
	repoPath, err := git.FindRepoByName(repoDir, repoName)
	if err != nil {
		return nil, err
	}

	branch, err := git.GetCurrentBranch(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	return &Target{Branch: branch, MainRepo: repoPath, Path: repoPath}, nil
}

// ByID resolves a worktree target by its numeric ID only.
// Returns error if ID not found, worktree was removed, or path no longer exists.
func ByID(id int, worktreeDir string) (*Target, error) {
	wtCache, err := cache.Load(worktreeDir)
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

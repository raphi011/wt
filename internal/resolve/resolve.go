package resolve

import (
	"context"
	"fmt"
	"os"

	"github.com/raphi011/wt/internal/git"
)

// Target represents a resolved worktree target
type Target struct {
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

// ByRepoOrPath resolves target with 2 modes:
// 1. repository != "": by repository name (uses repoDir, falls back to worktreeDir)
// 2. repository == "": from current path (worktree or main repo)
func ByRepoOrPath(ctx context.Context, repository, worktreeDir, repoDir, workDir string) (*Target, error) {
	if repository != "" {
		scanDir := repoDir
		if scanDir == "" {
			scanDir = worktreeDir
		}
		return ByRepoName(ctx, repository, scanDir)
	}
	return FromWorktreeOrRepoPath(ctx, workDir)
}

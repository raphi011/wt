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

// Package resolve provides unified target resolution for worktree commands.
package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
)

// Target represents a resolved worktree target
type Target struct {
	ID       int
	Branch   string
	MainRepo string
	Path     string
}

// ByIDOrBranch resolves a target from an argument that could be:
// - A worktree ID (integer)
// - A branch name
//
// Resolution order:
// 1. If arg is integer AND matches an ID → use that
// 2. If arg matches exactly one branch → use that
// 3. If arg matches multiple branches → error (ambiguous)
// 4. No match → error
func ByIDOrBranch(arg string, scanDir string) (*Target, error) {
	cache, err := forge.LoadCache(scanDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load cache: %w", err)
	}

	// 1. Try as ID first (if it's a number)
	if id, err := strconv.Atoi(arg); err == nil {
		branch, path, found, removed := cache.GetBranchByID(id)
		if found && !removed {
			// Check if path still exists on disk
			if _, err := os.Stat(path); err == nil {
				mainRepo, err := git.GetMainRepoPath(path)
				if err == nil {
					return &Target{ID: id, Branch: branch, MainRepo: mainRepo, Path: path}, nil
				}
			}
			// Path doesn't exist or can't get main repo - fall through to branch matching
		}
		// ID not found, removed, or path invalid - fall through to branch matching
	}

	// 2. Try as branch name
	var matches []Target
	for key, entry := range cache.Worktrees {
		if entry.RemovedAt != nil {
			continue // skip removed
		}
		// Skip if path no longer exists on disk
		if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
			continue
		}
		// Key format: "originURL::branch"
		parts := strings.SplitN(key, "::", 2)
		if len(parts) != 2 {
			continue
		}
		branch := parts[1]
		if branch == arg {
			mainRepo, err := git.GetMainRepoPath(entry.Path)
			if err != nil {
				continue
			}
			matches = append(matches, Target{
				ID:       entry.ID,
				Branch:   branch,
				MainRepo: mainRepo,
				Path:     entry.Path,
			})
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no worktree found for %q (run 'wt list' to see available worktrees)", arg)
	case 1:
		return &matches[0], nil
	default:
		// Ambiguous - list repos with their IDs so user can pick
		var options []string
		for _, m := range matches {
			options = append(options, fmt.Sprintf("%d (%s)", m.ID, filepath.Base(m.MainRepo)))
		}
		return nil, fmt.Errorf("branch %q exists in multiple repos:\n  %s\nUse worktree ID instead", arg, strings.Join(options, "\n  "))
	}
}

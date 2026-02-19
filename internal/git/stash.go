package git

import (
	"context"
	"fmt"
	"strings"
)

// Stash creates a stash entry with a specific message.
// Includes untracked files (-u) to capture all uncommitted changes.
// Returns the number of stashed files and any error.
func Stash(ctx context.Context, path string) (int, error) {
	if err := runGit(ctx, path, "stash", "push", "-u", "-m", "wt autostash"); err != nil {
		return 0, fmt.Errorf("failed to stash changes: %v", err)
	}
	// Count stashed files
	out, err := outputGit(ctx, path, "stash", "show", "--include-untracked", "--name-only")
	if err != nil {
		return 0, nil // stash succeeded but can't count — not fatal
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			count++
		}
	}
	return count, nil
}

// StashPop applies and removes the most recent stash entry.
// Returns nil if successful.
func StashPop(ctx context.Context, path string) error {
	if err := runGit(ctx, path, "stash", "pop"); err != nil {
		return fmt.Errorf("failed to pop stash: %v", err)
	}
	return nil
}

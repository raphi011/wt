package git

import (
	"context"
	"fmt"
)

// Stash creates a stash entry with a specific message.
// Includes untracked files (-u) to capture all uncommitted changes.
// Returns nil if successful.
func Stash(ctx context.Context, path string) error {
	if err := runGit(ctx, path, "stash", "push", "-u", "-m", "wt autostash"); err != nil {
		return fmt.Errorf("failed to stash changes: %v", err)
	}
	return nil
}

// StashPop applies and removes the most recent stash entry.
// Returns nil if successful.
func StashPop(ctx context.Context, path string) error {
	if err := runGit(ctx, path, "stash", "pop"); err != nil {
		return fmt.Errorf("failed to pop stash: %v", err)
	}
	return nil
}

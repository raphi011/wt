package git

import (
	"context"
	"fmt"
	"strings"
)

// Stash creates a stash entry with a specific message.
// Includes untracked files (-u) to capture all uncommitted changes.
// Returns the number of stashed files. If the stash succeeded but the file
// count cannot be determined, returns 1 as a safe fallback.
// Returns 0 when there is nothing to stash. Returns error only if git stash fails.
func Stash(ctx context.Context, path string) (int, error) {
	// Snapshot stash list before push to detect whether a new entry was created
	before, _ := outputGit(ctx, path, "stash", "list")

	if err := runGit(ctx, path, "stash", "push", "-u", "-m", "wt autostash"); err != nil {
		return 0, fmt.Errorf("failed to stash changes: %v", err)
	}

	after, _ := outputGit(ctx, path, "stash", "list")
	if string(before) == string(after) {
		return 0, nil // nothing was actually stashed
	}

	// Count stashed files
	out, err := outputGit(ctx, path, "stash", "show", "--include-untracked", "--name-only")
	if err != nil {
		return 1, nil // stash was created but can't count — return safe fallback
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

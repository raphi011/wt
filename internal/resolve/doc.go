// Package resolve provides target resolution for worktree commands.
//
// Many wt commands operate on a specific worktree identified by its numeric ID.
// This package handles looking up worktrees from the cache and validating they
// still exist on disk.
//
// # ID-Based Resolution
//
// Use [ByID] to resolve a worktree target:
//
//	target, err := resolve.ByID(5, worktreeDir)
//	if err != nil {
//	    // ID not found, worktree removed, or path doesn't exist
//	}
//	// target.Path, target.Branch, target.MainRepo available
//
// # Error Cases
//
// ByID returns descriptive errors for:
//
//   - ID not found in cache
//   - Worktree was marked as removed
//   - Path no longer exists on disk
//   - Cannot determine main repo path
//
// # Usage Pattern
//
// Commands that use ID resolution follow this pattern:
//
//   - Required ID: wt exec -n 5, wt cd -n 3
//   - Optional ID: wt note (defaults to current worktree), wt pr create
//
// The -i/--id flag is used consistently across commands for worktree targeting.
// Commands may also support -r/--repository and -l/--label for repo-based
// targeting, but those are handled separately in the command implementations.
package resolve

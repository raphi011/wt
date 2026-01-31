// Package cache manages persistent storage for PR information and worktree metadata.
//
// The cache is stored in ~/.wt/ directory and provides:
//
//   - PR info caching: Stores fetched PR metadata (state, author, comments, etc.)
//     to avoid repeated API calls. Cache entries expire after 24 hours.
//
//   - Worktree metadata: Stores path, branch, repo path, and origin URL for each
//     worktree to enable repair and recovery operations.
//
// # Cache Structure
//
// The cache uses folder names as keys (e.g., "repo-feature-branch") since they're
// unique within a worktree directory and human-readable:
//
//	{
//	  "worktrees": {
//	    "repo-feature-branch": {
//	      "path": "/path/to/repo-feature-branch",
//	      "repo_path": "/path/to/repo",
//	      "branch": "feature-branch",
//	      "origin_url": "git@github.com:user/repo.git",
//	      "pr": { ... }
//	    }
//	  }
//	}
//
// # Concurrency
//
// Use [LoadWithLock] for operations that modify the cache to prevent corruption
// from concurrent access. The lock uses a .wt-cache.lock file.
//
// # Entry Lifecycle
//
// Entries are never deleted, only marked as removed via RemovedAt timestamp.
// This enables the doctor command to detect issues.
// Use [Cache.SyncWorktrees] to update the cache with current disk state.
//
// # Related Commands
//
// The "wt doctor" command diagnoses and repairs cache issues including:
// stale entries, path mismatches, and missing metadata.
package cache

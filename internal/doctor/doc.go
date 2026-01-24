// Package doctor provides diagnostic and repair functionality for wt's worktree cache.
//
// The doctor package detects and optionally repairs issues including:
//
//   - Cache integrity issues: orphaned keys, stale entries, path mismatches,
//     missing metadata, and duplicate IDs.
//
//   - Git link issues: broken bidirectional links between worktrees and their
//     parent repositories, repos that have moved, and stale git references.
//
//   - Orphan issues: worktrees on disk not tracked in cache, and ghost entries
//     (cached but git doesn't recognize them).
//
// # Usage
//
// Run diagnostics:
//
//	err := doctor.Run(cfg, false)  // check only
//	err := doctor.Run(cfg, true)   // check and fix
//
// Reset cache:
//
//	err := doctor.Reset(cfg)  // rebuild from scratch
//
// # Issue Categories
//
// Issues are grouped into three categories:
//
//   - [CategoryCache]: Problems with cache data (entries, paths, metadata)
//   - [CategoryGit]: Problems with git worktree links
//   - [CategoryOrphan]: Untracked worktrees or ghost entries
//
// Each [Issue] includes a description and suggested fix action.
package doctor

// Package format handles worktree folder name generation and path sanitization.
//
// Worktree folders are named using a configurable format string with placeholders
// that are substituted at creation time.
//
// # Format Placeholders
//
// Available placeholders for worktree_format config:
//
//   - {repo}: Repository name extracted from git remote origin URL
//   - {branch}: Branch name as provided to the command
//   - {folder}: Actual folder name of the git repo on disk
//
// Default format is "{repo}-{branch}", creating folders like "my-repo-feature-x".
//
// # Path Sanitization
//
// Branch and repo names are sanitized to create valid filesystem paths.
// Characters replaced with "-": / \ : * ? " < > |
//
// This ensures branches like "feature/my-branch" become "feature-my-branch"
// in the folder name.
//
// # Validation
//
// Use [ValidateFormat] to check format strings before use. It ensures:
//   - All placeholders are recognized
//   - At least one placeholder is present (pure static names aren't useful)
package format

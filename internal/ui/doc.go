// Package ui provides terminal UI components for wt command output.
//
// This package uses the Charm libraries (lipgloss, bubbles) for styled
// terminal output including tables and spinners.
//
// # Table Formatting
//
// The primary components are table formatters for list and prune output:
//
//   - [FormatListTable]: Renders worktree list with ID, repo, branch, status,
//     last commit, notes, and PR info columns
//   - [FormatPruneTable]: Simplified table for prune preview showing which
//     worktrees will be removed and why
//
// Tables use lipgloss styling with:
//   - Normal borders in gray (color 240)
//   - Bold headers
//   - Cell padding for readability
//
// # PR Status Display
//
// The list table shows PR status with visual indicators:
//
//   - "-" : No upstream branch (can't have PR)
//   - "?" : Not fetched yet
//   - State + author + comments + reviews + URL for existing PRs
//   - "✓" for approved, "◐" for pending reviews
//
// # Spinner (Experimental)
//
// The [Spinner] type wraps Bubbletea for simple non-interactive progress
// indication. Currently unused but available for long-running operations.
//
// # Design Notes
//
// Output is designed for terminal display with:
//   - Monospace font assumptions
//   - ANSI color support
//   - Truncation for long values (e.g., notes limited to 30 chars)
package ui

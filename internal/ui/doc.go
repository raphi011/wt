// Package ui provides terminal UI components for wt command output.
//
// This package uses the Charm libraries (lipgloss, bubbles) for styled
// terminal output including tables and spinners.
//
// # Table Formatting
//
// Use [RenderTable] to render aligned tables:
//
//	headers := []string{"NAME", "VALUE"}
//	rows := [][]string{{"foo", "bar"}, {"baz", "qux"}}
//	output := ui.RenderTable(headers, rows)
//
// Tables use lipgloss styling with:
//   - Auto-calculated column widths based on content
//   - Bold headers
//   - No borders (clean output)
//   - Right padding for column separation
//
// # Spinner
//
// The [Spinner] type wraps Bubbletea for simple non-interactive progress
// indication during long-running operations.
//
// # Design Notes
//
// Output is designed for terminal display with:
//   - Monospace font assumptions
//   - ANSI color support
package ui

// Package static provides non-interactive terminal output components.
//
// This package contains components for rendering formatted output
// that does not require user interaction, such as tables and
// formatted text displays.
package static

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// RenderTable creates a formatted table with proper column alignment.
// Headers and rows are rendered using lipgloss/table which automatically
// calculates column widths based on content. No borders are rendered.
func RenderTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}

	var output strings.Builder

	t := table.New().
		Headers(headers...).
		Rows(rows...).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(false).
		BorderColumn(false).
		BorderRow(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).PaddingRight(2)
			}
			return lipgloss.NewStyle().PaddingRight(2)
		})

	output.WriteString(t.String())
	output.WriteString("\n")

	return output.String()
}

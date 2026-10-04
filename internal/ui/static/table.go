// Package static provides non-interactive terminal output components.
//
// This package contains components for rendering formatted output
// that does not require user interaction, such as tables and
// formatted text displays.
package static

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/ui/styles"
)

// WorktreeTableHeaders are the column headers for worktree tables used by list and prune.
var WorktreeTableHeaders = []string{"REPO", "BRANCH", "COMMIT", "AGE", "PR", "NOTE"}

// WorktreeTableRow formats a git.Worktree as a table row matching WorktreeTableHeaders.
// staleDays controls stale highlighting: if > 0 and the commit is older than staleDays,
// the AGE cell is rendered with WarningStyle. Set to 0 to disable.
func WorktreeTableRow(wt git.Worktree, staleDays int, hyperlinks bool) []string {
	commit := wt.CommitHash
	if len(commit) > 7 {
		commit = commit[:7]
	}
	pr := styles.FormatPRRef(wt.PRNumber, wt.PRState, wt.PRDraft, wt.PRURL, hyperlinks)

	age := wt.CommitAge
	if staleDays > 0 && !wt.CommitDate.IsZero() &&
		time.Since(wt.CommitDate) > time.Duration(staleDays)*24*time.Hour {
		age = styles.WarningStyle.Render(age)
	}

	return []string{wt.RepoName, wt.Branch, commit, age, pr, wt.Note}
}

// RenderTable creates a formatted table with proper column alignment.
// Headers and rows are rendered using lipgloss/table which automatically
// calculates column widths based on content. No borders are rendered.
func RenderTable(headers []string, rows [][]string) string {
	return RenderTableAtWidth(headers, rows, 0)
}

// RenderTableAtWidth constrains terminal tables. Width zero preserves the full,
// stable table for pipes and files. JSON is rendered separately by callers.
func RenderTableAtWidth(headers []string, rows [][]string, width int) string {
	if len(rows) == 0 {
		return ""
	}

	var widths []int
	if width > 0 {
		headers, rows, widths = fitColumns(headers, rows, width)
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
			style := lipgloss.NewStyle().PaddingRight(2)
			if len(widths) > col {
				style = style.Width(widths[col] + 2)
			}
			if row == table.HeaderRow {
				return style.Bold(true)
			}
			return style
		})

	if width > 0 {
		t.Width(width).Wrap(false)
	}
	output.WriteString(t.String())
	output.WriteString("\n")

	return output.String()
}

// fitColumns gives identity and PR state priority over secondary metadata.
func fitColumns(headers []string, rows [][]string, width int) ([]string, [][]string, []int) {
	indices := make([]int, 0, len(headers))
	for i, header := range headers {
		if width < 60 && (header == "NOTE" || header == "AGE" || header == "COMMIT") {
			continue
		}
		indices = append(indices, i)
	}
	// Keep at least one cell per column plus its separator on tiny terminals.
	if len(indices)*3 > width {
		indices = indices[:max(1, width/3)]
	}
	newHeaders := make([]string, len(indices))
	widths := make([]int, len(indices))
	for col, index := range indices {
		newHeaders[col] = headers[index]
		widths[col] = ansi.StringWidth(headers[index])
		for _, row := range rows {
			if index < len(row) {
				widths[col] = max(widths[col], ansi.StringWidth(row[index]))
			}
		}
		widths[col] = min(widths[col], width)
		if headers[index] == "NOTE" {
			widths[col] = min(widths[col], 24)
		}
	}
	budget := max(len(indices), width-2*len(indices))
	total := 0
	for _, n := range widths {
		total += n
	}
	for total > budget {
		// Shrink secondary columns first, then the widest remaining column.
		candidate := -1
		for _, name := range []string{"NOTE", "AGE", "COMMIT"} {
			for i, header := range newHeaders {
				if header == name && widths[i] > max(4, ansi.StringWidth(header)) {
					candidate = i
					break
				}
			}
			if candidate >= 0 {
				break
			}
		}
		if candidate < 0 {
			// Preserve the complete PR reference/state while identity columns can
			// still shrink to their header widths.
			for i, n := range widths {
				if newHeaders[i] != "PR" && n > max(1, ansi.StringWidth(newHeaders[i])) && (candidate < 0 || n > widths[candidate]) {
					candidate = i
				}
			}
		}
		if candidate < 0 {
			for i, n := range widths {
				if n > 1 && (candidate < 0 || n > widths[candidate]) {
					candidate = i
				}
			}
		}
		if candidate < 0 {
			break
		}
		widths[candidate]--
		total--
	}
	newRows := make([][]string, len(rows))
	for i, row := range rows {
		newRows[i] = make([]string, len(indices))
		for col, index := range indices {
			if index < len(row) {
				newRows[i][col] = ansi.Truncate(strings.ReplaceAll(row[index], "\n", " "), widths[col], "…")
			}
		}
	}
	for col := range newHeaders {
		newHeaders[col] = ansi.Truncate(newHeaders[col], widths[col], "…")
	}
	return newHeaders, newRows, widths
}

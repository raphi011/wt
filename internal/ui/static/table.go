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

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/ui/styles"
)

// WorktreeTableHeaders are the column headers for worktree tables used by list and prune.
var WorktreeTableHeaders = []string{"REPO", "BRANCH", "COMMIT", "AGE", "PR", "NOTE"}

// WorktreeTableRow formats a git.Worktree as a table row matching WorktreeTableHeaders.
// staleDays controls stale highlighting: if > 0 and the commit is older than staleDays,
// the AGE cell is rendered with WarningStyle. Set to 0 to disable.
func WorktreeTableRow(wt git.Worktree, staleDays int) []string {
	commit := wt.CommitHash
	if len(commit) > 7 {
		commit = commit[:7]
	}
	pr := styles.FormatPRRef(wt.PRNumber, wt.PRState, wt.PRDraft, wt.PRURL)

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

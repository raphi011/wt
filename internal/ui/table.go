package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/raphaelgruber/wt/internal/git"
	"github.com/raphaelgruber/wt/internal/github"
)

var (
	// Colors
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	orangeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
)

// FormatWorktreesTable creates a formatted table for worktrees
func FormatWorktreesTable(worktrees []git.Worktree, prMap map[string]*github.PRInfo, toRemove map[string]bool, dryRun bool) string {
	if len(worktrees) == 0 {
		return ""
	}

	var output strings.Builder

	// Track max widths for each column
	maxRepoWidth := len("REPO")
	maxFolderWidth := len("FOLDER")
	maxStatusWidth := len("STATUS")
	maxLastCommitWidth := len("LAST COMMIT")
	maxDiffWidth := len("DIFF")
	maxPRWidth := len("PR")

	// First pass: calculate widths and prepare row data
	type rowData struct {
		repo       string
		folder     string
		status     string
		lastCommit string
		diff       string
		pr         string
	}
	var rowsData []rowData

	for _, wt := range worktrees {
		// Format repo name
		repoName := wt.RepoName

		// Format folder name (no coloring - causes bleed in table)
		folderPlain := "./" + filepath.Base(wt.Path)
		folder := folderPlain

		// Format status (without last commit time), keep plain for width calc
		var status, statusPlain string
		if toRemove[wt.Path] && !dryRun {
			// Actually being pruned (not dry-run)
			status = greenStyle.Render("pruned")
			statusPlain = "pruned"
		} else if wt.IsMerged {
			status = greenStyle.Render("prunable")
			statusPlain = "prunable"
		} else {
			status = fmt.Sprintf("%d commit(s) ahead", wt.CommitCount)
			statusPlain = status
		}

		// Last commit time
		lastCommit := wt.LastCommit

		// Format diff (always show both if there are any changes)
		var diff, diffPlain string
		if wt.Additions > 0 || wt.Deletions > 0 {
			diff = fmt.Sprintf("+%d -%d", wt.Additions, wt.Deletions)
			diffPlain = diff
		}

		// Format PR (show URL)
		var prDisplay, prPlain string
		if pr, ok := prMap[wt.Branch]; ok && pr != nil {
			icon := github.FormatPRIcon(pr.State)
			prDisplay = fmt.Sprintf("%s %s", icon, pr.URL)
			prPlain = fmt.Sprintf("%s %s", icon, pr.URL)
		}

		// Track max widths using plain text (without ANSI codes)
		if len(repoName) > maxRepoWidth {
			maxRepoWidth = len(repoName)
		}
		if len(folderPlain) > maxFolderWidth {
			maxFolderWidth = len(folderPlain)
		}
		if len(statusPlain) > maxStatusWidth {
			maxStatusWidth = len(statusPlain)
		}
		if len(lastCommit) > maxLastCommitWidth {
			maxLastCommitWidth = len(lastCommit)
		}
		if len(diffPlain) > maxDiffWidth {
			maxDiffWidth = len(diffPlain)
		}
		if len(prPlain) > maxPRWidth {
			maxPRWidth = len(prPlain)
		}

		rowsData = append(rowsData, rowData{
			repo:       repoName,
			folder:     folder,
			status:     status,
			lastCommit: lastCommit,
			diff:       diff,
			pr:         prDisplay,
		})
	}

	// Create table columns with calculated widths + spacing
	columns := []table.Column{
		{Title: "REPO", Width: maxRepoWidth + 2},
		{Title: "FOLDER", Width: maxFolderWidth + 2},
		{Title: "STATUS", Width: maxStatusWidth + 2},
		{Title: "LAST COMMIT", Width: maxLastCommitWidth + 2},
		{Title: "DIFF", Width: maxDiffWidth + 2},
		{Title: "PR", Width: maxPRWidth},
	}

	// Build rows
	var rows []table.Row
	for _, rd := range rowsData {
		rows = append(rows, table.Row{rd.repo, rd.folder, rd.status, rd.lastCommit, rd.diff, rd.pr})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(len(rows)),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Padding(0) // Remove all padding
	// No padding on cells - table handles spacing
	s.Cell = lipgloss.NewStyle().Padding(0)
	s.Selected = lipgloss.NewStyle().Padding(0)
	t.SetStyles(s)

	output.WriteString(t.View())
	output.WriteString("\n")

	return output.String()
}

// FormatSummary formats the summary line
func FormatSummary(removed, skipped int, dryRun bool) string {
	var output strings.Builder

	output.WriteString("\n")

	if dryRun {
		output.WriteString(yellowStyle.Render("Dry run complete") + fmt.Sprintf(" - Would remove: %d, Not merged: %d\n", removed, skipped))
	} else {
		output.WriteString(greenStyle.Render("Cleanup complete") + fmt.Sprintf(" - Removed: %d, Skipped: %d\n", removed, skipped))
	}

	return output.String()
}

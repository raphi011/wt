package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
)

// FormatWorktreesTable creates a formatted table for worktrees
func FormatWorktreesTable(worktrees []git.Worktree, pathToID map[string]int, wtCache *cache.Cache, toRemove map[string]bool, dryRun bool) string {
	if len(worktrees) == 0 {
		return ""
	}

	var output strings.Builder

	// Prepare row data
	type rowData struct {
		id         string
		repo       string
		branch     string
		status     string
		lastCommit string
		note       string
		pr         string
	}
	var rowsData []rowData

	for _, wt := range worktrees {
		// Format ID
		id := fmt.Sprintf("%d", pathToID[wt.Path])

		// Format repo name
		repoName := wt.RepoName

		// Branch name
		branch := wt.Branch

		// Format status
		var status string
		if toRemove[wt.Path] && !dryRun {
			status = "pruned"
		} else if wt.IsMerged {
			status = "prunable"
		} else if wt.CommitCount == 0 && !wt.IsDirty {
			status = "clean"
		} else if wt.CommitCount == 0 && wt.IsDirty {
			status = "dirty"
		} else {
			status = fmt.Sprintf("%d ahead", wt.CommitCount)
		}

		// Last commit time
		lastCommit := wt.LastCommit

		// Format note (truncate if too long)
		note := wt.Note
		const maxNoteLen = 30
		if len(note) > maxNoteLen {
			note = note[:maxNoteLen-3] + "..."
		}

		// Format PR column
		var prCol string
		folderName := filepath.Base(wt.Path)
		pr := wtCache.GetPRForBranch(folderName)
		if !wt.HasUpstream {
			// No upstream branch - can't have a PR
			prCol = "-"
		} else if pr == nil || !pr.Fetched {
			// Not fetched yet - show ?
			prCol = "?"
		} else if pr.Number > 0 {
			// PR exists - show details
			f := forge.DetectFromRepo(wt.MainRepo, nil)
			state := f.FormatState(pr.State)
			if pr.IsDraft && pr.State == "OPEN" {
				state = "draft"
			}

			// Format: state author comments reviews url
			parts := []string{state}
			if pr.Author != "" {
				parts = append(parts, "@"+pr.Author)
			}
			if pr.CommentCount > 0 {
				parts = append(parts, fmt.Sprintf("💬%d", pr.CommentCount))
			}
			if pr.IsApproved {
				parts = append(parts, "✓")
			} else if pr.HasReviews {
				parts = append(parts, "◐") // partial/pending reviews
			}
			parts = append(parts, pr.URL)
			prCol = strings.Join(parts, " ")
		} else {
			// Fetched but no PR found
			prCol = "-"
		}

		rowsData = append(rowsData, rowData{
			id:         id,
			repo:       repoName,
			branch:     branch,
			status:     status,
			lastCommit: lastCommit,
			note:       note,
			pr:         prCol,
		})
	}

	// Build header row
	headers := []string{"ID", "REPO", "BRANCH", "STATUS", "LAST COMMIT", "NOTE", "PR"}

	// Build data rows
	var rows [][]string
	for _, rd := range rowsData {
		rows = append(rows, []string{rd.id, rd.repo, rd.branch, rd.status, rd.lastCommit, rd.note, rd.pr})
	}

	// Create table with lipgloss/table
	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	output.WriteString(t.String())
	output.WriteString("\n")

	return output.String()
}

// FormatSummary formats the summary line
func FormatSummary(removed, skipped int, dryRun bool) string {
	var output strings.Builder

	output.WriteString("\n")

	if dryRun {
		fmt.Fprintf(&output, "Dry run complete - Would remove: %d, Skipped: %d\n", removed, skipped)
	} else {
		fmt.Fprintf(&output, "Cleanup complete - Removed: %d, Skipped: %d\n", removed, skipped)
	}

	return output.String()
}

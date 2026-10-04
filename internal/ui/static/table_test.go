package static

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
	"time"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
)

func TestWorktreeTableRow(t *testing.T) {
	t.Parallel()

	wt := git.Worktree{
		RepoName:   "my-repo",
		Branch:     "feature-x",
		CommitHash: "abc1234def5678",
		CommitAge:  "3 hours ago",
		Note:       "wip",
		PRNumber:   99,
		PRState:    forge.PRStateOpen,
		PRURL:      "https://github.com/org/repo/pull/99",
	}

	row := WorktreeTableRow(wt, 0, false)

	// Must have exactly 6 columns matching headers: REPO, BRANCH, COMMIT, AGE, PR, NOTE
	if len(row) != 6 {
		t.Fatalf("expected 6 columns, got %d", len(row))
	}

	if row[0] != "my-repo" {
		t.Errorf("column 0 (REPO) = %q, want %q", row[0], "my-repo")
	}
	if row[1] != "feature-x" {
		t.Errorf("column 1 (BRANCH) = %q, want %q", row[1], "feature-x")
	}
	if row[2] != "abc1234" {
		t.Errorf("column 2 (COMMIT) = %q, want %q", row[2], "abc1234")
	}
	if row[3] != "3 hours ago" {
		t.Errorf("column 3 (AGE) = %q, want %q", row[3], "3 hours ago")
	}
	// PR column is formatted by styles.FormatPRRef, just verify it's non-empty
	if row[4] == "" {
		t.Error("column 4 (PR) should not be empty for PRNumber > 0")
	}
	if row[5] != "wip" {
		t.Errorf("column 5 (NOTE) = %q, want %q", row[5], "wip")
	}
}

func TestWorktreeTableRowStale(t *testing.T) {
	t.Parallel()

	wt := git.Worktree{
		RepoName:   "my-repo",
		Branch:     "old-feature",
		CommitHash: "abc1234def5678",
		CommitAge:  "30 days ago",
		CommitDate: time.Now().Add(-30 * 24 * time.Hour),
	}

	row := WorktreeTableRow(wt, 14, false)

	// AGE cell should contain ANSI escape codes (styled)
	if row[3] == "30 days ago" {
		t.Error("expected stale AGE cell to be styled, got plain text")
	}
	if !strings.Contains(row[3], "30 days ago") {
		t.Errorf("stale AGE cell should contain age text, got %q", row[3])
	}
}

func TestWorktreeTableRowNotStale(t *testing.T) {
	t.Parallel()

	wt := git.Worktree{
		RepoName:   "my-repo",
		Branch:     "feature-x",
		CommitHash: "abc1234def5678",
		CommitAge:  "3 hours ago",
		CommitDate: time.Now().Add(-3 * time.Hour),
	}

	row := WorktreeTableRow(wt, 14, false)

	if row[3] != "3 hours ago" {
		t.Errorf("non-stale AGE cell should be plain text, got %q", row[3])
	}
}

func TestWorktreeTableRowStaleDisabled(t *testing.T) {
	t.Parallel()

	wt := git.Worktree{
		RepoName:   "my-repo",
		Branch:     "old-feature",
		CommitHash: "abc1234def5678",
		CommitAge:  "30 days ago",
		CommitDate: time.Now().Add(-30 * 24 * time.Hour),
	}

	row := WorktreeTableRow(wt, 0, false)

	if row[3] != "30 days ago" {
		t.Errorf("disabled stale: AGE cell should be plain text, got %q", row[3])
	}
}

func TestRenderTableEmptyRows(t *testing.T) {
	t.Parallel()

	result := RenderTable([]string{"COL1", "COL2"}, nil)

	if result != "" {
		t.Errorf("empty rows: expected empty string, got %q", result)
	}
}

func TestRenderTableSingleRow(t *testing.T) {
	t.Parallel()

	headers := []string{"NAME", "STATUS"}
	rows := [][]string{{"alice", "active"}}

	result := RenderTable(headers, rows)

	if result == "" {
		t.Fatal("single row: expected non-empty output")
	}
	if !strings.Contains(result, "NAME") {
		t.Errorf("single row: output should contain header %q, got %q", "NAME", result)
	}
	if !strings.Contains(result, "STATUS") {
		t.Errorf("single row: output should contain header %q, got %q", "STATUS", result)
	}
	if !strings.Contains(result, "alice") {
		t.Errorf("single row: output should contain row value %q, got %q", "alice", result)
	}
	if !strings.Contains(result, "active") {
		t.Errorf("single row: output should contain row value %q, got %q", "active", result)
	}
}

func TestRenderTableMultipleRows(t *testing.T) {
	t.Parallel()

	headers := []string{"REPO", "BRANCH", "COMMIT"}
	rows := [][]string{
		{"repo-a", "main", "abc1234"},
		{"repo-b", "feature-x", "def5678"},
		{"repo-c", "bugfix-y", "ghi9012"},
	}

	result := RenderTable(headers, rows)

	for _, header := range headers {
		if !strings.Contains(result, header) {
			t.Errorf("multiple rows: output should contain header %q", header)
		}
	}
	for _, row := range rows {
		for _, cell := range row {
			if !strings.Contains(result, cell) {
				t.Errorf("multiple rows: output should contain cell value %q", cell)
			}
		}
	}
}

func TestResponsiveTableWidths(t *testing.T) {
	rows := [][]string{{"repository-界é👩‍💻", strings.Repeat("feature-", 20), "abc1234", "3 hours ago", "#123 Merged", strings.Repeat("notes 界é👩‍💻 ", 30)}}
	for _, width := range []int{80, 120, 40, 12} {
		view := RenderTableAtWidth(WorktreeTableHeaders, rows, width)
		for line := range strings.SplitSeq(view, "\n") {
			if n := ansi.StringWidth(line); n > width {
				t.Fatalf("width %d > %d: %q", n, width, line)
			}
		}
		if width >= 40 {
			for _, header := range []string{"REPO", "BRANCH", "PR"} {
				if !strings.Contains(view, header) {
					t.Errorf("priority header %s missing: %s", header, view)
				}
			}
		}
		if width == 40 && strings.Contains(view, "NOTE") {
			t.Fatal("secondary column retained on narrow terminal")
		}
	}
	full := RenderTableAtWidth(WorktreeTableHeaders, rows, 0)
	if full != RenderTable(WorktreeTableHeaders, rows) || !strings.Contains(full, rows[0][5]) {
		t.Fatal("pipe output lost full notes")
	}
	if rows[0][1] != strings.Repeat("feature-", 20) {
		t.Fatal("table mutated source rows")
	}
}

func TestPRStateRemainsReadableInNarrowTable(t *testing.T) {
	for _, tc := range []struct {
		state string
		draft bool
		text  string
	}{
		{forge.PRStateOpen, false, "Open"}, {forge.PRStateOpen, true, "Draft"},
		{forge.PRStateMerged, false, "Merged"}, {forge.PRStateClosed, false, "Closed"},
	} {
		wt := git.Worktree{RepoName: strings.Repeat("repo", 15), Branch: strings.Repeat("branch", 20), PRNumber: 123456, PRState: tc.state, PRDraft: tc.draft, PRURL: "https://example.com/123456", Note: strings.Repeat("long note", 20)}
		for _, links := range []bool{false, true} {
			row := WorktreeTableRow(wt, 0, links)
			for _, width := range []int{40, 80, 120} {
				rendered := RenderTableAtWidth(WorktreeTableHeaders, [][]string{row}, width)
				if !strings.Contains(ansi.Strip(rendered), tc.text) || !strings.Contains(ansi.Strip(rendered), "#123456") {
					t.Fatalf("PR state truncated at %d:\n%s", width, rendered)
				}
				for line := range strings.SplitSeq(rendered, "\n") {
					if ansi.StringWidth(line) > width {
						t.Fatalf("width overflow at %d: %q", width, line)
					}
				}
				if strings.Contains(rendered, "\x1b]8;") != links {
					t.Fatalf("hyperlink capability lost: %q", rendered)
				}
			}
		}
	}
}

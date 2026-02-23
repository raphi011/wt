package static

import (
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

	row := WorktreeTableRow(wt, 0)

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

	row := WorktreeTableRow(wt, 14)

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

	row := WorktreeTableRow(wt, 14)

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

	row := WorktreeTableRow(wt, 0)

	if row[3] != "30 days ago" {
		t.Errorf("disabled stale: AGE cell should be plain text, got %q", row[3])
	}
}

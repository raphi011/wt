package static

import (
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
		CreatedAt:  time.Now(),
		Note:       "wip",
		PRNumber:   99,
		PRState:    forge.PRStateOpen,
		PRURL:      "https://github.com/org/repo/pull/99",
	}

	row := WorktreeTableRow(wt)

	// Must have exactly 6 columns matching headers: REPO, BRANCH, COMMIT, CREATED, PR, NOTE
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
	// CREATED is relative time, just verify non-empty
	if row[3] == "" {
		t.Error("column 3 (CREATED) should not be empty")
	}
	// PR column is formatted by styles.FormatPRRef, just verify it's non-empty
	if row[4] == "" {
		t.Error("column 4 (PR) should not be empty for PRNumber > 0")
	}
	if row[5] != "wip" {
		t.Errorf("column 5 (NOTE) = %q, want %q", row[5], "wip")
	}
}

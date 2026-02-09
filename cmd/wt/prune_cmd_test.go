package main

import (
	"testing"
	"time"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
)

func TestParseBranchTarget(t *testing.T) {
	tests := []struct {
		input      string
		wantRepo   string
		wantBranch string
	}{
		{"feature", "", "feature"},
		{"myrepo:feature", "myrepo", "feature"},
		{"myrepo:feature/foo", "myrepo", "feature/foo"},
		{":feature", "", ":feature"},                // edge case: empty repo treated as no repo
		{"repo:feature:bar", "repo", "feature:bar"}, // only first colon splits
		{"repo:", "repo", ""},                       // empty branch
		{"a:b", "a", "b"},                           // single char repo
	}
	for _, tt := range tests {
		repo, branch := parseBranchTarget(tt.input)
		if repo != tt.wantRepo || branch != tt.wantBranch {
			t.Errorf("parseBranchTarget(%q) = (%q, %q), want (%q, %q)",
				tt.input, repo, branch, tt.wantRepo, tt.wantBranch)
		}
	}
}

func TestPruneTableRow(t *testing.T) {
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

	row := pruneTableRow(wt)

	// Must have exactly 6 columns matching headers: REPO, BRANCH, PR, COMMIT, CREATED, NOTE
	if len(row) != 6 {
		t.Fatalf("expected 6 columns, got %d", len(row))
	}

	if row[0] != "my-repo" {
		t.Errorf("column 0 (REPO) = %q, want %q", row[0], "my-repo")
	}
	if row[1] != "feature-x" {
		t.Errorf("column 1 (BRANCH) = %q, want %q", row[1], "feature-x")
	}
	// PR column is formatted by styles.FormatPRRef, just verify it's non-empty
	if row[2] == "" {
		t.Error("column 2 (PR) should not be empty for PRNumber > 0")
	}
	if row[3] != "abc1234" {
		t.Errorf("column 3 (COMMIT) = %q, want %q", row[3], "abc1234")
	}
	// CREATED is relative time, just verify non-empty
	if row[4] == "" {
		t.Error("column 4 (CREATED) should not be empty")
	}
	if row[5] != "wip" {
		t.Errorf("column 5 (NOTE) = %q, want %q", row[5], "wip")
	}
}

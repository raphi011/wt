package main

import "testing"

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

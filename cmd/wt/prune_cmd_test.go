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

func TestIsStaleWorktree(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name      string
		wt        git.Worktree
		staleDays int
		want      bool
	}{
		{
			name:      "stale worktree",
			wt:        git.Worktree{CommitDate: now.Add(-30 * 24 * time.Hour)},
			staleDays: 14,
			want:      true,
		},
		{
			name:      "fresh worktree",
			wt:        git.Worktree{CommitDate: now.Add(-1 * 24 * time.Hour)},
			staleDays: 14,
			want:      false,
		},
		{
			name:      "disabled staleDays=0",
			wt:        git.Worktree{CommitDate: now.Add(-30 * 24 * time.Hour)},
			staleDays: 0,
			want:      false,
		},
		{
			name:      "zero commit date",
			wt:        git.Worktree{},
			staleDays: 14,
			want:      false,
		},
		{
			name:      "exactly at boundary",
			wt:        git.Worktree{CommitDate: now.Add(-14*24*time.Hour - time.Second)},
			staleDays: 14,
			want:      true,
		},
		{
			name:      "just before boundary",
			wt:        git.Worktree{CommitDate: now.Add(-14*24*time.Hour + time.Hour)},
			staleDays: 14,
			want:      false,
		},
		{
			name:      "negative staleDays",
			wt:        git.Worktree{CommitDate: now.Add(-30 * 24 * time.Hour)},
			staleDays: -1,
			want:      false,
		},
		{
			name:      "open PR protects from stale",
			wt:        git.Worktree{CommitDate: now.Add(-30 * 24 * time.Hour), PRState: forge.PRStateOpen},
			staleDays: 14,
			want:      false,
		},
		{
			name:      "merged PR does not protect from stale check",
			wt:        git.Worktree{CommitDate: now.Add(-30 * 24 * time.Hour), PRState: forge.PRStateMerged},
			staleDays: 14,
			want:      true,
		},
		{
			name:      "no PR is stale by time",
			wt:        git.Worktree{CommitDate: now.Add(-30 * 24 * time.Hour)},
			staleDays: 14,
			want:      true,
		},
		{
			name:      "closed PR does not protect from stale check",
			wt:        git.Worktree{CommitDate: now.Add(-30 * 24 * time.Hour), PRState: forge.PRStateClosed},
			staleDays: 14,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStaleWorktree(tt.wt, tt.staleDays)
			if got != tt.want {
				t.Errorf("isStaleWorktree() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsWorktreePrunable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state string
		want  bool
	}{
		{"merged PR is prunable", forge.PRStateMerged, true},
		{"open PR is not prunable", forge.PRStateOpen, false},
		{"closed PR is not prunable", forge.PRStateClosed, false},
		{"no PR is not prunable", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wt := git.Worktree{PRState: tt.state}
			got := isWorktreePrunable(wt)
			if got != tt.want {
				t.Errorf("isWorktreePrunable(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

package main

import (
	"context"
	"path/filepath"
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

func TestRemovalReasonFor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	old := now.Add(-30 * 24 * time.Hour)

	tests := []struct {
		name      string
		wt        git.Worktree
		staleDays int
		want      removalReason
	}{
		{name: "merged PR", wt: git.Worktree{PRState: forge.PRStateMerged}, want: removalMerged},
		{name: "open PR", wt: git.Worktree{PRState: forge.PRStateOpen}, want: removalNone},
		{name: "closed PR", wt: git.Worktree{PRState: forge.PRStateClosed}, want: removalNone},
		{name: "no PR", wt: git.Worktree{}, want: removalNone},
		{name: "stale", wt: git.Worktree{CommitDate: old}, staleDays: 14, want: removalStale},
		{name: "fresh", wt: git.Worktree{CommitDate: now.Add(-24 * time.Hour)}, staleDays: 14, want: removalNone},
		{name: "stale rule disabled", wt: git.Worktree{CommitDate: old}, staleDays: 0, want: removalNone},
		{name: "negative staleDays", wt: git.Worktree{CommitDate: old}, staleDays: -1, want: removalNone},
		{name: "zero commit date", wt: git.Worktree{}, staleDays: 14, want: removalNone},
		{name: "just past the boundary", wt: git.Worktree{CommitDate: now.Add(-14*24*time.Hour - time.Second)}, staleDays: 14, want: removalStale},
		{name: "just before the boundary", wt: git.Worktree{CommitDate: now.Add(-14*24*time.Hour + time.Hour)}, staleDays: 14, want: removalNone},
		{name: "open PR protects from stale", wt: git.Worktree{CommitDate: old, PRState: forge.PRStateOpen}, staleDays: 14, want: removalNone},
		{name: "closed PR does not protect from stale", wt: git.Worktree{CommitDate: old, PRState: forge.PRStateClosed}, staleDays: 14, want: removalStale},
		{name: "merged wins over stale", wt: git.Worktree{CommitDate: old, PRState: forge.PRStateMerged}, staleDays: 14, want: removalMerged},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := removalReasonFor(tt.wt, tt.staleDays, now); got != tt.want {
				t.Errorf("removalReasonFor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsWorktreeDirty(t *testing.T) {
	t.Parallel()

	notARepo := t.TempDir()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"missing directory is clean", filepath.Join(notARepo, "gone"), false},
		{"unreadable state is dirty", notARepo, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWorktreeDirty(context.Background(), git.Worktree{Path: tt.path})
			if got != tt.want {
				t.Errorf("isWorktreeDirty(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

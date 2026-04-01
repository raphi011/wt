package main

import (
	"testing"
	"time"

	"github.com/raphi011/wt/internal/ui/wizard/flows"
)

func TestSortCdWorktrees(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name     string
		input    []flows.CdWorktreeInfo
		expected []string // expected order of Branch names
	}{
		{
			name:     "empty list",
			input:    nil,
			expected: nil,
		},
		{
			name: "single item",
			input: []flows.CdWorktreeInfo{
				{RepoName: "repo", Branch: "main", Path: "/a"},
			},
			expected: []string{"main"},
		},
		{
			name: "history items sorted by recency",
			input: []flows.CdWorktreeInfo{
				{RepoName: "repo", Branch: "old", LastAccess: now.Add(-2 * time.Hour)},
				{RepoName: "repo", Branch: "new", LastAccess: now.Add(-1 * time.Hour)},
				{RepoName: "repo", Branch: "newest", LastAccess: now},
			},
			expected: []string{"newest", "new", "old"},
		},
		{
			name: "history before no-history",
			input: []flows.CdWorktreeInfo{
				{RepoName: "repo", Branch: "no-history"},
				{RepoName: "repo", Branch: "has-history", LastAccess: now},
			},
			expected: []string{"has-history", "no-history"},
		},
		{
			name: "no-history sorted alphabetically by repo then branch",
			input: []flows.CdWorktreeInfo{
				{RepoName: "bravo", Branch: "feature"},
				{RepoName: "alpha", Branch: "zebra"},
				{RepoName: "alpha", Branch: "apple"},
			},
			expected: []string{"apple", "zebra", "feature"},
		},
		{
			name: "mixed history and no-history",
			input: []flows.CdWorktreeInfo{
				{RepoName: "repo", Branch: "no-hist-b"},
				{RepoName: "repo", Branch: "old-hist", LastAccess: now.Add(-1 * time.Hour)},
				{RepoName: "repo", Branch: "no-hist-a"},
				{RepoName: "repo", Branch: "new-hist", LastAccess: now},
			},
			expected: []string{"new-hist", "old-hist", "no-hist-a", "no-hist-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wts := make([]flows.CdWorktreeInfo, len(tt.input))
			copy(wts, tt.input)
			sortCdWorktrees(wts)

			if len(wts) != len(tt.expected) {
				t.Fatalf("len = %d, want %d", len(wts), len(tt.expected))
			}
			for i, wt := range wts {
				if wt.Branch != tt.expected[i] {
					t.Errorf("[%d] Branch = %q, want %q", i, wt.Branch, tt.expected[i])
				}
			}
		})
	}
}

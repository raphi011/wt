package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolvePath verifies worktree path resolution for all format patterns.
//
// Scenario: User provides various format strings for worktree paths
// Expected: Paths are resolved correctly based on format type (nested, sibling, home-relative, absolute)
func TestResolvePath(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	tests := []struct {
		name     string
		repoPath string
		repoName string
		branch   string
		format   string
		expected string
	}{
		{
			name:     "nested format",
			repoPath: "/home/user/repos/myrepo",
			repoName: "myrepo",
			branch:   "main",
			format:   "{branch}",
			expected: "/home/user/repos/myrepo/main",
		},
		{
			name:     "nested format with ./",
			repoPath: "/home/user/repos/myrepo",
			repoName: "myrepo",
			branch:   "main",
			format:   "./{branch}",
			expected: "/home/user/repos/myrepo/main",
		},
		{
			name:     "sibling format",
			repoPath: "/home/user/repos/myrepo",
			repoName: "myrepo",
			branch:   "main",
			format:   "../{repo}-{branch}",
			expected: "/home/user/repos/myrepo-main",
		},
		{
			name:     "sibling format branch only",
			repoPath: "/home/user/repos/myrepo",
			repoName: "myrepo",
			branch:   "feature",
			format:   "../{branch}",
			expected: "/home/user/repos/feature",
		},
		{
			name:     "home relative format",
			repoPath: "/home/user/repos/myrepo",
			repoName: "myrepo",
			branch:   "main",
			format:   "~/worktrees/{repo}-{branch}",
			expected: filepath.Join(home, "worktrees", "myrepo-main"),
		},
		{
			name:     "absolute format",
			repoPath: "/home/user/repos/myrepo",
			repoName: "myrepo",
			branch:   "main",
			format:   "/var/worktrees/{repo}-{branch}",
			expected: "/var/worktrees/myrepo-main",
		},
		{
			name:     "branch with slash",
			repoPath: "/home/user/repos/myrepo",
			repoName: "myrepo",
			branch:   "feature/foo",
			format:   "{branch}",
			expected: "/home/user/repos/myrepo/feature-foo",
		},
		{
			name:     "sibling with slash branch",
			repoPath: "/home/user/repos/myrepo",
			repoName: "myrepo",
			branch:   "feature/bar",
			format:   "../{repo}-{branch}",
			expected: "/home/user/repos/myrepo-feature-bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ResolvePath(tt.repoPath, tt.repoName, tt.branch, tt.format)
			if got != tt.expected {
				t.Errorf("ResolvePath() = %q, want %q", got, tt.expected)
			}
		})
	}
}

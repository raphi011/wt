package worktree

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolvePath computes the worktree path based on format string.
// Supports:
//   - "{branch}" or "./{branch}" = nested inside repo
//   - "../{repo}-{branch}" = sibling to repo
//   - "~/worktrees/{repo}-{branch}" = centralized folder
//   - "/absolute/{repo}-{branch}" = absolute path
func ResolvePath(repoPath, repoName, branch, format string) string {
	// Sanitize branch name (/ -> -)
	safeBranch := strings.ReplaceAll(branch, "/", "-")

	// Apply placeholders
	path := strings.ReplaceAll(format, "{repo}", repoName)
	path = strings.ReplaceAll(path, "{branch}", safeBranch)

	switch {
	case strings.HasPrefix(path, "../"):
		// Sibling to repo: ../repo-main → parent/repo-main
		return filepath.Join(filepath.Dir(repoPath), path[3:])

	case strings.HasPrefix(path, "~/"):
		// Home-relative absolute path
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			// If we can't get home dir, return the path unchanged
			// This preserves the ~ prefix for error messages
			return path
		}
		return filepath.Join(home, path[2:])

	case strings.HasPrefix(path, "/"):
		// Absolute path
		return path

	default:
		// Relative to repo (with or without ./ prefix)
		path = strings.TrimPrefix(path, "./")
		return filepath.Join(repoPath, path)
	}
}

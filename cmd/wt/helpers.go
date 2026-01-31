package main

import (
	"os"
	"path/filepath"
	"strings"
)

// resolveWorktreePathWithConfig computes the worktree path based on format string.
// Supports:
//   - "{branch}" or "./{branch}" = nested inside repo
//   - "../{repo}-{branch}" = sibling to repo
//   - "~/worktrees/{repo}-{branch}" = centralized folder
//   - "/absolute/{repo}-{branch}" = absolute path
func resolveWorktreePathWithConfig(repoPath, repoName, branch, format string) string {
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
		home, _ := os.UserHomeDir()
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

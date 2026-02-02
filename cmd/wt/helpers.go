package main

import "github.com/raphi011/wt/internal/worktree"

// resolveWorktreePathWithConfig computes the worktree path based on format string.
// Supports:
//   - "{branch}" or "./{branch}" = nested inside repo
//   - "../{repo}-{branch}" = sibling to repo
//   - "~/worktrees/{repo}-{branch}" = centralized folder
//   - "/absolute/{repo}-{branch}" = absolute path
func resolveWorktreePathWithConfig(repoPath, repoName, branch, format string) string {
	return worktree.ResolvePath(repoPath, repoName, branch, format)
}

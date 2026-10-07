package prstatus

import "github.com/raphi011/wt/internal/git"

// Branch returns the source branch used by the forge. Worktree and cache
// identity continue to use the local branch, which may have a different name.
func Branch(wt git.Worktree) string {
	if wt.UpstreamBranch != "" {
		return wt.UpstreamBranch
	}
	return wt.Branch
}

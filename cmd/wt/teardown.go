package main

import (
	"context"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/prcache"
)

// teardownOpts controls removeWorktree.
type teardownOpts struct {
	Force        bool           // pass --force to git worktree remove
	DeleteBranch bool           // delete the local branch after the worktree is removed
	PRCache      *prcache.Cache // the worktree's entry is deleted, the caller saves; nil to skip
}

// removeWorktree removes a worktree, its PR cache entry and, with
// opts.DeleteBranch, its local branch. Returns an error only if the worktree
// itself could not be removed; call forgetWorktrees for the removed ones.
func removeWorktree(ctx context.Context, wt git.Worktree, opts teardownOpts) error {
	l := log.FromContext(ctx)

	if err := git.RemoveWorktree(ctx, wt, opts.Force); err != nil {
		return err
	}

	if opts.PRCache != nil {
		opts.PRCache.Delete(prcache.CacheKey(wt.RepoPath, wt.Branch))
	}

	if opts.DeleteBranch {
		// Force delete if forge confirmed merge (handles squash merges),
		// safe delete (-d) otherwise (including locally-merged branches,
		// where git's own ancestry check in -d provides a safety net).
		forceDelete := wt.PRState == forge.PRStateMerged
		if err := git.DeleteLocalBranch(ctx, wt.RepoPath, wt.Branch, forceDelete); err != nil {
			l.Printf("Warning: failed to delete branch %s: %v\n", wt.Branch, err)
		} else {
			l.Debug("deleted branch", "branch", wt.Branch)
		}
	}
	return nil
}

// forgetWorktrees drops removed worktrees from the history and prunes stale
// worktree references in their repos.
func forgetWorktrees(ctx context.Context, removed []git.Worktree) {
	if len(removed) == 0 {
		return
	}
	l := log.FromContext(ctx)

	histPath, err := config.FromContext(ctx).GetHistoryPath()
	if err != nil {
		l.Printf("Warning: failed to determine history path: %v\n", err)
	} else {
		err := history.Update(histPath, func(h *history.History) error {
			for _, wt := range removed {
				h.RemoveByPath(wt.Path)
			}
			return nil
		})
		if err != nil {
			l.Printf("Warning: failed to save history after removal: %v\n", err)
		}
	}

	// Prune stale references
	processedRepos := make(map[string]bool)
	for _, wt := range removed {
		if !processedRepos[wt.RepoPath] {
			if err := git.PruneWorktrees(ctx, wt.RepoPath); err != nil {
				l.Printf("Warning: failed to prune stale references in %s: %v\n", wt.RepoPath, err)
			}
			processedRepos[wt.RepoPath] = true
		}
	}
}

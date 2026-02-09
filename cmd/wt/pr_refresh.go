package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/ui/progress"
)

// prFetchItem describes a single branch whose PR status needs to be fetched.
type prFetchItem struct {
	originURL string
	repoPath  string
	branch    string
	cacheKey  string // key for prCache (typically filepath.Base(path))
}

// refreshPRStatusForWorktrees fetches PR status for worktrees in parallel.
// If spinner is non-nil it is stopped before the progress bar starts.
// OriginURL is read from each worktree (populated by the loader).
func refreshPRStatusForWorktrees(ctx context.Context, worktrees []git.Worktree, prCache *prcache.Cache, hosts map[string]string, forgeConfig *config.ForgeConfig, sp *progress.Spinner) {
	l := log.FromContext(ctx)

	// Build fetch items, skipping worktrees without origin or already merged
	var items []prFetchItem
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}
		folderName := filepath.Base(wt.Path)
		if pr := prCache.Get(folderName); pr != nil && pr.Fetched && pr.State == forge.PRStateMerged {
			continue
		}
		items = append(items, prFetchItem{
			originURL: wt.OriginURL,
			repoPath:  wt.RepoPath,
			branch:    wt.Branch,
			cacheKey:  folderName,
		})
	}

	if len(items) == 0 {
		return
	}

	// Stop spinner before starting progress bar (if provided)
	if sp != nil {
		sp.Stop()
	}

	_, failed := refreshPRStatuses(ctx, items, prCache, hosts, forgeConfig)
	if failed > 0 {
		l.Printf("Warning: failed to fetch PR status for %d branch(es)\n", failed)
	}
}

// refreshPRStatuses fetches PR status for the given items in parallel using a
// progress bar. It updates prCache in-place and returns the number of
// successfully fetched and failed items.
func refreshPRStatuses(ctx context.Context, items []prFetchItem, prCache *prcache.Cache, hosts map[string]string, forgeConfig *config.ForgeConfig) (fetched, failed int) {
	if len(items) == 0 {
		return 0, 0
	}

	l := log.FromContext(ctx)

	pb := progress.NewProgressBar(len(items), "Fetching PR status...")
	pb.Start()
	defer pb.Stop()

	var prMutex sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, forge.MaxConcurrentFetches)
	var completedCount, failedCount int
	var countMutex sync.Mutex

	for _, item := range items {
		wg.Add(1)
		go func(item prFetchItem) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			f := forge.Detect(item.originURL, hosts, forgeConfig)

			if err := f.Check(ctx); err != nil {
				l.Debug("forge check failed", "origin", item.originURL, "err", err)
				countMutex.Lock()
				completedCount++
				failedCount++
				pb.SetProgress(completedCount, fmt.Sprintf("Fetching PR status... (%d failed)", failedCount))
				countMutex.Unlock()
				return
			}

			// Get upstream branch name (may differ from local branch name)
			upstreamBranch := git.GetUpstreamBranch(ctx, item.repoPath, item.branch)
			if upstreamBranch == "" {
				upstreamBranch = item.branch
			}

			pr, err := f.GetPRForBranch(ctx, item.originURL, upstreamBranch)
			if err != nil {
				l.Debug("PR fetch failed", "branch", item.branch, "err", err)
				countMutex.Lock()
				completedCount++
				failedCount++
				pb.SetProgress(completedCount, fmt.Sprintf("Fetching PR status... (%d failed)", failedCount))
				countMutex.Unlock()
				return
			}

			prMutex.Lock()
			prCache.Set(item.cacheKey, prcache.FromForge(pr))
			prMutex.Unlock()

			countMutex.Lock()
			completedCount++
			if failedCount > 0 {
				pb.SetProgress(completedCount, fmt.Sprintf("Fetching PR status... (%d failed)", failedCount))
			} else {
				pb.SetProgress(completedCount, "Fetching PR status...")
			}
			countMutex.Unlock()
		}(item)
	}

	wg.Wait()

	return completedCount - failedCount, failedCount
}

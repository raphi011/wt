package main

import (
	"context"
	"fmt"
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

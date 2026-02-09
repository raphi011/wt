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

// refreshPRs fetches PR status for the given worktrees in parallel with a
// progress bar. It updates prCache in-place and returns the number of failed fetches.
func refreshPRs(ctx context.Context, worktrees []git.Worktree, prCache *prcache.Cache, hosts map[string]string, forgeConfig *config.ForgeConfig) (failed int) {
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
		return 0
	}

	pb := progress.NewProgressBar(len(items), "Fetching PR status...")
	pb.Start()
	defer pb.Stop()

	var prMutex sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, forge.MaxConcurrentFetches)
	var completedCount, failedCount int
	var countMutex sync.Mutex

	// recordProgress must be called under countMutex.
	recordProgress := func(isFailed bool) {
		completedCount++
		if isFailed {
			failedCount++
		}
		msg := "Fetching PR status..."
		if failedCount > 0 {
			msg = fmt.Sprintf("Fetching PR status... (%d failed)", failedCount)
		}
		pb.SetProgress(completedCount, msg)
	}

	for _, item := range items {
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			f := forge.Detect(item.originURL, hosts, forgeConfig)

			if err := f.Check(ctx); err != nil {
				l.Debug("forge check failed", "origin", item.originURL, "err", err)
				countMutex.Lock()
				recordProgress(true)
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
				recordProgress(true)
				countMutex.Unlock()
				return
			}

			prMutex.Lock()
			prCache.Set(item.cacheKey, prcache.FromForge(pr))
			prMutex.Unlock()

			countMutex.Lock()
			recordProgress(false)
			countMutex.Unlock()
		}()
	}

	wg.Wait()

	return failedCount
}

// populatePRFields fills PR fields on worktrees from the cache.
func populatePRFields(worktrees []git.Worktree, prCache *prcache.Cache) {
	for i := range worktrees {
		folderName := filepath.Base(worktrees[i].Path)
		if pr := prCache.Get(folderName); pr != nil && pr.Fetched {
			worktrees[i].PRNumber = pr.Number
			worktrees[i].PRState = pr.State
			worktrees[i].PRURL = pr.URL
			worktrees[i].PRDraft = pr.IsDraft
		}
	}
}

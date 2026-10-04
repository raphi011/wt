package main

import (
	"context"
	"fmt"
	"slices"
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
	originURL      string
	upstreamBranch string
	branch         string
	cacheKey       string // key for prCache (repoName:folderName)
}

// refreshPRs fetches PR status for the given worktrees in parallel with a
// progress bar. It updates prCache in-place and returns the branches that failed.
func refreshPRs(ctx context.Context, worktrees []git.Worktree, prCache *prcache.Cache, hosts map[string]string, forgeConfig *config.ForgeConfig) (failedBranches []string) {
	l := log.FromContext(ctx)

	// Build fetch items, skipping worktrees without origin or already merged
	var items []prFetchItem
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}
		if !wt.HasUpstream {
			continue
		}
		cacheKey := prcache.CacheKey(wt.RepoPath, wt.Branch)
		if pr := prCache.Get(cacheKey); pr != nil && pr.Fetched && pr.State == forge.PRStateMerged {
			continue
		}
		items = append(items, prFetchItem{
			originURL:      wt.OriginURL,
			upstreamBranch: wt.UpstreamBranch,
			branch:         wt.Branch,
			cacheKey:       cacheKey,
		})
	}

	if len(items) == 0 {
		return nil
	}

	pb := progress.NewProgressBar(len(items), "Fetching PR status...", progress.WithContext(ctx))
	pb.Start()
	defer pb.Stop()

	session := forge.NewSession(hosts, forgeConfig)
	var prMutex sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, forge.MaxConcurrentFetches)
	var completedCount, failedCount int
	var countMutex sync.Mutex

	// recordProgress must be called under countMutex.
	recordProgress := func(branch string) {
		completedCount++
		if branch != "" {
			failedCount++
			failedBranches = append(failedBranches, branch)
		}
		msg := "Fetching PR status..."
		if failedCount > 0 {
			msg = fmt.Sprintf("Fetching PR status... (%d failed)", failedCount)
		}
		pb.SetProgress(completedCount, msg)
	}

schedule:
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		select {
		case semaphore <- struct{}{}:
		case <-ctx.Done():
			break schedule
		}
		if ctx.Err() != nil {
			<-semaphore
			break
		}
		wg.Go(func() {
			defer func() { <-semaphore }()

			f, err := session.Resolve(ctx, item.originURL)

			if err != nil {
				l.Debug("forge check failed", "origin", item.originURL, "err", err)
				countMutex.Lock()
				recordProgress(item.branch)
				countMutex.Unlock()
				return
			}

			// Get upstream branch name (may differ from local branch name)
			upstreamBranch := item.upstreamBranch
			if upstreamBranch == "" {
				upstreamBranch = item.branch
			}

			pr, err := f.GetPRForBranch(ctx, item.originURL, upstreamBranch)
			if err != nil {
				l.Debug("PR fetch failed", "branch", item.branch, "err", err)
				countMutex.Lock()
				recordProgress(item.branch)
				countMutex.Unlock()
				return
			}

			prMutex.Lock()
			prCache.Set(item.cacheKey, pr)
			prMutex.Unlock()

			countMutex.Lock()
			recordProgress("")
			countMutex.Unlock()
		})
	}

	wg.Wait()

	slices.Sort(failedBranches)
	return failedBranches
}

// populatePRFields fills PR fields on worktrees from the cache.
func populatePRFields(worktrees []git.Worktree, prCache *prcache.Cache) {
	for i := range worktrees {
		cacheKey := prcache.CacheKey(worktrees[i].RepoPath, worktrees[i].Branch)
		if pr := prCache.Get(cacheKey); pr != nil && pr.Fetched {
			worktrees[i].PRNumber = pr.Number
			worktrees[i].PRState = pr.State
			worktrees[i].PRURL = pr.URL
			worktrees[i].PRDraft = pr.IsDraft
		}
	}
}

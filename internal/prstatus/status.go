// Package prstatus joins forge status with worktrees and owns its persistence.
package prstatus

import (
	"context"
	"slices"
	"sync"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/fs"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/prcache"
)

// Options controls a status query. Progress runs serially, initially with zero
// completed, then after each fetch. It is omitted when nothing needs fetching.
type Options struct {
	Refresh  bool
	Reset    bool
	Progress func(completed, total, failed int)
}

// Result keeps display status independent of subsequent cache mutations.
// FailedBranches and SaveError are nonfatal diagnostics; cached status remains
// available when a fetch or save fails.
type Result struct {
	FailedBranches []string
	SaveError      error
	statuses       map[string]forge.PRInfo
	mu             sync.Mutex
	cache          *prcache.Cache
}

// For returns a value copy of fetched status, or a zero value on a cache miss.
// The snapshot remains valid after Forget, for removal result tables.
func (r *Result) For(wt git.Worktree) forge.PRInfo {
	if r == nil {
		return forge.PRInfo{}
	}
	return r.statuses[cacheKey(wt)]
}

func cacheKey(wt git.Worktree) string {
	return prcache.CacheKey(fs.ResolvePath(wt.RepoPath), wt.Branch)
}

// Record persists status fetched directly by a PR command. It does not alter
// the display snapshot produced by Load.
func (r *Result) Record(wt git.Worktree, info *forge.PRInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if info == nil {
		r.cache.Set(cacheKey(wt), nil)
	} else {
		copy := *info
		r.cache.Set(cacheKey(wt), &copy)
	}
	return r.cache.Save()
}

// Forget persists removal of a worktree's cache entry.
func (r *Result) Forget(wt git.Worktree) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache.Delete(cacheKey(wt))
	return r.cache.Save()
}

// Load answers status for worktrees, optionally refreshing it. Cache changes
// are saved before returning, including reset and partial or cancelled refresh.
// The returned error is reserved for configuration/path failures.
func Load(ctx context.Context, worktrees []git.Worktree, cfg *config.Config, opts Options) (*Result, error) {
	path, err := cfg.GetPRCachePath()
	if err != nil {
		return nil, err
	}
	r := &Result{cache: prcache.LoadFrom(path), statuses: make(map[string]forge.PRInfo, len(worktrees))}
	if opts.Reset {
		r.cache.Reset()
	}
	if opts.Refresh {
		r.refresh(ctx, worktrees, cfg, opts.Progress)
	}
	r.SaveError = r.cache.Save()
	for _, wt := range worktrees {
		if info := r.cache.Get(cacheKey(wt)); info != nil && info.Fetched {
			r.statuses[cacheKey(wt)] = *info
		}
	}
	return r, nil
}

func (r *Result) refresh(ctx context.Context, worktrees []git.Worktree, cfg *config.Config, progress func(int, int, int)) {
	l := log.FromContext(ctx)
	var items []git.Worktree
	for _, wt := range worktrees {
		if wt.OriginURL == "" || !wt.HasUpstream {
			continue
		}
		if pr := r.cache.Get(cacheKey(wt)); pr != nil && pr.Fetched && pr.State == forge.PRStateMerged {
			continue
		}
		items = append(items, wt)
	}
	if len(items) == 0 {
		return
	}
	if progress != nil {
		progress(0, len(items), 0)
	}
	session := forge.NewSession(forge.ResolverFromContext(ctx), cfg.Hosts, &cfg.Forge)
	semaphore := make(chan struct{}, forge.MaxConcurrentFetches)
	var wg sync.WaitGroup
	completed, failed := 0, 0

schedule:
	for _, wt := range items {
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
			f, err := session.Resolve(ctx, wt.OriginURL)
			var info *forge.PRInfo
			if err != nil {
				l.Debug("forge check failed", "origin", wt.OriginURL, "err", err)
			} else {
				info, err = f.GetPRForBranch(ctx, wt.OriginURL, Branch(wt))
				if err != nil {
					l.Debug("PR fetch failed", "branch", wt.Branch, "err", err)
				}
			}
			r.mu.Lock()
			defer r.mu.Unlock()
			if err != nil {
				failed++
				r.FailedBranches = append(r.FailedBranches, wt.Branch)
			} else {
				r.cache.Set(cacheKey(wt), info)
			}
			completed++
			if progress != nil {
				progress(completed, len(items), failed)
			}
		})
	}
	wg.Wait()
	slices.Sort(r.FailedBranches)
}

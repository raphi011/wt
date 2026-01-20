package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/resolve"
	"github.com/raphi011/wt/internal/ui"
)

// maxConcurrentPRFetches limits parallel gh API calls to avoid rate limiting
const maxConcurrentPRFetches = 5

func runPrune(cmd *PruneCmd, cfg *config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	scanPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// If IDs are specified, handle targeted worktree removal
	if len(cmd.ID) > 0 {
		if cmd.ResetCache {
			return fmt.Errorf("--reset-cache cannot be used with --id")
		}
		return runPruneTargets(cmd, cfg, scanPath)
	}

	if cmd.DryRun {
		fmt.Printf("Pruning worktrees in %s (dry run)\n", scanPath)
	} else {
		fmt.Printf("Pruning worktrees in %s\n", scanPath)
	}

	// Start spinner
	sp := ui.NewSpinner("Scanning worktrees...")
	sp.Start()

	// List all worktrees (include dirty check for prune decisions)
	allWorktrees, err := git.ListWorktrees(scanPath, true)
	if err != nil {
		sp.Stop()
		return err
	}

	if len(allWorktrees) == 0 {
		sp.Stop()
		fmt.Println("No worktrees found")
		return nil
	}

	// If in a git repo and not using --global, filter to only prune worktrees from that repo
	worktrees := allWorktrees
	var currentRepo string
	if !cmd.Global {
		currentRepo = git.GetCurrentRepoMainPath()
		if currentRepo != "" {
			var filtered []git.Worktree
			for _, wt := range allWorktrees {
				if wt.MainRepo == currentRepo {
					filtered = append(filtered, wt)
				}
			}
			worktrees = filtered
		}
	}

	if len(worktrees) == 0 {
		sp.Stop()
		fmt.Printf("No worktrees found for current repository\n")
		fmt.Printf("Use --global to prune all %d worktrees\n", len(allWorktrees))
		return nil
	}

	// Acquire lock on cache
	lock := cache.NewFileLock(forge.LockPath(scanPath))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Unlock()

	// Load cache
	wtCache, err := forge.LoadCache(scanPath)
	if err != nil {
		return fmt.Errorf("failed to load cache: %w", err)
	}

	// Reset cache if requested (before sync so worktrees get fresh IDs)
	if cmd.ResetCache {
		wtCache.Reset()
		fmt.Println("Cache reset: PR info cleared, IDs will be reassigned from 1")
	}

	// Convert ALL worktrees to WorktreeInfo for cache sync (to preserve IDs)
	wtInfos := make([]forge.WorktreeInfo, len(allWorktrees))
	for i, wt := range allWorktrees {
		wtInfos[i] = forge.WorktreeInfo{
			Path:      wt.Path,
			Branch:    wt.Branch,
			OriginURL: wt.OriginURL,
		}
	}

	// Sync cache to get IDs (sync all, even if we're only pruning a subset)
	pathToID := wtCache.SyncWorktrees(wtInfos)

	// Refresh: fetch remotes and PR status
	if cmd.Refresh {
		// Group by repo for fetching
		grouped := git.GroupWorktreesByRepo(worktrees)

		// Fetch default branch for each repo
		for repoName, wts := range grouped {
			if len(wts) == 0 {
				continue
			}
			defaultBranch := git.GetDefaultBranch(wts[0].MainRepo)
			sp.UpdateMessage(fmt.Sprintf("Fetching origin/%s for %s...", defaultBranch, repoName))
			if err := git.FetchDefaultBranch(wts[0].MainRepo); err != nil {
				// Non-fatal: log warning but continue
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}
		}

		sp.UpdateMessage("Fetching PR status...")
		refreshPRStatus(worktrees, wtCache, cfg, sp)
	}

	// Update merge status for worktrees based on cached PR state
	for i := range worktrees {
		pr := wtCache.GetPRForBranch(worktrees[i].OriginURL, worktrees[i].Branch)
		if pr != nil && pr.Fetched && pr.State == "MERGED" {
			worktrees[i].IsMerged = true
		}
	}

	// Stop spinner and clear line
	sp.Stop()

	// Sort worktrees by repo name
	sort.Slice(worktrees, func(i, j int) bool {
		return worktrees[i].RepoName < worktrees[j].RepoName
	})

	// Determine which to remove
	var toRemove []git.Worktree
	toRemoveMap := make(map[string]bool)
	var skipped int

	for _, wt := range worktrees {
		shouldRemove := false
		if wt.IsMerged && !wt.IsDirty {
			shouldRemove = true
		} else if cmd.IncludeClean && wt.CommitCount == 0 && !wt.IsDirty {
			shouldRemove = true
		}

		if shouldRemove {
			toRemove = append(toRemove, wt)
			toRemoveMap[wt.Path] = true
		} else {
			skipped++
		}
	}

	// Display table with only purgeable worktrees
	if len(toRemove) > 0 {
		fmt.Print(ui.FormatWorktreesTable(toRemove, pathToID, wtCache, toRemoveMap, cmd.DryRun))
	}

	// Save updated cache
	if err := forge.SaveCache(scanPath, wtCache); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
	}

	// Select hooks for prune (before removing, so we can report errors early)
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandPrune)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(cmd.Env)
	if err != nil {
		return err
	}

	// Remove worktrees
	if !cmd.DryRun && len(toRemove) > 0 {
		for _, wt := range toRemove {
			if err := git.RemoveWorktree(wt, true); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", wt.Path, err)
				continue // Skip hooks for failed removals
			}

			// Run prune hooks for this worktree
			ctx := hooks.Context{
				Path:     wt.Path,
				Branch:   wt.Branch,
				Repo:     wt.RepoName,
				Folder:   filepath.Base(wt.MainRepo),
				MainRepo: wt.MainRepo,
				Trigger:  string(hooks.CommandPrune),
				Env:      env,
			}
			hooks.RunForEach(hookMatches, ctx, wt.MainRepo)
		}

		// Prune stale references
		processedRepos := make(map[string]bool)
		for _, wt := range toRemove {
			if !processedRepos[wt.MainRepo] {
				git.PruneWorktrees(wt.MainRepo)
				processedRepos[wt.MainRepo] = true
			}
		}
	}

	// Print summary
	fmt.Print(ui.FormatSummary(len(toRemove), skipped, cmd.DryRun))

	return nil
}

// runPruneTargets handles removal of multiple targeted worktrees by ID
func runPruneTargets(cmd *PruneCmd, cfg *config.Config, scanPath string) error {
	var errs []error
	for _, id := range cmd.ID {
		if err := runPruneTargetByID(id, cmd, cfg, scanPath); err != nil {
			errs = append(errs, fmt.Errorf("ID %d: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to prune some worktrees:\n%w", joinErrors(errs))
	}
	return nil
}

// joinErrors joins multiple errors with newlines
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	msg := errs[0].Error()
	for _, e := range errs[1:] {
		msg += "\n" + e.Error()
	}
	return fmt.Errorf("%s", msg)
}

// runPruneTargetByID handles removal of a single targeted worktree by ID
func runPruneTargetByID(id int, cmd *PruneCmd, cfg *config.Config, scanPath string) error {
	// Resolve target by ID
	target, err := resolve.ByID(id, scanPath)
	if err != nil {
		return err
	}

	// Get worktree info
	wt, err := git.GetWorktreeInfo(target.Path)
	if err != nil {
		return fmt.Errorf("failed to get worktree info: %w", err)
	}

	// Check if removable (unless force)
	if !cmd.Force {
		isPrunable := (wt.IsMerged && !wt.IsDirty) ||
			(cmd.IncludeClean && wt.CommitCount == 0 && !wt.IsDirty)
		if !isPrunable {
			return fmt.Errorf("worktree %q is not removable: %s",
				target.Branch, formatNotRemovableReason(wt, cmd.IncludeClean))
		}
	}

	// Dry run output
	if cmd.DryRun {
		fmt.Printf("Would remove worktree: %s (%s)\n", target.Branch, target.Path)
		return nil
	}

	// Select hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandPrune)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(cmd.Env)
	if err != nil {
		return err
	}

	// Remove worktree
	if err := git.RemoveWorktree(*wt, cmd.Force); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	// Run hooks
	ctx := hooks.Context{
		Path:     wt.Path,
		Branch:   wt.Branch,
		Repo:     wt.RepoName,
		Folder:   filepath.Base(wt.MainRepo),
		MainRepo: wt.MainRepo,
		Trigger:  string(hooks.CommandPrune),
		Env:      env,
	}
	hooks.RunForEach(hookMatches, ctx, wt.MainRepo)

	// Prune stale references
	git.PruneWorktrees(wt.MainRepo)

	fmt.Printf("Removed worktree: %s (%s)\n", target.Branch, target.Path)
	return nil
}

// formatNotRemovableReason returns a helpful error message explaining why a worktree
// can't be removed and what flags could help
func formatNotRemovableReason(wt *git.Worktree, includeCleanSet bool) string {
	if wt.IsDirty {
		return "has uncommitted changes (use -f to force)"
	}

	// Not dirty, but not merged
	if wt.CommitCount == 0 {
		if includeCleanSet {
			// -c was set but still not removable - shouldn't happen if not dirty
			return "not merged (use -f to force)"
		}
		return "not merged, but has 0 commits ahead (use -c to include clean worktrees, or -f to force)"
	}

	// Has commits ahead and not merged
	commitWord := "commit"
	if wt.CommitCount > 1 {
		commitWord = "commits"
	}
	return fmt.Sprintf("not merged (%d %s ahead of default branch), use -f to force", wt.CommitCount, commitWord)
}

// refreshPRStatus fetches PR status for all worktrees in parallel and updates the cache
func refreshPRStatus(worktrees []git.Worktree, wtCache *forge.Cache, cfg *config.Config, sp *ui.Spinner) {
	// Filter to worktrees with upstream branches
	var toFetch []git.Worktree
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}
		if git.GetUpstreamBranch(wt.MainRepo, wt.Branch) == "" {
			continue
		}
		toFetch = append(toFetch, wt)
	}

	if len(toFetch) == 0 {
		return
	}

	sp.UpdateMessage(fmt.Sprintf("Fetching PR status (%d branches)...", len(toFetch)))

	// Fetch PRs in parallel with rate limiting
	var prMutex sync.Mutex
	var prWg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentPRFetches)
	var fetchedCount, failedCount int
	var countMutex sync.Mutex

	for _, wt := range toFetch {
		prWg.Add(1)
		go func(wt git.Worktree) {
			defer prWg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			// Detect forge for this repo
			f := forge.Detect(wt.OriginURL, cfg.Hosts)

			// Check if forge CLI is available
			if err := f.Check(); err != nil {
				countMutex.Lock()
				failedCount++
				countMutex.Unlock()
				return
			}

			// Use upstream branch name for PR lookup (may differ from local)
			upstreamBranch := git.GetUpstreamBranch(wt.MainRepo, wt.Branch)
			if upstreamBranch == "" {
				return
			}

			pr, err := f.GetPRForBranch(wt.OriginURL, upstreamBranch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: PR fetch failed for %s: %v\n", wt.Branch, err)
				countMutex.Lock()
				failedCount++
				countMutex.Unlock()
				return
			}

			prMutex.Lock()
			wtCache.SetPRForBranch(wt.OriginURL, wt.Branch, pr)
			prMutex.Unlock()

			countMutex.Lock()
			fetchedCount++
			countMutex.Unlock()
		}(wt)
	}

	prWg.Wait()

	// Print summary (spinner still running)
	if failedCount > 0 {
		sp.UpdateMessage(fmt.Sprintf("Fetched: %d, Failed: %d", fetchedCount, failedCount))
	}
}

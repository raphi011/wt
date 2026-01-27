package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/resolve"
	"github.com/raphi011/wt/internal/ui"
)

// maxConcurrentPRFetches limits parallel gh API calls to avoid rate limiting
const maxConcurrentPRFetches = 5

// pruneReason describes why a worktree is being pruned or skipped
type pruneReason string

const (
	// Prune reasons (will be removed)
	reasonMergedPR     pruneReason = "Merged PR"
	reasonMergedBranch pruneReason = "Merged branch"
	reasonClean        pruneReason = "Clean"

	// Skip reasons (will not be removed)
	skipDirty      pruneReason = "Dirty"
	skipNotMerged  pruneReason = "Not merged"
	skipHasCommits pruneReason = "Has commits"
)

func (c *PruneCmd) runPrune(ctx context.Context) error {
	l := log.FromContext(ctx)
	out := output.FromContext(ctx)
	cfg := c.Config
	workDir := c.WorkDir

	// Validate -f requires -n
	if c.Force && len(c.ID) == 0 && !c.Interactive {
		return fmt.Errorf("-f/--force requires -n/--number to target specific worktree(s)")
	}

	// Validate --verbose cannot be used with -n
	if c.Verbose && len(c.ID) > 0 {
		return fmt.Errorf("--verbose cannot be used with -n/--number")
	}

	// Validate --interactive cannot be used with -n
	if c.Interactive && len(c.ID) > 0 {
		return fmt.Errorf("--interactive cannot be used with -n/--number")
	}

	worktreeDir, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// If IDs are specified, handle targeted worktree removal
	if len(c.ID) > 0 {
		if c.ResetCache {
			return fmt.Errorf("--reset-cache cannot be used with --number")
		}
		return c.runPruneTargets(ctx, worktreeDir)
	}

	if c.DryRun {
		l.Printf("Pruning worktrees in %s (dry run)\n", worktreeDir)
	} else {
		l.Printf("Pruning worktrees in %s\n", worktreeDir)
	}

	// Start spinner
	sp := ui.NewSpinner("Scanning worktrees...")
	sp.Start()

	// List all worktrees (include dirty check for prune decisions)
	allWorktrees, err := git.ListWorktrees(ctx, worktreeDir, true)
	if err != nil {
		sp.Stop()
		return err
	}

	if len(allWorktrees) == 0 {
		sp.Stop()
		out.Println("No worktrees found")
		return nil
	}

	// If in a git repo and not using --global, filter to only prune worktrees from that repo
	worktrees := allWorktrees
	var currentRepo string
	if !c.Global {
		currentRepo = git.GetCurrentRepoMainPathFrom(ctx, workDir)
		if currentRepo != "" {
			worktrees = git.FilterWorktreesByRepo(allWorktrees, currentRepo)
		}
	}

	if len(worktrees) == 0 {
		sp.Stop()
		out.Printf("No worktrees found for current repository\n")
		out.Printf("Use --global to prune all %d worktrees\n", len(allWorktrees))
		return nil
	}

	// Load cache with lock
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		return err
	}
	defer unlock()

	// Reset cache if requested (before sync so worktrees get fresh IDs)
	if c.ResetCache {
		wtCache.Reset()
		l.Println("Cache reset: PR info cleared, IDs will be reassigned from 1")
	}

	// Convert ALL worktrees to WorktreeInfo for cache sync (to preserve IDs)
	wtInfos := make([]cache.WorktreeInfo, len(allWorktrees))
	for i, wt := range allWorktrees {
		wtInfos[i] = cache.WorktreeInfo{
			Path:      wt.Path,
			RepoPath:  wt.MainRepo,
			Branch:    wt.Branch,
			OriginURL: wt.OriginURL,
		}
	}

	// Sync cache to get IDs (sync all, even if we're only pruning a subset)
	pathToID := wtCache.SyncWorktrees(wtInfos)

	// Refresh: fetch remotes and PR status
	if c.Refresh {
		// Group by repo for fetching
		grouped := git.GroupWorktreesByRepo(worktrees)

		// Fetch default branch for each repo
		for repoName, wts := range grouped {
			if len(wts) == 0 {
				continue
			}
			defaultBranch := git.GetDefaultBranch(ctx, wts[0].MainRepo)
			sp.UpdateMessage(fmt.Sprintf("Fetching origin/%s for %s...", defaultBranch, repoName))
			if err := git.FetchDefaultBranch(ctx, wts[0].MainRepo); err != nil {
				// Non-fatal: log warning but continue
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}
		}

		sp.UpdateMessage("Fetching PR status...")
		refreshPRStatus(ctx, worktrees, wtCache, cfg.Hosts, &cfg.Forge, sp)
	}

	updateMergeStatusFromCache(worktrees, wtCache)

	// Stop spinner and clear line
	sp.Stop()

	// Sort worktrees by repo name
	slices.SortFunc(worktrees, func(a, b git.Worktree) int {
		if a.RepoName < b.RepoName {
			return -1
		}
		if a.RepoName > b.RepoName {
			return 1
		}
		return 0
	})

	// Determine which to remove and why
	var toRemove []git.Worktree
	var toSkip []git.Worktree
	toRemoveMap := make(map[string]bool)
	reasonMap := make(map[string]pruneReason)

	for _, wt := range worktrees {
		var reason pruneReason
		var skipReason pruneReason

		// Check for PR merged first (highest priority)
		folderName := filepath.Base(wt.Path)
		pr := wtCache.GetPRForBranch(folderName)
		if pr != nil && pr.Fetched && pr.State == "MERGED" && !wt.IsDirty {
			reason = reasonMergedPR
		} else if wt.IsMerged && !wt.IsDirty {
			reason = reasonMergedBranch
		} else if c.IncludeClean && wt.CommitCount == 0 && !wt.IsDirty {
			reason = reasonClean
		} else {
			// Determine skip reason
			if wt.IsDirty {
				skipReason = skipDirty
			} else if wt.CommitCount > 0 {
				skipReason = skipHasCommits
			} else {
				skipReason = skipNotMerged
			}
		}

		if reason != "" {
			toRemove = append(toRemove, wt)
			toRemoveMap[wt.Path] = true
			reasonMap[wt.Path] = reason
		} else {
			toSkip = append(toSkip, wt)
			reasonMap[wt.Path] = skipReason
		}
	}

	// Handle interactive mode
	if c.Interactive {
		// Build worktree info for the wizard
		wizardInfos := make([]ui.PruneWorktreeInfo, 0, len(worktrees))
		for _, wt := range worktrees {
			wizardInfos = append(wizardInfos, ui.PruneWorktreeInfo{
				ID:       pathToID[wt.Path],
				RepoName: wt.RepoName,
				Branch:   wt.Branch,
				Reason:   string(reasonMap[wt.Path]),
				IsDirty:  wt.IsDirty,
				Worktree: wt,
			})
		}

		opts, err := ui.PruneInteractive(ui.PruneWizardParams{
			Worktrees:    wizardInfos,
			IncludeClean: c.IncludeClean,
		})
		if err != nil {
			return fmt.Errorf("interactive mode error: %w", err)
		}
		if opts.Cancelled {
			return nil
		}

		// If no worktrees selected, nothing to do
		if len(opts.SelectedIDs) == 0 {
			out.Println("No worktrees selected for removal")
			return nil
		}

		// Rebuild toRemove/toRemoveMap based on user selection
		selectedIDSet := make(map[int]bool)
		for _, id := range opts.SelectedIDs {
			selectedIDSet[id] = true
		}

		toRemove = nil
		toRemoveMap = make(map[string]bool)
		toSkip = nil

		for _, wt := range worktrees {
			if selectedIDSet[pathToID[wt.Path]] {
				toRemove = append(toRemove, wt)
				toRemoveMap[wt.Path] = true
			} else {
				toSkip = append(toSkip, wt)
			}
		}
	}

	// Select hooks for prune (before removing, so we can report errors early)
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, c.Hook, c.NoHook, hooks.CommandPrune)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(c.Env)
	if err != nil {
		return err
	}

	// Track actual removal results
	var removed []git.Worktree
	var failed []git.Worktree
	removedMap := make(map[string]bool)

	// Remove worktrees (or just mark for dry run)
	if len(toRemove) > 0 {
		if c.DryRun {
			// Dry run: all would be removed
			removed = toRemove
			for _, wt := range toRemove {
				removedMap[wt.Path] = true
			}
		} else {
			// Actual removal
			for _, wt := range toRemove {
				if err := git.RemoveWorktree(ctx, wt, true); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", wt.Path, err)
					failed = append(failed, wt)
					continue
				}

				// Mark as removed in cache immediately
				folderName := filepath.Base(wt.Path)
				wtCache.MarkRemoved(folderName)
				removed = append(removed, wt)
				removedMap[wt.Path] = true

				// Run prune hooks for this worktree
				hookCtx := hooks.Context{
					WorktreeDir: wt.Path,
					RepoDir:     wt.MainRepo,
					Branch:      wt.Branch,
					Repo:        filepath.Base(wt.MainRepo),
					Origin:      wt.RepoName,
					Trigger:     string(hooks.CommandPrune),
					Env:         env,
				}
				if hookCtx.Origin == "" {
					hookCtx.Origin = hookCtx.Repo
				}
				hooks.RunForEach(hookMatches, hookCtx, wt.MainRepo)
			}

			// Prune stale references
			processedRepos := make(map[string]bool)
			for _, wt := range removed {
				if !processedRepos[wt.MainRepo] {
					git.PruneWorktrees(ctx, wt.MainRepo)
					processedRepos[wt.MainRepo] = true
				}
			}
		}
	}

	// Convert pruneReason map to string map for UI
	stringReasonMap := make(map[string]string)
	for path, reason := range reasonMap {
		stringReasonMap[path] = string(reason)
	}

	// Display table with actual results
	if len(removed) > 0 || (c.Verbose && len(toSkip) > 0) {
		var displayWorktrees []git.Worktree
		displayWorktrees = append(displayWorktrees, removed...)
		if c.Verbose {
			displayWorktrees = append(displayWorktrees, toSkip...)
		}
		out.Print(ui.FormatPruneTable(displayWorktrees, pathToID, stringReasonMap, toRemoveMap))
	}

	// Save updated cache (with RemovedAt timestamps)
	if err := cache.Save(worktreeDir, wtCache); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
	}

	// Print summary with actual counts
	out.Print(ui.FormatSummary(len(removed), len(toSkip)+len(failed), c.DryRun))

	return nil
}

// runPruneTargets handles removal of multiple targeted worktrees by ID
func (c *PruneCmd) runPruneTargets(ctx context.Context, worktreeDir string) error {
	var errs []error
	for _, id := range c.ID {
		if err := c.runPruneTargetByID(ctx, id, worktreeDir); err != nil {
			errs = append(errs, fmt.Errorf("ID %d: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to prune some worktrees:\n%w", errors.Join(errs...))
	}
	return nil
}

// runPruneTargetByID handles removal of a single targeted worktree by ID
func (c *PruneCmd) runPruneTargetByID(ctx context.Context, id int, worktreeDir string) error {
	l := log.FromContext(ctx)
	cfg := c.Config
	// Resolve target by ID
	target, err := resolve.ByID(id, worktreeDir)
	if err != nil {
		return err
	}

	// Get worktree info
	wt, err := git.GetWorktreeInfo(ctx, target.Path)
	if err != nil {
		return fmt.Errorf("failed to get worktree info: %w", err)
	}

	// Check if removable (unless force)
	if !c.Force {
		isPrunable := (wt.IsMerged && !wt.IsDirty) ||
			(c.IncludeClean && wt.CommitCount == 0 && !wt.IsDirty)
		if !isPrunable {
			return fmt.Errorf("worktree %q is not removable: %s",
				target.Branch, formatNotRemovableReason(wt, c.IncludeClean))
		}
	}

	// Dry run output
	if c.DryRun {
		l.Printf("Would remove worktree: %s (%s)\n", target.Branch, target.Path)
		return nil
	}

	// Select hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, c.Hook, c.NoHook, hooks.CommandPrune)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(c.Env)
	if err != nil {
		return err
	}

	// Remove worktree
	if err := git.RemoveWorktree(ctx, *wt, c.Force); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	// Run hooks
	hookCtx := hooks.Context{
		WorktreeDir: wt.Path,
		RepoDir:     wt.MainRepo,
		Branch:      wt.Branch,
		Repo:        filepath.Base(wt.MainRepo),
		Origin:      wt.RepoName,
		Trigger:     string(hooks.CommandPrune),
		Env:         env,
	}
	if hookCtx.Origin == "" {
		hookCtx.Origin = hookCtx.Repo
	}
	hooks.RunForEach(hookMatches, hookCtx, wt.MainRepo)

	// Prune stale references
	git.PruneWorktrees(ctx, wt.MainRepo)

	l.Printf("Removed worktree: %s (%s)\n", target.Branch, target.Path)
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
		return "clean (use -c to include clean worktrees, or -f to force)"
	}

	// Has commits ahead and not merged
	commitWord := "commit"
	if wt.CommitCount > 1 {
		commitWord = "commits"
	}
	return fmt.Sprintf("not merged (%d %s ahead of default branch), use -f to force", wt.CommitCount, commitWord)
}

// refreshPRStatus fetches PR status for all worktrees in parallel and updates the cache
func refreshPRStatus(ctx context.Context, worktrees []git.Worktree, wtCache *cache.Cache, hosts map[string]string, forgeConfig *config.ForgeConfig, sp *ui.Spinner) {
	// Filter to worktrees with upstream branches
	var toFetch []git.Worktree
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}
		if git.GetUpstreamBranch(ctx, wt.MainRepo, wt.Branch) == "" {
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
			f := forge.Detect(wt.OriginURL, hosts, forgeConfig)

			// Check if forge CLI is available
			if err := f.Check(ctx); err != nil {
				countMutex.Lock()
				failedCount++
				countMutex.Unlock()
				return
			}

			// Use upstream branch name for PR lookup (may differ from local)
			upstreamBranch := git.GetUpstreamBranch(ctx, wt.MainRepo, wt.Branch)
			if upstreamBranch == "" {
				return
			}

			pr, err := f.GetPRForBranch(ctx, wt.OriginURL, upstreamBranch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: PR fetch failed for %s: %v\n", wt.Branch, err)
				countMutex.Lock()
				failedCount++
				countMutex.Unlock()
				return
			}

			prMutex.Lock()
			folderName := filepath.Base(wt.Path)
			wtCache.SetPRForBranch(folderName, pr)
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

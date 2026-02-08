package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/progress"
	"github.com/raphi011/wt/internal/ui/static"
	"github.com/raphi011/wt/internal/ui/styles"
	"github.com/raphi011/wt/internal/ui/wizard/flows"
)


// pruneReason describes why a worktree is being pruned or skipped
type pruneReason string

const (
	// Skip reasons (will not be removed)
	skipDirty pruneReason = "Dirty"
	skipNoPR  pruneReason = "No PR"
)

// formatPruneReason returns a display string for the prune reason
// Uses symbols when nerdfont is enabled
func formatPruneReason(wt pruneWorktree) string {
	if wt.IsDirty {
		return string(skipDirty)
	}
	if wt.PRState == "" {
		return string(skipNoPR)
	}
	// Use PR state formatting with symbols
	return styles.FormatPRState(wt.PRState, wt.IsDraft)
}

// pruneWorktree holds worktree info for prune operations
type pruneWorktree struct {
	Path      string
	Branch    string
	RepoName  string
	RepoPath  string
	OriginURL string
	IsDirty   bool
	PRState   string
	IsDraft   bool
}

func newPruneCmd() *cobra.Command {
	var (
		dryRun           bool
		force            bool
		verbose          bool
		global           bool
		refresh          bool
		resetCache       bool
		hookNames        []string
		noHook           bool
		env              []string
		interactive      bool
		deleteBranches   bool
		noDeleteBranches bool
	)

	cmd := &cobra.Command{
		Use:     "prune [[scope:]branch...]",
		Short:   "Prune merged worktrees",
		Aliases: []string{"p"},
		GroupID: GroupCore,
		Long: `Remove worktrees with merged PRs.

Without arguments, removes all worktrees with merged PRs in current repo.
Use --global to prune all registered repos.
Use --interactive to select worktrees to prune.

Target specific worktrees using [scope:]branch arguments where scope can be
a repo name or label. Use -f when targeting specific worktrees.`,
		Example: `  wt prune                         # Remove worktrees with merged PRs
  wt prune --global                # Prune all repos
  wt prune -d                      # Dry-run: preview without removing
  wt prune -i                      # Interactive mode
  wt prune feature -f              # Remove feature worktree (all repos)
  wt prune myrepo:feature -f       # Remove specific worktree
  wt prune backend:main -f         # Remove main in backend-labeled repos`,
		ValidArgsFunction: completeScopedWorktreeArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// If specific targets provided, handle targeted removal
			if len(args) > 0 {
				if !force {
					return fmt.Errorf("targeting specific worktrees requires -f/--force")
				}
				// Determine if we should delete local branches
				shouldDeleteBranches := cfg.Prune.DeleteLocalBranches
				if cmd.Flags().Changed("delete-branches") {
					shouldDeleteBranches = deleteBranches
				} else if cmd.Flags().Changed("no-delete-branches") {
					shouldDeleteBranches = false
				}
				return runPruneTargets(ctx, reg, args, dryRun, shouldDeleteBranches, hookNames, noHook, env)
			}

			// Determine target repos for auto-prune
			var repos []registry.Repo
			if global {
				repos = reg.Repos
			} else {
				// Try current repo
				repo, err := findOrRegisterCurrentRepoFromContext(ctx, reg)
				if err != nil {
					// Not in a repo, prune all
					repos = reg.Repos
				} else {
					repos = []registry.Repo{repo}
				}
			}

			if len(repos) == 0 {
				out.Println("No repos found")
				return nil
			}

			l.Debug("pruning worktrees", "repos", len(repos), "dryRun", dryRun)

			// Collect all worktrees from target repos
			var allWorktrees []pruneWorktree
			for _, repo := range repos {
				wtInfos, err := git.ListWorktreesFromRepo(ctx, repo.Path)
				if err != nil {
					l.Printf("Warning: %s: %v\n", repo.Name, err)
					continue
				}
				// Get origin URL once per repo
				originURL, _ := git.GetOriginURL(ctx, repo.Path)
				for _, wti := range wtInfos {
					allWorktrees = append(allWorktrees, pruneWorktree{
						Path:      wti.Path,
						Branch:    wti.Branch,
						RepoName:  repo.Name,
						RepoPath:  repo.Path,
						OriginURL: originURL,
						IsDirty:   git.IsDirty(ctx, wti.Path),
					})
				}
			}

			if len(allWorktrees) == 0 {
				out.Println("No worktrees found")
				return nil
			}

			// Start spinner
			sp := progress.NewSpinner("Scanning worktrees...")
			sp.Start()

			// Load PR cache
			prCache, err := prcache.Load()
			if err != nil {
				sp.Stop()
				return fmt.Errorf("load cache: %w", err)
			}

			// Reset cache if requested
			if resetCache {
				prCache.Reset()
				l.Println("Cache reset: PR info cleared")
			}

			// Refresh PR status if requested
			if refresh {
				sp.UpdateMessage(fmt.Sprintf("Fetching PR status (%d branches)...", len(allWorktrees)))
				refreshPRStatusForPrune(ctx, allWorktrees, prCache, cfg.Hosts, &cfg.Forge, sp)
			}

			// Update PR state from cache
			for i := range allWorktrees {
				folderName := filepath.Base(allWorktrees[i].Path)
				if pr := prCache.Get(folderName); pr != nil && pr.Fetched {
					allWorktrees[i].PRState = pr.State
					allWorktrees[i].IsDraft = pr.IsDraft
				}
			}

			sp.Stop()

			// Sort by repo name
			slices.SortFunc(allWorktrees, func(a, b pruneWorktree) int {
				if a.RepoName < b.RepoName {
					return -1
				}
				if a.RepoName > b.RepoName {
					return 1
				}
				return 0
			})

			// Determine which to remove and why
			var toRemove []pruneWorktree
			var toSkip []pruneWorktree

			for _, wt := range allWorktrees {
				// Only auto-prune worktrees with merged PRs
				if wt.PRState == "MERGED" && !wt.IsDirty {
					toRemove = append(toRemove, wt)
				} else {
					toSkip = append(toSkip, wt)
				}
			}

			// Handle interactive mode
			if interactive {
				wizardInfos := make([]flows.PruneWorktreeInfo, 0, len(allWorktrees))
				for i, wt := range allWorktrees {
					// Worktree is prunable if it has a merged PR and no uncommitted changes
					isPrunable := wt.PRState == "MERGED" && !wt.IsDirty
					wizardInfos = append(wizardInfos, flows.PruneWorktreeInfo{
						ID:         i + 1, // Use index as ID
						RepoName:   wt.RepoName,
						Branch:     wt.Branch,
						Reason:     formatPruneReason(wt),
						IsDirty:    wt.IsDirty,
						IsPrunable: isPrunable,
					})
				}

				opts, err := flows.PruneInteractive(flows.PruneWizardParams{
					Worktrees: wizardInfos,
				})
				if err != nil {
					return fmt.Errorf("interactive mode error: %w", err)
				}
				if opts.Cancelled {
					return nil
				}

				if len(opts.SelectedIDs) == 0 {
					out.Println("No worktrees selected for removal")
					return nil
				}

				// Rebuild based on selection (IDs are 1-indexed)
				selectedSet := make(map[int]bool)
				for _, id := range opts.SelectedIDs {
					selectedSet[id] = true
				}

				toRemove = nil
				toSkip = nil

				for i, wt := range allWorktrees {
					if selectedSet[i+1] {
						toRemove = append(toRemove, wt)
					} else {
						toSkip = append(toSkip, wt)
					}
				}
			}

			// Determine if we should delete local branches
			shouldDeleteBranches := cfg.Prune.DeleteLocalBranches
			if cmd.Flags().Changed("delete-branches") {
				shouldDeleteBranches = deleteBranches
			} else if cmd.Flags().Changed("no-delete-branches") {
				shouldDeleteBranches = false
			}

			// Remove worktrees using shared helper
			// For auto-prune (merged PRs), force=true is implicit
			removed, failed := pruneWorktrees(ctx, toRemove, true, dryRun, shouldDeleteBranches, hookNames, noHook, env, prCache)

			// Display results
			if len(removed) > 0 || (verbose && len(toSkip) > 0) {
				fmt.Println()
				if dryRun {
					fmt.Println("Would remove:")
				} else {
					fmt.Println("Removed:")
				}
				headers := []string{"REPO", "BRANCH", "REASON"}
				var rows [][]string
				for _, wt := range removed {
					rows = append(rows, []string{wt.RepoName, wt.Branch, formatPruneReason(wt)})
				}
				out.Print(static.RenderTable(headers, rows))

				if verbose && len(toSkip) > 0 {
					fmt.Println("Skipped:")
					rows = nil
					for _, wt := range toSkip {
						rows = append(rows, []string{wt.RepoName, wt.Branch, formatPruneReason(wt)})
					}
					out.Print(static.RenderTable(headers, rows))
				}
			}

			// Save PR cache
			if err := prCache.Save(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
			}

			// Print summary
			if dryRun {
				out.Printf("\nWould remove %d worktree(s), skip %d\n", len(removed), len(toSkip)+len(failed))
			} else {
				out.Printf("\nRemoved %d worktree(s), skipped %d\n", len(removed), len(toSkip)+len(failed))
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Preview without removing")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force remove (required for targeted prune)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show non-prunable worktrees")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Prune all repos")
	cmd.Flags().BoolVarP(&refresh, "refresh", "R", false, "Refresh PR status first")
	cmd.Flags().BoolVar(&resetCache, "reset-cache", false, "Clear all cached data")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-removal hooks")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	cmd.Flags().BoolVarP(&deleteBranches, "delete-branches", "b", false, "Delete local branches after removal")
	cmd.Flags().BoolVar(&noDeleteBranches, "no-delete-branches", false, "Keep local branches (overrides config)")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")
	cmd.MarkFlagsMutuallyExclusive("delete-branches", "no-delete-branches")

	// Completions
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)

	return cmd
}

// runPruneTargets handles removal of specific worktrees by [scope:]branch args.
func runPruneTargets(ctx context.Context, reg *registry.Registry, targets []string, dryRun, deleteBranches bool, hookNames []string, noHook bool, env []string) error {
	l := log.FromContext(ctx)
	out := output.FromContext(ctx)

	// Resolve all targets
	wtTargets, err := resolveWorktreeTargets(ctx, reg, targets)
	if err != nil {
		return err
	}

	// Convert to pruneWorktree
	var toRemove []pruneWorktree
	for _, t := range wtTargets {
		toRemove = append(toRemove, pruneWorktree{
			Path:     t.Path,
			Branch:   t.Branch,
			RepoName: t.RepoName,
			RepoPath: t.RepoPath,
		})
	}

	if dryRun {
		out.Println("Would remove:")
		for _, wt := range toRemove {
			l.Printf("  %s:%s (%s)\n", wt.RepoName, wt.Branch, wt.Path)
		}
		return nil
	}

	// Use pruneWorktrees for consistent removal logic
	removed, failed := pruneWorktrees(ctx, toRemove, true, false, deleteBranches, hookNames, noHook, env, nil)

	for _, wt := range removed {
		l.Printf("Removed worktree: %s:%s (%s)\n", wt.RepoName, wt.Branch, wt.Path)
	}
	for _, wt := range failed {
		l.Printf("Failed to remove: %s:%s (%s)\n", wt.RepoName, wt.Branch, wt.Path)
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to remove %d worktree(s)", len(failed))
	}

	return nil
}

// refreshPRStatusForPrune fetches PR status for worktrees in parallel.
// Uses a progress bar to show determinate progress during fetching.
func refreshPRStatusForPrune(ctx context.Context, worktrees []pruneWorktree, prCache *prcache.Cache, hosts map[string]string, forgeConfig *config.ForgeConfig, sp *progress.Spinner) {
	// Filter to worktrees that need PR status fetched
	var toFetch []pruneWorktree
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}
		// Skip already merged
		folderName := filepath.Base(wt.Path)
		if pr := prCache.Get(folderName); pr != nil && pr.Fetched && pr.State == "MERGED" {
			continue
		}
		toFetch = append(toFetch, wt)
	}

	if len(toFetch) == 0 {
		return
	}

	// Stop spinner and switch to progress bar for determinate progress
	sp.Stop()
	pb := progress.NewProgressBar(len(toFetch), "Fetching PR status...")
	pb.Start()
	defer pb.Stop()

	var prMutex sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, forge.MaxConcurrentFetches)
	var completedCount, failedCount int
	var countMutex sync.Mutex

	for _, wt := range toFetch {
		wg.Add(1)
		go func(wt pruneWorktree) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			f := forge.Detect(wt.OriginURL, hosts, forgeConfig)

			if err := f.Check(ctx); err != nil {
				countMutex.Lock()
				completedCount++
				failedCount++
				pb.SetProgress(completedCount, fmt.Sprintf("Fetching PR status... (%d failed)", failedCount))
				countMutex.Unlock()
				return
			}

			// Get upstream branch
			upstreamBranch := git.GetUpstreamBranch(ctx, wt.RepoPath, wt.Branch)
			if upstreamBranch == "" {
				upstreamBranch = wt.Branch
			}

			pr, err := f.GetPRForBranch(ctx, wt.OriginURL, upstreamBranch)
			if err != nil {
				countMutex.Lock()
				completedCount++
				failedCount++
				pb.SetProgress(completedCount, fmt.Sprintf("Fetching PR status... (%d failed)", failedCount))
				countMutex.Unlock()
				return
			}

			prMutex.Lock()
			folderName := filepath.Base(wt.Path)
			prCache.Set(folderName, prcache.FromForge(pr))
			prMutex.Unlock()

			countMutex.Lock()
			completedCount++
			if failedCount > 0 {
				pb.SetProgress(completedCount, fmt.Sprintf("Fetching PR status... (%d failed)", failedCount))
			} else {
				pb.SetProgress(completedCount, "Fetching PR status...")
			}
			countMutex.Unlock()
		}(wt)
	}

	wg.Wait()
}

// pruneWorktrees removes the given worktrees and runs hooks.
// Returns slices of successfully removed and failed worktrees.
func pruneWorktrees(ctx context.Context, toRemove []pruneWorktree, force, dryRun, deleteBranches bool,
	hookNames []string, noHook bool, env []string, prCache *prcache.Cache) (removed, failed []pruneWorktree) {

	if len(toRemove) == 0 {
		return nil, nil
	}

	if dryRun {
		return toRemove, nil
	}

	cfg := config.FromContext(ctx)

	// Select hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, hookNames, noHook, hooks.CommandPrune)
	if err != nil {
		// Log error but continue - don't fail the whole prune
		fmt.Fprintf(os.Stderr, "Warning: failed to select hooks: %v\n", err)
	}

	hookEnv, err := hooks.ParseEnvWithStdin(env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to parse hook env: %v\n", err)
	}

	for _, wt := range toRemove {
		// Get full worktree info for removal
		wtInfo, err := git.GetWorktreeInfo(ctx, wt.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get info for %s: %v\n", wt.Path, err)
			failed = append(failed, wt)
			continue
		}

		if err := git.RemoveWorktree(ctx, *wtInfo, force); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", wt.Path, err)
			failed = append(failed, wt)
			continue
		}

		// Remove from PR cache
		if prCache != nil {
			folderName := filepath.Base(wt.Path)
			prCache.Delete(folderName)
		}
		removed = append(removed, wt)

		// Delete local branch if enabled
		if deleteBranches {
			l := log.FromContext(ctx)
			if err := git.DeleteLocalBranch(ctx, wt.RepoPath, wt.Branch, false); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete branch %s: %v\n", wt.Branch, err)
			} else {
				l.Debug("deleted branch", "branch", wt.Branch)
			}
		}

		// Run hooks
		if hookMatches != nil {
			hookCtx := hooks.Context{
				WorktreeDir: wt.Path,
				RepoDir:     wt.RepoPath,
				Branch:      wt.Branch,
				Repo:        filepath.Base(wt.RepoPath),
				Origin:      wt.RepoName,
				Trigger:     string(hooks.CommandPrune),
				Env:         hookEnv,
			}
			hooks.RunForEach(hookMatches, hookCtx, wt.RepoPath)
		}
	}

	// Prune stale references
	processedRepos := make(map[string]bool)
	for _, wt := range removed {
		if !processedRepos[wt.RepoPath] {
			git.PruneWorktrees(ctx, wt.RepoPath)
			processedRepos[wt.RepoPath] = true
		}
	}

	return removed, failed
}

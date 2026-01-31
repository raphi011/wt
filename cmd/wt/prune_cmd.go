package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui"
)

// maxConcurrentPRFetches limits parallel gh API calls to avoid rate limiting
const maxConcurrentPRFetches = 5

// pruneReason describes why a worktree is being pruned or skipped
type pruneReason string

const (
	// Prune reasons (will be removed)
	reasonMergedPR pruneReason = "Merged PR"

	// Skip reasons (will not be removed)
	skipDirty     pruneReason = "Dirty"
	skipNoPR      pruneReason = "No PR"
	skipPRNotDone pruneReason = "PR open"
)

// pruneWorktree holds worktree info for prune operations
type pruneWorktree struct {
	Path      string
	Branch    string
	RepoName  string
	RepoPath  string
	OriginURL string
	IsDirty   bool
	PRState   string
}

func newPruneCmd() *cobra.Command {
	var (
		dryRun      bool
		force       bool
		verbose     bool
		global      bool
		refresh     bool
		resetCache  bool
		hookNames   []string
		noHook      bool
		env         []string
		interactive bool
		repository  []string
		labels      []string
		branch      string
	)

	cmd := &cobra.Command{
		Use:     "prune",
		Short:   "Prune merged worktrees",
		Aliases: []string{"p"},
		Long: `Remove worktrees with merged PRs.

Without flags, removes all worktrees with merged PRs in current repo.
Use --global to prune all registered repos.
Use --interactive to select worktrees to prune.

Examples:
  wt prune                    # Remove worktrees with merged PRs
  wt prune --global           # Prune all repos
  wt prune -d                 # Dry-run: preview without removing
  wt prune -i                 # Interactive mode
  wt prune -r myrepo          # Prune specific repo
  wt prune -l backend         # Prune repos with label
  wt prune --branch feature   # Remove specific branch worktree`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Validate flags
			if force && branch == "" && !interactive {
				return fmt.Errorf("-f/--force requires --branch to target specific worktree")
			}

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine target repos
			var repos []*registry.Repo
			if global {
				for i := range reg.Repos {
					repos = append(repos, &reg.Repos[i])
				}
			} else if len(repository) > 0 || len(labels) > 0 {
				repos, err = resolveTargetRepos(ctx, reg, repository, labels)
				if err != nil {
					return err
				}
			} else {
				// Try current repo
				repo, err := findOrRegisterCurrentRepo(ctx, reg)
				if err != nil {
					// Not in a repo, prune all
					for i := range reg.Repos {
						repos = append(repos, &reg.Repos[i])
					}
				} else {
					repos = []*registry.Repo{repo}
				}
			}

			if len(repos) == 0 {
				out.Println("No repos found")
				return nil
			}

			l.Debug("pruning worktrees", "repos", len(repos), "dryRun", dryRun)

			// If specific branch targeted, handle targeted removal
			if branch != "" {
				return runPruneTargetBranch(ctx, repos, branch, force, dryRun, hookNames, noHook, env)
			}

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
			sp := ui.NewSpinner("Scanning worktrees...")
			sp.Start()

			// Load cache
			cacheDir := getCacheDir()
			wtCache, unlock, err := cache.LoadWithLock(cacheDir)
			if err != nil {
				sp.Stop()
				return fmt.Errorf("load cache: %w", err)
			}
			defer unlock()

			// Reset cache if requested
			if resetCache {
				wtCache.Reset()
				l.Println("Cache reset: PR info cleared")
			}

			// Refresh PR status if requested
			if refresh {
				sp.UpdateMessage(fmt.Sprintf("Fetching PR status (%d branches)...", len(allWorktrees)))
				refreshPRStatusForPrune(ctx, allWorktrees, wtCache, cfg.Hosts, &cfg.Forge, sp)
			}

			// Update PR state from cache
			for i := range allWorktrees {
				folderName := filepath.Base(allWorktrees[i].Path)
				if pr := wtCache.GetPRForBranch(folderName); pr != nil && pr.Fetched {
					allWorktrees[i].PRState = pr.State
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
			toRemoveMap := make(map[string]bool)
			reasonMap := make(map[string]pruneReason)

			for _, wt := range allWorktrees {
				var reason pruneReason
				var skipReason pruneReason

				// Only auto-prune worktrees with merged PRs
				if wt.PRState == "MERGED" && !wt.IsDirty {
					reason = reasonMergedPR
				} else {
					if wt.IsDirty {
						skipReason = skipDirty
					} else if wt.PRState == "" {
						skipReason = skipNoPR
					} else {
						skipReason = skipPRNotDone
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
			if interactive {
				wizardInfos := make([]ui.PruneWorktreeInfo, 0, len(allWorktrees))
				for i, wt := range allWorktrees {
					wizardInfos = append(wizardInfos, ui.PruneWorktreeInfo{
						ID:       i + 1, // Use index as ID
						RepoName: wt.RepoName,
						Branch:   wt.Branch,
						Reason:   string(reasonMap[wt.Path]),
						IsDirty:  wt.IsDirty,
					})
				}

				opts, err := ui.PruneInteractive(ui.PruneWizardParams{
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
				toRemoveMap = make(map[string]bool)
				toSkip = nil

				for i, wt := range allWorktrees {
					if selectedSet[i+1] {
						toRemove = append(toRemove, wt)
						toRemoveMap[wt.Path] = true
					} else {
						toSkip = append(toSkip, wt)
					}
				}
			}

			// Select hooks
			hookMatches, err := hooks.SelectHooks(cfg.Hooks, hookNames, noHook, hooks.CommandPrune)
			if err != nil {
				return err
			}

			hookEnv, err := hooks.ParseEnvWithStdin(env)
			if err != nil {
				return err
			}

			// Remove worktrees
			var removed []pruneWorktree
			var failed []pruneWorktree

			if len(toRemove) > 0 {
				if dryRun {
					removed = toRemove
				} else {
					for _, wt := range toRemove {
						// Get full worktree info for removal
						wtInfo, err := git.GetWorktreeInfo(ctx, wt.Path)
						if err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to get info for %s: %v\n", wt.Path, err)
							failed = append(failed, wt)
							continue
						}

						if err := git.RemoveWorktree(ctx, *wtInfo, true); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", wt.Path, err)
							failed = append(failed, wt)
							continue
						}

						// Mark as removed in cache
						folderName := filepath.Base(wt.Path)
						wtCache.MarkRemoved(folderName)
						removed = append(removed, wt)

						// Run hooks
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

					// Prune stale references
					processedRepos := make(map[string]bool)
					for _, wt := range removed {
						if !processedRepos[wt.RepoPath] {
							git.PruneWorktrees(ctx, wt.RepoPath)
							processedRepos[wt.RepoPath] = true
						}
					}
				}
			}

			// Display results
			if len(removed) > 0 || (verbose && len(toSkip) > 0) {
				fmt.Println()
				if dryRun {
					fmt.Println("Would remove:")
				} else {
					fmt.Println("Removed:")
				}
				for _, wt := range removed {
					fmt.Printf("  %-20s %s (%s)\n", wt.RepoName, wt.Branch, reasonMap[wt.Path])
				}
				if verbose && len(toSkip) > 0 {
					fmt.Println("\nSkipped:")
					for _, wt := range toSkip {
						fmt.Printf("  %-20s %s (%s)\n", wt.RepoName, wt.Branch, reasonMap[wt.Path])
					}
				}
			}

			// Save cache
			if err := cache.Save(cacheDir, wtCache); err != nil {
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
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force remove")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show non-prunable worktrees")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Prune all repos")
	cmd.Flags().BoolVarP(&refresh, "refresh", "R", false, "Refresh PR status first")
	cmd.Flags().BoolVar(&resetCache, "reset-cache", false, "Clear all cached data")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-removal hooks")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Target repo(s) by name")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Target repos by label")
	cmd.Flags().StringVar(&branch, "branch", "", "Target specific branch worktree")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")

	// Completions
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("label", completeLabels)

	return cmd
}

// runPruneTargetBranch handles removal of a specific branch worktree
func runPruneTargetBranch(ctx context.Context, repos []*registry.Repo, branch string, force, dryRun bool, hookNames []string, noHook bool, env []string) error {
	l := log.FromContext(ctx)

	if !force {
		return fmt.Errorf("worktree %q requires -f/--force to remove", branch)
	}

	// Find the worktree
	var targetWT *git.Worktree
	var targetRepoPath string

	for _, repo := range repos {
		wtInfos, err := git.ListWorktreesFromRepo(ctx, repo.Path)
		if err != nil {
			continue
		}
		for _, wti := range wtInfos {
			if wti.Branch == branch {
				// Get full info
				wt, err := git.GetWorktreeInfo(ctx, wti.Path)
				if err == nil {
					targetWT = wt
					targetRepoPath = repo.Path
					break
				}
			}
		}
		if targetWT != nil {
			break
		}
	}

	if targetWT == nil {
		return fmt.Errorf("worktree for branch %q not found", branch)
	}

	if dryRun {
		l.Printf("Would remove worktree: %s (%s)\n", branch, targetWT.Path)
		return nil
	}

	// Select hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, hookNames, noHook, hooks.CommandPrune)
	if err != nil {
		return err
	}

	hookEnv, err := hooks.ParseEnvWithStdin(env)
	if err != nil {
		return err
	}

	// Remove worktree
	if err := git.RemoveWorktree(ctx, *targetWT, force); err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	// Run hooks
	hookCtx := hooks.Context{
		WorktreeDir: targetWT.Path,
		RepoDir:     targetRepoPath,
		Branch:      targetWT.Branch,
		Repo:        filepath.Base(targetRepoPath),
		Origin:      targetWT.RepoName,
		Trigger:     string(hooks.CommandPrune),
		Env:         hookEnv,
	}
	if hookCtx.Origin == "" {
		hookCtx.Origin = hookCtx.Repo
	}
	hooks.RunForEach(hookMatches, hookCtx, targetRepoPath)

	// Prune stale refs
	git.PruneWorktrees(ctx, targetRepoPath)

	l.Printf("Removed worktree: %s (%s)\n", branch, targetWT.Path)
	return nil
}

// getCacheDir returns the cache directory path
func getCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wt")
}

// refreshPRStatusForPrune fetches PR status for worktrees in parallel
func refreshPRStatusForPrune(ctx context.Context, worktrees []pruneWorktree, wtCache *cache.Cache, hosts map[string]string, forgeConfig *config.ForgeConfig, sp *ui.Spinner) {
	// Filter to worktrees that need PR status fetched
	var toFetch []pruneWorktree
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}
		// Skip already merged
		folderName := filepath.Base(wt.Path)
		if pr := wtCache.GetPRForBranch(folderName); pr != nil && pr.Fetched && pr.State == "MERGED" {
			continue
		}
		toFetch = append(toFetch, wt)
	}

	if len(toFetch) == 0 {
		return
	}

	sp.UpdateMessage(fmt.Sprintf("Fetching PR status (%d branches)...", len(toFetch)))

	var prMutex sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentPRFetches)
	var fetchedCount, failedCount int
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
				failedCount++
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

	wg.Wait()

	if failedCount > 0 {
		sp.UpdateMessage(fmt.Sprintf("Fetched: %d, Failed: %d", fetchedCount, failedCount))
	}
}

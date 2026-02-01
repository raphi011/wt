package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// maxConcurrentPRFetches limits parallel gh API calls to avoid rate limiting
const maxConcurrentPRFetches = 5

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
		GroupID: GroupCore,
		Args:    cobra.NoArgs,
		Long: `Remove worktrees with merged PRs.

Without flags, removes all worktrees with merged PRs in current repo.
Use --global to prune all registered repos.
Use --interactive to select worktrees to prune.`,
		Example: `  wt prune                         # Remove worktrees with merged PRs
  wt prune --global                # Prune all repos
  wt prune -d                      # Dry-run: preview without removing
  wt prune -i                      # Interactive mode
  wt prune -r myrepo               # Prune specific repo
  wt prune -l backend              # Prune repos with label
  wt prune --branch feature -f     # Remove specific branch worktree
  wt prune --branch myrepo:feat -f # Remove branch in specific repo`,
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

			// Remove worktrees using shared helper
			// For auto-prune (merged PRs), force=true is implicit
			removed, failed := pruneWorktrees(ctx, toRemove, true, dryRun, hookNames, noHook, env, prCache)

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
	cmd.RegisterFlagCompletionFunc("branch", completeWorktrees)

	return cmd
}

// runPruneTargetBranch handles removal of a specific branch worktree.
// Supports "repo:branch" format to target a specific repo's worktree.
func runPruneTargetBranch(ctx context.Context, repos []*registry.Repo, target string, force, dryRun bool, hookNames []string, noHook bool, env []string) error {
	l := log.FromContext(ctx)

	// Parse repo:branch format
	targetRepo, branch := parseBranchTarget(target)

	if !force {
		return fmt.Errorf("worktree %q requires -f/--force to remove", branch)
	}

	// Find the worktree
	var targetWT pruneWorktree

	for _, repo := range repos {
		// Skip repos that don't match if a repo was specified
		if targetRepo != "" && repo.Name != targetRepo {
			continue
		}

		wtInfos, err := git.ListWorktreesFromRepo(ctx, repo.Path)
		if err != nil {
			continue
		}
		for _, wti := range wtInfos {
			if wti.Branch == branch {
				targetWT = pruneWorktree{
					Path:     wti.Path,
					Branch:   wti.Branch,
					RepoName: repo.Name,
					RepoPath: repo.Path,
				}
				break
			}
		}
		if targetWT.Path != "" {
			break
		}
	}

	if targetWT.Path == "" {
		if targetRepo != "" {
			return fmt.Errorf("worktree for branch %q not found in repo %q", branch, targetRepo)
		}
		return fmt.Errorf("worktree for branch %q not found", branch)
	}

	if dryRun {
		l.Printf("Would remove worktree: %s (%s)\n", branch, targetWT.Path)
		return nil
	}

	// Use pruneWorktrees for consistent removal logic
	removed, failed := pruneWorktrees(ctx, []pruneWorktree{targetWT}, force, false, hookNames, noHook, env, nil)
	if len(failed) > 0 {
		return fmt.Errorf("failed to remove worktree: %s", targetWT.Path)
	}
	if len(removed) > 0 {
		l.Printf("Removed worktree: %s (%s)\n", branch, targetWT.Path)
	}

	return nil
}

// refreshPRStatusForPrune fetches PR status for worktrees in parallel
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
			prCache.Set(folderName, convertForgePR(pr))
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

// convertForgePR converts forge.PRInfo to prcache.PRInfo
func convertForgePR(pr *forge.PRInfo) *prcache.PRInfo {
	return &prcache.PRInfo{
		Number:       pr.Number,
		State:        pr.State,
		IsDraft:      pr.IsDraft,
		URL:          pr.URL,
		Author:       pr.Author,
		CommentCount: pr.CommentCount,
		HasReviews:   pr.HasReviews,
		IsApproved:   pr.IsApproved,
		CachedAt:     pr.CachedAt,
		Fetched:      pr.Fetched,
	}
}

// parseBranchTarget parses "repo:branch" or "branch" format.
// Returns (repo, branch) where repo is empty if not specified.
// Uses colon separator to avoid ambiguity with branches containing "/".
func parseBranchTarget(target string) (repo, branch string) {
	if idx := strings.Index(target, ":"); idx > 0 {
		return target[:idx], target[idx+1:]
	}
	return "", target // no repo specified
}

// pruneWorktrees removes the given worktrees and runs hooks.
// Returns slices of successfully removed and failed worktrees.
func pruneWorktrees(ctx context.Context, toRemove []pruneWorktree, force, dryRun bool,
	hookNames []string, noHook bool, env []string, prCache *prcache.Cache) (removed, failed []pruneWorktree) {

	if len(toRemove) == 0 {
		return nil, nil
	}

	if dryRun {
		return toRemove, nil
	}

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

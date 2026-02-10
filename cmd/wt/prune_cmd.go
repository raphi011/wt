package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/static"
	"github.com/raphi011/wt/internal/ui/styles"
	"github.com/raphi011/wt/internal/ui/wizard/flows"
)

func newPruneCmd() *cobra.Command {
	var (
		dryRun           bool
		force            bool
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

			repos = filterOrphanedRepos(l, repos)

			if len(repos) == 0 {
				out.Println("No repos found")
				return nil
			}

			l.Debug("pruning worktrees", "repos", len(repos), "dryRun", dryRun)

			allWorktrees, warnings := git.LoadWorktreesForRepos(ctx, reposToRefs(repos))
			for _, w := range warnings {
				l.Printf("Warning: %s: %v\n", w.RepoName, w.Err)
			}

			if len(allWorktrees) == 0 {
				out.Println("No worktrees found")
				return nil
			}

			// Load PR cache
			prCache := prcache.Load()

			// Reset cache if requested
			if resetCache {
				prCache.Reset()
				l.Println("Cache reset: PR info cleared")
			}

			// Refresh PR status if requested
			if refresh {
				if failed := refreshPRs(ctx, allWorktrees, prCache, cfg.Hosts, &cfg.Forge); len(failed) > 0 {
					l.Printf("Warning: failed to fetch PR status for: %v\n", failed)
				}
			}

			populatePRFields(allWorktrees, prCache)

			// Sort by repo name
			slices.SortFunc(allWorktrees, func(a, b git.Worktree) int {
				return strings.Compare(a.RepoName, b.RepoName)
			})

			// Determine which to remove and why
			var toRemove []git.Worktree
			var toSkip []git.Worktree

			for _, wt := range allWorktrees {
				// Only auto-prune worktrees with merged PRs
				if wt.PRState == forge.PRStateMerged {
					toRemove = append(toRemove, wt)
				} else {
					toSkip = append(toSkip, wt)
				}
			}

			// Handle interactive mode
			if interactive {
				wizardInfos := make([]flows.PruneWorktreeInfo, 0, len(allWorktrees))
				for i, wt := range allWorktrees {
					isPrunable := wt.PRState == forge.PRStateMerged
					wizardInfos = append(wizardInfos, flows.PruneWorktreeInfo{
						ID:         i + 1, // Use index as ID
						RepoName:   wt.RepoName,
						Branch:     wt.Branch,
						Reason:     styles.FormatPRState(wt.PRState, wt.PRDraft),
						IsPrunable: isPrunable,
					})
				}

				pruneOpts, err := flows.PruneInteractive(flows.PruneWizardParams{
					Worktrees: wizardInfos,
				})
				if err != nil {
					return fmt.Errorf("interactive mode error: %w", err)
				}
				if pruneOpts.Cancelled {
					return nil
				}

				if len(pruneOpts.SelectedIDs) == 0 {
					out.Println("No worktrees selected for removal")
					return nil
				}

				// Rebuild based on selection (IDs are 1-indexed)
				selectedSet := make(map[int]bool)
				for _, id := range pruneOpts.SelectedIDs {
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

			// Print summary
			if dryRun {
				out.Printf("Would remove %d worktree(s), skip %d\n", len(removed), len(toSkip)+len(failed))
			} else {
				out.Printf("Removed %d worktree(s), skipped %d\n", len(removed), len(toSkip)+len(failed))
			}

			// Display results table
			if len(removed) > 0 {
				out.Println()
				var rows [][]string
				for _, wt := range removed {
					rows = append(rows, static.WorktreeTableRow(wt))
				}
				out.Print(static.RenderTable(static.WorktreeTableHeaders, rows))
			}

			// Save PR cache once at the end
			if err := prCache.SaveIfDirty(); err != nil {
				l.Printf("Warning: failed to save cache: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Preview without removing")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force remove (required for targeted prune)")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Prune all repos")
	cmd.Flags().BoolVarP(&refresh, "refresh-pr", "R", false, "Refresh PR status first")
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

	// Convert to git.Worktree
	var toRemove []git.Worktree
	for _, t := range wtTargets {
		toRemove = append(toRemove, git.Worktree{
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

// pruneWorktrees removes the given worktrees and runs hooks.
// Returns slices of successfully removed and failed worktrees.
func pruneWorktrees(ctx context.Context, toRemove []git.Worktree, force, dryRun, deleteBranches bool,
	hookNames []string, noHook bool, env []string, prCache *prcache.Cache) (removed, failed []git.Worktree) {

	if len(toRemove) == 0 {
		return nil, nil
	}

	if dryRun {
		return toRemove, nil
	}

	l := log.FromContext(ctx)
	cfg := config.FromContext(ctx)

	// Load history for cleanup
	hist, err := history.Load(cfg.GetHistoryPath())
	if err != nil {
		l.Printf("Warning: failed to load history for prune cleanup: %v\n", err)
		hist = &history.History{}
	}
	historyChanged := false

	// Select hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, hookNames, noHook, hooks.CommandPrune)
	if err != nil {
		// Log error but continue - don't fail the whole prune
		l.Printf("Warning: failed to select hooks: %v\n", err)
	}

	hookEnv, err := hooks.ParseEnvWithStdin(env)
	if err != nil {
		l.Printf("Warning: failed to parse hook env: %v\n", err)
	}

	for _, wt := range toRemove {
		if err := git.RemoveWorktree(ctx, wt, force); err != nil {
			l.Printf("Warning: failed to remove %s: %v\n", wt.Path, err)
			failed = append(failed, wt)
			continue
		}

		// Remove from PR cache
		if prCache != nil {
			prCache.Delete(prcache.CacheKey(wt.RepoPath, wt.Branch))
		}

		// Remove from history
		if hist.RemoveByPath(wt.Path) {
			historyChanged = true
		}

		removed = append(removed, wt)

		// Delete local branch if enabled
		if deleteBranches {
			// Force delete if forge confirmed merge (handles squash merges),
			// safe delete (-d) otherwise.
			// Note: PRState is empty for targeted prune (no forge lookup),
			// so forceDelete is always false in that path — this is intentional.
			forceDelete := wt.PRState == forge.PRStateMerged
			if err := git.DeleteLocalBranch(ctx, wt.RepoPath, wt.Branch, forceDelete); err != nil {
				l.Printf("Warning: failed to delete branch %s: %v\n", wt.Branch, err)
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

	// Save history if any entries were removed
	if historyChanged {
		if err := hist.Save(cfg.GetHistoryPath()); err != nil {
			l.Printf("Warning: failed to save history after prune: %v\n", err)
		}
	}

	// Prune stale references
	processedRepos := make(map[string]bool)
	for _, wt := range removed {
		if !processedRepos[wt.RepoPath] {
			if err := git.PruneWorktrees(ctx, wt.RepoPath); err != nil {
				l.Printf("Warning: failed to prune stale references in %s: %v\n", wt.RepoPath, err)
			}
			processedRepos[wt.RepoPath] = true
		}
	}

	return removed, failed
}

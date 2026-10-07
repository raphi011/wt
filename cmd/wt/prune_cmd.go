package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
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
		hf               hookFlags
		interactive      bool
		deleteBranches   bool
		noDeleteBranches bool
		stale            bool
	)

	cmd := &cobra.Command{
		Use:     "prune [[scope:]branch...]",
		Short:   "Prune merged worktrees",
		Aliases: []string{"p"},
		GroupID: GroupCore,
		Long: `Remove worktrees with merged PRs.

Without arguments, removes all worktrees with merged PRs in current repo.
Use --stale to also prune worktrees older than stale_days (default 14).
Use --global to prune all registered repos.
Use --interactive to select worktrees to prune.

Target specific worktrees using [scope:]branch arguments where scope can be
a repo name or label. Worktrees with a merged PR can be pruned without -f.
Use -f to prune worktrees whose PR is not yet confirmed merged.
Use -R to fetch the PR status of the targets first.
--stale and --interactive cannot be combined with targets.

Worktrees with uncommitted changes (modified, staged or untracked files) are
never removed without -f.`,
		Example: `  wt prune                         # Remove worktrees with merged PRs
  wt prune --stale                 # Also prune stale worktrees
  wt prune --global                # Prune all repos
  wt prune -d                      # Dry-run: preview without removing
  wt prune -f                      # Also remove worktrees with uncommitted changes
  wt prune -i                      # Interactive mode
  wt prune feature                 # Remove merged feature worktree
  wt prune feature -R              # Refresh PR status, remove if merged
  wt prune feature -f              # Remove unmerged feature worktree
  wt prune feature -f -g           # Remove feature worktree (all repos)
  wt prune myrepo:feature -f       # Remove specific unmerged worktree
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
				if stale {
					return fmt.Errorf("--stale cannot be combined with branch targets")
				}
				if interactive {
					return fmt.Errorf("--interactive cannot be combined with branch targets")
				}
				deleteBranchesExplicit := cmd.Flags().Changed("delete-branches") || cmd.Flags().Changed("no-delete-branches")
				// Determine if we should delete local branches
				shouldDeleteBranches := cfg.Prune.DeleteLocalBranches
				if cmd.Flags().Changed("delete-branches") {
					shouldDeleteBranches = deleteBranches
				} else if cmd.Flags().Changed("no-delete-branches") {
					shouldDeleteBranches = false
				}
				return runPruneTargets(ctx, reg, args, global, force, dryRun, pruneOpts{
					DeleteBranches:         shouldDeleteBranches,
					DeleteBranchesExplicit: deleteBranchesExplicit,
					Hooks:                  hf,
					RefreshPR:              refresh,
					ResetCache:             resetCache,
				})
			}

			// Determine target repos for auto-prune
			// Current repo, or all repos outside a repo or with -g
			repos, _, err := unscopedRepos(ctx, reg, global)
			if err != nil {
				return err
			}

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
			prCache, err := loadPRCache(cfg)
			if err != nil {
				return err
			}

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

			if stale && cfg.Prune.StaleDays <= 0 {
				l.Println("Warning: --stale has no effect because stale_days is 0 (disabled)")
			}

			// The stale rule only applies with --stale
			staleDays := 0
			if stale {
				staleDays = cfg.Prune.StaleDays
			}
			now := time.Now()
			reasons := make([]removalReason, len(allWorktrees))
			for i, wt := range allWorktrees {
				reasons[i] = removalReasonFor(wt, staleDays, now)
				if reasons[i] != removalNone {
					toRemove = append(toRemove, wt)
				} else {
					toSkip = append(toSkip, wt)
				}
			}

			// Handle interactive mode
			if interactive {
				wizardInfos := make([]flows.PruneWorktreeInfo, 0, len(allWorktrees))
				for i, wt := range allWorktrees {
					reason := styles.FormatPRState(wt.PRState, wt.PRDraft)
					if reasons[i] == removalStale {
						reason = styles.FormatStaleReason(wt.CommitAge)
					}
					wizardInfos = append(wizardInfos, flows.PruneWorktreeInfo{
						ID:         i + 1, // Use index as ID
						RepoName:   wt.RepoName,
						Branch:     wt.Branch,
						Reason:     reason,
						IsPrunable: reasons[i] != removalNone,
						IsStale:    reasons[i] == removalStale,
						IsDirty:    isWorktreeDirty(ctx, wt),
						Worktree:   wt,
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

			// Worktrees with uncommitted changes are only removed with --force
			var dirty []git.Worktree
			if !force {
				toRemove, dirty = splitDirtyWorktrees(ctx, toRemove)
				toSkip = append(toSkip, dirty...)
			}

			deleteBranchesExplicit := cmd.Flags().Changed("delete-branches") || cmd.Flags().Changed("no-delete-branches")
			// Determine if we should delete local branches (default from global config)
			shouldDeleteBranches := cfg.Prune.DeleteLocalBranches
			if cmd.Flags().Changed("delete-branches") {
				shouldDeleteBranches = deleteBranches
			} else if cmd.Flags().Changed("no-delete-branches") {
				shouldDeleteBranches = false
			}

			removed, failed := pruneWorktrees(ctx, toRemove, pruneOpts{
				DryRun:                 dryRun,
				DeleteBranches:         shouldDeleteBranches,
				DeleteBranchesExplicit: deleteBranchesExplicit,
				Hooks:                  hf,
				PRCache:                prCache,
			})

			// Print summary: a dry-run preview is data (stdout), the result of
			// a real run is a diagnostic (stderr)
			report := l.Printf
			if dryRun {
				report = out.Printf
				report("Would remove %d worktree(s), skip %d\n", len(removed), len(toSkip)+len(failed))
			} else {
				report("Removed %d worktree(s), skipped %d\n", len(removed), len(toSkip)+len(failed))
			}

			if len(dirty) > 0 {
				report("Skipped (uncommitted changes, use -f to remove):\n")
				for _, wt := range dirty {
					report("  %s:%s (%s)\n", wt.RepoName, wt.Branch, wt.Path)
				}
			}

			// Display results table
			if len(removed) > 0 {
				out.Println()
				var rows [][]string
				hyperlinks := out.HyperlinksSupported()
				for _, wt := range removed {
					rows = append(rows, static.WorktreeTableRow(wt, cfg.Prune.StaleDays, hyperlinks))
				}
				out.Print(static.RenderTableAtWidth(static.WorktreeTableHeaders, rows, out.TerminalWidth()))
			}

			// Save PR cache once at the end
			if err := prCache.Save(); err != nil {
				l.Printf("Warning: failed to save cache: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Preview without removing")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force remove unmerged worktrees and worktrees with uncommitted changes")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Prune all repos")
	cmd.Flags().BoolVarP(&refresh, "refresh-pr", "R", false, "Refresh PR status first")
	cmd.Flags().BoolVar(&resetCache, "reset-cache", false, "Clear all cached data")
	registerHookFlags(cmd, &hf)
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	cmd.Flags().BoolVar(&stale, "stale", false, "Also prune stale worktrees (older than stale_days)")
	cmd.Flags().BoolVarP(&deleteBranches, "delete-branches", "b", false, "Delete local branches after removal")
	cmd.Flags().BoolVar(&noDeleteBranches, "no-delete-branches", false, "Keep local branches (overrides config)")

	cmd.MarkFlagsMutuallyExclusive("delete-branches", "no-delete-branches")

	return cmd
}

// pruneOpts holds options for pruneWorktrees to avoid a long positional parameter list.
type pruneOpts struct {
	DryRun                 bool
	DeleteBranches         bool
	DeleteBranchesExplicit bool // true when --delete-branches or --no-delete-branches was passed
	Hooks                  hookFlags
	PRCache                *prcache.Cache
	RefreshPR              bool // targeted prune: fetch PR status before checking prunability
	ResetCache             bool // targeted prune: clear the PR cache first
}

// removalReason says why a worktree may be removed without --force.
type removalReason int

const (
	removalNone   removalReason = iota // PR not merged, not stale
	removalMerged                      // PR merged, confirmed by the forge
	removalStale                       // last commit older than staleDays
)

// removalReasonFor decides whether wt may be removed without --force.
// The stale rule is off with staleDays <= 0. Worktrees with an open PR are
// never stale: active work is always protected.
func removalReasonFor(wt git.Worktree, staleDays int, now time.Time) removalReason {
	if wt.PRState == forge.PRStateMerged {
		return removalMerged
	}
	if staleDays <= 0 || wt.CommitDate.IsZero() || wt.PRState == forge.PRStateOpen {
		return removalNone
	}
	if now.Sub(wt.CommitDate) > time.Duration(staleDays)*24*time.Hour {
		return removalStale
	}
	return removalNone
}

// runPruneTargets handles removal of specific worktrees by [scope:]branch args.
// When global is false, unscoped targets are scoped to the current repo if inside one.
// Force is only required when at least one target is not prunable (not merged).
func runPruneTargets(ctx context.Context, reg *registry.Registry, targets []string, global, force, dryRun bool, opts pruneOpts) error {
	l := log.FromContext(ctx)
	out := output.FromContext(ctx)

	// Resolve all targets
	wtTargets, err := resolveWorktreeTargets(ctx, reg, targets, targetOpts{Global: global, Multi: true})
	if err != nil {
		return err
	}

	// Load full worktree data for the targets (origin and upstream are needed to refresh PR status)
	targetPaths := make(map[string]bool, len(wtTargets))
	seenRepos := make(map[string]bool)
	var refs []git.RepoRef
	for _, t := range wtTargets {
		targetPaths[t.Path] = true
		if !seenRepos[t.RepoPath] {
			seenRepos[t.RepoPath] = true
			refs = append(refs, git.RepoRef{Name: t.RepoName, Path: t.RepoPath})
		}
	}
	loaded, warnings := git.LoadWorktreesForRepos(ctx, refs)
	if len(warnings) > 0 {
		return fmt.Errorf("%s: %w", warnings[0].RepoName, warnings[0].Err)
	}
	var toRemove []git.Worktree
	for _, wt := range loaded {
		if targetPaths[wt.Path] {
			toRemove = append(toRemove, wt)
		}
	}

	// Enrich with merge info to determine if force is needed
	cfg := config.FromContext(ctx)
	prCache, err := loadPRCache(cfg)
	if err != nil {
		return err
	}

	if opts.ResetCache {
		prCache.Reset()
		l.Println("Cache reset: PR info cleared")
	}

	if opts.RefreshPR {
		if failed := refreshPRs(ctx, toRemove, prCache, cfg.Hosts, &cfg.Forge); len(failed) > 0 {
			l.Printf("Warning: failed to fetch PR status for: %v\n", failed)
		}
	}

	// Save now: the checks below may return before any worktree is removed
	if opts.ResetCache || opts.RefreshPR {
		if err := prCache.Save(); err != nil {
			l.Printf("Warning: failed to save cache: %v\n", err)
		}
	}

	// Enrich with PR state from cache
	populatePRFields(toRemove, prCache)

	// Require force only when at least one target is not prunable
	if !force {
		now := time.Now()
		var unprunable []string
		for _, wt := range toRemove {
			if removalReasonFor(wt, 0, now) == removalNone {
				unprunable = append(unprunable, wt.RepoName+":"+wt.Branch)
			}
		}
		if len(unprunable) > 0 {
			hint := "use -f to force removal"
			if !opts.RefreshPR {
				hint = "run with -R/--refresh-pr to fetch latest PR status, or " + hint
			}
			return fmt.Errorf("cannot prune unmerged worktrees without -f/--force: %s\nHint: %s", strings.Join(unprunable, ", "), hint)
		}

		if _, dirty := splitDirtyWorktrees(ctx, toRemove); len(dirty) > 0 {
			names := make([]string, 0, len(dirty))
			for _, wt := range dirty {
				names = append(names, wt.RepoName+":"+wt.Branch)
			}
			return fmt.Errorf("cannot prune worktrees with uncommitted changes without -f/--force: %s", strings.Join(names, ", "))
		}
	}

	if dryRun {
		out.Println("Would remove:")
		for _, wt := range toRemove {
			out.Printf("  %s:%s (%s)\n", wt.RepoName, wt.Branch, wt.Path)
		}
		return nil
	}

	opts.PRCache = prCache
	removed, failed := pruneWorktrees(ctx, toRemove, opts)

	for _, wt := range removed {
		l.Printf("Removed worktree: %s:%s (%s)\n", wt.RepoName, wt.Branch, wt.Path)
	}
	for _, wt := range failed {
		l.Printf("Failed to remove: %s:%s (%s)\n", wt.RepoName, wt.Branch, wt.Path)
	}

	// Save PR cache (entries may have been deleted during removal)
	if err := prCache.Save(); err != nil {
		l.Printf("Warning: failed to save cache: %v\n", err)
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to remove %d worktree(s)", len(failed))
	}

	return nil
}

// isWorktreeDirty reports whether a worktree has uncommitted changes.
// A worktree whose state cannot be determined counts as dirty.
func isWorktreeDirty(ctx context.Context, wt git.Worktree) bool {
	// A worktree whose directory is gone has nothing left to lose
	if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
		return false
	}
	dirty, err := git.HasUncommittedChanges(ctx, wt.Path)
	if err != nil {
		log.FromContext(ctx).Printf("Warning: cannot check %s for uncommitted changes: %v\n", wt.Path, err)
		return true
	}
	return dirty
}

// splitDirtyWorktrees separates worktrees with uncommitted changes from clean ones.
func splitDirtyWorktrees(ctx context.Context, worktrees []git.Worktree) (clean, dirty []git.Worktree) {
	for _, wt := range worktrees {
		if isWorktreeDirty(ctx, wt) {
			dirty = append(dirty, wt)
		} else {
			clean = append(clean, wt)
		}
	}
	return clean, dirty
}

// pruneWorktrees removes the given worktrees and runs hooks.
// Returns slices of successfully removed and failed worktrees.
func pruneWorktrees(ctx context.Context, toRemove []git.Worktree, opts pruneOpts) (removed, failed []git.Worktree) {
	if len(toRemove) == 0 {
		return nil, nil
	}

	if opts.DryRun {
		return toRemove, nil
	}

	l := log.FromContext(ctx)

	if err := opts.Hooks.parseArgs(); err != nil {
		l.Printf("Warning: failed to parse hook env, skipping hooks: %v\n", err)
		opts.Hooks.NoHook = true
	}

	for _, wt := range toRemove {
		// Resolve per-repo config for hooks and delete_local_branches
		effCfg := resolveEffectiveConfig(ctx, wt.RepoPath)

		hp, err := buildHookParams(effCfg, registry.Repo{Name: wt.RepoName, Path: wt.RepoPath}, wt.Path, wt.Branch, hooks.CommandPrune, "", opts.Hooks)
		if err != nil {
			l.Printf("Skipping %s: %v\n", wt.Branch, err)
			failed = append(failed, wt)
			continue
		}
		// The worktree is gone by the time after-hooks run
		hp.AfterWorkDir = wt.RepoPath

		wtRemoved := false
		err = withHooks(ctx, hp, func() error {
			// Delete local branch if enabled (per-repo config unless CLI flag was explicit)
			shouldDelete := opts.DeleteBranches
			if !opts.DeleteBranchesExplicit {
				shouldDelete = effCfg.Prune.DeleteLocalBranches
			}
			// Force: the caller checked for uncommitted changes, or the user passed --force
			err := removeWorktree(ctx, wt, teardownOpts{Force: true, DeleteBranch: shouldDelete, PRCache: opts.PRCache})
			if err != nil {
				return fmt.Errorf("failed to remove %s: %w", wt.Path, err)
			}
			wtRemoved = true
			removed = append(removed, wt)
			return nil
		})
		switch {
		case err == nil:
		case wtRemoved:
			// Removal succeeded; only after-hook selection can still fail
			l.Printf("Warning: failed to select hooks for %s: %v\n", wt.RepoName, err)
		default:
			l.Printf("Skipping %s: %v\n", wt.Branch, err)
			failed = append(failed, wt)
		}
	}

	forgetWorktrees(ctx, removed)

	return removed, failed
}

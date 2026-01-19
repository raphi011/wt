package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/ui"
)

func runTidy(cmd *TidyCmd, cfg config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Detect forge per-worktree (optional - PR status is a nice-to-have)
	// We'll check availability when we actually need it

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	// Expand path
	scanPath, err := expandPath(dir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
	scanPath, err = filepath.Abs(scanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	if cmd.DryRun {
		fmt.Printf("Cleaning worktrees in %s (dry run)\n", scanPath)
	} else {
		fmt.Printf("Cleaning worktrees in %s\n", scanPath)
	}

	// Start spinner
	sp := ui.NewSpinner("Scanning worktrees...")
	sp.Start()

	// List worktrees
	worktrees, err := git.ListWorktrees(scanPath)
	if err != nil {
		sp.Stop()
		return err
	}

	if len(worktrees) == 0 {
		sp.Stop()
		fmt.Println("No worktrees found")
		return nil
	}

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

	// PR status handling - load from cache only (run 'wt pr list' to fetch)
	prCache := make(forge.PRCache)
	prMap := make(map[string]*forge.PRInfo)
	prUnknown := make(map[string]bool)

	// Load PR cache
	prCache, err = forge.LoadPRCache(scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load PR cache: %v\n", err)
		prCache = make(forge.PRCache)
	}

	// Build PR map from cache for display, track unknown branches
	for _, wt := range worktrees {
		if originCache, ok := prCache[wt.OriginURL]; ok {
			if pr, ok := originCache[wt.Branch]; ok && pr != nil && pr.Fetched {
				prMap[wt.Branch] = pr
				continue
			}
		}
		// Not in cache or not fetched - mark as unknown
		prUnknown[wt.Branch] = true
	}

	// Update merge status for worktrees based on PR state
	for i := range worktrees {
		if pr, ok := prMap[worktrees[i].Branch]; ok && pr != nil {
			if pr.State == "MERGED" {
				worktrees[i].IsMerged = true
			}
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

	// Display table with cleaned worktrees marked
	fmt.Print(ui.FormatWorktreesTable(worktrees, prMap, prUnknown, toRemoveMap, cmd.DryRun))

	// Select hooks for tidy (before removing, so we can report errors early)
	// alreadyExists=false since tidy hooks always run for removed worktrees
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, false, hooks.CommandTidy)
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

			// Run tidy hooks for this worktree
			if len(hookMatches) > 0 {
				ctx := hooks.Context{
					Path:     wt.Path,
					Branch:   wt.Branch,
					Repo:     wt.RepoName,
					Folder:   filepath.Base(wt.MainRepo),
					MainRepo: wt.MainRepo,
					Trigger:  string(hooks.CommandTidy),
				}

				for _, match := range hookMatches {
					fmt.Printf("Running hook '%s' for %s...\n", match.Name, wt.Branch)
					// Use RunWithDir with main repo since worktree path is deleted
					if err := hooks.RunWithDir(match.Hook, ctx, wt.MainRepo); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: hook %q failed for %s: %v\n", match.Name, wt.Branch, err)
					} else if match.Hook.Description != "" {
						fmt.Printf("  ✓ %s\n", match.Hook.Description)
					}
				}
			}
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

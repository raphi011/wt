package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/ui"
)

func (c *ListCmd) runList(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	out := output.FromContext(ctx).Writer()
	worktreeDir, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Load cache with lock
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		return err
	}
	defer unlock()

	// List worktrees (no dirty check needed for list)
	allWorktrees, err := git.ListWorktrees(ctx, worktreeDir, false)
	if err != nil {
		return err
	}

	// Convert ALL worktrees to WorktreeInfo for cache sync (before filtering)
	wtInfos := make([]cache.WorktreeInfo, len(allWorktrees))
	for i, wt := range allWorktrees {
		wtInfos[i] = cache.WorktreeInfo{
			Path:      wt.Path,
			RepoPath:  wt.MainRepo,
			Branch:    wt.Branch,
			OriginURL: wt.OriginURL,
		}
	}

	// Sync cache with ALL worktrees to avoid losing IDs
	pathToID := wtCache.SyncWorktrees(wtInfos)

	// Refresh PR status if requested
	if c.Refresh {
		sp := ui.NewSpinner("Fetching PR status...")
		sp.Start()
		refreshPRStatus(ctx, allWorktrees, wtCache, cfg, sp)
		sp.Stop()
	}

	// Resolve sort order: flag > config > "id"
	sortBy := c.Sort
	if sortBy == "" {
		sortBy = cfg.DefaultSort
	}
	if sortBy == "" {
		sortBy = "id"
	}

	// Filter worktrees based on flags
	worktrees := allWorktrees
	var currentRepo string
	hasRepoOrLabelFilter := len(c.Repository) > 0 || len(c.Label) > 0

	if hasRepoOrLabelFilter {
		// Filter by -r and/or -l flags (overrides current repo filter)
		repoPaths, errs := collectRepoPaths(ctx, c.Repository, c.Label, worktreeDir, cfg)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
		}
		if len(repoPaths) > 0 {
			var filtered []git.Worktree
			for _, wt := range allWorktrees {
				if repoPaths[wt.MainRepo] {
					filtered = append(filtered, wt)
				}
			}
			worktrees = filtered
		}
	} else if !c.Global {
		// Default behavior: filter by current repo if inside one
		currentRepo = git.GetCurrentRepoMainPathFrom(ctx, workDir)
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

	// Save updated cache
	if err := cache.Save(worktreeDir, wtCache); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
	}

	// Sort worktrees for JSON output
	sortWorktrees(worktrees, pathToID, sortBy)

	if c.JSON {
		// Build JSON output with IDs and PR info
		type prJSON struct {
			Number     int    `json:"number"`
			State      string `json:"state"`
			IsDraft    bool   `json:"is_draft,omitempty"`
			IsApproved bool   `json:"is_approved,omitempty"`
			URL        string `json:"url,omitempty"`
		}
		type worktreeJSON struct {
			ID          int     `json:"id"`
			Path        string  `json:"path"`
			Branch      string  `json:"branch"`
			MainRepo    string  `json:"main_repo,omitempty"`
			RepoName    string  `json:"repo_name,omitempty"`
			OriginURL   string  `json:"origin_url"`
			IsMerged    bool    `json:"is_merged"`
			CommitCount int     `json:"commit_count"`
			HasUpstream bool    `json:"has_upstream"`
			LastCommit  string  `json:"last_commit,omitempty"`
			Note        string  `json:"note,omitempty"`
			PR          *prJSON `json:"pr,omitempty"`
		}
		result := make([]worktreeJSON, 0, len(worktrees))
		for _, wt := range worktrees {
			wtJSON := worktreeJSON{
				ID:          pathToID[wt.Path],
				Path:        wt.Path,
				Branch:      wt.Branch,
				MainRepo:    wt.MainRepo,
				RepoName:    wt.RepoName,
				OriginURL:   wt.OriginURL,
				IsMerged:    wt.IsMerged,
				CommitCount: wt.CommitCount,
				HasUpstream: wt.HasUpstream,
				LastCommit:  wt.LastCommit,
				Note:        wt.Note,
			}
			// Add PR info if available
			folderName := filepath.Base(wt.Path)
			if pr := wtCache.GetPRForBranch(folderName); pr != nil && pr.Fetched && pr.Number > 0 {
				wtJSON.PR = &prJSON{
					Number:     pr.Number,
					State:      pr.State,
					IsDraft:    pr.IsDraft,
					IsApproved: pr.IsApproved,
					URL:        pr.URL,
				}
			}
			result = append(result, wtJSON)
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}

	if len(worktrees) == 0 {
		// No worktrees found - show appropriate message
		if currentRepo != "" && len(allWorktrees) > 0 {
			// We're in a repo but no worktrees for this repo (others exist)
			fmt.Fprintf(out, "No worktrees found for current repository\n")
			fmt.Fprintf(out, "Use --global to show all %d worktrees\n", len(allWorktrees))
		}
		return nil
	}

	// Format sort description
	sortDesc := sortBy
	if sortBy == "commit" {
		sortDesc = "recent commit"
	}

	if currentRepo != "" && len(allWorktrees) != len(worktrees) {
		fmt.Fprintf(out, "Listing worktrees for %s (%d of %d, sorted by %s). Use --global to show all\n\n", worktrees[0].RepoName, len(worktrees), len(allWorktrees), sortDesc)
	} else {
		fmt.Fprintf(out, "Listing worktrees in %s (%d, sorted by %s)\n\n", worktreeDir, len(worktrees), sortDesc)
	}

	// Update merge status for worktrees based on cached PR state
	for i := range worktrees {
		folderName := filepath.Base(worktrees[i].Path)
		pr := wtCache.GetPRForBranch(folderName)
		if pr != nil && pr.Fetched && pr.State == "MERGED" {
			worktrees[i].IsMerged = true
		}
	}

	// Sort worktrees
	sortWorktrees(worktrees, pathToID, sortBy)

	// Display table
	fmt.Fprint(out, ui.FormatListTable(ctx, worktrees, pathToID, wtCache))

	return nil
}

func sortWorktrees(wts []git.Worktree, pathToID map[string]int, sortBy string) {
	slices.SortFunc(wts, func(a, b git.Worktree) int {
		switch sortBy {
		case "id":
			return cmp.Compare(pathToID[a.Path], pathToID[b.Path])
		case "repo":
			return cmp.Compare(a.RepoName, b.RepoName)
		case "branch":
			return cmp.Compare(a.Branch, b.Branch)
		case "commit":
			// Sort by most recent commit first (newest first)
			return b.LastCommitTime.Compare(a.LastCommitTime)
		default:
			return cmp.Compare(pathToID[a.Path], pathToID[b.Path])
		}
	})
}

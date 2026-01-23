package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/ui"
)

func runList(cmd *ListCmd, cfg *config.Config) error {
	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	scanPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Acquire lock on cache
	lock := cache.NewFileLock(cache.LockPath(scanPath))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Unlock()

	// Load cache
	wtCache, err := cache.Load(scanPath)
	if err != nil {
		return fmt.Errorf("failed to load cache: %w", err)
	}

	// List worktrees (no dirty check needed for list)
	allWorktrees, err := git.ListWorktrees(scanPath, false)
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
	if cmd.Refresh {
		sp := ui.NewSpinner("Fetching PR status...")
		sp.Start()
		refreshPRStatus(allWorktrees, wtCache, cfg, sp)
		sp.Stop()
	}

	// Resolve sort order: flag > config > "id"
	sortBy := cmd.Sort
	if sortBy == "" {
		sortBy = cfg.DefaultSort
	}
	if sortBy == "" {
		sortBy = "id"
	}

	// Filter worktrees based on flags
	worktrees := allWorktrees
	var currentRepo string
	hasRepoOrLabelFilter := len(cmd.Repository) > 0 || len(cmd.Label) > 0

	if hasRepoOrLabelFilter {
		// Filter by -r and/or -l flags (overrides current repo filter)
		repoPaths, errs := collectRepoPaths(cmd.Repository, cmd.Label, scanPath, cfg)
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
	} else if !cmd.Global {
		// Default behavior: filter by current repo if inside one
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

	// Save updated cache
	if err := cache.Save(scanPath, wtCache); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
	}

	// Sort worktrees for JSON output
	sortWorktrees(worktrees, pathToID, sortBy)

	if cmd.JSON {
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
		fmt.Println(string(data))
		return nil
	}

	if len(worktrees) == 0 {
		// No worktrees found - show appropriate message
		if currentRepo != "" && len(allWorktrees) > 0 {
			// We're in a repo but no worktrees for this repo (others exist)
			fmt.Printf("No worktrees found for current repository\n")
			fmt.Printf("Use --global to show all %d worktrees\n", len(allWorktrees))
		}
		return nil
	}

	if currentRepo != "" && len(allWorktrees) != len(worktrees) {
		fmt.Printf("Listing worktrees for %s (%d of %d). Use --global to show all\n\n", worktrees[0].RepoName, len(worktrees), len(allWorktrees))
	} else {
		fmt.Printf("Listing worktrees in %s (%d)\n\n", scanPath, len(worktrees))
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

	// Display table (no items marked for removal in list)
	toRemoveMap := make(map[string]bool)
	fmt.Print(ui.FormatWorktreesTable(worktrees, pathToID, wtCache, toRemoveMap, false))

	return nil
}

func sortWorktrees(wts []git.Worktree, pathToID map[string]int, sortBy string) {
	sort.Slice(wts, func(i, j int) bool {
		switch sortBy {
		case "id":
			return pathToID[wts[i].Path] < pathToID[wts[j].Path]
		case "repo":
			return wts[i].RepoName < wts[j].RepoName
		case "branch":
			return wts[i].Branch < wts[j].Branch
		case "commit":
			// Sort by most recent commit first (newest first)
			return wts[i].LastCommitTime.After(wts[j].LastCommitTime)
		default:
			return pathToID[wts[i].Path] < pathToID[wts[j].Path]
		}
	})
}

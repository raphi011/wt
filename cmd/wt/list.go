package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
)

func runList(cmd *ListCmd) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

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

	// List worktrees using optimized lightweight function
	allWorktrees, err := git.ListWorktreesLight(scanPath)
	if err != nil {
		return err
	}

	// Convert ALL worktrees to WorktreeInfo for cache sync (before filtering)
	wtInfos := make([]forge.WorktreeInfo, len(allWorktrees))
	for i, wt := range allWorktrees {
		wtInfos[i] = forge.WorktreeInfo{
			Path:      wt.Path,
			Branch:    wt.Branch,
			OriginURL: wt.OriginURL,
		}
	}

	// Sync cache with ALL worktrees to avoid losing IDs
	pathToID := wtCache.SyncWorktrees(wtInfos)

	// If in a git repo and not using --all, filter to only show worktrees from that repo
	worktrees := allWorktrees
	var currentRepo string
	if !cmd.All {
		currentRepo = git.GetCurrentRepoMainPath()
		if currentRepo != "" {
			var filtered []git.WorktreeLight
			for _, wt := range allWorktrees {
				if wt.MainRepo == currentRepo {
					filtered = append(filtered, wt)
				}
			}
			worktrees = filtered
		}
	}

	// Save updated cache
	if err := forge.SaveCache(scanPath, wtCache); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
	}

	if cmd.JSON {
		// Build JSON output with IDs and PR info
		type prJSON struct {
			Number     int    `json:"number"`
			State      string `json:"state"`
			IsApproved bool   `json:"is_approved,omitempty"`
			URL        string `json:"url,omitempty"`
		}
		type worktreeJSON struct {
			ID         int     `json:"id"`
			Path       string  `json:"path"`
			Branch     string  `json:"branch"`
			MainRepo   string  `json:"main_repo,omitempty"`
			RepoName   string  `json:"repo_name,omitempty"`
			OriginURL  string  `json:"origin_url"`
			CommitHash string  `json:"commit_hash,omitempty"`
			Note       string  `json:"note,omitempty"`
			PR         *prJSON `json:"pr,omitempty"`
		}
		result := make([]worktreeJSON, 0, len(worktrees))
		for _, wt := range worktrees {
			wtJSON := worktreeJSON{
				ID:         pathToID[wt.Path],
				Path:       wt.Path,
				Branch:     wt.Branch,
				MainRepo:   wt.MainRepo,
				RepoName:   wt.RepoName,
				OriginURL:  wt.OriginURL,
				CommitHash: wt.CommitHash,
				Note:       wt.Note,
			}
			// Add PR info if available
			if originCache, ok := wtCache.PRs[wt.OriginURL]; ok {
				if pr, ok := originCache[wt.Branch]; ok && pr != nil && pr.Fetched && pr.Number > 0 {
					wtJSON.PR = &prJSON{
						Number:     pr.Number,
						State:      pr.State,
						IsApproved: pr.IsApproved,
						URL:        pr.URL,
					}
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

	// Build list of items to display
	type displayItem struct {
		ID         int
		Path       string
		Origin     string
		Branch     string
		CommitHash string
		Note       string
		OriginURL  string
	}
	items := make([]displayItem, 0, len(worktrees))
	for _, wt := range worktrees {
		items = append(items, displayItem{
			ID:         pathToID[wt.Path],
			Path:       wt.Path,
			Origin:     wt.RepoName,
			Branch:     wt.Branch,
			CommitHash: wt.CommitHash,
			Note:       wt.Note,
			OriginURL:  wt.OriginURL,
		})
	}

	if len(items) == 0 {
		// No worktrees found - show appropriate message
		if currentRepo != "" && len(allWorktrees) > 0 {
			// We're in a repo but no worktrees for this repo (others exist)
			fmt.Printf("No worktrees found for current repository\n")
			fmt.Printf("Use --all to show all %d worktrees\n", len(allWorktrees))
		}
		return nil
	}

	// Find max widths for alignment
	maxID := 0
	maxOriginLen := 0
	maxBranchLen := 0
	maxPRLen := 0
	for _, item := range items {
		if item.ID > maxID {
			maxID = item.ID
		}
		if len(item.Origin) > maxOriginLen {
			maxOriginLen = len(item.Origin)
		}
		if len(item.Branch) > maxBranchLen {
			maxBranchLen = len(item.Branch)
		}
		// Calculate PR display width
		if originCache, ok := wtCache.PRs[item.OriginURL]; ok {
			if pr, ok := originCache[item.Branch]; ok && pr != nil && pr.Fetched && pr.Number > 0 {
				prLen := len(formatPRDisplay(pr))
				if prLen > maxPRLen {
					maxPRLen = prLen
				}
			}
		}
	}
	idWidth := len(fmt.Sprintf("%d", maxID))

	// Output: ID  origin  branch  commit  [PR]  [note]
	for _, item := range items {
		hash := item.CommitHash
		if hash == "" {
			hash = "???????"
		}

		// Get PR info
		var prDisplay string
		if originCache, ok := wtCache.PRs[item.OriginURL]; ok {
			if pr, ok := originCache[item.Branch]; ok && pr != nil && pr.Fetched && pr.Number > 0 {
				prDisplay = formatPRDisplay(pr)
			}
		}

		// Build output line
		if prDisplay != "" {
			if item.Note != "" {
				fmt.Printf("%*d  %-*s  %-*s  %s  %-*s  %s\n", idWidth, item.ID, maxOriginLen, item.Origin, maxBranchLen, item.Branch, hash, maxPRLen, prDisplay, item.Note)
			} else {
				fmt.Printf("%*d  %-*s  %-*s  %s  %s\n", idWidth, item.ID, maxOriginLen, item.Origin, maxBranchLen, item.Branch, hash, prDisplay)
			}
		} else {
			if item.Note != "" {
				fmt.Printf("%*d  %-*s  %-*s  %s  %s\n", idWidth, item.ID, maxOriginLen, item.Origin, maxBranchLen, item.Branch, hash, item.Note)
			} else {
				fmt.Printf("%*d  %-*s  %-*s  %s\n", idWidth, item.ID, maxOriginLen, item.Origin, maxBranchLen, item.Branch, hash)
			}
		}
	}

	return nil
}

// formatPRDisplay formats PR info for display: #123 open ✓
func formatPRDisplay(pr *forge.PRInfo) string {
	if pr == nil || pr.Number == 0 {
		return ""
	}

	// State text
	var state string
	switch pr.State {
	case "MERGED":
		state = "merged"
	case "OPEN":
		state = "open"
	case "CLOSED":
		state = "closed"
	default:
		state = pr.State
	}

	// Approval indicator
	approval := ""
	if pr.IsApproved {
		approval = " ✓"
	}

	return fmt.Sprintf("#%d %s%s", pr.Number, state, approval)
}

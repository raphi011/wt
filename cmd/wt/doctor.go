package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
)

// Issue represents a cache problem detected by doctor
type Issue struct {
	Key         string // cache key (folder name)
	Description string
	FixAction   string // what --fix would do
}

func runDoctor(cmd *DoctorCmd, _ *config.Config) error {
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

	// Handle reset
	if cmd.Reset {
		return runDoctorReset(scanPath)
	}

	// Load cache
	wtCache, err := cache.Load(scanPath)
	if err != nil {
		return fmt.Errorf("failed to load cache: %w", err)
	}

	fmt.Println("Checking cache...")

	// Check for issues
	issues := checkCacheIssues(wtCache, scanPath)

	if len(issues) == 0 {
		fmt.Println("\n✓ No issues found")
		return nil
	}

	fmt.Printf("\nFound %d issues:\n", len(issues))
	for _, issue := range issues {
		fmt.Printf("  ⚠ %s: %s\n", issue.Key, issue.Description)
	}

	if cmd.Fix {
		return fixCacheIssues(wtCache, issues, scanPath)
	}

	fmt.Println("\nRun 'wt doctor --fix' to repair.")
	return nil
}

// checkCacheIssues finds problems in the cache
func checkCacheIssues(wtCache *cache.Cache, scanPath string) []Issue {
	var issues []Issue

	for key, entry := range wtCache.Worktrees {
		// Skip already-removed entries
		if entry.RemovedAt != nil {
			continue
		}

		// Check 1: Old format key (contains "::")
		if strings.Contains(key, "::") {
			issues = append(issues, Issue{
				Key:         key,
				Description: "orphaned key (old format)",
				FixAction:   "remove",
			})
			continue
		}

		// Check 2: Stale entry (path doesn't exist)
		if entry.Path != "" {
			if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
				issues = append(issues, Issue{
					Key:         key,
					Description: fmt.Sprintf("worktree no longer exists at %s", entry.Path),
					FixAction:   "mark_removed",
				})
				continue
			}
		}

		// Check 3: Wrong path (folder exists elsewhere in worktree_dir)
		expectedPath := filepath.Join(scanPath, key)
		if entry.Path != "" && entry.Path != expectedPath {
			if _, err := os.Stat(expectedPath); err == nil {
				// Folder exists at expected location but path is different
				issues = append(issues, Issue{
					Key:         key,
					Description: fmt.Sprintf("path mismatch: cached %s, actual %s", entry.Path, expectedPath),
					FixAction:   "update_path",
				})
				continue
			}
		}

		// Check 4: Missing metadata (repo_path or branch)
		if entry.Path != "" && entry.RepoPath == "" {
			if _, err := os.Stat(entry.Path); err == nil {
				issues = append(issues, Issue{
					Key:         key,
					Description: "missing repo_path metadata",
					FixAction:   "update_metadata",
				})
				continue
			}
		}

		// Check 5: Broken git link (.git file missing/invalid)
		if entry.Path != "" {
			gitFile := filepath.Join(entry.Path, ".git")
			if info, err := os.Stat(gitFile); err != nil || info.IsDir() {
				issues = append(issues, Issue{
					Key:         key,
					Description: "broken git link",
					FixAction:   "mark_removed",
				})
				continue
			}
		}
	}

	// Check 6: Duplicate IDs
	idCounts := make(map[int][]string)
	for key, entry := range wtCache.Worktrees {
		if entry.RemovedAt == nil {
			idCounts[entry.ID] = append(idCounts[entry.ID], key)
		}
	}
	for id, keys := range idCounts {
		if len(keys) > 1 {
			for _, key := range keys[1:] { // first one keeps ID
				issues = append(issues, Issue{
					Key:         key,
					Description: fmt.Sprintf("duplicate ID %d", id),
					FixAction:   "reassign_id",
				})
			}
		}
	}

	return issues
}

// fixCacheIssues applies fixes for detected issues
func fixCacheIssues(wtCache *cache.Cache, issues []Issue, scanPath string) error {
	var fixed int

	for _, issue := range issues {
		switch issue.FixAction {
		case "remove":
			delete(wtCache.Worktrees, issue.Key)
			fmt.Printf("  ✓ Removed orphaned entry %q\n", issue.Key)
			fixed++

		case "mark_removed":
			cache.MarkRemovedByKey(wtCache, issue.Key)
			fmt.Printf("  ✓ Marked %q as removed\n", issue.Key)
			fixed++

		case "update_path":
			expectedPath := filepath.Join(scanPath, issue.Key)
			if entry, ok := wtCache.Worktrees[issue.Key]; ok {
				entry.Path = expectedPath
				fmt.Printf("  ✓ Updated path for %q\n", issue.Key)
				fixed++
			}

		case "update_metadata":
			if entry, ok := wtCache.Worktrees[issue.Key]; ok && entry.Path != "" {
				if repoPath, err := git.GetMainRepoPath(entry.Path); err == nil {
					entry.RepoPath = repoPath
				}
				if branch, err := git.GetCurrentBranch(entry.Path); err == nil {
					entry.Branch = branch
				}
				if entry.RepoPath != "" {
					entry.OriginURL, _ = git.GetOriginURL(entry.RepoPath)
				}
				fmt.Printf("  ✓ Updated metadata for %q\n", issue.Key)
				fixed++
			}

		case "reassign_id":
			if entry, ok := wtCache.Worktrees[issue.Key]; ok {
				entry.ID = wtCache.NextID
				wtCache.NextID++
				fmt.Printf("  ✓ Reassigned ID for %q\n", issue.Key)
				fixed++
			}
		}
	}

	// Save updated cache
	if err := cache.Save(scanPath, wtCache); err != nil {
		return fmt.Errorf("failed to save cache: %w", err)
	}

	fmt.Printf("\nFixed %d issues.\n", fixed)
	return nil
}

// runDoctorReset rebuilds the cache from scratch
func runDoctorReset(scanPath string) error {
	fmt.Println("Rebuilding cache from scratch...")

	// Scan for worktrees
	worktrees, err := git.ListWorktrees(scanPath, false)
	if err != nil {
		return fmt.Errorf("failed to scan worktrees: %w", err)
	}

	// Create fresh cache
	wtCache := &cache.Cache{
		Worktrees: make(map[string]*cache.WorktreeIDEntry),
		NextID:    1,
	}

	// Add each worktree
	for _, wt := range worktrees {
		info := cache.WorktreeInfo{
			Path:      wt.Path,
			RepoPath:  wt.MainRepo,
			Branch:    wt.Branch,
			OriginURL: wt.OriginURL,
		}
		wtCache.GetOrAssignID(info)
	}

	// Save
	if err := cache.Save(scanPath, wtCache); err != nil {
		return fmt.Errorf("failed to save cache: %w", err)
	}

	fmt.Printf("✓ Cache rebuilt with %d worktrees (IDs reset from 1)\n", len(worktrees))
	return nil
}

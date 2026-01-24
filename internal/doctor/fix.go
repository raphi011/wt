package doctor

import (
	"fmt"
	"path/filepath"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/git"
)

// fixAllIssues applies fixes for all detected issues.
func fixAllIssues(wtCache *cache.Cache, issues []Issue, scanPath string) error {
	var fixed int
	var failed int

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

		case "repair":
			// Try to repair using git worktree repair
			if issue.RepoPath != "" {
				if entry, ok := wtCache.Worktrees[issue.Key]; ok && entry.Path != "" {
					if err := git.RepairWorktree(issue.RepoPath, entry.Path); err != nil {
						fmt.Printf("  ✗ Failed to repair %q: %v\n", issue.Key, err)
						failed++
					} else {
						// Update cache if repo moved to new location
						if entry.RepoPath != issue.RepoPath {
							entry.RepoPath = issue.RepoPath
							// Update origin URL from new repo location
							if originURL, err := git.GetOriginURL(issue.RepoPath); err == nil {
								entry.OriginURL = originURL
							}
							fmt.Printf("  ✓ Repaired git links for %q (updated repo path)\n", issue.Key)
						} else {
							fmt.Printf("  ✓ Repaired git links for %q\n", issue.Key)
						}
						fixed++
					}
				}
			} else {
				fmt.Printf("  ✗ Cannot repair %q: missing repo path\n", issue.Key)
				failed++
			}

		case "prune":
			// Prune stale git worktree references
			if issue.RepoPath != "" {
				if err := git.PruneWorktrees(issue.RepoPath); err != nil {
					fmt.Printf("  ✗ Failed to prune %q: %v\n", issue.Key, err)
					failed++
				} else {
					fmt.Printf("  ✓ Pruned stale reference %q\n", issue.Key)
					fixed++
				}
			} else {
				fmt.Printf("  ✗ Cannot prune %q: missing repo path\n", issue.Key)
				failed++
			}

		case "add_to_cache":
			// Add untracked worktree to cache
			path := filepath.Join(scanPath, issue.Key)
			wtInfo, err := git.GetWorktreeInfo(path)
			if err != nil {
				fmt.Printf("  ✗ Failed to get info for %q: %v\n", issue.Key, err)
				failed++
				continue
			}

			info := cache.WorktreeInfo{
				Path:      path,
				RepoPath:  wtInfo.MainRepo,
				Branch:    wtInfo.Branch,
				OriginURL: wtInfo.OriginURL,
			}
			wtCache.GetOrAssignID(info)
			fmt.Printf("  ✓ Added %q to cache\n", issue.Key)
			fixed++

		case "repair_and_add":
			// Repair worktree link and add to cache
			path := filepath.Join(scanPath, issue.Key)
			if issue.RepoPath == "" {
				fmt.Printf("  ✗ Cannot repair %q: missing repo path\n", issue.Key)
				failed++
				continue
			}

			// Repair the worktree using the found repo
			if err := git.RepairWorktree(issue.RepoPath, path); err != nil {
				fmt.Printf("  ✗ Failed to repair %q: %v\n", issue.Key, err)
				failed++
				continue
			}

			// Now get worktree info and add to cache
			wtInfo, err := git.GetWorktreeInfo(path)
			if err != nil {
				fmt.Printf("  ✗ Repaired %q but failed to get info: %v\n", issue.Key, err)
				failed++
				continue
			}

			info := cache.WorktreeInfo{
				Path:      path,
				RepoPath:  wtInfo.MainRepo,
				Branch:    wtInfo.Branch,
				OriginURL: wtInfo.OriginURL,
			}
			wtCache.GetOrAssignID(info)
			fmt.Printf("  ✓ Repaired and added %q to cache\n", issue.Key)
			fixed++

		case "remove_orphan_dir":
			// Report unfixable orphan - don't delete automatically
			path := filepath.Join(scanPath, issue.Key)
			fmt.Printf("  ⚠ Cannot fix %q: repo not found. Delete manually: rm -rf %s\n", issue.Key, path)
			failed++
		}
	}

	// Save updated cache
	if err := cache.Save(scanPath, wtCache); err != nil {
		return fmt.Errorf("failed to save cache: %w", err)
	}

	if failed > 0 {
		fmt.Printf("\nFixed %d issues, %d failed.\n", fixed, failed)
	} else {
		fmt.Printf("\nFixed %d issues.\n", fixed)
	}
	return nil
}

// rebuildCache rebuilds the cache from scratch.
func rebuildCache(scanPath string) error {
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

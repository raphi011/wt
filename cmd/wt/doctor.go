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

// IssueCategory groups issues by type
type IssueCategory string

const (
	CategoryCache  IssueCategory = "cache"
	CategoryGit    IssueCategory = "git"
	CategoryOrphan IssueCategory = "orphan"
)

// Issue represents a problem detected by doctor
type Issue struct {
	Key         string        // cache key or path
	Description string
	FixAction   string // what --fix would do
	Category    IssueCategory
	RepoPath    string // for git repair operations
}

// IssueStats tracks counts by category
type IssueStats struct {
	CacheValid      int
	CacheIssues     int
	GitHealthy      int
	GitRepairable   int
	GitUnrepairable int
	GitPrunable     int
	OrphanUntracked int
	OrphanGhost     int
}

func runDoctor(cmd *DoctorCmd, cfg *config.Config) error {
	scanPath, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Handle reset (needs separate lock handling)
	if cmd.Reset {
		lock := cache.NewFileLock(cache.LockPath(scanPath))
		if err := lock.Lock(); err != nil {
			return fmt.Errorf("failed to acquire lock: %w", err)
		}
		defer lock.Unlock()
		return runDoctorReset(scanPath)
	}

	// Load cache with lock
	wtCache, unlock, err := cache.LoadWithLock(scanPath)
	if err != nil {
		return err
	}
	defer unlock()

	var stats IssueStats
	var allIssues []Issue

	// Category 1: Cache integrity checks
	fmt.Println("Checking cache integrity...")
	cacheIssues := checkCacheIssues(wtCache, scanPath)
	for i := range cacheIssues {
		cacheIssues[i].Category = CategoryCache
	}
	allIssues = append(allIssues, cacheIssues...)
	stats.CacheIssues = len(cacheIssues)
	stats.CacheValid = countActiveEntries(wtCache) - stats.CacheIssues

	// Category 2: Git link checks
	fmt.Println("Checking git links...")
	gitIssues := checkGitLinkIssues(wtCache, scanPath)
	for i := range gitIssues {
		gitIssues[i].Category = CategoryGit
	}
	allIssues = append(allIssues, gitIssues...)
	for _, issue := range gitIssues {
		switch issue.FixAction {
		case "repair":
			stats.GitRepairable++
		case "prune":
			stats.GitPrunable++
		default:
			stats.GitUnrepairable++
		}
	}
	stats.GitHealthy = countActiveEntries(wtCache) - len(gitIssues) - stats.CacheIssues

	// Category 3: Orphan checks
	fmt.Println("Checking for orphans...")
	orphanIssues := checkOrphanIssues(wtCache, scanPath)
	for i := range orphanIssues {
		orphanIssues[i].Category = CategoryOrphan
	}
	allIssues = append(allIssues, orphanIssues...)
	for _, issue := range orphanIssues {
		if issue.FixAction == "add_to_cache" {
			stats.OrphanUntracked++
		} else {
			stats.OrphanGhost++
		}
	}

	// Print summary
	printDoctorSummary(stats)

	if len(allIssues) == 0 {
		fmt.Println("\n✓ No issues found")
		return nil
	}

	fmt.Printf("\nFound %d issues:\n", len(allIssues))
	printIssuesByCategory(allIssues)

	if cmd.Fix {
		return fixAllIssues(wtCache, allIssues, scanPath)
	}

	fmt.Println("\nRun 'wt doctor --fix' to repair.")
	return nil
}

// countActiveEntries returns count of non-removed cache entries
func countActiveEntries(wtCache *cache.Cache) int {
	count := 0
	for _, entry := range wtCache.Worktrees {
		if entry.RemovedAt == nil {
			count++
		}
	}
	return count
}

// printDoctorSummary prints a categorized summary
func printDoctorSummary(stats IssueStats) {
	fmt.Println()

	// Cache summary
	if stats.CacheIssues == 0 {
		fmt.Printf("  ✓ %d cache entries valid\n", stats.CacheValid)
	} else {
		fmt.Printf("  ✓ %d cache entries valid\n", stats.CacheValid)
		fmt.Printf("  ⚠ %d cache issues\n", stats.CacheIssues)
	}

	// Git summary
	if stats.GitRepairable == 0 && stats.GitUnrepairable == 0 && stats.GitPrunable == 0 {
		if stats.GitHealthy > 0 {
			fmt.Printf("  ✓ %d worktrees healthy\n", stats.GitHealthy)
		}
	} else {
		if stats.GitHealthy > 0 {
			fmt.Printf("  ✓ %d worktrees healthy\n", stats.GitHealthy)
		}
		if stats.GitRepairable > 0 {
			fmt.Printf("  ⚠ %d repairable (broken links)\n", stats.GitRepairable)
		}
		if stats.GitPrunable > 0 {
			fmt.Printf("  ⚠ %d stale git references (prunable)\n", stats.GitPrunable)
		}
		if stats.GitUnrepairable > 0 {
			fmt.Printf("  ✗ %d unrepairable\n", stats.GitUnrepairable)
		}
	}

	// Orphan summary
	if stats.OrphanUntracked > 0 {
		fmt.Printf("  ⚠ %d untracked worktrees found\n", stats.OrphanUntracked)
	}
	if stats.OrphanGhost > 0 {
		fmt.Printf("  ⚠ %d ghost entries (in cache but not in git)\n", stats.OrphanGhost)
	}
}

// printIssuesByCategory groups and prints issues
func printIssuesByCategory(issues []Issue) {
	// Group by category
	byCategory := make(map[IssueCategory][]Issue)
	for _, issue := range issues {
		byCategory[issue.Category] = append(byCategory[issue.Category], issue)
	}

	categoryNames := map[IssueCategory]string{
		CategoryCache:  "Cache issues",
		CategoryGit:    "Git link issues",
		CategoryOrphan: "Orphan issues",
	}

	for _, cat := range []IssueCategory{CategoryCache, CategoryGit, CategoryOrphan} {
		catIssues := byCategory[cat]
		if len(catIssues) == 0 {
			continue
		}

		fmt.Printf("\n%s:\n", categoryNames[cat])
		for _, issue := range catIssues {
			fmt.Printf("  • %s: %s\n", issue.Key, issue.Description)
		}
	}
}

// checkCacheIssues finds problems in the cache (existing checks)
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
					Description: fmt.Sprintf("path no longer exists: %s", entry.Path),
					FixAction:   "mark_removed",
				})
				continue
			}
		}

		// Check 3: Wrong path (folder exists elsewhere in worktree_dir)
		expectedPath := filepath.Join(scanPath, key)
		if entry.Path != "" && entry.Path != expectedPath {
			if _, err := os.Stat(expectedPath); err == nil {
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
	}

	// Check 5: Duplicate IDs
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

// checkGitLinkIssues checks for broken git worktree links
func checkGitLinkIssues(wtCache *cache.Cache, _ string) []Issue {
	var issues []Issue

	// Track repos we've checked for prunable worktrees
	checkedRepos := make(map[string]bool)

	for key, entry := range wtCache.Worktrees {
		// Skip already-removed entries or entries without path
		if entry.RemovedAt != nil || entry.Path == "" {
			continue
		}

		// Skip if path doesn't exist (handled by cache check)
		if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
			continue
		}

		gitFile := filepath.Join(entry.Path, ".git")
		info, err := os.Stat(gitFile)

		// Check if .git file exists and is a file (not directory)
		if err != nil {
			issues = append(issues, Issue{
				Key:         key,
				Description: ".git file missing",
				FixAction:   "mark_removed",
				RepoPath:    entry.RepoPath,
			})
			continue
		}

		if info.IsDir() {
			// It's a main repo, not a worktree - shouldn't be in worktree cache
			issues = append(issues, Issue{
				Key:         key,
				Description: "not a worktree (has .git directory)",
				FixAction:   "mark_removed",
				RepoPath:    entry.RepoPath,
			})
			continue
		}

		// Check if links are valid
		if !git.IsWorktreeLinkValid(entry.Path) {
			if git.CanRepairWorktree(entry.Path) {
				issues = append(issues, Issue{
					Key:         key,
					Description: "broken bidirectional link (repairable)",
					FixAction:   "repair",
					RepoPath:    entry.RepoPath,
				})
			} else {
				issues = append(issues, Issue{
					Key:         key,
					Description: "broken git link (unrepairable)",
					FixAction:   "mark_removed",
					RepoPath:    entry.RepoPath,
				})
			}
		}

		// Track repo for prunable check
		if entry.RepoPath != "" {
			checkedRepos[entry.RepoPath] = true
		}
	}

	// Check for prunable worktrees in each repo (git's stale references)
	for repoPath := range checkedRepos {
		prunable, err := git.ListPrunableWorktrees(repoPath)
		if err != nil {
			continue
		}
		for _, wtRef := range prunable {
			issues = append(issues, Issue{
				Key:         wtRef,
				Description: fmt.Sprintf("stale git reference in %s", filepath.Base(repoPath)),
				FixAction:   "prune",
				RepoPath:    repoPath,
			})
		}
	}

	return issues
}

// checkOrphanIssues finds untracked worktrees and ghost entries
func checkOrphanIssues(wtCache *cache.Cache, scanPath string) []Issue {
	var issues []Issue

	// Build set of known worktree paths from cache
	cachedPaths := make(map[string]bool)
	for _, entry := range wtCache.Worktrees {
		if entry.RemovedAt == nil && entry.Path != "" {
			cachedPaths[entry.Path] = true
		}
	}

	// Scan directory for worktrees not in cache
	entries, err := os.ReadDir(scanPath)
	if err != nil {
		return issues
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(scanPath, entry.Name())

		// Check if it's a worktree
		if !git.IsWorktree(path) {
			continue
		}

		// Check if it's in cache
		if !cachedPaths[path] {
			issues = append(issues, Issue{
				Key:         entry.Name(),
				Description: "worktree not in cache",
				FixAction:   "add_to_cache",
			})
		}
	}

	// Check for ghost entries (in cache but git doesn't know about them)
	// This happens when worktree was removed outside of wt tool
	for key, entry := range wtCache.Worktrees {
		if entry.RemovedAt != nil || entry.Path == "" || entry.RepoPath == "" {
			continue
		}

		// Skip if path doesn't exist (already handled by cache check)
		if _, err := os.Stat(entry.Path); os.IsNotExist(err) {
			continue
		}

		// Check if git knows about this worktree
		worktrees, err := git.ListWorktreesFromRepo(entry.RepoPath)
		if err != nil {
			continue
		}

		found := false
		for _, wt := range worktrees {
			if wt.Path == entry.Path {
				found = true
				break
			}
		}

		if !found {
			issues = append(issues, Issue{
				Key:         key,
				Description: "ghost entry (git doesn't recognize worktree)",
				FixAction:   "mark_removed",
				Category:    CategoryOrphan,
			})
		}
	}

	return issues
}

// fixAllIssues applies fixes for all detected issues
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
						fmt.Printf("  ✓ Repaired git links for %q\n", issue.Key)
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

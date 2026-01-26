package doctor

import (
	"context"
	"fmt"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
)

// Run performs diagnostic checks on the worktree cache and optionally fixes issues.
func Run(ctx context.Context, cfg *config.Config, fix bool) error {
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

	var stats IssueStats
	var allIssues []Issue

	// Category 1: Cache integrity checks
	fmt.Println("Checking cache integrity...")
	cacheIssues := checkCacheIssues(wtCache, worktreeDir)
	for i := range cacheIssues {
		cacheIssues[i].Category = CategoryCache
	}
	allIssues = append(allIssues, cacheIssues...)
	stats.CacheIssues = len(cacheIssues)
	stats.CacheValid = countActiveEntries(wtCache) - stats.CacheIssues

	// Category 2: Git link checks
	fmt.Println("Checking git links...")
	gitIssues := checkGitLinkIssues(ctx, wtCache, worktreeDir, cfg)
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
	orphanIssues := checkOrphanIssues(ctx, wtCache, worktreeDir, cfg)
	for i := range orphanIssues {
		orphanIssues[i].Category = CategoryOrphan
	}
	allIssues = append(allIssues, orphanIssues...)
	for _, issue := range orphanIssues {
		switch issue.FixAction {
		case "add_to_cache", "repair_and_add", "remove_orphan_dir":
			// Worktrees found on disk but not properly tracked
			stats.OrphanUntracked++
		default:
			// Ghost entries: in cache but git doesn't recognize
			stats.OrphanGhost++
		}
	}

	// Print summary
	printSummary(stats)

	if len(allIssues) == 0 {
		fmt.Println("\n✓ No issues found")
		return nil
	}

	fmt.Printf("\nFound %d issues:\n", len(allIssues))
	printIssuesByCategory(allIssues)

	if fix {
		return fixAllIssues(ctx, wtCache, allIssues, worktreeDir)
	}

	fmt.Println("\nRun 'wt doctor --fix' to repair.")
	return nil
}

// Reset rebuilds the cache from scratch, discarding all existing entries.
func Reset(ctx context.Context, cfg *config.Config) error {
	worktreeDir, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	lock := cache.NewFileLock(cache.LockPath(worktreeDir))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Unlock()

	return rebuildCache(ctx, worktreeDir)
}

// printSummary prints a categorized summary.
func printSummary(stats IssueStats) {
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

// printIssuesByCategory groups and prints issues.
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

package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
)

// countActiveEntries returns count of non-removed cache entries.
func countActiveEntries(wtCache *cache.Cache) int {
	count := 0
	for _, entry := range wtCache.Worktrees {
		if entry.RemovedAt == nil {
			count++
		}
	}
	return count
}

// checkCacheIssues finds problems in the cache.
func checkCacheIssues(wtCache *cache.Cache, scanPath string) []Issue {
	var issues []Issue

	for key, entry := range wtCache.Worktrees {
		// Skip already-removed entries
		if entry.RemovedAt != nil {
			continue
		}

		// Check 1: Stale entry (path doesn't exist)
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

		// Check 2: Wrong path (folder exists elsewhere in worktree_dir)
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

		// Check 3: Missing metadata (repo_path or branch)
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

	// Check 4: Duplicate IDs
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

// checkGitLinkIssues checks for broken git worktree links.
func checkGitLinkIssues(ctx context.Context, wtCache *cache.Cache, scanPath string, cfg *config.Config) []Issue {
	var issues []Issue

	// Track repos we've checked for prunable worktrees
	checkedRepos := make(map[string]bool)

	// Build search directories for finding moved repos
	var searchDirs []string
	if cfg.RepoDir != "" {
		searchDirs = append(searchDirs, cfg.RepoDir)
	}
	if scanPath != cfg.RepoDir {
		searchDirs = append(searchDirs, scanPath)
	}

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
			// Determine which repo path to use for repair
			repoPath := entry.RepoPath
			repoMoved := false

			// Check if cached repo path still exists
			if repoPath != "" {
				if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
					// Repo moved - try to find it
					repoName := git.GetRepoNameFromWorktree(entry.Path)
					if repoName != "" {
						foundRepo := git.FindRepoInDirs(repoName, searchDirs...)
						if foundRepo != "" {
							repoPath = foundRepo
							repoMoved = true
						} else {
							repoPath = "" // Can't find repo
						}
					} else {
						repoPath = "" // Can't parse .git file
					}
				}
			}

			if repoPath != "" && git.CanRepairWorktree(entry.Path) {
				desc := "broken bidirectional link (repairable)"
				if repoMoved {
					desc = fmt.Sprintf("broken link, repo moved to %s (repairable)", repoPath)
				}
				issues = append(issues, Issue{
					Key:         key,
					Description: desc,
					FixAction:   "repair",
					RepoPath:    repoPath,
				})
			} else {
				desc := "broken git link (unrepairable)"
				if repoPath == "" {
					repoName := git.GetRepoNameFromWorktree(entry.Path)
					if repoName != "" {
						desc = fmt.Sprintf("broken git link (repo %q not found)", repoName)
					}
				}
				issues = append(issues, Issue{
					Key:         key,
					Description: desc,
					FixAction:   "mark_removed",
					RepoPath:    entry.RepoPath,
				})
			}
			continue
		}

		// Track repo for prunable check
		if entry.RepoPath != "" {
			checkedRepos[entry.RepoPath] = true
		}
	}

	// Check for prunable worktrees in each repo (git's stale references)
	for repoPath := range checkedRepos {
		prunable, err := git.ListPrunableWorktrees(ctx, repoPath)
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

// checkOrphanIssues finds untracked worktrees and ghost entries.
func checkOrphanIssues(ctx context.Context, wtCache *cache.Cache, scanPath string, cfg *config.Config) []Issue {
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

	// Build search directories for finding moved repos
	var searchDirs []string
	if cfg.RepoDir != "" {
		searchDirs = append(searchDirs, cfg.RepoDir)
	}
	if scanPath != cfg.RepoDir {
		searchDirs = append(searchDirs, scanPath)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(scanPath, entry.Name())

		// Check if it's a worktree (has .git file, not directory)
		if !git.IsWorktree(path) {
			continue
		}

		// Check if it's in cache - if so, skip (other checks handle cached entries)
		if cachedPaths[path] {
			continue
		}

		// Check if worktree link is valid
		if !git.IsWorktreeLinkValid(path) {
			// Broken link - try to find the moved repo
			repoName := git.GetRepoNameFromWorktree(path)
			if repoName == "" {
				issues = append(issues, Issue{
					Key:         entry.Name(),
					Description: "orphan worktree (cannot parse .git file)",
					FixAction:   "remove_orphan_dir",
				})
				continue
			}

			foundRepo := git.FindRepoInDirs(repoName, searchDirs...)
			if foundRepo != "" {
				issues = append(issues, Issue{
					Key:         entry.Name(),
					Description: fmt.Sprintf("broken link (repo found at %s)", foundRepo),
					FixAction:   "repair_and_add",
					RepoPath:    foundRepo,
				})
			} else {
				issues = append(issues, Issue{
					Key:         entry.Name(),
					Description: fmt.Sprintf("orphan worktree (repo %q not found)", repoName),
					FixAction:   "remove_orphan_dir",
				})
			}
			continue
		}

		// Valid worktree not in cache
		issues = append(issues, Issue{
			Key:         entry.Name(),
			Description: "worktree not in cache",
			FixAction:   "add_to_cache",
		})
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
		worktrees, err := git.ListWorktreesFromRepo(ctx, entry.RepoPath)
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

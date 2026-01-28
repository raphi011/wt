//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/doctor"
	"github.com/raphi011/wt/internal/git"
)

// TestDoctor_NoIssues verifies that doctor reports no issues for healthy setup.
//
// Scenario: User runs `wt doctor` with valid repo and worktree
// Expected: No errors, doctor completes successfully
func TestDoctor_NoIssues(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache properly
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor - should find no issues
	err := doctor.Run(context.Background(), cfg, false)
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
}

// TestDoctor_StaleEntry verifies that doctor detects and fixes stale cache entries
// (worktrees that no longer exist on disk).
//
// Scenario: User runs `wt doctor --fix` after deleting worktree manually
// Expected: Doctor marks stale entry as removed
func TestDoctor_StaleEntry(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	// Remove worktree directory manually (simulating deletion outside wt)
	if err := os.RemoveAll(worktreePath); err != nil {
		t.Fatalf("failed to remove worktree: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor without fix - should detect stale entry
	err := doctor.Run(context.Background(), cfg, false)
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}

	// Run doctor with fix - should mark entry as removed
	err = doctor.Run(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}

	// Verify entry is marked as removed
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	defer unlock()

	key := "myrepo-feature"
	entry, ok := wtCache.Worktrees[key]
	if !ok {
		t.Fatalf("entry %q not found in cache", key)
	}
	if entry.RemovedAt == nil {
		t.Errorf("entry %q should be marked as removed", key)
	}
}

// TestDoctor_DuplicateIDs verifies that doctor fixes duplicate worktree IDs
// by reassigning new unique IDs.
//
// Scenario: User runs `wt doctor --fix` with cache containing duplicate IDs
// Expected: IDs are made unique
func TestDoctor_DuplicateIDs(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with two worktrees
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	wt1 := filepath.Join(worktreeDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")
	wt2 := filepath.Join(worktreeDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	// Create cache manually with duplicate IDs
	wtCache := &cache.Cache{
		Worktrees: make(map[string]*cache.WorktreeIDEntry),
		NextID:    2, // intentionally low to cause issue
	}
	wtCache.Worktrees["myrepo-feature1"] = &cache.WorktreeIDEntry{
		ID:       1,
		Path:     wt1,
		RepoPath: repoPath,
		Branch:   "feature1",
	}
	wtCache.Worktrees["myrepo-feature2"] = &cache.WorktreeIDEntry{
		ID:       1, // duplicate ID!
		Path:     wt2,
		RepoPath: repoPath,
		Branch:   "feature2",
	}
	if err := cache.Save(worktreeDir, wtCache); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor with fix - should reassign duplicate ID
	err := doctor.Run(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}

	// Verify IDs are now unique
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	defer unlock()

	ids := make(map[int]string)
	for key, entry := range wtCache.Worktrees {
		if entry.RemovedAt != nil {
			continue
		}
		if existing, ok := ids[entry.ID]; ok {
			t.Errorf("duplicate ID %d for entries %q and %q", entry.ID, existing, key)
		}
		ids[entry.ID] = key
	}
}

// TestDoctor_MissingMetadata verifies that doctor populates missing cache metadata
// (repo_path, branch) by reading from the actual worktree.
//
// Scenario: User runs `wt doctor --fix` with cache entry missing repo_path/branch
// Expected: Metadata populated from actual worktree
func TestDoctor_MissingMetadata(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Create cache manually with missing metadata
	wtCache := &cache.Cache{
		Worktrees: make(map[string]*cache.WorktreeIDEntry),
		NextID:    2,
	}
	wtCache.Worktrees["myrepo-feature"] = &cache.WorktreeIDEntry{
		ID:       1,
		Path:     worktreePath,
		RepoPath: "", // missing!
		Branch:   "", // missing!
	}
	if err := cache.Save(worktreeDir, wtCache); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor with fix - should update metadata
	err := doctor.Run(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}

	// Verify metadata is now populated
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	defer unlock()

	entry := wtCache.Worktrees["myrepo-feature"]
	if entry.RepoPath == "" {
		t.Errorf("repo_path should be populated after fix")
	}
	if entry.Branch == "" {
		t.Errorf("branch should be populated after fix")
	}
}

// TestDoctor_OrphanWorktree verifies that doctor adds untracked worktrees
// (orphans) to the cache.
//
// Scenario: User runs `wt doctor --fix` with worktree not in cache
// Expected: Orphan worktree added to cache with new ID
func TestDoctor_OrphanWorktree(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Create empty cache (worktree not tracked)
	wtCache := &cache.Cache{
		Worktrees: make(map[string]*cache.WorktreeIDEntry),
		NextID:    1,
	}
	if err := cache.Save(worktreeDir, wtCache); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor with fix - should add orphan to cache
	err := doctor.Run(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}

	// Verify worktree is now in cache
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	defer unlock()

	if _, ok := wtCache.Worktrees["myrepo-feature"]; !ok {
		t.Errorf("orphan worktree should be added to cache after fix")
	}
}

// TestDoctor_Reset verifies that --reset rebuilds the cache from scratch
// with IDs starting from 1.
//
// Scenario: User runs `wt doctor --reset` with cache having high IDs (50, 75)
// Expected: Cache rebuilt with IDs 1 and 2
func TestDoctor_Reset(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with two worktrees
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	wt1 := filepath.Join(worktreeDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")
	wt2 := filepath.Join(worktreeDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	// Create cache with high IDs (to verify reset resets IDs)
	wtCache := &cache.Cache{
		Worktrees: make(map[string]*cache.WorktreeIDEntry),
		NextID:    100,
	}
	wtCache.Worktrees["myrepo-feature1"] = &cache.WorktreeIDEntry{
		ID:       50,
		Path:     wt1,
		RepoPath: repoPath,
		Branch:   "feature1",
	}
	wtCache.Worktrees["myrepo-feature2"] = &cache.WorktreeIDEntry{
		ID:       75,
		Path:     wt2,
		RepoPath: repoPath,
		Branch:   "feature2",
	}

	// Set a note on one of the branches (stored in git config)
	if err := git.SetBranchNote(context.Background(), repoPath, "feature1", "old note"); err != nil {
		t.Fatalf("failed to set branch note: %v", err)
	}
	if err := cache.Save(worktreeDir, wtCache); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run reset
	err := doctor.Reset(context.Background(), cfg)
	if err != nil {
		t.Fatalf("doctor --reset failed: %v", err)
	}

	// Verify cache is rebuilt with IDs starting from 1
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	defer unlock()

	// Check that we have 2 entries
	activeCount := 0
	for _, entry := range wtCache.Worktrees {
		if entry.RemovedAt == nil {
			activeCount++
		}
	}
	if activeCount != 2 {
		t.Errorf("expected 2 active entries, got %d", activeCount)
	}

	// Check that IDs are 1 and 2 (not 50 and 75)
	ids := make(map[int]bool)
	for _, entry := range wtCache.Worktrees {
		if entry.RemovedAt == nil {
			ids[entry.ID] = true
		}
	}
	if !ids[1] || !ids[2] {
		t.Errorf("expected IDs 1 and 2, got %v", ids)
	}

	// Note: Branch notes are stored in git config, not in the cache,
	// so they are preserved across reset. Only cache IDs are reset.
}

// TestDoctor_BrokenGitLink verifies that doctor repairs broken git links
// when the repo has been moved.
//
// Scenario: User moves repo to new location, runs `wt doctor --fix`
// Expected: Worktree .git file updated to point to new repo location
func TestDoctor_BrokenGitLink(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	// Break the git link by moving the main repo to a subdirectory
	// This simulates a common scenario where the repo is moved and worktree links break
	// Keep the same name so doctor can find it by name
	newRepoDir := filepath.Join(repoDir, "moved")
	if err := os.MkdirAll(newRepoDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	newRepoPath := filepath.Join(newRepoDir, "myrepo")
	if err := os.Rename(repoPath, newRepoPath); err != nil {
		t.Fatalf("failed to move repo: %v", err)
	}

	// Note: git status will fail now because the repo link is broken

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        newRepoDir, // Point to the new location where repos are
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor without fix - should detect broken link
	err := doctor.Run(context.Background(), cfg, false)
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}

	// Run doctor with fix - should repair the link
	err = doctor.Run(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}

	// Verify worktree is now functional
	verifyWorktreeWorks(t, worktreePath)
}

// TestDoctor_WorktreeMoved verifies that doctor repairs broken git links
// when the worktree has been moved.
//
// Scenario: User moves worktree to new location, runs `wt doctor --fix`
// Expected: Repo's gitdir reference updated to point to new worktree location
func TestDoctor_WorktreeMoved(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	// Move worktree to a subdirectory (simulating worktree moved, repo stays)
	newWorktreeDir := filepath.Join(worktreeDir, "moved")
	if err := os.MkdirAll(newWorktreeDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	newWorktreePath := filepath.Join(newWorktreeDir, "myrepo-feature")
	if err := os.Rename(worktreePath, newWorktreePath); err != nil {
		t.Fatalf("failed to move worktree: %v", err)
	}

	// Note: git status will fail now because repo's gitdir points to old location

	cfg := &config.Config{
		WorktreeDir:    newWorktreeDir, // Point to new worktree location
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor without fix - should detect broken link
	err := doctor.Run(context.Background(), cfg, false)
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}

	// Run doctor with fix - should repair the link
	err = doctor.Run(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}

	// Verify worktree is now functional
	verifyWorktreeWorks(t, newWorktreePath)
}

// TestDoctor_BothMoved verifies that doctor repairs broken git links
// when both the repo and worktree have been moved.
//
// Scenario: User moves both repo and worktree to new locations, runs `wt doctor --fix`
// Expected: Both directions of git links repaired
func TestDoctor_BothMoved(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	// Move both repo and worktree to new locations
	newRepoDir := filepath.Join(repoDir, "moved")
	if err := os.MkdirAll(newRepoDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	newRepoPath := filepath.Join(newRepoDir, "myrepo")
	if err := os.Rename(repoPath, newRepoPath); err != nil {
		t.Fatalf("failed to move repo: %v", err)
	}

	newWorktreeDir := filepath.Join(worktreeDir, "moved")
	if err := os.MkdirAll(newWorktreeDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	newWorktreePath := filepath.Join(newWorktreeDir, "myrepo-feature")
	if err := os.Rename(worktreePath, newWorktreePath); err != nil {
		t.Fatalf("failed to move worktree: %v", err)
	}

	// Note: both directions of git link are now broken

	cfg := &config.Config{
		WorktreeDir:    newWorktreeDir, // Point to new worktree location
		RepoDir:        newRepoDir,     // Point to new repo location
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor without fix - should detect broken links
	err := doctor.Run(context.Background(), cfg, false)
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}

	// Run doctor with fix - should repair both links
	err = doctor.Run(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}

	// Verify worktree is now functional
	verifyWorktreeWorks(t, newWorktreePath)
}

// TestDoctor_MultipleRepos verifies doctor handles multiple repos correctly.
//
// Scenario: User runs `wt doctor` with 2 repos each having 1 worktree
// Expected: Doctor completes successfully, finds no issues
func TestDoctor_MultipleRepos(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create two repos with worktrees
	repoA := setupTestRepo(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor - should find no issues
	err := doctor.Run(context.Background(), cfg, false)
	if err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
}

// TestDoctor_PathMismatch verifies that doctor fixes cache entries with
// incorrect paths.
//
// Scenario: User runs `wt doctor --fix` with cache having wrong path
// Expected: Path updated to actual worktree location
func TestDoctor_PathMismatch(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Create cache with wrong path
	wtCache := &cache.Cache{
		Worktrees: make(map[string]*cache.WorktreeIDEntry),
		NextID:    2,
	}
	wtCache.Worktrees["myrepo-feature"] = &cache.WorktreeIDEntry{
		ID:       1,
		Path:     "/wrong/path/myrepo-feature", // wrong path!
		RepoPath: repoPath,
		Branch:   "feature",
	}
	if err := cache.Save(worktreeDir, wtCache); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor with fix - should update path
	err := doctor.Run(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("doctor --fix failed: %v", err)
	}

	// Verify path is corrected
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	defer unlock()

	entry := wtCache.Worktrees["myrepo-feature"]
	if entry.Path != worktreePath {
		t.Errorf("expected path %s, got %s", worktreePath, entry.Path)
	}
}

// TestDoctor_EmptyWorktreeDir verifies doctor handles empty directories gracefully.
//
// Scenario: User runs `wt doctor` with empty worktree_dir
// Expected: Doctor completes successfully with no issues
func TestDoctor_EmptyWorktreeDir(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())

	// Create empty cache
	wtCache := &cache.Cache{
		Worktrees: make(map[string]*cache.WorktreeIDEntry),
		NextID:    1,
	}
	if err := cache.Save(worktreeDir, wtCache); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run doctor - should succeed with no issues
	err := doctor.Run(context.Background(), cfg, false)
	if err != nil {
		t.Fatalf("doctor failed on empty dir: %v", err)
	}
}

//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

// TestMv_MoveWorktreesToWorktreeDir verifies that worktrees and their repos
// are moved to the configured worktree_dir.
//
// Scenario: User runs `wt mv` with worktree_dir configured, repo has worktree
// Expected: Both repo and worktree moved to worktree_dir
func TestMv_MoveWorktreesToWorktreeDir(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo in source dir
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Create worktree in source dir
	worktreePath := filepath.Join(sourceDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Configure: worktree_dir = destDir
	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run mv from source dir
	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify worktree moved
	newWorktreePath := filepath.Join(destDir, "myrepo-feature")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("worktree not moved to %s", newWorktreePath)
	}

	// Verify old location is gone
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("old worktree path still exists: %s", worktreePath)
	}

	// Verify worktree still works
	verifyWorktreeWorks(t, newWorktreePath)

	// Verify repo also moved to worktree_dir (since repo_dir not configured)
	newRepoPath := filepath.Join(destDir, "myrepo")
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		t.Errorf("repo should be moved to %s (no repo_dir set)", newRepoPath)
	}

	// Verify old repo location is gone
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Errorf("old repo path should not exist: %s", repoPath)
	}
}

// TestMv_MoveReposToRepoDir verifies that repos are moved to repo_dir
// while worktrees go to worktree_dir when both are configured.
//
// Scenario: User runs `wt mv` with both worktree_dir and repo_dir configured
// Expected: Repos in repo_dir, worktrees in worktree_dir, .git references updated
func TestMv_MoveReposToRepoDir(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	worktreeDestDir := resolvePath(t, t.TempDir())
	repoDestDir := resolvePath(t, t.TempDir())

	// Create repo in source dir
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Create worktree in source dir
	worktreePath := filepath.Join(sourceDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Configure: worktree_dir and repo_dir
	cfg := &config.Config{
		WorktreeDir:    worktreeDestDir,
		RepoDir:        repoDestDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run mv
	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify worktree moved to worktree_dir
	newWorktreePath := filepath.Join(worktreeDestDir, "myrepo-feature")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("worktree not moved to %s", newWorktreePath)
	}

	// Verify repo moved to repo_dir
	newRepoPath := filepath.Join(repoDestDir, "myrepo")
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		t.Errorf("repo not moved to %s", newRepoPath)
	}

	// Verify worktree's .git file points to new repo location
	verifyGitdirPoints(t, newWorktreePath, newRepoPath)

	// Verify worktree still works
	verifyWorktreeWorks(t, newWorktreePath)
}

// TestMv_MoveReposToWorktreeDirWhenNoRepoDir verifies that repos are moved
// to worktree_dir when repo_dir is not configured.
//
// Scenario: User runs `wt mv` with only worktree_dir configured (no repo_dir)
// Expected: Repo moved to worktree_dir alongside worktrees
func TestMv_MoveReposToWorktreeDirWhenNoRepoDir(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo only (no worktrees)
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Configure: only worktree_dir (no repo_dir)
	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run mv
	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify repo moved to worktree_dir (since repo_dir not set)
	newRepoPath := filepath.Join(destDir, "myrepo")
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		t.Errorf("repo not moved to %s", newRepoPath)
	}

	// Verify old location is gone
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Errorf("old repo path still exists: %s", repoPath)
	}
}

// TestMv_CollisionAddsNumberedSuffix verifies that when the destination path
// already exists, a numbered suffix (-1) is added to avoid collision.
//
// Scenario: User runs `wt mv` but myrepo-feature already exists at destination
// Expected: Worktree moved to myrepo-feature-1
func TestMv_CollisionAddsNumberedSuffix(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo and worktree in source
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	worktreePath := filepath.Join(sourceDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Create conflicting directory at destination
	conflictPath := filepath.Join(destDir, "myrepo-feature")
	if err := os.MkdirAll(conflictPath, 0755); err != nil {
		t.Fatalf("failed to create conflict dir: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify worktree was moved with numbered suffix
	newWorktreePath := filepath.Join(destDir, "myrepo-feature-1")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("worktree should be moved to %s with numbered suffix", newWorktreePath)
	}

	// Verify original worktree location is gone
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", worktreePath)
	}

	// Verify conflicting directory still exists
	if _, err := os.Stat(conflictPath); os.IsNotExist(err) {
		t.Errorf("original conflict path should still exist: %s", conflictPath)
	}

	// Verify worktree still works
	verifyWorktreeWorks(t, newWorktreePath)
}

// TestMv_MultipleCollisionsIncrementSuffix verifies that collision suffixes
// increment when multiple conflicts exist (-1, -2, -3, etc.).
//
// Scenario: User runs `wt mv` but myrepo-feature, myrepo-feature-1, myrepo-feature-2 exist
// Expected: Worktree moved to myrepo-feature-3
func TestMv_MultipleCollisionsIncrementSuffix(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo and worktree in source
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	worktreePath := filepath.Join(sourceDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Create multiple conflicting directories at destination
	for _, suffix := range []string{"", "-1", "-2"} {
		conflictPath := filepath.Join(destDir, "myrepo-feature"+suffix)
		if err := os.MkdirAll(conflictPath, 0755); err != nil {
			t.Fatalf("failed to create conflict dir: %v", err)
		}
	}

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify worktree was moved with suffix -3 (since "", "-1", "-2" are taken)
	newWorktreePath := filepath.Join(destDir, "myrepo-feature-3")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("worktree should be moved to %s", newWorktreePath)
	}

	// Verify worktree still works
	verifyWorktreeWorks(t, newWorktreePath)
}

// TestMv_SkipRepoIfTargetExists verifies that repos are skipped (not moved)
// when a repo with the same name already exists at the destination.
//
// Scenario: User runs `wt mv` but myrepo already exists at destination as a git repo
// Expected: Source repo left in place (skipped), no error returned
func TestMv_SkipRepoIfTargetExists(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo in source
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Create conflicting repo at destination
	setupTestRepo(t, destDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	// Should not return error, just skip
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify original repo still exists (was skipped)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Errorf("repo should still exist at %s (should have been skipped)", repoPath)
	}
}

// TestMv_DryRunDoesNotMove verifies that --dry-run previews moves
// without actually performing them.
//
// Scenario: User runs `wt mv --dry-run`
// Expected: Nothing moved, original files remain, destination empty
func TestMv_DryRunDoesNotMove(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	worktreePath := filepath.Join(sourceDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
		DryRun: true,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv --dry-run failed: %v", err)
	}

	// Verify nothing moved
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("worktree should still exist at %s (dry-run)", worktreePath)
	}
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Errorf("repo should still exist at %s (dry-run)", repoPath)
	}

	// Verify nothing at destination
	newWorktreePath := filepath.Join(destDir, "myrepo-feature")
	if _, err := os.Stat(newWorktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree should not exist at %s (dry-run)", newWorktreePath)
	}
}

// TestMv_FilterByRepository verifies that -r flag filters which repos are moved.
//
// Scenario: User runs `wt mv -r repo-a` with repo-a and repo-b present
// Expected: Only repo-a and its worktrees moved, repo-b unchanged
func TestMv_FilterByRepository(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create two repos with worktrees
	repoA := setupTestRepo(t, sourceDir, "repo-a")
	worktreeA := filepath.Join(sourceDir, "repo-a-feature")
	setupWorktree(t, repoA, worktreeA, "feature")

	repoB := setupTestRepo(t, sourceDir, "repo-b")
	worktreeB := filepath.Join(sourceDir, "repo-b-feature")
	setupWorktree(t, repoB, worktreeB, "feature")

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Filter to only move repo-a
	cmd := &MvCmd{
		Format:     config.DefaultWorktreeFormat,
		Repository: []string{"repo-a"},
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv -r repo-a failed: %v", err)
	}

	// Verify repo-a worktree moved
	newWorktreeA := filepath.Join(destDir, "repo-a-feature")
	if _, err := os.Stat(newWorktreeA); os.IsNotExist(err) {
		t.Errorf("repo-a worktree should be moved to %s", newWorktreeA)
	}

	// Verify repo-b worktree NOT moved
	if _, err := os.Stat(worktreeB); os.IsNotExist(err) {
		t.Errorf("repo-b worktree should still be at %s", worktreeB)
	}
}

// TestMv_MovesDirtyWorktree verifies that worktrees with uncommitted changes
// are moved with their changes preserved.
//
// Scenario: User runs `wt mv` on worktree with dirty.txt uncommitted
// Expected: Worktree moved, dirty.txt preserved at new location
func TestMv_MovesDirtyWorktree(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	worktreePath := filepath.Join(sourceDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make worktree dirty
	makeDirty(t, worktreePath)

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify worktree was moved (dirty worktrees are moved, uncommitted changes preserved)
	newWorktreePath := filepath.Join(destDir, "myrepo-feature")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("dirty worktree should be moved to %s", newWorktreePath)
	}

	// Verify old location is gone
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", worktreePath)
	}

	// Verify dirty file is still there (uncommitted changes preserved)
	dirtyFile := filepath.Join(newWorktreePath, "dirty.txt")
	if _, err := os.Stat(dirtyFile); os.IsNotExist(err) {
		t.Errorf("dirty file should exist at %s", dirtyFile)
	}

	// Verify worktree still works
	verifyWorktreeWorks(t, newWorktreePath)
}

// TestMv_SkipIfAlreadyAtDestination verifies that worktrees already at the
// configured destination are not moved (no-op).
//
// Scenario: User runs `wt mv` but worktree is already in worktree_dir
// Expected: Worktree remains in place, no errors
func TestMv_SkipIfAlreadyAtDestination(t *testing.T) {
	t.Parallel()
	// Setup: worktree already in worktree_dir
	destDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo in repo dir
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	// Create worktree directly in dest dir
	worktreePath := filepath.Join(destDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	cfg := &config.Config{
		WorktreeDir:    destDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	// Run from dest dir (where worktree already is)
	if err := runMvCommand(t, destDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify worktree still exists at same location
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("worktree should still exist at %s", worktreePath)
	}

	// Verify worktree still works
	verifyWorktreeWorks(t, worktreePath)
}

// TestMv_WorktreeReferenceUpdatedAfterRepoMove verifies that worktree .git files
// are updated to point to the new repo location after the repo is moved.
//
// Scenario: User runs `wt mv` moving repo from source to repo_dir
// Expected: Worktree's .git file updated to reference new repo path
func TestMv_WorktreeReferenceUpdatedAfterRepoMove(t *testing.T) {
	t.Parallel()
	// Setup: repo in source, worktree in different location
	sourceDir := resolvePath(t, t.TempDir())
	worktreeDir := resolvePath(t, t.TempDir())
	repoDestDir := resolvePath(t, t.TempDir())

	// Create repo in source
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Create worktree in worktree dir (not in source)
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir, // worktrees already here
		RepoDir:        repoDestDir, // repos go here
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	// Run from source dir to move repos
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify repo moved
	newRepoPath := filepath.Join(repoDestDir, "myrepo")
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		t.Errorf("repo should be moved to %s", newRepoPath)
	}

	// Verify worktree's .git file points to NEW repo location
	verifyGitdirPoints(t, worktreePath, newRepoPath)

	// Verify worktree still works
	verifyWorktreeWorks(t, worktreePath)
}

// TestMv_MultipleWorktreesFromSameRepo verifies that all worktrees from
// a single repo are moved correctly and all references updated.
//
// Scenario: User runs `wt mv` on repo with feature1, feature2, feature3 worktrees
// Expected: All 3 worktrees moved, all .git files point to new repo location
func TestMv_MultipleWorktreesFromSameRepo(t *testing.T) {
	t.Parallel()
	// Setup
	sourceDir := resolvePath(t, t.TempDir())
	worktreeDestDir := resolvePath(t, t.TempDir())
	repoDestDir := resolvePath(t, t.TempDir())

	// Create repo with 3 worktrees
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	wt1 := filepath.Join(sourceDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")

	wt2 := filepath.Join(sourceDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	wt3 := filepath.Join(sourceDir, "myrepo-feature3")
	setupWorktree(t, repoPath, wt3, "feature3")

	cfg := &config.Config{
		WorktreeDir:    worktreeDestDir,
		RepoDir:        repoDestDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify all worktrees moved
	newRepoPath := filepath.Join(repoDestDir, "myrepo")
	for _, branch := range []string{"feature1", "feature2", "feature3"} {
		newWtPath := filepath.Join(worktreeDestDir, "myrepo-"+branch)
		if _, err := os.Stat(newWtPath); os.IsNotExist(err) {
			t.Errorf("worktree %s should be moved to %s", branch, newWtPath)
			continue
		}

		// Verify each worktree points to new repo and works
		verifyGitdirPoints(t, newWtPath, newRepoPath)
		verifyWorktreeWorks(t, newWtPath)
	}

	// Verify repo moved
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		t.Errorf("repo should be moved to %s", newRepoPath)
	}
}

// TestMv_EmptyDirectory verifies that running mv on an empty directory
// succeeds without errors (no-op).
//
// Scenario: User runs `wt mv` in directory with no repos or worktrees
// Expected: Command succeeds with no-op
func TestMv_EmptyDirectory(t *testing.T) {
	t.Parallel()
	// Setup empty directory
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}

	// Should not error on empty directory
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv should not fail on empty directory: %v", err)
	}
}

// TestMv_DestinationDoesNotExist verifies that mv fails with error when
// the configured worktree_dir doesn't exist.
//
// Scenario: User runs `wt mv` with worktree_dir pointing to non-existent path
// Expected: Error returned mentioning destination doesn't exist
func TestMv_DestinationDoesNotExist(t *testing.T) {
	t.Parallel()
	sourceDir := resolvePath(t, t.TempDir())
	nonExistentDir := filepath.Join(resolvePath(t, t.TempDir()), "does-not-exist")

	// Create a repo so there's something to move
	setupTestRepo(t, sourceDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    nonExistentDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}

	// Should return error when destination doesn't exist
	err := runMvCommand(t, sourceDir, cfg, cmd)
	if err == nil {
		t.Fatalf("wt mv should fail when destination doesn't exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should mention destination doesn't exist, got: %v", err)
	}
}

// TestMv_NoWorktreeDirConfigured verifies that mv fails with error when
// worktree_dir is not configured.
//
// Scenario: User runs `wt mv` without worktree_dir in config
// Expected: Error returned mentioning destination not configured
func TestMv_NoWorktreeDirConfigured(t *testing.T) {
	t.Parallel()
	sourceDir := resolvePath(t, t.TempDir())
	setupTestRepo(t, sourceDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    "", // Not configured
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}

	// Should return error when worktree_dir not configured
	err := runMvCommand(t, sourceDir, cfg, cmd)
	if err == nil {
		t.Fatalf("wt mv should fail when worktree_dir not configured")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("error should mention destination not configured, got: %v", err)
	}
}

// TestMv_CustomFormatRenamesWorktree verifies that --format flag renames
// worktrees to match the new naming format.
//
// Scenario: User runs `wt mv --format {branch}` (default was {repo}-{branch})
// Expected: myrepo-feature renamed to just "feature"
func TestMv_CustomFormatRenamesWorktree(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo and worktree with default naming
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	worktreePath := filepath.Join(sourceDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Use custom format: just branch name
	cmd := &MvCmd{
		Format: "{branch}",
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv --format failed: %v", err)
	}

	// Verify worktree moved with new name (just "feature", not "myrepo-feature")
	newWorktreePath := filepath.Join(destDir, "feature")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("worktree should be renamed to %s", newWorktreePath)
	}

	// Verify old name doesn't exist at destination
	oldNamePath := filepath.Join(destDir, "myrepo-feature")
	if _, err := os.Stat(oldNamePath); !os.IsNotExist(err) {
		t.Errorf("worktree should not exist at old name %s", oldNamePath)
	}

	// Verify worktree still works
	verifyWorktreeWorks(t, newWorktreePath)
}

// TestMv_FolderFormatPlaceholder verifies that {repo} placeholder uses
// the local folder name (not the origin remote name).
//
// Scenario: User runs `wt mv --format {repo}_{branch}` on repo "my-local-folder"
// Expected: Worktree named "my-local-folder_feature"
func TestMv_FolderFormatPlaceholder(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo with folder name different from origin name
	repoPath := setupTestRepo(t, sourceDir, "my-local-folder")
	worktreePath := filepath.Join(sourceDir, "my-local-folder-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Use {repo} placeholder (folder name) instead of {origin}
	cmd := &MvCmd{
		Format: "{repo}_{branch}",
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv --format failed: %v", err)
	}

	// Verify worktree uses folder name (my-local-folder_feature)
	newWorktreePath := filepath.Join(destDir, "my-local-folder_feature")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("worktree should be at %s (using repo placeholder)", newWorktreePath)
	}

	verifyWorktreeWorks(t, newWorktreePath)
}

// TestMv_NestedWorktreeMovedToWorktreeDir verifies that worktrees nested
// inside a repo directory are moved out to worktree_dir.
//
// Scenario: User runs `wt mv` on repo with worktree at repo/worktrees/feature
// Expected: Nested worktree moved to worktree_dir/myrepo-feature
func TestMv_NestedWorktreeMovedToWorktreeDir(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	worktreeDestDir := resolvePath(t, t.TempDir())
	repoDestDir := resolvePath(t, t.TempDir())

	// Create repo in source dir
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Create a NESTED worktree inside the repo directory
	nestedWorktreePath := filepath.Join(repoPath, "worktrees", "feature")
	if err := os.MkdirAll(filepath.Dir(nestedWorktreePath), 0755); err != nil {
		t.Fatalf("failed to create nested worktrees dir: %v", err)
	}
	setupWorktree(t, repoPath, nestedWorktreePath, "feature")

	// Verify nested worktree is inside repo
	if !strings.HasPrefix(nestedWorktreePath, repoPath) {
		t.Fatalf("test setup error: worktree should be nested inside repo")
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDestDir,
		RepoDir:        repoDestDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify nested worktree was moved to worktree_dir (not left inside repo)
	newWorktreePath := filepath.Join(worktreeDestDir, "myrepo-feature")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("nested worktree should be moved to %s", newWorktreePath)
	}

	// Verify old nested path no longer exists
	if _, err := os.Stat(nestedWorktreePath); !os.IsNotExist(err) {
		t.Errorf("old nested worktree path should not exist: %s", nestedWorktreePath)
	}

	// Verify repo moved to repo_dir
	newRepoPath := filepath.Join(repoDestDir, "myrepo")
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		t.Errorf("repo should be moved to %s", newRepoPath)
	}

	// Verify worktree's .git file points to new repo location
	verifyGitdirPoints(t, newWorktreePath, newRepoPath)

	// Verify worktree still works
	verifyWorktreeWorks(t, newWorktreePath)
}

// TestMv_MultipleNestedWorktrees verifies that multiple worktrees nested
// inside a repo directory are all moved to worktree_dir.
//
// Scenario: User runs `wt mv` on repo with 3 nested worktrees
// Expected: All 3 nested worktrees moved to worktree_dir
func TestMv_MultipleNestedWorktrees(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	worktreeDestDir := resolvePath(t, t.TempDir())
	repoDestDir := resolvePath(t, t.TempDir())

	// Create repo with multiple nested worktrees
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Create worktrees dir inside repo
	worktreesDir := filepath.Join(repoPath, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		t.Fatalf("failed to create worktrees dir: %v", err)
	}

	// Create 3 nested worktrees
	for _, branch := range []string{"feature1", "feature2", "feature3"} {
		wtPath := filepath.Join(worktreesDir, branch)
		setupWorktree(t, repoPath, wtPath, branch)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDestDir,
		RepoDir:        repoDestDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify all nested worktrees moved to worktree_dir
	newRepoPath := filepath.Join(repoDestDir, "myrepo")
	for _, branch := range []string{"feature1", "feature2", "feature3"} {
		newWtPath := filepath.Join(worktreeDestDir, "myrepo-"+branch)
		if _, err := os.Stat(newWtPath); os.IsNotExist(err) {
			t.Errorf("nested worktree %s should be moved to %s", branch, newWtPath)
			continue
		}

		// Verify each worktree points to new repo and works
		verifyGitdirPoints(t, newWtPath, newRepoPath)
		verifyWorktreeWorks(t, newWtPath)
	}

	// Verify no worktrees directory left in repo
	oldWorktreesDir := filepath.Join(newRepoPath, "worktrees")
	if entries, err := os.ReadDir(oldWorktreesDir); err == nil && len(entries) > 0 {
		t.Errorf("worktrees directory in repo should be empty after move, found: %v", entries)
	}
}

// TestMv_MixedNestedAndExternalWorktrees verifies that both nested worktrees
// (inside repo dir) and external worktrees (sibling to repo) are moved.
//
// Scenario: User runs `wt mv` on repo with nested and external worktrees
// Expected: Both worktrees moved to worktree_dir
func TestMv_MixedNestedAndExternalWorktrees(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	worktreeDestDir := resolvePath(t, t.TempDir())
	repoDestDir := resolvePath(t, t.TempDir())

	// Create repo
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Create a nested worktree inside the repo
	nestedWtPath := filepath.Join(repoPath, "worktrees", "nested-feature")
	if err := os.MkdirAll(filepath.Dir(nestedWtPath), 0755); err != nil {
		t.Fatalf("failed to create nested worktrees dir: %v", err)
	}
	setupWorktree(t, repoPath, nestedWtPath, "nested-feature")

	// Create an external worktree (sibling to repo)
	externalWtPath := filepath.Join(sourceDir, "myrepo-external-feature")
	setupWorktree(t, repoPath, externalWtPath, "external-feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDestDir,
		RepoDir:        repoDestDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	newRepoPath := filepath.Join(repoDestDir, "myrepo")

	// Verify nested worktree moved to worktree_dir
	newNestedWtPath := filepath.Join(worktreeDestDir, "myrepo-nested-feature")
	if _, err := os.Stat(newNestedWtPath); os.IsNotExist(err) {
		t.Errorf("nested worktree should be moved to %s", newNestedWtPath)
	}
	verifyGitdirPoints(t, newNestedWtPath, newRepoPath)
	verifyWorktreeWorks(t, newNestedWtPath)

	// Verify external worktree also moved to worktree_dir
	newExternalWtPath := filepath.Join(worktreeDestDir, "myrepo-external-feature")
	if _, err := os.Stat(newExternalWtPath); os.IsNotExist(err) {
		t.Errorf("external worktree should be moved to %s", newExternalWtPath)
	}
	verifyGitdirPoints(t, newExternalWtPath, newRepoPath)
	verifyWorktreeWorks(t, newExternalWtPath)
}

// TestMv_NestedDirtyWorktreeMoved verifies that nested worktrees with
// uncommitted changes are moved with changes preserved.
//
// Scenario: User runs `wt mv` on repo with dirty nested worktree
// Expected: Nested worktree moved with dirty.txt preserved
func TestMv_NestedDirtyWorktreeMoved(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	worktreeDestDir := resolvePath(t, t.TempDir())
	repoDestDir := resolvePath(t, t.TempDir())

	// Create repo with nested worktree
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	nestedWtPath := filepath.Join(repoPath, "worktrees", "feature")
	if err := os.MkdirAll(filepath.Dir(nestedWtPath), 0755); err != nil {
		t.Fatalf("failed to create nested worktrees dir: %v", err)
	}
	setupWorktree(t, repoPath, nestedWtPath, "feature")

	// Make nested worktree dirty
	makeDirty(t, nestedWtPath)

	cfg := &config.Config{
		WorktreeDir:    worktreeDestDir,
		RepoDir:        repoDestDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Repo should be moved
	newRepoPath := filepath.Join(repoDestDir, "myrepo")
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		t.Errorf("repo should be moved to %s", newRepoPath)
	}

	// Nested worktree should be moved to worktree_dir (dirty worktrees are moved)
	newWtPath := filepath.Join(worktreeDestDir, "myrepo-feature")
	if _, err := os.Stat(newWtPath); os.IsNotExist(err) {
		t.Errorf("nested worktree should be moved to %s", newWtPath)
	}

	// Verify dirty file is preserved
	dirtyFile := filepath.Join(newWtPath, "dirty.txt")
	if _, err := os.Stat(dirtyFile); os.IsNotExist(err) {
		t.Errorf("dirty file should exist at %s", dirtyFile)
	}

	// Verify worktree points to new repo and works
	verifyGitdirPoints(t, newWtPath, newRepoPath)
	verifyWorktreeWorks(t, newWtPath)
}

// TestMv_FormatChangeRenamesWorktreesInPlace verifies that worktrees already
// at destination are renamed in place when format changes.
//
// Scenario: User runs `wt mv --format {branch}` on existing {repo}-{branch} worktrees
// Expected: Worktrees renamed from myrepo-feature1 to feature1 (in place)
func TestMv_FormatChangeRenamesWorktreesInPlace(t *testing.T) {
	t.Parallel()
	// Setup: worktrees already in worktree_dir with old format
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	// Create worktrees with OLD format: {repo}-{branch}
	wt1 := filepath.Join(worktreeDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")

	wt2 := filepath.Join(worktreeDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	// Config with NEW format: just {branch}
	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: "{branch}", // Changed from default {repo}-{branch}
	}

	// Run mv from worktree_dir to rename in place
	cmd := &MvCmd{
		Format: "{branch}",
	}
	if err := runMvCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify worktrees renamed to new format
	newWt1 := filepath.Join(worktreeDir, "feature1")
	if _, err := os.Stat(newWt1); os.IsNotExist(err) {
		t.Errorf("worktree should be renamed to %s", newWt1)
	}

	newWt2 := filepath.Join(worktreeDir, "feature2")
	if _, err := os.Stat(newWt2); os.IsNotExist(err) {
		t.Errorf("worktree should be renamed to %s", newWt2)
	}

	// Verify old names are gone
	if _, err := os.Stat(wt1); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", wt1)
	}
	if _, err := os.Stat(wt2); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", wt2)
	}

	// Verify worktrees still work
	verifyWorktreeWorks(t, newWt1)
	verifyWorktreeWorks(t, newWt2)
}

// TestMv_FormatChangeWithCollision verifies that format changes that would
// cause naming collisions add numbered suffixes.
//
// Scenario: User runs `wt mv --format {branch}` on repo-a/feature and repo-b/feature
// Expected: One becomes "feature", other becomes "feature-1"
func TestMv_FormatChangeWithCollision(t *testing.T) {
	t.Parallel()
	// Setup: two worktrees from different repos that would collide with new format
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	// Create worktrees with format {repo}-{branch}
	// Both have branch "feature" so they'll collide if format changes to {branch}
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Change format to just {branch} - this will cause collision
	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: "{branch}",
	}

	cmd := &MvCmd{
		Format: "{branch}",
	}
	if err := runMvCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// One worktree should be "feature", the other "feature-1"
	featurePath := filepath.Join(worktreeDir, "feature")
	feature1Path := filepath.Join(worktreeDir, "feature-1")

	featureExists := false
	if _, err := os.Stat(featurePath); err == nil {
		featureExists = true
	}

	feature1Exists := false
	if _, err := os.Stat(feature1Path); err == nil {
		feature1Exists = true
	}

	if !featureExists || !feature1Exists {
		t.Errorf("expected both 'feature' and 'feature-1' to exist, got feature=%v feature-1=%v", featureExists, feature1Exists)
	}

	// Verify both worktrees still work
	if featureExists {
		verifyWorktreeWorks(t, featurePath)
	}
	if feature1Exists {
		verifyWorktreeWorks(t, feature1Path)
	}

	// Verify old paths are gone
	if _, err := os.Stat(wtA); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", wtA)
	}
	if _, err := os.Stat(wtB); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", wtB)
	}
}

// TestMv_PathArgumentSingleWorktree verifies that PATH argument moves only
// the specified worktree, leaving other worktrees and repos in place.
//
// Scenario: User runs `wt mv /path/to/myrepo-feature1` with feature1 and feature2
// Expected: Only feature1 worktree moved, feature2 and repo unchanged
func TestMv_PathArgumentSingleWorktree(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	// Create repo and two worktrees
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	wt1 := filepath.Join(sourceDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")
	wt2 := filepath.Join(sourceDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Move only the first worktree by path
	cmd := &MvCmd{
		Path:   wt1,
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify first worktree was moved
	newWt1 := filepath.Join(destDir, "myrepo-feature1")
	if _, err := os.Stat(newWt1); os.IsNotExist(err) {
		t.Errorf("worktree should be moved to %s", newWt1)
	}

	// Verify second worktree was NOT moved (still in source)
	if _, err := os.Stat(wt2); os.IsNotExist(err) {
		t.Errorf("second worktree should still be at %s", wt2)
	}

	// Verify repo was NOT moved
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Errorf("repo should still be at %s", repoPath)
	}

	// Verify moved worktree works
	verifyWorktreeWorks(t, newWt1)
}

// TestMv_PathArgumentSingleRepo verifies that PATH argument for a repo moves
// that repo and all its worktrees, leaving other repos unchanged.
//
// Scenario: User runs `wt mv /path/to/repo-a` with repo-a and repo-b present
// Expected: repo-a and its worktrees moved, repo-b unchanged
func TestMv_PathArgumentSingleRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	worktreeDestDir := resolvePath(t, t.TempDir())
	repoDestDir := resolvePath(t, t.TempDir())

	// Create two repos with worktrees
	repoA := setupTestRepo(t, sourceDir, "repo-a")
	wtA := filepath.Join(sourceDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, sourceDir, "repo-b")
	wtB := filepath.Join(sourceDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDestDir,
		RepoDir:        repoDestDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Move only repo-a by path
	cmd := &MvCmd{
		Path:   repoA,
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify repo-a was moved
	newRepoA := filepath.Join(repoDestDir, "repo-a")
	if _, err := os.Stat(newRepoA); os.IsNotExist(err) {
		t.Errorf("repo-a should be moved to %s", newRepoA)
	}

	// Verify repo-a's worktree was moved
	newWtA := filepath.Join(worktreeDestDir, "repo-a-feature")
	if _, err := os.Stat(newWtA); os.IsNotExist(err) {
		t.Errorf("repo-a worktree should be moved to %s", newWtA)
	}

	// Verify repo-b was NOT moved
	if _, err := os.Stat(repoB); os.IsNotExist(err) {
		t.Errorf("repo-b should still be at %s", repoB)
	}

	// Verify repo-b's worktree was NOT moved
	if _, err := os.Stat(wtB); os.IsNotExist(err) {
		t.Errorf("repo-b worktree should still be at %s", wtB)
	}

	// Verify moved worktree works
	verifyWorktreeWorks(t, newWtA)
}

// TestMv_PathArgumentFolder verifies that PATH argument for a folder moves
// all repos and worktrees within that folder.
//
// Scenario: User runs `wt mv /path/to/projects` containing repos
// Expected: All repos and worktrees in /projects moved to destination
func TestMv_PathArgumentFolder(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	subDir := filepath.Join(sourceDir, "projects")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	destDir := resolvePath(t, t.TempDir())

	// Create repo and worktree in subDir
	repoPath := setupTestRepo(t, subDir, "myrepo")
	wtPath := filepath.Join(subDir, "myrepo-feature")
	setupWorktree(t, repoPath, wtPath, "feature")

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Move all from subDir by specifying it as path
	cmd := &MvCmd{
		Path:   subDir,
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify worktree was moved
	newWtPath := filepath.Join(destDir, "myrepo-feature")
	if _, err := os.Stat(newWtPath); os.IsNotExist(err) {
		t.Errorf("worktree should be moved to %s", newWtPath)
	}

	// Verify repo was moved (to worktree_dir since no repo_dir set)
	newRepoPath := filepath.Join(destDir, "myrepo")
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		t.Errorf("repo should be moved to %s", newRepoPath)
	}

	// Verify old locations are gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", wtPath)
	}
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Errorf("old repo path should not exist: %s", repoPath)
	}

	// Verify worktree works
	verifyWorktreeWorks(t, newWtPath)
}

// TestMv_PathArgumentRepoWithNestedWorktree verifies that PATH argument for a repo
// with nested worktrees moves both the repo and extracts nested worktrees.
//
// Scenario: User runs `wt mv /path/to/myrepo` which has nested worktree
// Expected: Repo moved to repo_dir, nested worktree moved to worktree_dir
func TestMv_PathArgumentRepoWithNestedWorktree(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := resolvePath(t, t.TempDir())
	worktreeDestDir := resolvePath(t, t.TempDir())
	repoDestDir := resolvePath(t, t.TempDir())

	// Create repo with nested worktree
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	nestedWtPath := filepath.Join(repoPath, "worktrees", "feature")
	if err := os.MkdirAll(filepath.Dir(nestedWtPath), 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	setupWorktree(t, repoPath, nestedWtPath, "feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDestDir,
		RepoDir:        repoDestDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Move repo by path
	cmd := &MvCmd{
		Path:   repoPath,
		Format: config.DefaultWorktreeFormat,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify repo was moved
	newRepoPath := filepath.Join(repoDestDir, "myrepo")
	if _, err := os.Stat(newRepoPath); os.IsNotExist(err) {
		t.Errorf("repo should be moved to %s", newRepoPath)
	}

	// Verify nested worktree was moved OUT to worktree_dir
	newWtPath := filepath.Join(worktreeDestDir, "myrepo-feature")
	if _, err := os.Stat(newWtPath); os.IsNotExist(err) {
		t.Errorf("nested worktree should be moved to %s", newWtPath)
	}

	// Verify worktree works
	verifyGitdirPoints(t, newWtPath, newRepoPath)
	verifyWorktreeWorks(t, newWtPath)
}

// TestMv_PathDoesNotExist verifies that mv fails with error when
// the specified PATH argument doesn't exist.
//
// Scenario: User runs `wt mv /path/that/does-not-exist`
// Expected: Error returned mentioning path doesn't exist
func TestMv_PathDoesNotExist(t *testing.T) {
	t.Parallel()
	sourceDir := resolvePath(t, t.TempDir())
	destDir := resolvePath(t, t.TempDir())

	cfg := &config.Config{
		WorktreeDir:    destDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &MvCmd{
		Path:   filepath.Join(sourceDir, "does-not-exist"),
		Format: config.DefaultWorktreeFormat,
	}

	err := runMvCommand(t, sourceDir, cfg, cmd)
	if err == nil {
		t.Fatalf("wt mv should fail when path doesn't exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should mention path doesn't exist, got: %v", err)
	}
}

// runMvCommand runs wt mv with the given config and command in the specified directory.
func runMvCommand(t *testing.T, workDir string, cfg *config.Config, cmd *MvCmd) error {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx := testContext(t)
	return cmd.runMv(ctx)
}

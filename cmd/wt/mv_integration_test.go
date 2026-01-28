//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

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

func TestMv_FormatChangeWithCollision(t *testing.T) {
	t.Parallel()
	// Setup: two worktrees from different repos that would collide with new format
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	// Create worktrees with format {repo}-{branch}
	// Both have branch "main" so they'll collide if format changes to {branch}
	wtA := filepath.Join(worktreeDir, "repo-a-main")
	setupWorktree(t, repoA, wtA, "main")

	wtB := filepath.Join(worktreeDir, "repo-b-main")
	setupWorktree(t, repoB, wtB, "main")

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

	// One worktree should be "main", the other "main-1"
	mainPath := filepath.Join(worktreeDir, "main")
	main1Path := filepath.Join(worktreeDir, "main-1")

	mainExists := false
	if _, err := os.Stat(mainPath); err == nil {
		mainExists = true
	}

	main1Exists := false
	if _, err := os.Stat(main1Path); err == nil {
		main1Exists = true
	}

	if !mainExists || !main1Exists {
		t.Errorf("expected both 'main' and 'main-1' to exist, got main=%v main-1=%v", mainExists, main1Exists)
	}

	// Verify both worktrees still work
	if mainExists {
		verifyWorktreeWorks(t, mainPath)
	}
	if main1Exists {
		verifyWorktreeWorks(t, main1Path)
	}

	// Verify old paths are gone
	if _, err := os.Stat(wtA); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", wtA)
	}
	if _, err := os.Stat(wtB); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", wtB)
	}
}

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

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

func TestMv_SkipWorktreeIfTargetExists(t *testing.T) {
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
	// Should not return error, just skip
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify original worktree still exists (was skipped)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("worktree should still exist at %s (should have been skipped)", worktreePath)
	}
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

func TestMv_SkipDirtyWorktree(t *testing.T) {
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
		Force:  false,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv failed: %v", err)
	}

	// Verify worktree NOT moved (dirty)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("dirty worktree should still be at %s", worktreePath)
	}

	newWorktreePath := filepath.Join(destDir, "myrepo-feature")
	if _, err := os.Stat(newWorktreePath); !os.IsNotExist(err) {
		t.Errorf("dirty worktree should not be moved to %s", newWorktreePath)
	}
}

func TestMv_ForceMovesDirtyWorktree(t *testing.T) {
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
		Force:  true,
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv -f failed: %v", err)
	}

	// Verify worktree moved despite being dirty
	newWorktreePath := filepath.Join(destDir, "myrepo-feature")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("dirty worktree should be moved to %s with -f flag", newWorktreePath)
	}

	// Verify old location is gone
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("old worktree path should not exist: %s", worktreePath)
	}

	// Verify dirty file is still there
	dirtyFile := filepath.Join(newWorktreePath, "dirty.txt")
	if _, err := os.Stat(dirtyFile); os.IsNotExist(err) {
		t.Errorf("dirty file should exist at %s", dirtyFile)
	}
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

	// Use {folder} placeholder instead of {repo}
	cmd := &MvCmd{
		Format: "{folder}_{branch}",
	}
	if err := runMvCommand(t, sourceDir, cfg, cmd); err != nil {
		t.Fatalf("wt mv --format failed: %v", err)
	}

	// Verify worktree uses folder name (my-local-folder_feature)
	newWorktreePath := filepath.Join(destDir, "my-local-folder_feature")
	if _, err := os.Stat(newWorktreePath); os.IsNotExist(err) {
		t.Errorf("worktree should be at %s (using folder placeholder)", newWorktreePath)
	}

	verifyWorktreeWorks(t, newWorktreePath)
}

// runMvCommand runs wt mv with the given config and command in the specified directory.
func runMvCommand(t *testing.T, workDir string, cfg *config.Config, cmd *MvCmd) error {
	t.Helper()
	return runMv(cmd, cfg, workDir)
}

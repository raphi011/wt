//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

func TestPrune_MergedBranch(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with local origin (required for merge detection)
	repoPath := setupTestRepoWithLocalOrigin(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit in worktree so merge has something to do
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Merge the branch into main and push (makes it prunable)
	mergeBranchToMain(t, repoPath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		Global: true, // Outside repo, need global
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune failed: %v", err)
	}

	// Verify worktree was removed
	verifyWorktreeRemoved(t, worktreePath)
}

func TestPrune_SkipsDirty(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with local origin (required for merge detection)
	repoPath := setupTestRepoWithLocalOrigin(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit so merge has something to do
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Merge the branch but also make it dirty
	mergeBranchToMain(t, repoPath, "feature")
	makeDirty(t, worktreePath)

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		Global: true,
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune failed: %v", err)
	}

	// Verify worktree still exists (skipped due to dirty)
	verifyWorktreeExists(t, worktreePath)
}

func TestPrune_SkipsUnmergedWithCommits(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit (not merged)
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		Global: true,
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune failed: %v", err)
	}

	// Verify worktree still exists (skipped due to unmerged commits)
	verifyWorktreeExists(t, worktreePath)
}

func TestPrune_IncludeClean(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// No commits on branch - it's "clean" (0 commits ahead)

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// First, verify without -c it's skipped
	cmdNoClean := &PruneCmd{
		Global: true,
	}
	if err := runPruneCommand(t, worktreeDir, cfg, cmdNoClean); err != nil {
		t.Fatalf("wt prune failed: %v", err)
	}
	verifyWorktreeExists(t, worktreePath)

	// Now with -c it should be removed
	cmdWithClean := &PruneCmd{
		Global:       true,
		IncludeClean: true,
	}
	if err := runPruneCommand(t, worktreeDir, cfg, cmdWithClean); err != nil {
		t.Fatalf("wt prune -c failed: %v", err)
	}
	verifyWorktreeRemoved(t, worktreePath)
}

func TestPrune_DryRun(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with local origin (required for merge detection)
	repoPath := setupTestRepoWithLocalOrigin(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit so merge has something to do
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Merge the branch (makes it prunable)
	mergeBranchToMain(t, repoPath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		Global: true,
		DryRun: true,
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune -n failed: %v", err)
	}

	// Verify worktree still exists (dry run)
	verifyWorktreeExists(t, worktreePath)
}

func TestPrune_ByID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with local origin (required for merge detection)
	repoPath := setupTestRepoWithLocalOrigin(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit so merge has something to do
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Merge the branch (makes it prunable)
	mergeBranchToMain(t, repoPath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		ID: []int{1},
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune -i 1 failed: %v", err)
	}

	// Verify worktree was removed
	verifyWorktreeRemoved(t, worktreePath)
}

func TestPrune_ByID_Force(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit (not merged) - normally not prunable
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// First verify it can't be pruned without force
	cmdNoForce := &PruneCmd{
		ID: []int{1},
	}
	err := runPruneCommand(t, worktreeDir, cfg, cmdNoForce)
	if err == nil {
		t.Fatal("expected error when pruning unmerged worktree without -f")
	}
	verifyWorktreeExists(t, worktreePath)

	// Now with force
	cmdForce := &PruneCmd{
		ID:    []int{1},
		Force: true,
	}
	if err := runPruneCommand(t, worktreeDir, cfg, cmdForce); err != nil {
		t.Fatalf("wt prune -i 1 -f failed: %v", err)
	}

	verifyWorktreeRemoved(t, worktreePath)
}

func TestPrune_MultipleIDs(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with local origin (required for merge detection)
	repoPath := setupTestRepoWithLocalOrigin(t, repoDir, "myrepo")

	wt1 := filepath.Join(worktreeDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")
	makeCommitInWorktree(t, wt1, "feature1.txt")
	mergeBranchToMain(t, repoPath, "feature1")

	wt2 := filepath.Join(worktreeDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")
	makeCommitInWorktree(t, wt2, "feature2.txt")
	mergeBranchToMain(t, repoPath, "feature2")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		ID: []int{1, 2},
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune -i 1 -i 2 failed: %v", err)
	}

	// Verify both worktrees were removed
	verifyWorktreeRemoved(t, wt1)
	verifyWorktreeRemoved(t, wt2)
}

func TestPrune_InsideRepoOnly(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos with local origins (required for merge detection)
	repoA := setupTestRepoWithLocalOrigin(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")
	makeCommitInWorktree(t, wtA, "feature.txt")
	mergeBranchToMain(t, repoA, "feature")

	repoB := setupTestRepoWithLocalOrigin(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")
	makeCommitInWorktree(t, wtB, "feature.txt")
	mergeBranchToMain(t, repoB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run from inside repo-a (no --global)
	cmd := &PruneCmd{
		// Global: false (default)
	}

	if err := runPruneCommand(t, repoA, cfg, cmd); err != nil {
		t.Fatalf("wt prune failed: %v", err)
	}

	// Only repo-a's worktree should be removed
	verifyWorktreeRemoved(t, wtA)
	verifyWorktreeExists(t, wtB)
}

func TestPrune_Global(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos with local origins (required for merge detection)
	repoA := setupTestRepoWithLocalOrigin(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")
	makeCommitInWorktree(t, wtA, "feature.txt")
	mergeBranchToMain(t, repoA, "feature")

	repoB := setupTestRepoWithLocalOrigin(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")
	makeCommitInWorktree(t, wtB, "feature.txt")
	mergeBranchToMain(t, repoB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run from inside repo-a with --global
	cmd := &PruneCmd{
		Global: true,
	}

	if err := runPruneCommand(t, repoA, cfg, cmd); err != nil {
		t.Fatalf("wt prune -g failed: %v", err)
	}

	// Both worktrees should be removed
	verifyWorktreeRemoved(t, wtA)
	verifyWorktreeRemoved(t, wtB)
}

func TestPrune_ErrorForceWithoutID(t *testing.T) {
	t.Parallel()
	worktreeDir := t.TempDir()

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		Force: true, // -f without -i
	}

	err := runPruneCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when using -f without -i")
	}
	if !strings.Contains(err.Error(), "-f/--force requires -i/--id") {
		t.Errorf("expected '-f/--force requires -i/--id' error, got: %v", err)
	}
}

func TestPrune_ErrorVerboseWithID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		ID:      []int{1},
		Verbose: true, // --verbose with -i
	}

	err := runPruneCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when using --verbose with -i")
	}
	if !strings.Contains(err.Error(), "--verbose cannot be used with -i/--id") {
		t.Errorf("expected '--verbose cannot be used with -i/--id' error, got: %v", err)
	}
}

func TestPrune_ErrorInvalidID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		ID: []int{999}, // Invalid ID
	}

	err := runPruneCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

func TestPrune_ErrorResetCacheWithID(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		ID:         []int{1},
		ResetCache: true, // --reset-cache with -i
	}

	err := runPruneCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when using --reset-cache with -i")
	}
	if !strings.Contains(err.Error(), "--reset-cache cannot be used with --id") {
		t.Errorf("expected '--reset-cache cannot be used with --id' error, got: %v", err)
	}
}

func TestPrune_ByID_DryRun(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with local origin (required for merge detection)
	repoPath := setupTestRepoWithLocalOrigin(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit so merge has something to do
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Merge the branch (makes it prunable)
	mergeBranchToMain(t, repoPath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		ID:     []int{1},
		DryRun: true,
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune -i 1 -n failed: %v", err)
	}

	// Verify worktree still exists (dry run)
	verifyWorktreeExists(t, worktreePath)
}

func TestPrune_ForceRemovesDirty(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make it dirty
	makeDirty(t, worktreePath)

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		ID:    []int{1},
		Force: true,
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune -i 1 -f failed: %v", err)
	}

	// Verify worktree was removed despite being dirty
	verifyWorktreeRemoved(t, worktreePath)
}

// runPruneCommand runs wt prune with the given config and command in the specified directory.
func runPruneCommand(t *testing.T, workDir string, cfg *config.Config, cmd *PruneCmd) error {
	t.Helper()
	return runPrune(cmd, cfg, workDir)
}

// verifyWorktreeRemoved verifies that a worktree directory no longer exists.
func verifyWorktreeRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree should have been removed: %s", path)
	}
}

// verifyWorktreeExists verifies that a worktree directory still exists.
func verifyWorktreeExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("worktree should still exist: %s", path)
	}
}

func TestPrune_MergedPR_RemovesWorktree(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo (use simple setup - PR status comes from cache, not merge detection)
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit so we have something
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Populate cache WITH PR info showing MERGED state
	populateCacheWithPR(t, worktreeDir, "myrepo-feature", "MERGED", 123)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		Global: true, // Outside repo, need global
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune failed: %v", err)
	}

	// Verify worktree was removed (PR state=MERGED makes it prunable)
	verifyWorktreeRemoved(t, worktreePath)
}

func TestPrune_OpenPR_KeepsWorktree(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit so we have something
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Populate cache WITH PR info showing OPEN state
	populateCacheWithPR(t, worktreeDir, "myrepo-feature", "OPEN", 456)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &PruneCmd{
		Global: true,
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune failed: %v", err)
	}

	// Verify worktree still exists (PR state=OPEN means NOT prunable)
	verifyWorktreeExists(t, worktreePath)
}

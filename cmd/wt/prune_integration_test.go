//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

// TestPrune_NoPR_NotRemoved verifies that locally merged branches without PRs
// are not auto-pruned.
//
// Scenario: Branch merged locally but no PR exists
// Expected: Worktree NOT removed (no merged PR to trigger auto-prune)
func TestPrune_NoPR_NotRemoved(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with local origin (required for merge detection)
	repoPath := setupTestRepoWithLocalOrigin(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Make a commit in worktree
	makeCommitInWorktree(t, worktreePath, "feature.txt")

	// Merge the branch locally (but no PR - won't be auto-pruned)
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

	// Verify worktree NOT removed - no merged PR means not auto-pruned
	verifyWorktreeExists(t, worktreePath)
}

// TestPrune_SkipsDirty verifies that dirty worktrees are skipped during auto-prune.
//
// Scenario: Merged branch has uncommitted changes
// Expected: Worktree preserved due to dirty state
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

// TestPrune_SkipsUnmergedWithCommits verifies that branches with unmerged commits
// are not auto-pruned.
//
// Scenario: Branch has commits that haven't been merged
// Expected: Worktree preserved to prevent data loss
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

// TestPrune_CleanBranch_NotRemoved verifies that clean branches (0 commits ahead)
// without merged PRs are not auto-pruned.
//
// Scenario: Clean branch with no merged PR
// Expected: Worktree NOT removed (no merged PR)
func TestPrune_CleanBranch_NotRemoved(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo and worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// No commits on branch - it's "clean" (0 commits ahead)
	// Without a merged PR, it won't be auto-pruned

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

	// Verify worktree NOT removed - no merged PR
	verifyWorktreeExists(t, worktreePath)
}

// TestPrune_DryRun verifies --dry-run flag shows what would be pruned without
// actually removing anything.
//
// Scenario: User runs `wt prune -g -d` with merged branch
// Expected: Worktree still exists (dry run)
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

// TestPrune_ByID_RequiresForce verifies that pruning by ID requires -f flag for safety.
//
// Scenario: User runs `wt prune -n 1` without -f flag
// Expected: Error requiring --force, then succeeds with -f
func TestPrune_ByID_RequiresForce(t *testing.T) {
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

	// Without -f flag, should fail
	cmdNoForce := &PruneCmd{
		ID: []int{1},
	}

	err := runPruneCommand(t, worktreeDir, cfg, cmdNoForce)
	if err == nil {
		t.Fatal("expected error when pruning by ID without -f")
	}
	verifyWorktreeExists(t, worktreePath)

	// With -f flag, should succeed
	cmdWithForce := &PruneCmd{
		ID:    []int{1},
		Force: true,
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmdWithForce); err != nil {
		t.Fatalf("wt prune -n 1 -f failed: %v", err)
	}

	// Verify worktree was removed
	verifyWorktreeRemoved(t, worktreePath)
}

// TestPrune_MultipleIDs verifies pruning multiple worktrees by ID.
//
// Scenario: User runs `wt prune -n 1 -n 2 -f`
// Expected: Both worktrees removed
func TestPrune_MultipleIDs(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	wt1 := filepath.Join(worktreeDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")

	wt2 := filepath.Join(worktreeDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Prune by ID requires force
	cmd := &PruneCmd{
		ID:    []int{1, 2},
		Force: true,
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune -n 1 -n 2 -f failed: %v", err)
	}

	// Verify both worktrees were removed
	verifyWorktreeRemoved(t, wt1)
	verifyWorktreeRemoved(t, wt2)
}

// TestPrune_InsideRepoOnly_NoPR verifies that without -g flag, prune only
// considers the current repo.
//
// Scenario: User runs `wt prune` from inside repo-a (no -g flag)
// Expected: Only repo-a worktrees considered, no auto-prune without merged PRs
func TestPrune_InsideRepoOnly_NoPR(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos
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
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run from inside repo-a (no --global)
	// Without merged PRs, nothing should be auto-pruned
	cmd := &PruneCmd{
		// Global: false (default)
	}

	if err := runPruneCommand(t, repoA, cfg, cmd); err != nil {
		t.Fatalf("wt prune failed: %v", err)
	}

	// No worktrees removed (no merged PRs)
	verifyWorktreeExists(t, wtA)
	verifyWorktreeExists(t, wtB)
}

// TestPrune_Global_NoPR verifies that -g flag considers all repos.
//
// Scenario: User runs `wt prune -g`
// Expected: All repos' worktrees considered, no auto-prune without merged PRs
func TestPrune_Global_NoPR(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos
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
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Run with --global
	// Without merged PRs, nothing should be auto-pruned
	cmd := &PruneCmd{
		Global: true,
	}

	if err := runPruneCommand(t, repoA, cfg, cmd); err != nil {
		t.Fatalf("wt prune -g failed: %v", err)
	}

	// No worktrees removed (no merged PRs)
	verifyWorktreeExists(t, wtA)
	verifyWorktreeExists(t, wtB)
}

// TestPrune_ErrorForceWithoutID verifies error when -f is used without -n.
//
// Scenario: User runs `wt prune -f` without specifying -n
// Expected: Error "-f/--force requires -n/--number"
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
		t.Fatal("expected error when using -f without -n")
	}
	if !strings.Contains(err.Error(), "-f/--force requires -n/--number") {
		t.Errorf("expected '-f/--force requires -n/--number' error, got: %v", err)
	}
}

// TestPrune_ErrorVerboseWithID verifies error when --verbose is used with -n.
//
// Scenario: User runs `wt prune -n 1 --verbose`
// Expected: Error "--verbose cannot be used with -n/--number"
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
		t.Fatal("expected error when using --verbose with -n")
	}
	if !strings.Contains(err.Error(), "--verbose cannot be used with -n/--number") {
		t.Errorf("expected '--verbose cannot be used with -n/--number' error, got: %v", err)
	}
}

// TestPrune_ErrorInvalidID verifies error when specified ID doesn't exist.
//
// Scenario: User runs `wt prune -n 999`
// Expected: Error for invalid ID
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

// TestPrune_ErrorResetCacheWithID verifies error when --reset-cache is used with -n.
//
// Scenario: User runs `wt prune -n 1 --reset-cache`
// Expected: Error "--reset-cache cannot be used with --number"
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
		t.Fatal("expected error when using --reset-cache with -n")
	}
	if !strings.Contains(err.Error(), "--reset-cache cannot be used with --number") {
		t.Errorf("expected '--reset-cache cannot be used with --number' error, got: %v", err)
	}
}

// TestPrune_ByID_DryRun verifies dry run with -n flag doesn't remove worktree.
//
// Scenario: User runs `wt prune -n 1 -f -d`
// Expected: Worktree still exists (dry run)
func TestPrune_ByID_DryRun(t *testing.T) {
	t.Parallel()
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Dry run with force flag (required for -n)
	cmd := &PruneCmd{
		ID:     []int{1},
		Force:  true,
		DryRun: true,
	}

	if err := runPruneCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt prune -n 1 -f -d failed: %v", err)
	}

	// Verify worktree still exists (dry run)
	verifyWorktreeExists(t, worktreePath)
}

// TestPrune_ForceRemovesDirty verifies that -f flag removes dirty worktrees.
//
// Scenario: User runs `wt prune -n 1 -f` on dirty worktree
// Expected: Worktree removed despite uncommitted changes
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
		t.Fatalf("wt prune -n 1 -f failed: %v", err)
	}

	// Verify worktree was removed despite being dirty
	verifyWorktreeRemoved(t, worktreePath)
}

// runPruneCommand runs wt prune with the given config and command in the specified directory.
func runPruneCommand(t *testing.T, workDir string, cfg *config.Config, cmd *PruneCmd) error {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx := testContext(t)
	return cmd.runPrune(ctx)
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

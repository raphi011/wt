//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/registry"
)

// TestPrune_NoWorktrees tests pruning when no worktrees exist.
//
// Scenario: User runs `wt prune` in a repo with no extra worktrees
// Expected: Nothing to prune, no error
func TestPrune_NoWorktrees(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}
}

// TestPrune_WithWorktree tests pruning a worktree.
//
// Scenario: User runs `wt prune feature -f` to prune a specific worktree
// Expected: Worktree is removed
func TestPrune_WithWorktree(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Verify worktree exists first
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatal("worktree should exist before prune")
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}
}

// TestPrune_DryRun tests dry-run mode.
//
// Scenario: User runs `wt prune feature -d -f`
// Expected: Shows what would be pruned without actually pruning
func TestPrune_DryRun(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	// Need --force even in dry-run since targeting specific worktree
	cmd.SetArgs([]string{"feature", "-d", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Worktree should still exist (dry-run)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree should still exist after dry-run")
	}
}

// TestPrune_ByRepoName tests pruning in a specific repo.
//
// Scenario: User runs `wt prune myrepo:feature -f`
// Expected: Worktree is pruned from the specified repo
func TestPrune_ByRepoName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Work from a different directory
	otherDir := filepath.Join(tmpDir, "other")
	os.MkdirAll(otherDir, 0755)

	ctx := testContextWithConfig(t, cfg, otherDir)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo:feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}
}

// TestPrune_WithRepoBranchFormat tests pruning with repo:branch format.
//
// Scenario: User has two repos with same branch name, runs `wt prune repo1:feature -f`
// Expected: Only the worktree in the specified repo is pruned
func TestPrune_WithRepoBranchFormat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create two repos with the same branch name
	repo1Path := setupTestRepoWithBranches(t, tmpDir, "repo1", []string{"feature"})
	repo2Path := setupTestRepoWithBranches(t, tmpDir, "repo2", []string{"feature"})

	// Create worktrees in both repos
	wt1Path := createTestWorktree(t, repo1Path, "feature")
	wt2Path := createTestWorktree(t, repo2Path, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path},
			{Name: "repo2", Path: repo2Path},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Work from a different directory
	otherDir := filepath.Join(tmpDir, "other")
	os.MkdirAll(otherDir, 0755)

	ctx := testContextWithConfig(t, cfg, otherDir)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	// Use repo:branch format to target only repo1
	cmd.SetArgs([]string{"repo1:feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify only repo1's worktree was removed
	if _, err := os.Stat(wt1Path); err == nil {
		t.Error("repo1 worktree should be removed after prune")
	}

	// Verify repo2's worktree still exists
	if _, err := os.Stat(wt2Path); os.IsNotExist(err) {
		t.Error("repo2 worktree should NOT be removed")
	}
}

// TestPrune_RepoBranchFormat_RepoNotFound tests error when repo in repo:branch format is not found.
//
// Scenario: User runs `wt prune nonexistent:feature -f`
// Expected: Command fails with informative error
func TestPrune_RepoBranchFormat_RepoNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	// Use nonexistent repo name
	cmd.SetArgs([]string{"nonexistent:feature", "-f"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo")
	}

	// Error should mention the repo name
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention nonexistent repo, got: %v", err)
	}
}

// TestPrune_DeleteBranchesFlag tests that --delete-branches deletes local branch.
//
// Scenario: User runs `wt prune feature -f --delete-branches`
// Expected: Both worktree and local branch are removed
func TestPrune_DeleteBranchesFlag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Verify worktree and branch exist before prune
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatal("worktree should exist before prune")
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f", "--delete-branches"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}

	// Verify branch was deleted
	output, err := runGitCommand(repoPath, "branch", "--list", "feature")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if strings.TrimSpace(output) != "" {
		t.Error("branch should be deleted after prune with --delete-branches")
	}
}

// TestPrune_NoDeleteBranchesDefault tests that branches are kept by default.
//
// Scenario: User runs `wt prune feature -f` without --delete-branches
// Expected: Worktree is removed but local branch is kept
func TestPrune_NoDeleteBranchesDefault(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}

	// Verify branch was NOT deleted
	output, err := runGitCommand(repoPath, "branch", "--list", "feature")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if strings.TrimSpace(output) == "" {
		t.Error("branch should still exist after prune without --delete-branches")
	}
}

// TestPrune_ConfigDeleteBranches tests that config option enables branch deletion.
//
// Scenario: User has delete_local_branches=true in config, runs `wt prune feature -f`
// Expected: Both worktree and local branch are removed
func TestPrune_ConfigDeleteBranches(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Set config to delete branches by default
	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			DeleteLocalBranches: true,
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}

	// Verify branch was deleted (due to config)
	output, err := runGitCommand(repoPath, "branch", "--list", "feature")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if strings.TrimSpace(output) != "" {
		t.Error("branch should be deleted after prune with delete_local_branches=true in config")
	}
}

// TestPrune_NoDeleteBranchesOverridesConfig tests that --no-delete-branches overrides config.
//
// Scenario: User has delete_local_branches=true in config, runs `wt prune feature -f --no-delete-branches`
// Expected: Worktree is removed but local branch is kept
func TestPrune_NoDeleteBranchesOverridesConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Set config to delete branches by default
	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			DeleteLocalBranches: true,
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f", "--no-delete-branches"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}

	// Verify branch was NOT deleted (--no-delete-branches overrides config)
	output, err := runGitCommand(repoPath, "branch", "--list", "feature")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if strings.TrimSpace(output) == "" {
		t.Error("branch should still exist after prune with --no-delete-branches")
	}
}

// TestPrune_DeleteBranches_UnmergedBranch tests that unmerged branches survive safe delete.
//
// Scenario: User creates a branch with a unique commit (not in main), creates a worktree,
//
//	then runs `wt prune feature -f --delete-branches`
//
// Expected: Worktree is removed, but branch is kept because git branch -d refuses
//
//	(the branch has commits not reachable from main)
func TestPrune_DeleteBranches_UnmergedBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	// Add a unique commit on the feature branch (not in main)
	// This makes git branch -d refuse to delete it
	addCommit(t, wtPath, "feature-only.txt", "feature-only commit")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	// Targeted prune (no forge PR state) → safe delete (-d) is used
	cmd.SetArgs([]string{"feature", "-f", "--delete-branches"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}

	// Verify branch was NOT deleted (safe delete refuses because commits aren't merged)
	output, err := runGitCommand(repoPath, "branch", "--list", "feature")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if strings.TrimSpace(output) == "" {
		t.Error("branch should survive safe delete when it has unmerged commits")
	}
}

// TestPrune_DryRun_DoesNotDeleteBranch tests that dry-run preserves both worktree and branch.
//
// Scenario: User runs `wt prune feature -f -d --delete-branches`
// Expected: Neither worktree nor branch are removed (dry-run)
func TestPrune_DryRun_DoesNotDeleteBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f", "-d", "--delete-branches"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree still exists (dry-run)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree should still exist after dry-run")
	}

	// Verify branch still exists (dry-run)
	output, err := runGitCommand(repoPath, "branch", "--list", "feature")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if strings.TrimSpace(output) == "" {
		t.Error("branch should still exist after dry-run")
	}
}

// TestPrune_DeleteBranchesFlag_OverridesConfigFalse tests that --delete-branches flag
// overrides config delete_local_branches=false.
//
// Scenario: Config has delete_local_branches=false, user runs `wt prune feature -f --delete-branches`
// Expected: Branch is deleted (explicit flag wins over config)
func TestPrune_DeleteBranchesFlag_OverridesConfigFalse(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Config explicitly disables branch deletion
	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			DeleteLocalBranches: false,
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	// Explicit --delete-branches flag should override config
	cmd.SetArgs([]string{"feature", "-f", "--delete-branches"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}

	// Verify branch was deleted (explicit flag overrides config=false)
	output, err := runGitCommand(repoPath, "branch", "--list", "feature")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if strings.TrimSpace(output) != "" {
		t.Error("branch should be deleted when --delete-branches flag is explicitly passed")
	}
}

// TestPrune_UnscopedTarget_OnlyCurrentRepo tests that `wt prune feature -f` (without -g)
// only removes the worktree in the current repo, not in other repos.
//
// Scenario: Two repos both have a "feature" worktree, user runs from repo1
// Expected: Only repo1's worktree is removed
func TestPrune_UnscopedTarget_OnlyCurrentRepo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repo1Path := setupTestRepoWithBranches(t, tmpDir, "repo1", []string{"feature"})
	repo2Path := setupTestRepoWithBranches(t, tmpDir, "repo2", []string{"feature"})

	wt1Path := createTestWorktree(t, repo1Path, "feature")
	wt2Path := createTestWorktree(t, repo2Path, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path},
			{Name: "repo2", Path: repo2Path},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Run from repo1 (workDir = repo1Path)
	ctx := testContextWithConfig(t, cfg, repo1Path)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// repo1's worktree should be removed
	if _, err := os.Stat(wt1Path); err == nil {
		t.Error("repo1 worktree should be removed after prune")
	}

	// repo2's worktree should still exist
	if _, err := os.Stat(wt2Path); os.IsNotExist(err) {
		t.Error("repo2 worktree should NOT be removed (not in scope)")
	}
}

// TestPrune_UnscopedTarget_GlobalFlag tests that `wt prune feature -f -g`
// removes worktrees from all repos.
//
// Scenario: Two repos both have a "feature" worktree, user runs with -g
// Expected: Both worktrees are removed
func TestPrune_UnscopedTarget_GlobalFlag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repo1Path := setupTestRepoWithBranches(t, tmpDir, "repo1", []string{"feature"})
	repo2Path := setupTestRepoWithBranches(t, tmpDir, "repo2", []string{"feature"})

	wt1Path := createTestWorktree(t, repo1Path, "feature")
	wt2Path := createTestWorktree(t, repo2Path, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path},
			{Name: "repo2", Path: repo2Path},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Run from repo1 with -g flag
	ctx := testContextWithConfig(t, cfg, repo1Path)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f", "-g"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Both worktrees should be removed
	if _, err := os.Stat(wt1Path); err == nil {
		t.Error("repo1 worktree should be removed after global prune")
	}
	if _, err := os.Stat(wt2Path); err == nil {
		t.Error("repo2 worktree should be removed after global prune")
	}
}

// TestPrune_UnscopedTarget_NotInRepo_FallsBackToAll tests that running from a
// non-repo directory (without -g) falls back to searching all repos.
//
// Scenario: Two repos both have a "feature" worktree, user runs from non-repo dir
// Expected: Both worktrees are removed (fallback to all repos)
func TestPrune_UnscopedTarget_NotInRepo_FallsBackToAll(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repo1Path := setupTestRepoWithBranches(t, tmpDir, "repo1", []string{"feature"})
	repo2Path := setupTestRepoWithBranches(t, tmpDir, "repo2", []string{"feature"})

	wt1Path := createTestWorktree(t, repo1Path, "feature")
	wt2Path := createTestWorktree(t, repo2Path, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path},
			{Name: "repo2", Path: repo2Path},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Run from a non-repo directory
	otherDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create non-repo dir: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, otherDir)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Both worktrees should be removed (fallback to all repos)
	if _, err := os.Stat(wt1Path); err == nil {
		t.Error("repo1 worktree should be removed (fallback to all repos)")
	}
	if _, err := os.Stat(wt2Path); err == nil {
		t.Error("repo2 worktree should be removed (fallback to all repos)")
	}
}

// TestPrune_ForceDeleteBranch_MergedPRState tests that branches with unmerged commits
// are force-deleted when PRState is MERGED (the squash-merge scenario).
//
// Scenario: A branch has commits not reachable from main (like after a squash merge on GitHub).
//
//	The forge reports the PR as merged, so pruneWorktrees should use git branch -D.
//
// Expected: Branch is deleted despite having unmerged commits.
func TestPrune_ForceDeleteBranch_MergedPRState(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	// Add a unique commit on the feature branch (not in main).
	// This simulates a squash-merged PR: GitHub merged the PR, but the local
	// branch has commits not reachable from main, so git branch -d would refuse.
	addCommit(t, wtPath, "feature-only.txt", "feature-only commit")

	cfg := &config.Config{}
	ctx := testContextWithConfig(t, cfg, repoPath)

	// Construct git.Worktree with PRState = MERGED (as if forge confirmed merge)
	toRemove := []git.Worktree{
		{
			Path:     wtPath,
			Branch:   "feature",
			RepoName: "test-repo",
			RepoPath: repoPath,
			PRState:  forge.PRStateMerged,
		},
	}

	removed, failed := pruneWorktrees(ctx, toRemove, pruneOpts{
		Force:                  true,
		DeleteBranches:         true,
		DeleteBranchesExplicit: true,
	})

	if len(failed) > 0 {
		t.Fatalf("expected no failures, got %d", len(failed))
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removed))
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}

	// Verify branch was force-deleted (git branch -D succeeds for unmerged commits)
	output, err := runGitCommand(repoPath, "branch", "--list", "feature")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if strings.TrimSpace(output) != "" {
		t.Error("branch should be force-deleted when PRState is MERGED (squash-merge scenario)")
	}
}

// TestPrune_StaleFlag_RemovesOldWorktrees tests that --stale removes worktrees
// with old commits.
//
// Scenario: A worktree has a commit from 30 days ago, StaleDays=1
// Expected: Worktree is removed with --stale flag
func TestPrune_StaleFlag_RemovesOldWorktrees(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"stale-branch"})
	wtPath := createTestWorktree(t, repoPath, "stale-branch")

	// Backdate the commit in the worktree to 30 days ago
	addCommitWithDate(t, wtPath, "old-file.txt", "old commit", "2020-01-01T00:00:00+00:00")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			StaleDays: 1, // 1 day, so the 30-day-old commit is definitely stale
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--stale"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune --stale command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("stale worktree should be removed with --stale flag")
	}
}

// TestPrune_StaleFlag_KeepsFreshWorktrees tests that --stale keeps fresh worktrees.
//
// Scenario: A worktree has a recent commit, StaleDays=14
// Expected: Worktree is kept (not stale)
func TestPrune_StaleFlag_KeepsFreshWorktrees(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"fresh-branch"})
	wtPath := createTestWorktree(t, repoPath, "fresh-branch")

	// Add a recent commit (no date override = now)
	addCommit(t, wtPath, "new-file.txt", "recent commit")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			StaleDays: 14,
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--stale"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune --stale command failed: %v", err)
	}

	// Verify worktree still exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("fresh worktree should NOT be removed with --stale")
	}
}

// TestPrune_StaleFlag_MergedAlwaysPruned tests that merged PRs are always pruned
// regardless of --stale flag.
//
// Scenario: A worktree has a merged PR (via PR cache), no --stale flag
// Expected: Worktree is removed (merged PRs always pruned)
func TestPrune_StaleFlag_MergedAlwaysPruned(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"merged-branch"})
	wtPath := createTestWorktree(t, repoPath, "merged-branch")

	// Add a recent commit (not stale)
	addCommit(t, wtPath, "merged-file.txt", "merged commit")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			StaleDays: 14,
		},
	}

	// Directly test pruneWorktrees with a merged worktree
	ctx := testContextWithConfig(t, cfg, repoPath)
	toRemove := []git.Worktree{
		{
			Path:     wtPath,
			Branch:   "merged-branch",
			RepoName: "test-repo",
			RepoPath: repoPath,
			PRState:  forge.PRStateMerged,
		},
	}

	removed, failed := pruneWorktrees(ctx, toRemove, pruneOpts{
		Force: true,
	})

	if len(failed) > 0 {
		t.Fatalf("expected no failures, got %d", len(failed))
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removed))
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("merged worktree should always be removed (without --stale)")
	}
}

// TestPrune_StaleFlag_DryRun tests that --stale with --dry-run doesn't remove.
//
// Scenario: A stale worktree exists, user runs `wt prune --stale -d`
// Expected: Worktree survives (dry-run)
func TestPrune_StaleFlag_DryRun(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"stale-branch"})
	wtPath := createTestWorktree(t, repoPath, "stale-branch")

	// Backdate the commit
	addCommitWithDate(t, wtPath, "old-file.txt", "old commit", "2020-01-01T00:00:00+00:00")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			StaleDays: 1,
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--stale", "-d"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune --stale -d command failed: %v", err)
	}

	// Verify worktree still exists (dry-run)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("stale worktree should survive dry-run")
	}
}

// TestPrune_StaleFlag_Disabled tests that StaleDays=0 disables stale pruning.
//
// Scenario: StaleDays=0, worktree is very old, --stale flag is set
// Expected: Worktree is NOT removed (stale detection disabled)
func TestPrune_StaleFlag_Disabled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"old-branch"})
	wtPath := createTestWorktree(t, repoPath, "old-branch")

	// Backdate the commit
	addCommitWithDate(t, wtPath, "old-file.txt", "old commit", "2020-01-01T00:00:00+00:00")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			StaleDays: 0, // Disabled
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--stale"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune --stale command failed: %v", err)
	}

	// Verify worktree still exists (stale detection disabled)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree should NOT be removed when StaleDays=0")
	}
}

// TestPrune_WithoutStaleFlag_KeepsStaleWorktrees tests that stale worktrees
// are NOT pruned when the --stale flag is omitted.
//
// Scenario: A worktree has a very old commit, StaleDays=1, but --stale is NOT passed
// Expected: Worktree survives (stale pruning is opt-in)
func TestPrune_WithoutStaleFlag_KeepsStaleWorktrees(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"stale-branch"})
	wtPath := createTestWorktree(t, repoPath, "stale-branch")

	// Backdate the commit to make it stale
	addCommitWithDate(t, wtPath, "old-file.txt", "old commit", "2020-01-01T00:00:00+00:00")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			StaleDays: 1, // Would be stale, but --stale not passed
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{}) // No --stale flag

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree still exists (stale pruning is opt-in)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("stale worktree should NOT be removed without --stale flag")
	}
}

// TestPrune_LocalConfigOverridesDeleteBranches tests that a per-repo .wt.toml
// overrides the global config's delete_local_branches setting.
//
// Scenario: Global config has delete_local_branches=false, local .wt.toml sets it to true.
//
//	User runs `wt prune feature -f` (no explicit --delete-branches flag).
//
// Expected: Branch is deleted because the local config override takes effect.
func TestPrune_LocalConfigOverridesDeleteBranches(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Global config: delete_local_branches = false
	cfg := &config.Config{
		RegistryPath: regFile,
		Prune: config.PruneConfig{
			DeleteLocalBranches: false,
		},
	}

	// Write local .wt.toml that overrides delete_local_branches to true
	localConfig := []byte("[prune]\ndelete_local_branches = true\n")
	if err := os.WriteFile(filepath.Join(repoPath, ".wt.toml"), localConfig, 0644); err != nil {
		t.Fatalf("failed to write .wt.toml: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}

	// Verify branch was deleted (local config override takes effect)
	output, err := runGitCommand(repoPath, "branch", "--list", "feature")
	if err != nil {
		t.Fatalf("failed to list branches: %v", err)
	}
	if strings.TrimSpace(output) != "" {
		t.Error("branch should be deleted when local .wt.toml sets delete_local_branches=true")
	}
}

// TestPrune_AfterHookRuns tests that a prune hook with on=["prune"] fires after pruning.
//
// Scenario: User runs `wt prune feature -f` with a hook on=["prune"]
// Expected: Worktree is removed and hook runs
func TestPrune_AfterHookRuns(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "prune-hook-ran")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"cleanup": {
					Command: "touch " + markerPath,
					On:      []string{"prune"},
				},
			},
		},
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatal("worktree should exist before prune")
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune")
	}

	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("prune hook should have run")
	}
}

// TestPrune_BeforeHookAborts tests that a failing before:prune hook prevents removal.
//
// Scenario: User has a before:prune hook that exits 1
// Expected: Worktree is NOT removed
func TestPrune_BeforeHookAborts(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"guard": {
					Command: "exit 1",
					On:      []string{"before:prune"},
				},
			},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	// Prune should succeed (before-hook abort is logged, not fatal for the command)
	// but the worktree should still exist because the abort skips that worktree
	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	// Worktree should still exist — before hook aborted its removal
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree should still exist after before-hook abort")
	}
}

// TestPrune_BeforeHookCWD tests that before:prune hooks run in the worktree directory.
//
// Scenario: Before-prune hook writes pwd to a file
// Expected: Output matches the worktree path
func TestPrune_BeforeHookCWD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "before-pwd.txt")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"pwd-before": {
					Command: "pwd > " + outputPath,
					On:      []string{"before:prune"},
				},
			},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read pwd output: %v", err)
	}

	got := strings.TrimSpace(string(content))
	if got != wtPath {
		t.Errorf("before-prune hook CWD should be worktree path\nexpected: %s\ngot:      %s", wtPath, got)
	}
}

// TestPrune_AfterHookCWD tests that after:prune hooks run in the repo root directory.
//
// Scenario: After-prune hook writes pwd to a file
// Expected: Output matches the repo root path (worktree is already deleted)
func TestPrune_AfterHookCWD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "after-pwd.txt")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"pwd-after": {
					Command: "pwd > " + outputPath,
					On:      []string{"prune"},
				},
			},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read pwd output: %v", err)
	}

	got := strings.TrimSpace(string(content))
	if got != repoPath {
		t.Errorf("after-prune hook CWD should be repo root\nexpected: %s\ngot:      %s", repoPath, got)
	}
}

// TestPrune_AllTriggerMatchesPrune tests that on=["all"] matches prune.
//
// Scenario: Hook with on=["all"], user prunes a worktree
// Expected: Hook fires
func TestPrune_AllTriggerMatchesPrune(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "all-hook-ran")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"catch-all": {
					Command: "touch " + markerPath,
					On:      []string{"all"},
				},
			},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("hook with on=[\"all\"] should have run for prune")
	}
}

// TestPrune_Placeholders tests that prune hooks get correct placeholder values.
//
// Scenario: After-prune hook writes trigger/phase/branch to a file
// Expected: File contains "prune after feature"
func TestPrune_Placeholders(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "placeholders.txt")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"placeholder-test": {
					Command: "echo {trigger} {phase} {branch} > " + outputPath,
					On:      []string{"prune"},
				},
			},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read placeholder output: %v", err)
	}

	got := strings.TrimSpace(string(content))
	if got != "prune after feature" {
		t.Errorf("expected 'prune after feature', got %q", got)
	}
}

// TestPrune_NoHookFlag tests that --no-hook suppresses prune hooks.
//
// Scenario: User runs `wt prune feature -f --no-hook` with a default prune hook
// Expected: Worktree is removed but hook does not run
func TestPrune_NoHookFlag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "should-not-exist")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"cleanup": {
					Command: "touch " + markerPath,
					On:      []string{"prune"},
				},
			},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f", "--no-hook"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	if _, err := os.Stat(markerPath); err == nil {
		t.Error("hook should NOT have run with --no-hook flag")
	}
}

// TestPrune_ExplicitHookFlag tests that --hook runs only the named hook.
//
// Scenario: User runs `wt prune feature -f --hook myhook` with two hooks
// Expected: Only the named hook runs, not the default one
func TestPrune_ExplicitHookFlag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	explicitMarker := filepath.Join(tmpDir, "explicit-hook-ran")
	defaultMarker := filepath.Join(tmpDir, "default-hook-ran")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command: "touch " + explicitMarker,
				},
				"default-cleanup": {
					Command: "touch " + defaultMarker,
					On:      []string{"prune"},
				},
			},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-f", "--hook", "myhook"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune command failed: %v", err)
	}

	if _, err := os.Stat(explicitMarker); os.IsNotExist(err) {
		t.Error("explicit hook should have run")
	}

	if _, err := os.Stat(defaultMarker); err == nil {
		t.Error("default hook should NOT have run when --hook is used")
	}
}

// TestPrune_LocallyMergedBranch_RequiresForce tests that a branch merged via
// git ancestry alone still requires --force, since only forge-confirmed PR merges
// allow force-free pruning.
//
// Scenario: User runs `wt prune feature` where feature is merged into main via
// git but has no PR cache entry
// Expected: Error requiring -f
func TestPrune_LocallyMergedBranch_RequiresForce(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	// Create feature branch with a commit, then merge it into main
	if _, err := runGitCommand(repoPath, "checkout", "-b", "feature"); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}
	addCommit(t, repoPath, "feature.txt", "feature commit")

	// Merge feature into main
	if _, err := runGitCommand(repoPath, "checkout", "main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := runGitCommand(repoPath, "merge", "feature", "--no-ff", "-m", "Merge feature"); err != nil {
		t.Fatalf("failed to merge feature: %v", err)
	}

	// Create worktree for the (git-merged) feature branch
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	// Without -f: should fail — git ancestry alone is not sufficient
	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("prune should require -f when there is no forge-confirmed PR merge")
	}
	if !strings.Contains(err.Error(), "cannot prune unmerged worktrees without -f/--force") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify worktree was NOT removed
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree should not be removed without -f")
	}

	// With -f: should succeed
	cmd2 := newPruneCmd()
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{"feature", "-f"})

	if err := cmd2.Execute(); err != nil {
		t.Fatalf("prune with -f should succeed: %v", err)
	}

	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after prune -f")
	}
}

// TestPrune_UnmergedBranch_RequiresForce tests that unmerged branches require
// --force for targeted prune.
//
// Scenario: User runs `wt prune feature` where feature has unique commits
// Expected: Error listing the unmerged branch, suggesting -f
func TestPrune_UnmergedBranch_RequiresForce(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	// Add a commit on the feature branch so it's not merged
	addCommit(t, wtPath, "feature-only.txt", "unmerged commit")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newPruneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"}) // No -f flag

	err := cmd.Execute()
	if err == nil {
		t.Fatal("prune should fail without -f for unmerged branch")
	}
	if !strings.Contains(err.Error(), "cannot prune unmerged worktrees without -f/--force") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "test-repo:feature") {
		t.Fatalf("error should list the unmerged branch: %v", err)
	}

	// Verify worktree was NOT removed
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree should not be removed when force is missing")
	}
}

// TestPrune_MixedTargets_RequiresForce tests that mixed merged/unmerged
// targets require --force, listing only the unmerged ones.
//
// Scenario: User runs `wt prune merged unmerged` where only one is merged
// Expected: Error listing only the unmerged branch
func TestPrune_MixedTargets_RequiresForce(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	// Create and merge the "merged" branch
	if _, err := runGitCommand(repoPath, "checkout", "-b", "merged"); err != nil {
		t.Fatalf("failed to create merged branch: %v", err)
	}
	addCommit(t, repoPath, "merged.txt", "merged commit")

	if _, err := runGitCommand(repoPath, "checkout", "main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := runGitCommand(repoPath, "merge", "merged", "--no-ff", "-m", "Merge merged"); err != nil {
		t.Fatalf("failed to merge merged branch: %v", err)
	}

	// Create the "unmerged" branch with a unique commit
	if _, err := runGitCommand(repoPath, "checkout", "-b", "unmerged"); err != nil {
		t.Fatalf("failed to create unmerged branch: %v", err)
	}
	addCommit(t, repoPath, "unmerged.txt", "unmerged commit")

	// Switch back to main before creating worktrees
	if _, err := runGitCommand(repoPath, "checkout", "main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	// Create worktrees for both branches
	mergedWtPath := createTestWorktree(t, repoPath, "merged")
	unmergedWtPath := createTestWorktree(t, repoPath, "unmerged")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	pruneCmd := newPruneCmd()
	pruneCmd.SetContext(ctx)
	pruneCmd.SetArgs([]string{"merged", "unmerged"}) // No -f flag

	err := pruneCmd.Execute()
	if err == nil {
		t.Fatal("prune should fail when any target is unmerged")
	}
	if !strings.Contains(err.Error(), "test-repo:unmerged") {
		t.Fatalf("error should list the unmerged branch: %v", err)
	}
	// Without a forge-confirmed PR merge in the cache, git-merged branches also
	// require -f — so both branches appear in the error.
	if !strings.Contains(err.Error(), "test-repo:merged") {
		t.Fatalf("error should list the git-merged branch too (no PR cache entry): %v", err)
	}

	// Verify neither worktree was removed (error returned before any removal)
	if _, err := os.Stat(mergedWtPath); os.IsNotExist(err) {
		t.Error("merged worktree should not be removed on error")
	}
	if _, err := os.Stat(unmergedWtPath); os.IsNotExist(err) {
		t.Error("unmerged worktree should not be removed on error")
	}
}

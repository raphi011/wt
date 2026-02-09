//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
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
//           then runs `wt prune feature -f --delete-branches`
// Expected: Worktree is removed, but branch is kept because git branch -d refuses
//           (the branch has commits not reachable from main)
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
	os.MkdirAll(filepath.Dir(regFile), 0755)

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

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
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	oldDir, _ := os.Getwd()
	os.Chdir(repoPath)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
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
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	oldDir, _ := os.Getwd()
	os.Chdir(repoPath)
	defer os.Chdir(oldDir)

	// Verify worktree exists first
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatal("worktree should exist before prune")
	}

	ctx := testContext(t)
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
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	oldDir, _ := os.Getwd()
	os.Chdir(repoPath)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
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
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	// Work from a different directory
	workDir := filepath.Join(tmpDir, "other")
	os.MkdirAll(workDir, 0755)

	oldDir, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
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
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create two repos with the same branch name
	repo1Path := setupTestRepoWithBranches(t, tmpDir, "repo1", []string{"feature"})
	repo2Path := setupTestRepoWithBranches(t, tmpDir, "repo2", []string{"feature"})

	// Create worktrees in both repos
	wt1Path := createTestWorktree(t, repo1Path, "feature")
	wt2Path := createTestWorktree(t, repo2Path, "feature")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path},
			{Name: "repo2", Path: repo2Path},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	// Work from a different directory
	workDir := filepath.Join(tmpDir, "other")
	os.MkdirAll(workDir, 0755)

	oldDir, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
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
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	oldDir, _ := os.Getwd()
	os.Chdir(repoPath)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
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

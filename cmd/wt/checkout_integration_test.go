//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestCheckout_ExistingBranch tests checking out an existing branch.
//
// Scenario: User runs `wt checkout feature` in a repo with the feature branch
// Expected: Worktree is created for the branch
func TestCheckout_ExistingBranch(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create repo with a feature branch
	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})

	// Setup registry directory
	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Register the repo
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Set config
	oldCfg := cfg
	cfg = &config.Config{
		WorktreeFormat: "../{repo}-{branch}",
	}
	defer func() { cfg = oldCfg }()

	// Change to repo directory
	oldDir, _ := os.Getwd()
	os.Chdir(repoPath)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created as sibling
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree should exist at %s", wtPath)
	}

	// Verify it's on the correct branch
	branch := getGitBranch(t, wtPath)
	if branch != "feature" {
		t.Errorf("expected branch 'feature', got %q", branch)
	}
}

// TestCheckout_NewBranch tests creating a new branch.
//
// Scenario: User runs `wt checkout -b new-feature`
// Expected: New branch and worktree are created
func TestCheckout_NewBranch(t *testing.T) {
	// Not parallel - modifies HOME

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
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{
		WorktreeFormat: "../{repo}-{branch}",
		BaseRef:        "local", // Use local branches, not origin/
	}
	defer func() { cfg = oldCfg }()

	oldDir, _ := os.Getwd()
	os.Chdir(repoPath)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "new-feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created as sibling
	wtPath := filepath.Join(tmpDir, "test-repo-new-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree should exist at %s", wtPath)
	}

	// Verify it's on the new branch
	branch := getGitBranch(t, wtPath)
	if branch != "new-feature" {
		t.Errorf("expected branch 'new-feature', got %q", branch)
	}
}

// TestCheckout_ByRepoName tests checkout in a specific repo by name.
//
// Scenario: User runs `wt checkout feature -r myrepo`
// Expected: Worktree created in the specified repo
func TestCheckout_ByRepoName(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{
		WorktreeFormat: "../{repo}-{branch}",
	}
	defer func() { cfg = oldCfg }()

	// Work from a different directory
	workDir := filepath.Join(tmpDir, "other")
	os.MkdirAll(workDir, 0755)

	oldDir, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "-r", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created as sibling
	wtPath := filepath.Join(tmpDir, "myrepo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree should exist at %s", wtPath)
	}
}

// TestCheckout_ByLabel tests checkout in repos by label.
//
// Scenario: User runs `wt checkout -b feature -l backend`
// Expected: Worktree created in all repos with backend label
func TestCheckout_ByLabel(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create two repos, one with backend label
	repo1Path := setupTestRepo(t, tmpDir, "api-server")
	repo2Path := setupTestRepo(t, tmpDir, "web-client")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "api-server", Path: repo1Path, Labels: []string{"backend"}, WorktreeFormat: "../{repo}-{branch}"},
			{Name: "web-client", Path: repo2Path, Labels: []string{"frontend"}, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{
		WorktreeFormat: "../{repo}-{branch}",
		BaseRef:        "local",
	}
	defer func() { cfg = oldCfg }()

	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	oldDir, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "-l", "backend"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created for api-server (backend label)
	wtPath := filepath.Join(tmpDir, "api-server-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree should exist at %s", wtPath)
	}

	// Verify no worktree for web-client (frontend label)
	wtPath2 := filepath.Join(tmpDir, "web-client-feature")
	if _, err := os.Stat(wtPath2); !os.IsNotExist(err) {
		t.Errorf("worktree should NOT exist at %s", wtPath2)
	}
}

// TestCheckout_SlashBranchName tests checkout with slash in branch name.
//
// Scenario: User runs `wt checkout feature/auth`
// Expected: Worktree created with sanitized path name
func TestCheckout_SlashBranchName(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature/auth"})

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{
		WorktreeFormat: "../{repo}-{branch}",
	}
	defer func() { cfg = oldCfg }()

	oldDir, _ := os.Getwd()
	os.Chdir(repoPath)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature/auth"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created with sanitized name (/ -> -)
	wtPath := filepath.Join(tmpDir, "test-repo-feature-auth")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree should exist at %s", wtPath)
	}
}

// TestCheckout_NotInRepo tests that checkout fails when not in repo and no -r flag.
//
// Scenario: User runs `wt checkout branch` outside of any git repo
// Expected: Command fails with error
func TestCheckout_NotInRepo(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Empty registry
	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	// Work from a non-repo directory
	workDir := filepath.Join(tmpDir, "not-a-repo")
	os.MkdirAll(workDir, 0755)

	oldDir, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when not in repo")
	}
}

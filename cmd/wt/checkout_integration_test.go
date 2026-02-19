//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/preserve"
	"github.com/raphi011/wt/internal/registry"
)

// TestCheckout_ExistingBranch tests checking out an existing branch.
//
// Scenario: User runs `wt checkout feature` in a repo with the feature branch
// Expected: Worktree is created for the branch
func TestCheckout_ExistingBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create repo with a feature branch
	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})

	// Setup registry file
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	// Register the repo
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Create context with config
	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local", // Use local branches, not origin/
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
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
// Scenario: User runs `wt checkout myrepo:feature`
// Expected: Worktree created in the specified repo
func TestCheckout_ByRepoName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}

	// Work from a different directory
	otherDir := filepath.Join(tmpDir, "other")
	os.MkdirAll(otherDir, 0755)

	ctx := testContextWithConfig(t, cfg, otherDir)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo:feature"})

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
// Scenario: User runs `wt checkout -b backend:feature`
// Expected: Worktree created in all repos with backend label
func TestCheckout_ByLabel(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create two repos, one with backend label
	repo1Path := setupTestRepo(t, tmpDir, "api-server")
	repo2Path := setupTestRepo(t, tmpDir, "web-client")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "api-server", Path: repo1Path, Labels: []string{"backend"}, WorktreeFormat: "../{repo}-{branch}"},
			{Name: "web-client", Path: repo2Path, Labels: []string{"frontend"}, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
	}

	workingDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workingDir, 0755)

	ctx := testContextWithConfig(t, cfg, workingDir)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "backend:feature"})

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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature/auth"})

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
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

// TestCheckout_NotInRepo tests that checkout fails when not in repo.
//
// Scenario: User runs `wt checkout branch` outside of any git repo
// Expected: Command fails with error
func TestCheckout_NotInRepo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	// Empty registry
	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
	}

	// Work from a non-repo directory
	notARepoDir := filepath.Join(tmpDir, "not-a-repo")
	os.MkdirAll(notARepoDir, 0755)

	ctx := testContextWithConfig(t, cfg, notARepoDir)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when not in repo")
	}
}

// TestCheckout_NewBranchPushesAndSetsUpstream tests that new branches are pushed and get upstream set.
//
// Scenario: User runs `wt checkout -b feature` with set_upstream = true
// Expected: Branch is pushed to origin and upstream tracking is set
func TestCheckout_NewBranchPushesAndSetsUpstream(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath, _ := setupTestRepoWithOrigin(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	setUpstreamTrue := true
	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
			SetUpstream:    &setUpstreamTrue,
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify upstream is set (branch was pushed)
	upstream := getGitUpstream(t, repoPath, "feature")
	if upstream != "refs/heads/feature" {
		t.Errorf("expected upstream 'refs/heads/feature', got %q", upstream)
	}
}

// TestCheckout_ExistingBranchWithRemoteSetsUpstream tests upstream for existing remote branches.
//
// Scenario: User runs `wt checkout feature` where feature exists on origin, with set_upstream = true
// Expected: Upstream tracking is set to origin/feature
func TestCheckout_ExistingBranchWithRemoteSetsUpstream(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath, _ := setupTestRepoWithOrigin(t, tmpDir, "test-repo")

	// Create and push a branch to origin
	pushBranchToOrigin(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	setUpstreamTrue := true
	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			SetUpstream:    &setUpstreamTrue,
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify upstream is set
	upstream := getGitUpstream(t, repoPath, "feature")
	if upstream != "refs/heads/feature" {
		t.Errorf("expected upstream 'refs/heads/feature', got %q", upstream)
	}
}

// TestCheckout_LocalOnlyBranchNoUpstream tests that local-only branches don't get upstream.
//
// Scenario: User runs `wt checkout local-only` where branch only exists locally, with set_upstream = true
// Expected: No upstream is set (remote branch doesn't exist)
func TestCheckout_LocalOnlyBranchNoUpstream(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath, _ := setupTestRepoWithOrigin(t, tmpDir, "test-repo")

	// Create a local branch without pushing
	runGitCommand(repoPath, "branch", "local-only")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	setUpstreamTrue := true
	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			SetUpstream:    &setUpstreamTrue,
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"local-only"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify no upstream is set (branch only exists locally, not a new branch so won't push)
	upstream := getGitUpstream(t, repoPath, "local-only")
	if upstream != "" {
		t.Errorf("expected no upstream for local-only branch, got %q", upstream)
	}
}

// TestCheckout_SetUpstreamDisabled tests that upstream is not set when disabled.
//
// Scenario: User runs `wt checkout -b feature` with set_upstream = false
// Expected: No upstream tracking is set
func TestCheckout_SetUpstreamDisabled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath, _ := setupTestRepoWithOrigin(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	setUpstreamFalse := false
	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
			SetUpstream:    &setUpstreamFalse, // Explicitly disabled
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "no-upstream-feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify no upstream is set
	upstream := getGitUpstream(t, repoPath, "no-upstream-feature")
	if upstream != "" {
		t.Errorf("expected no upstream when disabled, got %q", upstream)
	}
}

// TestCheckout_NoOriginNoUpstream tests checkout works without origin remote.
//
// Scenario: User runs `wt checkout -b feature` in repo without origin
// Expected: Worktree created, no upstream (no error)
func TestCheckout_NoOriginNoUpstream(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Use setupTestRepo which adds a fake origin URL but no actual remote
	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	// Remove the origin remote
	runGitCommand(repoPath, "remote", "remove", "origin")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify no upstream (no origin remote)
	upstream := getGitUpstream(t, repoPath, "feature")
	if upstream != "" {
		t.Errorf("expected no upstream without origin, got %q", upstream)
	}
}

// TestCheckout_AlreadyCheckedOut_ScopedTarget tests checkout blocking with repo:branch syntax.
//
// Scenario: User runs `wt checkout myrepo:feature` when feature already has a worktree
// Expected: Command fails with error from git indicating branch is already checked out
func TestCheckout_AlreadyCheckedOut_ScopedTarget(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}

	// Work from a different directory (not inside repo)
	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)
	ctx := testContextWithConfig(t, cfg, workDir)

	// First checkout with scoped target should succeed
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo:feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("first checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "myrepo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Second checkout with scoped target should fail
	cmd2 := newCheckoutCmd()
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{"myrepo:feature"})

	err := cmd2.Execute()
	if err == nil {
		t.Fatal("expected error when checking out already checked-out branch with scoped target")
	}

	// Verify error mentions the branch (git error for existing worktree)
	errStr := err.Error()
	if !strings.Contains(errStr, "feature") {
		t.Errorf("error should mention branch name 'feature', got: %s", errStr)
	}
}

// TestCheckout_AlreadyCheckedOut tests that checkout fails for already checked-out branches.
//
// Scenario: User runs `wt checkout feature` when feature already has a worktree
// Expected: Command fails with error indicating branch is already checked out
func TestCheckout_AlreadyCheckedOut(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

	// First checkout should succeed
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("first checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Second checkout of same branch should fail
	cmd2 := newCheckoutCmd()
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{"feature"})

	err := cmd2.Execute()
	if err == nil {
		t.Fatal("expected error when checking out already checked-out branch")
	}

	// Verify error message mentions the branch and path
	errStr := err.Error()
	if !strings.Contains(errStr, "feature") {
		t.Errorf("error should mention branch name 'feature', got: %s", errStr)
	}
	if !strings.Contains(errStr, "already checked out") {
		t.Errorf("error should mention 'already checked out', got: %s", errStr)
	}
}

// TestCheckout_BaseBranch tests creating a new branch from a specific base.
//
// Scenario: User runs `wt checkout -b feature --base develop`
// Expected: New branch is created from develop, not main
func TestCheckout_BaseBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	// Create a develop branch with a unique commit
	runGitCommand(repoPath, "branch", "develop")
	runGitCommand(repoPath, "checkout", "develop")
	addCommit(t, repoPath, "develop.txt", "Develop commit")
	runGitCommand(repoPath, "checkout", "main")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--base", "develop"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify the branch was created from develop (should have develop.txt)
	developFile := filepath.Join(wtPath, "develop.txt")
	if _, err := os.Stat(developFile); os.IsNotExist(err) {
		t.Error("feature branch should have develop.txt (created from develop)")
	}
}

// TestCheckout_Fetch tests that --fetch fetches before creating branch.
//
// Scenario: User runs `wt checkout -b feature --fetch`
// Expected: Fetch is performed before creating the branch
func TestCheckout_Fetch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath, originPath := setupTestRepoWithOrigin(t, tmpDir, "test-repo")

	// Add a commit to origin that the local repo doesn't have
	// We need to clone the origin again to make changes
	clonePath := filepath.Join(tmpDir, "origin-clone")
	runGitCommand(tmpDir, "clone", originPath, clonePath)
	runGitCommand(clonePath, "config", "user.email", "test@test.com")
	runGitCommand(clonePath, "config", "user.name", "Test User")
	addCommit(t, clonePath, "origin-only.txt", "Origin commit")
	runGitCommand(clonePath, "push", "origin", "main")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--fetch"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify the branch was created from the fetched origin/main (should have origin-only.txt)
	originFile := filepath.Join(wtPath, "origin-only.txt")
	if _, err := os.Stat(originFile); os.IsNotExist(err) {
		t.Error("feature branch should have origin-only.txt (created from fetched origin/main)")
	}
}

// TestCheckout_FetchExistingBranch tests that --fetch fetches the target branch for existing branches.
//
// Scenario: Branch exists on remote but not locally, user runs `wt checkout feature --fetch`
// Expected: Fetch pulls the target branch, worktree is created via git DWIM
func TestCheckout_FetchExistingBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath, originPath := setupTestRepoWithOrigin(t, tmpDir, "test-repo")

	// Clone origin to push a new branch with unique content
	clonePath := filepath.Join(tmpDir, "origin-clone")
	runGitCommand(tmpDir, "clone", originPath, clonePath)
	runGitCommand(clonePath, "config", "user.email", "test@test.com")
	runGitCommand(clonePath, "config", "user.name", "Test User")
	runGitCommand(clonePath, "config", "commit.gpgsign", "false")
	runGitCommand(clonePath, "checkout", "-b", "feature")
	addCommit(t, clonePath, "feature-file.txt", "Feature commit")
	runGitCommand(clonePath, "push", "origin", "feature")

	// Verify the local repo does NOT have the feature branch
	out, _ := runGitCommand(repoPath, "branch", "--list", "feature")
	if strings.TrimSpace(out) != "" {
		t.Fatalf("feature branch should not exist locally yet")
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--fetch"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify the branch has content from the fetched remote branch
	featureFile := filepath.Join(wtPath, "feature-file.txt")
	if _, err := os.Stat(featureFile); os.IsNotExist(err) {
		t.Error("worktree should have feature-file.txt from the fetched remote branch")
	}
}

// TestCheckout_FetchWithBase tests that --fetch with --base fetches the specified base branch.
//
// Scenario: User runs `wt checkout -b feature --fetch --base develop`
// Expected: Fetch pulls the develop branch (not default), worktree is created from it
func TestCheckout_FetchWithBase(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath, originPath := setupTestRepoWithOrigin(t, tmpDir, "test-repo")

	// Clone origin to create a develop branch with unique content
	clonePath := filepath.Join(tmpDir, "origin-clone")
	runGitCommand(tmpDir, "clone", originPath, clonePath)
	runGitCommand(clonePath, "config", "user.email", "test@test.com")
	runGitCommand(clonePath, "config", "user.name", "Test User")
	runGitCommand(clonePath, "config", "commit.gpgsign", "false")
	runGitCommand(clonePath, "checkout", "-b", "develop")
	addCommit(t, clonePath, "develop-file.txt", "Develop commit")
	runGitCommand(clonePath, "push", "origin", "develop")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--fetch", "--base", "develop"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify the branch was created from the fetched develop branch (should have develop-file.txt)
	developFile := filepath.Join(wtPath, "develop-file.txt")
	if _, err := os.Stat(developFile); os.IsNotExist(err) {
		t.Error("feature branch should have develop-file.txt (created from fetched develop)")
	}
}

// TestCheckout_AutoStash tests that --autostash stashes and applies changes.
//
// Scenario: User has uncommitted changes, runs `wt checkout feature --autostash`
// Expected: Changes are stashed, worktree created, changes applied to new worktree
func TestCheckout_AutoStash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})

	// Create uncommitted changes in main worktree
	changedFile := filepath.Join(repoPath, "uncommitted.txt")
	if err := os.WriteFile(changedFile, []byte("uncommitted changes\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	runGitCommand(repoPath, "add", "uncommitted.txt")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--autostash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify the stashed changes were applied to the new worktree
	stashedFile := filepath.Join(wtPath, "uncommitted.txt")
	if _, err := os.Stat(stashedFile); os.IsNotExist(err) {
		t.Error("stashed changes should be applied to new worktree")
	}

	// Verify the original repo no longer has the uncommitted changes
	if _, err := os.Stat(changedFile); err == nil {
		// File still exists - check if it's still staged
		out, _ := runGitCommand(repoPath, "status", "--porcelain")
		if strings.Contains(out, "uncommitted.txt") {
			t.Error("original repo should not have uncommitted changes after autostash")
		}
	}
}

// TestCheckout_Note tests that --note sets a note on the branch.
//
// Scenario: User runs `wt checkout -b feature --note "Work in progress"`
// Expected: Branch is created with the note attached
func TestCheckout_Note(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--note", "Work in progress"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify the note was set
	out, err := runGitCommand(repoPath, "config", "branch.feature.description")
	if err != nil {
		t.Fatalf("failed to get branch description: %v", err)
	}
	note := strings.TrimSpace(out)
	if note != "Work in progress" {
		t.Errorf("expected note 'Work in progress', got %q", note)
	}
}

// TestCheckout_Hook tests that --hook runs a specific hook after checkout.
//
// Scenario: User runs `wt checkout -b feature --hook myhook`
// Expected: Worktree is created and the specified hook runs
func TestCheckout_Hook(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "hook-ran")

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "touch " + markerPath,
					Description: "Test hook",
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--hook", "myhook"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify hook ran
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("hook should have run and created marker file")
	}
}

// TestCheckout_NoHook tests that --no-hook skips default hooks.
//
// Scenario: User runs `wt checkout -b feature --no-hook` with a default hook
// Expected: Worktree is created but the hook does not run
func TestCheckout_NoHook(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "hook-ran")

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"default-hook": {
					Command:     "touch " + markerPath,
					Description: "Default checkout hook",
					On:          []string{"checkout"}, // This is a default hook
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--no-hook"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify hook did NOT run
	if _, err := os.Stat(markerPath); err == nil {
		t.Error("hook should NOT have run with --no-hook flag")
	}
}

// TestCheckout_HookWithArg tests that --arg passes variables to hooks.
//
// Scenario: User runs `wt checkout -b feature --hook myhook --arg myvar=hello`
// Expected: Hook receives the variable and can use it
func TestCheckout_HookWithArg(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "hook-output.txt")

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "echo {myvar} > " + outputPath,
					Description: "Test hook with variable",
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--hook", "myhook", "--arg", "myvar=hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify the variable was substituted in the hook
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read hook output: %v", err)
	}
	if strings.TrimSpace(string(content)) != "hello" {
		t.Errorf("expected hook output 'hello', got %q", string(content))
	}
}

// TestCheckout_DefaultHookRuns tests that default hooks run automatically.
//
// Scenario: User runs `wt checkout -b feature` with a hook that has on=["checkout"]
// Expected: The default hook runs automatically
func TestCheckout_DefaultHookRuns(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "default-hook-ran")

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"auto-hook": {
					Command:     "touch " + markerPath,
					Description: "Auto checkout hook",
					On:          []string{"checkout"},
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature"}) // No --hook flag

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify default hook ran automatically
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("default hook should have run automatically")
	}
}

// TestCheckout_RecordsHistory tests that checkout records to history.
//
// Scenario: User runs `wt checkout -b feature`
// Expected: History is updated with the new worktree path
func TestCheckout_RecordsHistory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	historyFile := filepath.Join(tmpDir, ".wt", "history.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create .wt dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		HistoryPath:  historyFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify history was recorded
	mostRecent, err := history.GetMostRecent(historyFile)
	if err != nil {
		t.Fatalf("failed to get most recent from history: %v", err)
	}
	if mostRecent != wtPath {
		t.Errorf("expected history to contain %q, got %q", wtPath, mostRecent)
	}
}

// TestCheckout_NewBranchEmptyRepo tests creating a new branch on an empty (no commits) repo.
//
// Scenario: User clones an empty repo, then runs `wt checkout -b repo:new-branch`
// Expected: Worktree is created using orphan branch (git worktree add --orphan)
func TestCheckout_NewBranchEmptyRepo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a bare-in-.git repo (simulating wt repo clone of an empty repo)
	repoPath := setupBareInGitRepo(t, tmpDir, "empty-repo")

	// Add an origin remote (so GetDefaultBranch/GetRepoName work)
	gitDir := filepath.Join(repoPath, ".git")
	runGitCommand(gitDir, "remote", "add", "origin", "https://github.com/test/empty-repo.git")

	// Setup registry
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "empty-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}

	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	ctx := testContextWithConfig(t, cfg, workDir)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "empty-repo:initial-branch"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "empty-repo-initial-branch")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree should exist at %s", wtPath)
	}

	// Verify it's on the correct branch
	branch := getGitBranch(t, wtPath)
	if branch != "initial-branch" {
		t.Errorf("expected branch 'initial-branch', got %q", branch)
	}

	// Verify it's an orphan branch (no commits)
	logCmd := exec.Command("git", "log", "--oneline")
	logCmd.Dir = wtPath
	logOut, err := logCmd.CombinedOutput()
	if err == nil && len(strings.TrimSpace(string(logOut))) > 0 {
		t.Errorf("expected orphan branch with no commits, got: %s", string(logOut))
	}
}

// TestCheckout_NewBranchEmptyRepoWithFetch tests that --fetch is safely skipped on empty repos.
//
// Scenario: User runs `wt checkout -b -f repo:branch` on an empty repo
// Expected: Worktree is created, fetch is skipped without errors
func TestCheckout_NewBranchEmptyRepoWithFetch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupBareInGitRepo(t, tmpDir, "empty-repo-fetch")

	gitDir := filepath.Join(repoPath, ".git")
	runGitCommand(gitDir, "remote", "add", "origin", "https://github.com/test/empty-repo.git")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "empty-repo-fetch", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}

	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	ctx := testContextWithConfig(t, cfg, workDir)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "-f", "empty-repo-fetch:feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout with --fetch on empty repo should succeed, got: %v", err)
	}

	wtPath := filepath.Join(tmpDir, "empty-repo-fetch-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree should exist at %s", wtPath)
	}

	branch := getGitBranch(t, wtPath)
	if branch != "feature" {
		t.Errorf("expected branch 'feature', got %q", branch)
	}
}

// TestCheckout_NewBranchEmptyRepoLocalBaseRef tests empty repo with BaseRef="local" config.
//
// Scenario: User has BaseRef="local" configured and runs checkout -b on empty repo
// Expected: Worktree is created as orphan (local ref doesn't exist either)
func TestCheckout_NewBranchEmptyRepoLocalBaseRef(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupBareInGitRepo(t, tmpDir, "empty-repo-local")

	gitDir := filepath.Join(repoPath, ".git")
	runGitCommand(gitDir, "remote", "add", "origin", "https://github.com/test/empty-repo.git")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "empty-repo-local", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
	}

	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	ctx := testContextWithConfig(t, cfg, workDir)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "empty-repo-local:feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout on empty repo with BaseRef=local should succeed, got: %v", err)
	}

	wtPath := filepath.Join(tmpDir, "empty-repo-local-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree should exist at %s", wtPath)
	}

	branch := getGitBranch(t, wtPath)
	if branch != "feature" {
		t.Errorf("expected branch 'feature', got %q", branch)
	}
}

// TestCheckout_NewBranchInvalidBaseRef tests that an invalid base ref on a non-empty repo returns an error.
//
// Scenario: User runs `wt checkout -b --base nonexistent repo:branch` on a repo with commits
// Expected: Command fails with a git error about the invalid ref
func TestCheckout_NewBranchInvalidBaseRef(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "valid-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "valid-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}

	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	ctx := testContextWithConfig(t, cfg, workDir)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "--base", "nonexistent-branch", "valid-repo:feature"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid base ref on non-empty repo, got nil")
	}
}

// TestCheckout_HistoryEnablesCdNoArgs tests that wt cd (no args) works after checkout.
//
// Scenario: User runs `wt checkout -b feature`, then `wt cd` with no args
// Expected: wt cd returns the newly created worktree path
func TestCheckout_HistoryEnablesCdNoArgs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	historyFile := filepath.Join(tmpDir, ".wt", "history.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create .wt dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		HistoryPath:  historyFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

	// First, run checkout
	checkoutCmd := newCheckoutCmd()
	checkoutCmd.SetContext(ctx)
	checkoutCmd.SetArgs([]string{"-b", "feature"})

	if err := checkoutCmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	wtPath := filepath.Join(tmpDir, "test-repo-feature")

	// Now run wt cd with no args and capture output
	cdCtx, cdOut := testContextWithConfigAndOutput(t, cfg, repoPath)
	cdCmd := newCdCmd()
	cdCmd.SetContext(cdCtx)
	cdCmd.SetArgs([]string{})

	if err := cdCmd.Execute(); err != nil {
		t.Fatalf("cd command failed: %v", err)
	}

	// Verify the output is the worktree path
	output := strings.TrimSpace(cdOut.String())
	if output != wtPath {
		t.Errorf("expected cd output %q, got %q", wtPath, output)
	}
}

// TestCheckout_ExplicitUpstreamRemoteRef tests --base with upstream/branch syntax.
//
// Scenario: User runs `wt checkout -b feature --fetch --base upstream/develop`
// Expected: Fetch pulls develop from upstream (not origin), worktree created from it
func TestCheckout_ExplicitUpstreamRemoteRef(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create the main repo with origin
	repoPath, originPath := setupTestRepoWithOrigin(t, tmpDir, "test-repo")

	// Create a second remote (upstream) with different content
	upstreamPath := filepath.Join(tmpDir, "upstream-repo")
	runGitCommand(tmpDir, "clone", "--bare", originPath, upstreamPath)

	// Clone upstream to add a unique commit
	upstreamClone := filepath.Join(tmpDir, "upstream-clone")
	runGitCommand(tmpDir, "clone", upstreamPath, upstreamClone)
	runGitCommand(upstreamClone, "config", "user.email", "test@test.com")
	runGitCommand(upstreamClone, "config", "user.name", "Test User")
	runGitCommand(upstreamClone, "checkout", "-b", "develop")
	addCommit(t, upstreamClone, "upstream-only.txt", "Upstream commit")
	runGitCommand(upstreamClone, "push", "origin", "develop")

	// Add upstream as remote to main repo
	runGitCommand(repoPath, "remote", "add", "upstream", upstreamPath)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--fetch", "--base", "upstream/develop"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify the branch was created from upstream/develop (should have upstream-only.txt)
	upstreamFile := filepath.Join(wtPath, "upstream-only.txt")
	if _, err := os.Stat(upstreamFile); os.IsNotExist(err) {
		t.Error("feature branch should have upstream-only.txt (created from upstream/develop)")
	}
}

// TestCheckout_LocalBaseRefWithFetchWarning tests that --fetch with local base_ref prints warning.
//
// Scenario: User runs `wt checkout -b feature --fetch --base develop` with base_ref=local
// Expected: Warning is printed, fetch is skipped, branch is created from local develop
func TestCheckout_LocalBaseRefWithFetchWarning(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	// Create a local develop branch
	runGitCommand(repoPath, "checkout", "-b", "develop")
	addCommit(t, repoPath, "local-develop.txt", "Local develop commit")
	runGitCommand(repoPath, "checkout", "main")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local", // Key: local base ref
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--fetch", "--base", "develop"})

	// Command should succeed (fetch is skipped with warning)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify branch was created from LOCAL develop (has local-develop.txt)
	localFile := filepath.Join(wtPath, "local-develop.txt")
	if _, err := os.Stat(localFile); os.IsNotExist(err) {
		t.Error("feature branch should have local-develop.txt (created from local develop)")
	}
}

// TestCheckout_ExplicitOriginRemoteRef tests --base with origin/branch syntax.
//
// Scenario: User runs `wt checkout -b feature --base origin/develop` (even with base_ref=local)
// Expected: Explicit remote ref overrides base_ref config
func TestCheckout_ExplicitOriginRemoteRef(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath, originPath := setupTestRepoWithOrigin(t, tmpDir, "test-repo")

	// Create and push develop to origin with unique content
	clonePath := filepath.Join(tmpDir, "origin-clone")
	runGitCommand(tmpDir, "clone", originPath, clonePath)
	runGitCommand(clonePath, "config", "user.email", "test@test.com")
	runGitCommand(clonePath, "config", "user.name", "Test User")
	runGitCommand(clonePath, "checkout", "-b", "develop")
	addCommit(t, clonePath, "origin-develop.txt", "Origin develop commit")
	runGitCommand(clonePath, "push", "origin", "develop")

	// Fetch in main repo so origin/develop exists
	runGitCommand(repoPath, "fetch", "origin", "develop")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local", // Even with local, explicit remote ref should be used
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--base", "origin/develop"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify branch was created from origin/develop (has origin-develop.txt)
	originFile := filepath.Join(wtPath, "origin-develop.txt")
	if _, err := os.Stat(originFile); os.IsNotExist(err) {
		t.Error("feature branch should have origin-develop.txt (explicit origin/develop overrides base_ref=local)")
	}
}

// TestCheckout_PreserveFiles tests that git-ignored files matching preserve patterns
// are copied from the existing worktree into the new one.
//
// Scenario: Repo has .env, .envrc, and .env.production (all git-ignored), user runs `wt checkout -b feature`
// Expected: All three are copied to new worktree (all match configured patterns)
func TestCheckout_PreserveFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	// Add .gitignore
	if err := os.WriteFile(filepath.Join(repoPath, ".gitignore"), []byte(".env\n.env.*\n.envrc\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	cmds := [][]string{
		{"git", "add", ".gitignore"},
		{"git", "commit", "-m", "Add gitignore"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	// Create git-ignored files in main worktree
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("SECRET=abc\n"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, ".envrc"), []byte("dotenv\n"), 0644); err != nil {
		t.Fatalf("failed to write .envrc: %v", err)
	}
	// Also create .env.production (matches .env.* pattern)
	if err := os.WriteFile(filepath.Join(repoPath, ".env.production"), []byte("PROD=true\n"), 0644); err != nil {
		t.Fatalf("failed to write .env.production: %v", err)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
		Preserve: config.PreserveConfig{
			Patterns: []string{".env", ".env.*", ".envrc"},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	wtPath := filepath.Join(tmpDir, "test-repo-feature")

	// Verify all matching files were copied
	for _, file := range []string{".env", ".envrc", ".env.production"} {
		data, err := os.ReadFile(filepath.Join(wtPath, file))
		if err != nil {
			t.Errorf("preserved file %s should exist in new worktree: %v", file, err)
			continue
		}
		srcData, _ := os.ReadFile(filepath.Join(repoPath, file))
		if string(data) != string(srcData) {
			t.Errorf("preserved file %s content mismatch: got %q, want %q", file, data, srcData)
		}
	}
}

// TestCheckout_PreserveNoOverwrite tests that preserved files never overwrite
// existing files in the target worktree.
//
// Scenario: Target worktree already has a .env file, source has a different one
// Expected: Existing .env in target is not overwritten
func TestCheckout_PreserveNoOverwrite(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	// Add .gitignore
	if err := os.WriteFile(filepath.Join(repoPath, ".gitignore"), []byte(".env\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	cmds := [][]string{
		{"git", "add", ".gitignore"},
		{"git", "commit", "-m", "Add gitignore"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	// Create .env in main worktree
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("SOURCE_SECRET=old\n"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Create worktree path manually and put a .env there first
	wtPath := filepath.Join(tmpDir, "test-repo-feature")

	preserveCfg := config.PreserveConfig{
		Patterns: []string{".env"},
	}

	// Directly test the preserve function since we need to create the .env before checkout
	ctx := testContext(t)

	// First create the worktree directory with an existing .env
	os.MkdirAll(wtPath, 0755)
	if err := os.WriteFile(filepath.Join(wtPath, ".env"), []byte("EXISTING=keep\n"), 0644); err != nil {
		t.Fatalf("failed to write existing .env: %v", err)
	}

	// Run preserve
	copied, err := preserve.PreserveFiles(ctx, preserveCfg, repoPath, wtPath)
	if err != nil {
		t.Fatalf("PreserveFiles failed: %v", err)
	}

	// .env should NOT be in copied list (it was skipped)
	for _, f := range copied {
		if f == ".env" {
			t.Error(".env should not have been copied (file already exists)")
		}
	}

	// Verify existing content was preserved
	data, err := os.ReadFile(filepath.Join(wtPath, ".env"))
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	if string(data) != "EXISTING=keep\n" {
		t.Errorf("existing .env was overwritten: got %q, want %q", data, "EXISTING=keep\n")
	}
}

// TestCheckout_NoPreserveFlag tests that --no-preserve skips file preservation.
//
// Scenario: User runs `wt checkout -b feature --no-preserve` with preserve config
// Expected: No files are copied despite matching patterns
func TestCheckout_NoPreserveFlag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	// Add .gitignore and create ignored file
	if err := os.WriteFile(filepath.Join(repoPath, ".gitignore"), []byte(".env\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	cmds := [][]string{
		{"git", "add", ".gitignore"},
		{"git", "commit", "-m", "Add gitignore"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("SECRET=abc\n"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
		Preserve: config.PreserveConfig{
			Patterns: []string{".env"},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature", "--no-preserve"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	wtPath := filepath.Join(tmpDir, "test-repo-feature")

	// Verify .env was NOT copied
	if _, err := os.Stat(filepath.Join(wtPath, ".env")); !os.IsNotExist(err) {
		t.Error(".env should NOT exist in new worktree when --no-preserve is used")
	}
}

// TestCheckout_PreserveExclude tests that exclude patterns filter out matching path segments.
//
// Scenario: .env inside node_modules is ignored when "node_modules" is in exclude
// Expected: Only non-excluded .env files are copied
func TestCheckout_PreserveExclude(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	// Add .gitignore that ignores .env and node_modules
	if err := os.WriteFile(filepath.Join(repoPath, ".gitignore"), []byte(".env\nnode_modules/\n"), 0644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	cmds := [][]string{
		{"git", "add", ".gitignore"},
		{"git", "commit", "-m", "Add gitignore"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	// Create root .env (should be preserved)
	if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("ROOT=true\n"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Create node_modules/.env (should be excluded)
	os.MkdirAll(filepath.Join(repoPath, "node_modules"), 0755)
	if err := os.WriteFile(filepath.Join(repoPath, "node_modules", ".env"), []byte("MODULES=true\n"), 0644); err != nil {
		t.Fatalf("failed to write node_modules/.env: %v", err)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
		Preserve: config.PreserveConfig{
			Patterns: []string{".env"},
			Exclude:  []string{"node_modules"},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	wtPath := filepath.Join(tmpDir, "test-repo-feature")

	// Root .env should be preserved
	data, err := os.ReadFile(filepath.Join(wtPath, ".env"))
	if err != nil {
		t.Fatalf("root .env should exist: %v", err)
	}
	if string(data) != "ROOT=true\n" {
		t.Errorf("root .env content = %q, want %q", data, "ROOT=true\n")
	}

	// node_modules/.env should NOT be preserved
	if _, err := os.Stat(filepath.Join(wtPath, "node_modules", ".env")); !os.IsNotExist(err) {
		t.Error("node_modules/.env should NOT be preserved (excluded)")
	}
}

// TestCheckout_AutoStash_NoChanges tests that --autostash with clean working tree succeeds.
//
// Scenario: User runs `wt checkout feature --autostash` with no uncommitted changes
// Expected: Stash fails silently (nothing to stash), checkout succeeds, pop skipped gracefully
func TestCheckout_AutoStash_NoChanges(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--autostash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout with --autostash and no changes should succeed, got: %v", err)
	}

	// Verify worktree was created
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

// TestCheckout_AutoStash_UntrackedFiles tests that untracked files are stashed and popped.
//
// Scenario: User has untracked files, runs `wt checkout feature --autostash`
// Expected: Untracked files are stashed, worktree created, files applied to new worktree
func TestCheckout_AutoStash_UntrackedFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})

	// Create untracked files (not staged)
	for _, name := range []string{"untracked1.txt", "untracked2.txt"} {
		f := filepath.Join(repoPath, name)
		if err := os.WriteFile(f, []byte("content of "+name+"\n"), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--autostash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify untracked files were stashed and applied to new worktree
	for _, name := range []string{"untracked1.txt", "untracked2.txt"} {
		f := filepath.Join(wtPath, name)
		content, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("untracked file %s should exist in new worktree: %v", name, err)
			continue
		}
		expected := "content of " + name + "\n"
		if string(content) != expected {
			t.Errorf("%s content = %q, want %q", name, content, expected)
		}
	}

	// Verify original repo no longer has the untracked files
	for _, name := range []string{"untracked1.txt", "untracked2.txt"} {
		f := filepath.Join(repoPath, name)
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("untracked file %s should not exist in original repo after stash", name)
		}
	}
}

// TestCheckout_AutoStash_StagedAndModified tests autostash with a mix of staged and modified files.
//
// Scenario: User has both staged and modified files, runs `wt checkout feature --autostash`
// Expected: All changes are stashed and applied to new worktree
func TestCheckout_AutoStash_StagedAndModified(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})

	// Create a staged file
	stagedFile := filepath.Join(repoPath, "staged.txt")
	if err := os.WriteFile(stagedFile, []byte("staged content\n"), 0644); err != nil {
		t.Fatalf("failed to write staged file: %v", err)
	}
	runGitCommand(repoPath, "add", "staged.txt")

	// Modify an existing tracked file (README.md from setupTestRepo)
	readmePath := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readmePath, []byte("modified readme\n"), 0644); err != nil {
		t.Fatalf("failed to modify README.md: %v", err)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--autostash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	// Verify worktree was created
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	// Verify staged file was applied to new worktree
	content, err := os.ReadFile(filepath.Join(wtPath, "staged.txt"))
	if err != nil {
		t.Errorf("staged.txt should exist in new worktree: %v", err)
	} else if string(content) != "staged content\n" {
		t.Errorf("staged.txt content = %q, want %q", content, "staged content\n")
	}

	// Verify modified README was applied to new worktree
	content, err = os.ReadFile(filepath.Join(wtPath, "README.md"))
	if err != nil {
		t.Errorf("README.md should exist in new worktree: %v", err)
	} else if string(content) != "modified readme\n" {
		t.Errorf("README.md content = %q, want %q", content, "modified readme\n")
	}
}

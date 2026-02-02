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

// TestCheckout_NotInRepo tests that checkout fails when not in repo and no -r flag.
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

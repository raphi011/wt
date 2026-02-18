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

// TestList_EmptyRepo tests listing worktrees when none exist.
//
// Scenario: User runs `wt list` in a repo with no worktrees
// Expected: Only shows the main worktree
func TestList_EmptyRepo(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
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
	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, repoPath)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	// Should show something (the main branch)
	output := out.String()
	if output == "" {
		t.Error("expected some output for main worktree")
	}
}

// TestList_WithWorktrees tests listing existing worktrees.
//
// Scenario: User runs `wt list` in a repo with worktrees
// Expected: Shows all worktrees
func TestList_WithWorktrees(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})

	// Create a worktree
	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
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
	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, repoPath)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "feature") {
		t.Errorf("expected output to contain 'feature', got %q", output)
	}

	_ = wtPath // used in setup
}

// TestList_ByRepoName tests listing worktrees for a specific repo.
//
// Scenario: User runs `wt list myrepo` from any directory
// Expected: Shows worktrees for the specified repo
func TestList_ByRepoName(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"develop"})
	createTestWorktree(t, repoPath, "develop")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

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
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, otherDir)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "develop") {
		t.Errorf("expected output to contain 'develop', got %q", output)
	}
}

// TestList_ByLabel tests listing worktrees filtered by label.
//
// Scenario: User runs `wt list backend` where backend is a label
// Expected: Shows worktrees for repos with that label
func TestList_ByLabel(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Work from a different directory
	otherDir := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, otherDir)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"backend"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "feature") {
		t.Errorf("expected output to contain 'feature', got %q", output)
	}
}

// TestList_MultipleScopes tests listing worktrees for multiple scopes.
//
// Scenario: User runs `wt list repo1 backend`
// Expected: Shows combined worktrees from repo1 and repos with backend label
func TestList_MultipleScopes(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create two repos
	repo1Path := setupTestRepoWithBranches(t, tmpDir, "repo1", []string{"feat1"})
	createTestWorktree(t, repo1Path, "feat1")

	repo2Path := setupTestRepoWithBranches(t, tmpDir, "repo2", []string{"feat2"})
	createTestWorktree(t, repo2Path, "feat2")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path},
			{Name: "repo2", Path: repo2Path, Labels: []string{"backend"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Work from a different directory
	otherDir := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, otherDir)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"repo1", "backend"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "feat1") {
		t.Errorf("expected output to contain 'feat1', got %q", output)
	}
	if !strings.Contains(output, "feat2") {
		t.Errorf("expected output to contain 'feat2', got %q", output)
	}
}

// TestList_ScopeNotFound tests error when scope doesn't exist.
//
// Scenario: User runs `wt list nonexistent`
// Expected: Returns error indicating no repo or label found
func TestList_ScopeNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Work from a different directory
	otherDir := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx, _ := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, otherDir)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent scope, got nil")
	}
	if !strings.Contains(err.Error(), "no repo or label found") {
		t.Errorf("expected error about repo/label not found, got %q", err.Error())
	}
}

// TestList_JSON tests JSON output format.
//
// Scenario: User runs `wt list --json`
// Expected: Output is valid JSON
func TestList_JSON(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
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
	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, repoPath)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.HasPrefix(strings.TrimSpace(output), "[") {
		t.Errorf("expected JSON array output, got %q", output)
	}
}

// TestList_OrphanedRepoFiltered tests that orphaned repos are silently skipped.
//
// Scenario: Registry has two repos, one with a non-existent path
// Expected: Command succeeds, only valid repo's worktrees are shown
func TestList_OrphanedRepoFiltered(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "valid-repo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "valid-repo", Path: repoPath},
			{Name: "orphaned-repo", Path: filepath.Join(tmpDir, "no-such-path")},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	otherDir := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, otherDir)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "feature") {
		t.Errorf("expected valid repo's worktree in output, got %q", output)
	}
	if strings.Contains(output, "orphaned-repo") {
		t.Errorf("expected orphaned repo to be filtered from output, got %q", output)
	}
}

// TestList_SortByBranch tests sorting worktrees by branch name.
//
// Scenario: User runs `wt list --sort branch`
// Expected: Worktrees are sorted alphabetically by branch name
func TestList_SortByBranch(t *testing.T) {
	t.Parallel()
	tmpDir := resolvePath(t, t.TempDir())

	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"beta", "alpha", "gamma"})
	createTestWorktree(t, repoPath, "beta")
	createTestWorktree(t, repoPath, "alpha")
	createTestWorktree(t, repoPath, "gamma")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
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
	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, repoPath)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--sort", "branch"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	// Verify all branches are present
	for _, branch := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(output, branch) {
			t.Errorf("expected output to contain %q, got %q", branch, output)
		}
	}

	// Verify alphabetical order: alpha should appear before beta, beta before gamma
	alphaIdx := strings.Index(output, "alpha")
	betaIdx := strings.Index(output, "beta")
	gammaIdx := strings.Index(output, "gamma")
	if alphaIdx > betaIdx || betaIdx > gammaIdx {
		t.Errorf("expected alphabetical order (alpha < beta < gamma), got alpha=%d beta=%d gamma=%d", alphaIdx, betaIdx, gammaIdx)
	}
}

// TestList_SortByRepo tests sorting worktrees by repo name.
//
// Scenario: User runs `wt list --sort repo` with multiple repos
// Expected: Worktrees are sorted by repo name, then branch name within each repo
func TestList_SortByRepo(t *testing.T) {
	t.Parallel()
	tmpDir := resolvePath(t, t.TempDir())

	repo1Path := setupTestRepoWithBranches(t, tmpDir, "zeta-repo", []string{"feature"})
	createTestWorktree(t, repo1Path, "feature")

	repo2Path := setupTestRepoWithBranches(t, tmpDir, "alpha-repo", []string{"develop"})
	createTestWorktree(t, repo2Path, "develop")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "zeta-repo", Path: repo1Path},
			{Name: "alpha-repo", Path: repo2Path},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	otherDir := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, otherDir)
	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"zeta-repo", "alpha-repo", "--sort", "repo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	// alpha-repo should appear before zeta-repo
	alphaIdx := strings.Index(output, "alpha-repo")
	zetaIdx := strings.Index(output, "zeta-repo")
	if alphaIdx == -1 || zetaIdx == -1 {
		t.Fatalf("expected both repo names in output, got %q", output)
	}
	if alphaIdx > zetaIdx {
		t.Errorf("expected alpha-repo before zeta-repo in sorted output")
	}
}

// TestList_Global tests the --global flag shows all repos.
//
// Scenario: User runs `wt list --global` from inside a repo
// Expected: Shows worktrees from all repos, not just current
func TestList_Global(t *testing.T) {
	t.Parallel()
	tmpDir := resolvePath(t, t.TempDir())

	repo1Path := setupTestRepoWithBranches(t, tmpDir, "repo1", []string{"feat1"})
	createTestWorktree(t, repo1Path, "feat1")

	repo2Path := setupTestRepoWithBranches(t, tmpDir, "repo2", []string{"feat2"})
	createTestWorktree(t, repo2Path, "feat2")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
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
	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, repo1Path) // Inside repo1

	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--global"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	// Should show worktrees from both repos
	if !strings.Contains(output, "feat1") {
		t.Errorf("expected output to contain 'feat1', got %q", output)
	}
	if !strings.Contains(output, "feat2") {
		t.Errorf("expected output to contain 'feat2', got %q", output)
	}
}

// TestList_GlobalFromNonRepo tests --global from outside any git repo.
//
// Scenario: User runs `wt list --global` from a non-git directory
// Expected: Shows worktrees from all registered repos
func TestList_GlobalFromNonRepo(t *testing.T) {
	t.Parallel()
	tmpDir := resolvePath(t, t.TempDir())

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	nonRepoDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonRepoDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, nonRepoDir)

	cmd := newListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--global"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "feature") {
		t.Errorf("expected output to contain 'feature', got %q", output)
	}
}


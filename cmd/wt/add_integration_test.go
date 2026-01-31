//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/registry"
)

// TestAdd_RegisterRepo tests registering an existing git repo.
//
// Scenario: User runs `wt add /path/to/repo`
// Expected: Repo is registered in the registry
func TestAdd_RegisterRepo(t *testing.T) {
	// Not parallel - modifies HOME

	// Setup temp dir
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a test repo
	repoPath := setupTestRepo(t, tmpDir, "testrepo")

	// Create test registry
	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	// Override HOME for registry
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Run add command
	ctx := testContext(t)
	cmd := newAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	// Verify repo was registered
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(reg.Repos))
	}

	if reg.Repos[0].Name != "testrepo" {
		t.Errorf("expected name 'testrepo', got %q", reg.Repos[0].Name)
	}

	if reg.Repos[0].Path != repoPath {
		t.Errorf("expected path %q, got %q", repoPath, reg.Repos[0].Path)
	}
}

// TestAdd_WithLabels tests registering a repo with labels.
//
// Scenario: User runs `wt add /path/to/repo -l backend -l api`
// Expected: Repo is registered with labels
func TestAdd_WithLabels(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "labeled-repo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-l", "backend", "-l", "api"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos[0].Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(reg.Repos[0].Labels))
	}

	hasBackend := false
	hasAPI := false
	for _, l := range reg.Repos[0].Labels {
		if l == "backend" {
			hasBackend = true
		}
		if l == "api" {
			hasAPI = true
		}
	}

	if !hasBackend || !hasAPI {
		t.Errorf("expected labels [backend, api], got %v", reg.Repos[0].Labels)
	}
}

// TestAdd_DuplicatePath tests that adding the same path twice fails.
//
// Scenario: User runs `wt add /path/to/repo` twice
// Expected: Second add fails with error
func TestAdd_DuplicatePath(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "dup-repo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)

	// First add
	cmd1 := newAddCmd()
	cmd1.SetContext(ctx)
	cmd1.SetArgs([]string{repoPath})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	// Second add should fail
	cmd2 := newAddCmd()
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{repoPath})
	if err := cmd2.Execute(); err == nil {
		t.Error("expected error for duplicate path")
	}
}

// TestAdd_NotAGitRepo tests that adding a non-git directory fails.
//
// Scenario: User runs `wt add /path/to/non-git-dir`
// Expected: Command fails with error
func TestAdd_NotAGitRepo(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	notGitPath := filepath.Join(tmpDir, "not-a-repo")
	os.MkdirAll(notGitPath, 0755)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{notGitPath})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-git directory")
	}
}

// TestAdd_MultiplePaths tests adding multiple repos at once.
//
// Scenario: User runs `wt add repo1 repo2`
// Expected: Both repos are registered
func TestAdd_MultiplePaths(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create two repos
	repo1 := setupTestRepo(t, tmpDir, "repo1")
	repo2 := setupTestRepo(t, tmpDir, "repo2")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repo1, repo2})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	// Verify both repos were registered
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(reg.Repos))
	}

	names := make(map[string]bool)
	for _, r := range reg.Repos {
		names[r.Name] = true
	}

	if !names["repo1"] || !names["repo2"] {
		t.Errorf("expected repos [repo1, repo2], got %v", reg.Repos)
	}
}

// TestAdd_SkipsNonGitDirs tests that non-git directories are skipped.
//
// Scenario: User runs `wt add repo1 notgit repo2`
// Expected: Only git repos are registered, non-git dirs are skipped
func TestAdd_SkipsNonGitDirs(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create one repo and one non-git directory
	repo1 := setupTestRepo(t, tmpDir, "repo1")
	notGit := filepath.Join(tmpDir, "notgit")
	os.MkdirAll(notGit, 0755)
	repo2 := setupTestRepo(t, tmpDir, "repo2")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repo1, notGit, repo2})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("add command failed: %v", err)
	}

	// Verify only git repos were registered
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(reg.Repos))
	}

	names := make(map[string]bool)
	for _, r := range reg.Repos {
		names[r.Name] = true
	}

	if names["notgit"] {
		t.Error("non-git directory should not be registered")
	}
}

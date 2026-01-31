//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/registry"
)

// TestReposList_ListEmpty tests listing repos when none are registered.
//
// Scenario: User runs `wt repos list` with no registered repos
// Expected: Shows "No repos registered" message
func TestReposList_ListEmpty(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir, err := os.MkdirTemp("", "wt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newReposListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos list command failed: %v", err)
	}
}

// TestReposList_ListRepos tests listing registered repos.
//
// Scenario: User runs `wt repos list` with registered repos
// Expected: Shows all registered repos
func TestReposList_ListRepos(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create registry with repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: "/tmp/repo1", Labels: []string{"backend"}},
			{Name: "repo2", Path: "/tmp/repo2", Labels: []string{"frontend"}},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContext(t)
	cmd := newReposListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos list command failed: %v", err)
	}
}

// TestReposList_FilterByLabel tests filtering repos by label.
//
// Scenario: User runs `wt repos list -l backend`
// Expected: Shows only repos with the backend label
func TestReposList_FilterByLabel(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create registry with repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "backend-api", Path: "/tmp/backend-api", Labels: []string{"backend"}},
			{Name: "frontend-app", Path: "/tmp/frontend-app", Labels: []string{"frontend"}},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContext(t)
	cmd := newReposListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-l", "backend"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos list command failed: %v", err)
	}
}

// TestReposList_JSON tests JSON output.
//
// Scenario: User runs `wt repos list --json`
// Expected: Outputs repos in JSON format
func TestReposList_JSON(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create registry with repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: "/tmp/test-repo"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// The JSON output goes through the output.Printer, so use testContextWithOutput
	ctx, out := testContextWithOutput(t)
	cmd := newReposListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "test-repo") {
		t.Errorf("expected output to contain 'test-repo', got: %s", output)
	}
}

// TestReposAdd_RegisterRepo tests registering an existing git repo.
//
// Scenario: User runs `wt repos add /path/to/repo`
// Expected: Repo is registered in the registry
func TestReposAdd_RegisterRepo(t *testing.T) {
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
	cmd := newReposAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos add command failed: %v", err)
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

// TestReposAdd_WithLabels tests registering a repo with labels.
//
// Scenario: User runs `wt repos add /path/to/repo -l backend -l api`
// Expected: Repo is registered with labels
func TestReposAdd_WithLabels(t *testing.T) {
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
	cmd := newReposAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-l", "backend", "-l", "api"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos add command failed: %v", err)
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

// TestReposAdd_DuplicatePath tests that adding the same path twice fails.
//
// Scenario: User runs `wt repos add /path/to/repo` twice
// Expected: Second add fails with error
func TestReposAdd_DuplicatePath(t *testing.T) {
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
	cmd1 := newReposAddCmd()
	cmd1.SetContext(ctx)
	cmd1.SetArgs([]string{repoPath})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	// Second add should fail
	cmd2 := newReposAddCmd()
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{repoPath})
	if err := cmd2.Execute(); err == nil {
		t.Error("expected error for duplicate path")
	}
}

// TestReposAdd_NotAGitRepo tests that adding a non-git directory fails.
//
// Scenario: User runs `wt repos add /path/to/non-git-dir`
// Expected: Command fails with error
func TestReposAdd_NotAGitRepo(t *testing.T) {
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
	cmd := newReposAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{notGitPath})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-git directory")
	}
}

// TestReposAdd_MultiplePaths tests adding multiple repos at once.
//
// Scenario: User runs `wt repos add repo1 repo2`
// Expected: Both repos are registered
func TestReposAdd_MultiplePaths(t *testing.T) {
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
	cmd := newReposAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repo1, repo2})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos add command failed: %v", err)
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

// TestReposAdd_SkipsNonGitDirs tests that non-git directories are skipped.
//
// Scenario: User runs `wt repos add repo1 notgit repo2`
// Expected: Only git repos are registered, non-git dirs are skipped
func TestReposAdd_SkipsNonGitDirs(t *testing.T) {
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
	cmd := newReposAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repo1, notGit, repo2})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos add command failed: %v", err)
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

// TestReposRemove_UnregisterRepo tests unregistering a repo.
//
// Scenario: User runs `wt repos remove testrepo`
// Expected: Repo is removed from registry
func TestReposRemove_UnregisterRepo(t *testing.T) {
	// Don't run in parallel - modifies HOME env var

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "remove-test")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	// Set HOME BEFORE saving registry
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Add the repo via registry
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "remove-test", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Now remove it
	ctx := testContext(t)
	cmd := newReposRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"remove-test"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos remove command failed: %v", err)
	}

	// Verify repo was removed
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(reg.Repos))
	}

	// Files should still exist
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Error("repo files should not be deleted without --delete flag")
	}
}

// TestReposRemove_NonExistent tests removing a non-existent repo.
//
// Scenario: User runs `wt repos remove nonexistent`
// Expected: Command fails with error
func TestReposRemove_NonExistent(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newReposRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-existent repo")
	}
}

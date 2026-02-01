//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/registry"
)

// TestRepoList_ListEmpty tests listing repos when none are registered.
//
// Scenario: User runs `wt repo list` with no registered repos
// Expected: Shows "No repos registered" message
func TestRepoList_ListEmpty(t *testing.T) {
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
	cmd := newRepoListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo list command failed: %v", err)
	}
}

// TestRepoList_ListRepos tests listing registered repos.
//
// Scenario: User runs `wt repo list` with registered repos
// Expected: Shows all registered repos
func TestRepoList_ListRepos(t *testing.T) {
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
	cmd := newRepoListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo list command failed: %v", err)
	}
}

// TestRepoList_FilterByLabel tests filtering repos by label.
//
// Scenario: User runs `wt repo list -l backend`
// Expected: Shows only repos with the backend label
func TestRepoList_FilterByLabel(t *testing.T) {
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
	cmd := newRepoListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-l", "backend"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo list command failed: %v", err)
	}
}

// TestRepoList_JSON tests JSON output.
//
// Scenario: User runs `wt repo list --json`
// Expected: Outputs repos in JSON format
func TestRepoList_JSON(t *testing.T) {
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
	cmd := newRepoListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo list command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "test-repo") {
		t.Errorf("expected output to contain 'test-repo', got: %s", output)
	}
}

// TestRepoAdd_RegisterRepo tests registering an existing git repo.
//
// Scenario: User runs `wt repo add /path/to/repo`
// Expected: Repo is registered in the registry
func TestRepoAdd_RegisterRepo(t *testing.T) {
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
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo add command failed: %v", err)
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

// TestRepoAdd_WithLabels tests registering a repo with labels.
//
// Scenario: User runs `wt repo add /path/to/repo -l backend -l api`
// Expected: Repo is registered with labels
func TestRepoAdd_WithLabels(t *testing.T) {
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
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-l", "backend", "-l", "api"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo add command failed: %v", err)
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

// TestRepoAdd_DuplicatePath tests that adding the same path twice fails.
//
// Scenario: User runs `wt repo add /path/to/repo` twice
// Expected: Second add fails with error
func TestRepoAdd_DuplicatePath(t *testing.T) {
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
	cmd1 := newRepoAddCmd()
	cmd1.SetContext(ctx)
	cmd1.SetArgs([]string{repoPath})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	// Second add should fail
	cmd2 := newRepoAddCmd()
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{repoPath})
	if err := cmd2.Execute(); err == nil {
		t.Error("expected error for duplicate path")
	}
}

// TestRepoAdd_NotAGitRepo tests that adding a non-git directory fails.
//
// Scenario: User runs `wt repo add /path/to/non-git-dir`
// Expected: Command fails with error
func TestRepoAdd_NotAGitRepo(t *testing.T) {
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
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{notGitPath})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-git directory")
	}
}

// TestRepoAdd_MultiplePaths tests adding multiple repos at once.
//
// Scenario: User runs `wt repo add repo1 repo2`
// Expected: Both repos are registered
func TestRepoAdd_MultiplePaths(t *testing.T) {
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
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repo1, repo2})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo add command failed: %v", err)
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

// TestRepoAdd_SkipsNonGitDirs tests that non-git directories are skipped.
//
// Scenario: User runs `wt repo add repo1 notgit repo2`
// Expected: Only git repos are registered, non-git dirs are skipped
func TestRepoAdd_SkipsNonGitDirs(t *testing.T) {
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
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repo1, notGit, repo2})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo add command failed: %v", err)
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

// TestRepoRemove_UnregisterRepo tests unregistering a repo.
//
// Scenario: User runs `wt repo remove testrepo`
// Expected: Repo is removed from registry
func TestRepoRemove_UnregisterRepo(t *testing.T) {
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
	cmd := newRepoRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"remove-test"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo remove command failed: %v", err)
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

// TestRepoRemove_NonExistent tests removing a non-existent repo.
//
// Scenario: User runs `wt repo remove nonexistent`
// Expected: Command fails with error
func TestRepoRemove_NonExistent(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newRepoRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-existent repo")
	}
}

// TestRepoMakeBare_BasicMigration tests basic migration from regular repo to bare-in-.git.
//
// Scenario: User runs `wt repo make-bare` in a regular git repo
// Expected: Repo is converted to bare-in-.git structure and registered
func TestRepoMakeBare_BasicMigration(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "migrate-test")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	// Verify structure - .git should be a directory with bare repo
	gitDir := filepath.Join(repoPath, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		t.Fatalf("failed to stat .git: %v", err)
	}
	if !info.IsDir() {
		t.Error(".git should be a directory")
	}

	// Verify main worktree was created
	mainWorktree := filepath.Join(repoPath, "main")
	if _, err := os.Stat(mainWorktree); os.IsNotExist(err) {
		t.Error("main worktree should exist")
	}

	// Verify repo was registered
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(reg.Repos))
	}

	if reg.Repos[0].Name != "migrate-test" {
		t.Errorf("expected name 'migrate-test', got %q", reg.Repos[0].Name)
	}
}

// TestRepoMakeBare_WithCustomName tests migration with custom display name.
//
// Scenario: User runs `wt repo make-bare -n myapp ./repo`
// Expected: Repo is migrated and registered with custom name
func TestRepoMakeBare_WithCustomName(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "original-name")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-n", "custom-name"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if reg.Repos[0].Name != "custom-name" {
		t.Errorf("expected name 'custom-name', got %q", reg.Repos[0].Name)
	}
}

// TestRepoMakeBare_WithLabels tests migration with labels.
//
// Scenario: User runs `wt repo make-bare -l backend -l api ./repo`
// Expected: Repo is migrated and registered with labels
func TestRepoMakeBare_WithLabels(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "labeled-migrate")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-l", "backend", "-l", "api"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos[0].Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(reg.Repos[0].Labels))
	}

	labels := make(map[string]bool)
	for _, l := range reg.Repos[0].Labels {
		labels[l] = true
	}

	if !labels["backend"] || !labels["api"] {
		t.Errorf("expected labels [backend, api], got %v", reg.Repos[0].Labels)
	}
}

// TestRepoMakeBare_WithWorktreeFormat tests migration with worktree format.
//
// Scenario: User runs `wt repo make-bare -w "./{branch}" ./repo`
// Expected: Repo is migrated and registered with worktree format
func TestRepoMakeBare_WithWorktreeFormat(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "format-migrate")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-w", "./{branch}"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if reg.Repos[0].WorktreeFormat != "./{branch}" {
		t.Errorf("expected worktree format './{branch}', got %q", reg.Repos[0].WorktreeFormat)
	}
}

// TestRepoMakeBare_DryRun tests dry run mode.
//
// Scenario: User runs `wt repo make-bare --dry-run ./repo`
// Expected: Shows migration plan without making changes
func TestRepoMakeBare_DryRun(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "dryrun-migrate")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx, out := testContextWithOutput(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	// Verify dry run message in output
	if !strings.Contains(out.String(), "dry run") {
		t.Error("expected dry run message in output")
	}

	// Verify no changes were made - main worktree should NOT exist
	mainWorktree := filepath.Join(repoPath, "main")
	if _, err := os.Stat(mainWorktree); !os.IsNotExist(err) {
		t.Error("main worktree should not exist in dry run")
	}

	// Verify repo was NOT registered
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos in dry run, got %d", len(reg.Repos))
	}
}

// TestRepoMakeBare_WithExistingWorktrees tests migration with existing worktrees.
//
// Scenario: User runs `wt repo make-bare` on repo with existing worktrees
// Expected: Repo is migrated and existing worktrees are updated
func TestRepoMakeBare_WithExistingWorktrees(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	// Create an existing worktree with repo prefix (e.g., myrepo-feature)
	// Migration will strip the prefix, renaming it to just "feature"
	wtOrigPath := filepath.Join(tmpDir, "myrepo-feature")
	cmd := exec.Command("git", "worktree", "add", wtOrigPath, "feature")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
	}

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd2 := newRepoMakeBareCmd()
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{repoPath})

	if err := cmd2.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	// Verify main worktree was created
	mainWorktree := filepath.Join(repoPath, "main")
	if _, err := os.Stat(mainWorktree); os.IsNotExist(err) {
		t.Error("main worktree should exist")
	}

	// Migration strips the repo prefix from worktree names
	// So myrepo-feature becomes just "feature"
	wtNewPath := filepath.Join(tmpDir, "feature")
	wtGitFile := filepath.Join(wtNewPath, ".git")
	if _, err := os.Stat(wtGitFile); os.IsNotExist(err) {
		t.Errorf("worktree .git file should exist at %s", wtNewPath)
	}

	// Verify we can run git status in the renamed worktree
	cmd = exec.Command("git", "status")
	cmd.Dir = wtNewPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git status in worktree failed: %v\n%s", err, out)
	}
}

// TestRepoMakeBare_IsWorktree tests error when path is a worktree.
//
// Scenario: User runs `wt repo make-bare` on a worktree path
// Expected: Command fails with error about being a worktree
func TestRepoMakeBare_IsWorktree(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "has-worktree", []string{"feature"})

	// Create a worktree
	wtPath := filepath.Join(tmpDir, "the-worktree")
	cmd := exec.Command("git", "worktree", "add", wtPath, "feature")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
	}

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd2 := newRepoMakeBareCmd()
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{wtPath}) // Try to migrate the worktree, not the main repo

	err := cmd2.Execute()
	if err == nil {
		t.Error("expected error for worktree path")
	}

	if !strings.Contains(err.Error(), "worktree") {
		t.Errorf("expected error about worktree, got: %v", err)
	}
}

// TestRepoMakeBare_AlreadyBare tests error when repo is already bare-in-.git.
//
// Scenario: User runs `wt repo make-bare` on already migrated repo
// Expected: Command fails with error
func TestRepoMakeBare_AlreadyBare(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupBareInGitRepo(t, tmpDir, "already-bare")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for already bare repo")
	}
}

// TestRepoMakeBare_NotGitRepo tests error when path is not a git repo.
//
// Scenario: User runs `wt repo make-bare` on non-git directory
// Expected: Command fails with error
func TestRepoMakeBare_NotGitRepo(t *testing.T) {
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
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{notGitPath})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-git directory")
	}
}

// TestRepoMakeBare_HasSubmodules tests error when repo has submodules.
//
// Scenario: User runs `wt repo make-bare` on repo with submodules
// Expected: Command fails with error about submodules
func TestRepoMakeBare_HasSubmodules(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithSubmodule(t, tmpDir, "with-submodule")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for repo with submodules")
	}

	if !strings.Contains(err.Error(), "submodule") {
		t.Errorf("expected error about submodules, got: %v", err)
	}
}

// TestRepoMakeBare_AlreadyRegistered tests migration of already registered repo.
//
// Scenario: User runs `wt repo make-bare` on repo already in registry
// Expected: Repo is migrated, registration is skipped
func TestRepoMakeBare_AlreadyRegistered(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "already-registered")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Pre-register the repo
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "already-registered", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx, out := testContextWithOutput(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	// Verify migration completed
	mainWorktree := filepath.Join(repoPath, "main")
	if _, err := os.Stat(mainWorktree); os.IsNotExist(err) {
		t.Error("main worktree should exist after migration")
	}

	// Verify output mentions already registered
	if !strings.Contains(out.String(), "Already registered") {
		t.Error("expected 'Already registered' in output")
	}

	// Verify still only one repo in registry
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(reg.Repos))
	}
}

// TestRepoMakeBare_NameConflict tests error when name conflicts with existing repo.
//
// Scenario: User runs `wt repo make-bare` with name that already exists
// Expected: Command fails with name conflict error
func TestRepoMakeBare_NameConflict(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "name-conflict")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Register a different repo with the same name
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "name-conflict", Path: "/some/other/path"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContext(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for name conflict")
	}

	if !strings.Contains(err.Error(), "name already exists") {
		t.Errorf("expected name conflict error, got: %v", err)
	}
}

// TestRepoMakeBare_ByPath tests migration when providing explicit path argument.
//
// Scenario: User runs `wt repo make-bare ./myrepo` from outside the repo
// Expected: Repo at path is migrated and registered
func TestRepoMakeBare_ByPath(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create repo in a subdirectory
	reposDir := filepath.Join(tmpDir, "repos")
	os.MkdirAll(reposDir, 0755)
	repoPath := setupTestRepo(t, reposDir, "path-migrate")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Change to a different directory (not the repo)
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	ctx := testContext(t)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	// Verify migration completed
	mainWorktree := filepath.Join(repoPath, "main")
	if _, err := os.Stat(mainWorktree); os.IsNotExist(err) {
		t.Error("main worktree should exist")
	}

	// Verify registration
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(reg.Repos))
	}

	if reg.Repos[0].Name != "path-migrate" {
		t.Errorf("expected name 'path-migrate', got %q", reg.Repos[0].Name)
	}
}

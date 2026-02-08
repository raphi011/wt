//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestRepoList_ListEmpty tests listing repos when none are registered.
//
// Scenario: User runs `wt repo list` with no registered repos
// Expected: Shows "No repos registered" message
func TestRepoList_ListEmpty(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}

	// Create registry with repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: "/tmp/repo1", Labels: []string{"backend"}},
			{Name: "repo2", Path: "/tmp/repo2", Labels: []string{"frontend"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo list command failed: %v", err)
	}
}

// TestRepoList_FilterByLabel tests filtering repos by label.
//
// Scenario: User runs `wt repo list backend`
// Expected: Shows only repos with the backend label
func TestRepoList_FilterByLabel(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}

	// Create registry with repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "backend-api", Path: "/tmp/backend-api", Labels: []string{"backend"}},
			{Name: "frontend-app", Path: "/tmp/frontend-app", Labels: []string{"frontend"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"backend"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo list command failed: %v", err)
	}
}

// TestRepoList_LabelNotFound tests error when filtering by nonexistent label.
//
// Scenario: User runs `wt repo list nonexistent`
// Expected: Returns error about no repos found with label
func TestRepoList_LabelNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}

	// Create registry with repos that don't have the searched label
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: "/tmp/repo1", Labels: []string{"backend"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoListCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent label, got nil")
	}
	if !strings.Contains(err.Error(), "no repos found with label") {
		t.Errorf("expected error about no repos found with label, got %q", err.Error())
	}
}

// TestRepoList_JSON tests JSON output.
//
// Scenario: User runs `wt repo list --json`
// Expected: Outputs repos in JSON format
func TestRepoList_JSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}

	// Create registry with repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: "/tmp/test-repo"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// The JSON output goes through the output.Printer, so use testContextWithOutput
	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, tmpDir)
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
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}

	// Run add command
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo add command failed: %v", err)
	}

	// Verify repo was registered
	reg, err := registry.Load(regFile)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "labeled-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-l", "backend", "-l", "api"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo add command failed: %v", err)
	}

	reg, err := registry.Load(regFile)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "dup-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	notGitPath := filepath.Join(tmpDir, "not-a-repo")
	os.MkdirAll(notGitPath, 0755)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create two repos
	repo1 := setupTestRepo(t, tmpDir, "repo1")
	repo2 := setupTestRepo(t, tmpDir, "repo2")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repo1, repo2})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo add command failed: %v", err)
	}

	// Verify both repos were registered
	reg, err := registry.Load(regFile)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create one repo and one non-git directory
	repo1 := setupTestRepo(t, tmpDir, "repo1")
	notGit := filepath.Join(tmpDir, "notgit")
	os.MkdirAll(notGit, 0755)
	repo2 := setupTestRepo(t, tmpDir, "repo2")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repo1, notGit, repo2})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo add command failed: %v", err)
	}

	// Verify only git repos were registered
	reg, err := registry.Load(regFile)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "remove-test")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}

	// Add the repo via registry
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "remove-test", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Now remove it
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"remove-test"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repo remove command failed: %v", err)
	}

	// Verify repo was removed
	reg, err := registry.Load(regFile)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-existent repo")
	}
}

// TestRepoRemove_OutputShowsCorrectName tests that the output message shows the
// name of the removed repo, not the repo that shifted into its slice position.
//
// Scenario: Two repos registered, user removes the first
// Expected: Output contains the first repo's name, not the second
func TestRepoRemove_OutputShowsCorrectName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repo1Path := setupTestRepo(t, tmpDir, "first-repo")
	repo2Path := setupTestRepo(t, tmpDir, "second-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Register both repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "first-repo", Path: repo1Path},
			{Name: "second-repo", Path: repo2Path},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoRemoveCmd()

	output, err := executeCommand(ctx, cmd, "first-repo")
	if err != nil {
		t.Fatalf("repo remove command failed: %v", err)
	}

	if !strings.Contains(output, "first-repo") {
		t.Errorf("expected output to contain 'first-repo', got: %s", output)
	}
	if strings.Contains(output, "second-repo") {
		t.Errorf("output should not contain 'second-repo', got: %s", output)
	}

	// Verify only second-repo remains
	reg, err = registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(reg.Repos))
	}
	if reg.Repos[0].Name != "second-repo" {
		t.Errorf("expected remaining repo to be 'second-repo', got %q", reg.Repos[0].Name)
	}
}

// TestRepoMakeBare_BasicMigration tests basic migration from regular repo to bare-in-.git.
//
// Scenario: User runs `wt repo make-bare` in a regular git repo
// Expected: Repo is converted to bare-in-.git structure and registered
func TestRepoMakeBare_BasicMigration(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "migrate-test")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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
	reg, err := registry.Load(regFile)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "original-name")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-n", "custom-name"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	reg, err := registry.Load(regFile)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "labeled-migrate")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-l", "backend", "-l", "api"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	reg, err := registry.Load(regFile)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "format-migrate")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-w", "./{branch}"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if reg.Repos[0].WorktreeFormat != "./{branch}" {
		t.Errorf("expected worktree format './{branch}', got %q", reg.Repos[0].WorktreeFormat)
	}
}

// TestRepoMakeBare_WithSiblingFormat tests migration with sibling worktree format.
//
// Scenario: User runs `wt repo make-bare -w "../{repo}-{branch}" ./repo`
// Expected: Main worktree is created as sibling to repo directory
func TestRepoMakeBare_WithSiblingFormat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "sibling-format")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoMakeBareCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-w", "../{repo}-{branch}"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	// Verify main worktree is at sibling location
	mainWorktree := filepath.Join(tmpDir, "sibling-format-main")
	if _, err := os.Stat(mainWorktree); os.IsNotExist(err) {
		t.Errorf("main worktree should exist at sibling location: %s", mainWorktree)
	}

	// Verify we can run git status in the main worktree
	gitCmd := exec.Command("git", "status")
	gitCmd.Dir = mainWorktree
	if out, err := gitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git status in main worktree failed: %v\n%s", err, out)
	}
}

// TestRepoMakeBare_SiblingFormatWithExistingWorktrees tests migration with sibling format and existing worktrees.
//
// Scenario: User runs `wt repo make-bare -w "../{repo}-{branch}"` on repo with existing worktrees
// Expected: Existing worktrees are moved to format-based sibling locations
func TestRepoMakeBare_SiblingFormatWithExistingWorktrees(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	// Create an existing worktree inside repo (nested)
	wtOrigPath := filepath.Join(repoPath, "nested-feature")
	cmd := exec.Command("git", "worktree", "add", wtOrigPath, "feature")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd2 := newRepoMakeBareCmd()
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{repoPath, "-w", "../{repo}-{branch}"})

	if err := cmd2.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	// Verify main worktree at sibling location
	mainWorktree := filepath.Join(tmpDir, "myrepo-main")
	if _, err := os.Stat(mainWorktree); os.IsNotExist(err) {
		t.Errorf("main worktree should exist at: %s", mainWorktree)
	}

	// Verify feature worktree moved to sibling location
	wtNewPath := filepath.Join(tmpDir, "myrepo-feature")
	wtGitFile := filepath.Join(wtNewPath, ".git")
	if _, err := os.Stat(wtGitFile); os.IsNotExist(err) {
		t.Errorf("worktree .git file should exist at %s", wtNewPath)
	}

	// Verify we can run git status in the moved worktree
	cmd = exec.Command("git", "status")
	cmd.Dir = wtNewPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git status in worktree failed: %v\n%s", err, out)
	}
}

// TestRepoMakeBare_DryRun tests dry run mode.
//
// Scenario: User runs `wt repo make-bare --dry-run ./repo`
// Expected: Shows migration plan without making changes
func TestRepoMakeBare_DryRun(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "dryrun-migrate")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, tmpDir)
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
	reg, err := registry.Load(regFile)
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
// Expected: Repo is migrated and existing worktrees are moved to format-based paths
func TestRepoMakeBare_WithExistingWorktrees(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	// Create an existing worktree as sibling (e.g., myrepo-feature)
	// With default "{branch}" format, migration moves it to myrepo/feature
	wtOrigPath := filepath.Join(tmpDir, "myrepo-feature")
	cmd := exec.Command("git", "worktree", "add", wtOrigPath, "feature")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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

	// With default "{branch}" format, worktree moves to nested location
	// myrepo-feature → myrepo/feature
	wtNewPath := filepath.Join(repoPath, "feature")
	wtGitFile := filepath.Join(wtNewPath, ".git")
	if _, err := os.Stat(wtGitFile); os.IsNotExist(err) {
		t.Errorf("worktree .git file should exist at %s", wtNewPath)
	}

	// Verify we can run git status in the moved worktree
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
	t.Parallel()

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

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupBareInGitRepo(t, tmpDir, "already-bare")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	notGitPath := filepath.Join(tmpDir, "not-a-repo")
	os.MkdirAll(notGitPath, 0755)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithSubmodule(t, tmpDir, "with-submodule")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "already-registered")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}

	// Pre-register the repo
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "already-registered", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, tmpDir)
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
	reg, err := registry.Load(regFile)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "name-conflict")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}

	// Register a different repo with the same name
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "name-conflict", Path: "/some/other/path"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, tmpDir)
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
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create repo in a subdirectory
	reposDir := filepath.Join(tmpDir, "repos")
	os.MkdirAll(reposDir, 0755)
	repoPath := setupTestRepo(t, reposDir, "path-migrate")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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
	reg, err := registry.Load(regFile)
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

// TestRepoMakeBare_WorktreeMetadataNameMismatch tests migration when worktree folder name
// differs from its metadata directory name in .git/worktrees/.
//
// Scenario: User has a worktree created with a different folder name than its metadata dir
// Expected: Migration moves worktree to format-based path regardless of folder name
func TestRepoMakeBare_WorktreeMetadataNameMismatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature-checkout"})

	// Create worktree with specific metadata name, then rename the folder
	// This simulates when folder name differs from metadata name
	wtMetadataName := "feature-checkout"  // what git uses internally
	wtFolderName := "wt-feature-checkout" // what the folder is actually named

	// First create worktree with the metadata name
	wtOrigPath := filepath.Join(tmpDir, wtMetadataName)
	cmd := exec.Command("git", "worktree", "add", wtOrigPath, "feature-checkout")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
	}

	// Now rename the folder to simulate a user renaming their worktree folder
	wtRenamedPath := filepath.Join(tmpDir, wtFolderName)
	if err := os.Rename(wtOrigPath, wtRenamedPath); err != nil {
		t.Fatalf("failed to rename worktree folder: %v", err)
	}

	// Run git worktree repair to update paths
	cmd = exec.Command("git", "worktree", "repair", wtRenamedPath)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to repair worktree: %v\n%s", err, out)
	}

	// Verify the mismatch: folder is "wt-feature-checkout" but metadata is "feature-checkout"
	metadataPath := filepath.Join(repoPath, ".git", "worktrees", wtMetadataName)
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatalf("expected metadata at %s but not found", metadataPath)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)
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

	// With default "{branch}" format, worktree moves to nested location
	// wt-feature-checkout → myrepo/feature-checkout
	wtNewPath := filepath.Join(repoPath, "feature-checkout")
	cmd = exec.Command("git", "status")
	cmd.Dir = wtNewPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git status in worktree failed: %v\n%s", err, out)
	}
}

// TestRepoMakeBare_PreservesUpstream tests that upstream tracking is preserved during migration.
//
// Scenario: User runs `wt repo make-bare` on repo with upstream tracking configured
// Expected: Main branch and worktrees retain their upstream tracking after migration
func TestRepoMakeBare_PreservesUpstream(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a "remote" repo to clone from (so we have a proper origin)
	remoteDir := filepath.Join(tmpDir, "remote")
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatalf("failed to create remote dir: %v", err)
	}

	// Initialize bare remote
	cmd := exec.Command("git", "init", "--bare", "origin.git")
	cmd.Dir = remoteDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init remote: %v\n%s", err, out)
	}
	remotePath := filepath.Join(remoteDir, "origin.git")

	// Clone from remote
	repoPath := filepath.Join(tmpDir, "local-repo")
	cmd = exec.Command("git", "clone", remotePath, repoPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to clone: %v\n%s", err, out)
	}

	// Configure git user for commits
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to configure git: %v\n%s", err, out)
		}
	}

	// Create initial commit on main
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to commit: %v\n%s", err, out)
	}

	// Rename branch to main (may be master depending on git config)
	cmd = exec.Command("git", "branch", "-m", "main")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to rename branch to main: %v\n%s", err, out)
	}

	// Push main to origin
	cmd = exec.Command("git", "push", "-u", "origin", "main")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to push main: %v\n%s", err, out)
	}

	// Create feature branch with upstream
	cmd = exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create feature branch: %v\n%s", err, out)
	}

	// Make a commit on feature
	if err := os.WriteFile(filepath.Join(repoPath, "feature.txt"), []byte("feature"), 0644); err != nil {
		t.Fatalf("failed to write feature file: %v", err)
	}
	cmd = exec.Command("git", "add", "feature.txt")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to git add feature: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "feature commit")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to commit feature: %v\n%s", err, out)
	}

	// Push feature with upstream tracking
	cmd = exec.Command("git", "push", "-u", "origin", "feature")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to push feature: %v\n%s", err, out)
	}

	// Go back to main
	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to checkout main: %v\n%s", err, out)
	}

	// Create worktree for feature branch
	wtPath := filepath.Join(tmpDir, "local-repo-feature")
	cmd = exec.Command("git", "worktree", "add", wtPath, "feature")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
	}

	// Verify upstream is set before migration
	cmd = exec.Command("git", "config", "branch.main.merge")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("main branch should have upstream before migration: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "refs/heads/main") {
		t.Fatalf("expected main upstream to be refs/heads/main, got %s", out)
	}

	cmd = exec.Command("git", "config", "branch.feature.merge")
	cmd.Dir = wtPath
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("feature branch should have upstream before migration: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "refs/heads/feature") {
		t.Fatalf("expected feature upstream to be refs/heads/feature, got %s", out)
	}

	// Setup registry
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Run make-bare
	ctx := testContextWithConfig(t, cfg, tmpDir)
	makeBareCmd := newRepoMakeBareCmd()
	makeBareCmd.SetContext(ctx)
	makeBareCmd.SetArgs([]string{repoPath})

	if err := makeBareCmd.Execute(); err != nil {
		t.Fatalf("make-bare command failed: %v", err)
	}

	// Verify main worktree was created
	mainWorktree := filepath.Join(repoPath, "main")
	if _, err := os.Stat(mainWorktree); os.IsNotExist(err) {
		t.Fatal("main worktree should exist")
	}

	// Verify upstream is preserved for main branch
	cmd = exec.Command("git", "config", "branch.main.merge")
	cmd.Dir = mainWorktree
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("main branch should have upstream after migration: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "refs/heads/main") {
		t.Errorf("expected main upstream to be refs/heads/main, got %s", out)
	}

	// With default "{branch}" format, feature worktree moves to nested location
	// local-repo-feature → local-repo/feature
	featureWorktree := filepath.Join(repoPath, "feature")
	if _, err := os.Stat(featureWorktree); os.IsNotExist(err) {
		t.Fatal("feature worktree should exist")
	}

	// Verify upstream is preserved for feature branch
	cmd = exec.Command("git", "config", "branch.feature.merge")
	cmd.Dir = featureWorktree
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("feature branch should have upstream after migration: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "refs/heads/feature") {
		t.Errorf("expected feature upstream to be refs/heads/feature, got %s", out)
	}
}

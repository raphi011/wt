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

// TestMigrate_BasicRepo tests migrating a simple repository.
//
// Scenario: User runs `wt migrate` in a regular git repo
// Expected: Repo is converted to bare-in-.git structure and registered
func TestMigrate_BasicRepo(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a source repo
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	// Setup registry directory
	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	// Verify .git is now a bare repo
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(filepath.Join(gitDir, "HEAD")); os.IsNotExist(err) {
		t.Error(".git should contain bare repo with HEAD file")
	}

	// Verify main worktree was created
	mainWorktree := filepath.Join(repoPath, "main")
	if _, err := os.Stat(mainWorktree); os.IsNotExist(err) {
		t.Error("main worktree directory should exist")
	}

	// Verify .git file in worktree points to bare repo
	// The path can be relative or absolute (git worktree repair may convert to absolute)
	gitFile := filepath.Join(mainWorktree, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		t.Fatalf("failed to read .git file: %v", err)
	}
	contentStr := string(content)
	if !strings.HasPrefix(contentStr, "gitdir: ") {
		t.Errorf("unexpected .git file format: %s", contentStr)
	}
	// Check it points to the worktrees directory
	if !strings.Contains(contentStr, ".git/worktrees/main") {
		t.Errorf(".git file should point to .git/worktrees/main, got: %s", contentStr)
	}

	// Verify files were moved to main worktree
	if _, err := os.Stat(filepath.Join(mainWorktree, "README.md")); os.IsNotExist(err) {
		t.Error("README.md should be in main worktree")
	}

	// Verify repo was registered
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(reg.Repos))
	}

	if reg.Repos[0].Name != "myrepo" {
		t.Errorf("expected name 'myrepo', got %q", reg.Repos[0].Name)
	}
}

// TestMigrate_WithUncommittedChanges tests that uncommitted changes are preserved.
//
// Scenario: User runs `wt migrate` on a repo with dirty working tree
// Expected: All uncommitted changes are preserved in the new worktree
func TestMigrate_WithUncommittedChanges(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "dirtyrepo")

	// Create uncommitted changes
	dirtyFile := filepath.Join(repoPath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted changes\n"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Modify an existing file
	readmeFile := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readmeFile, []byte("modified content\n"), 0644); err != nil {
		t.Fatalf("failed to modify README: %v", err)
	}

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	// Verify uncommitted file was preserved
	mainWorktree := filepath.Join(repoPath, "main")
	newDirtyFile := filepath.Join(mainWorktree, "dirty.txt")
	content, err := os.ReadFile(newDirtyFile)
	if err != nil {
		t.Fatalf("dirty file should exist in main worktree: %v", err)
	}
	if string(content) != "uncommitted changes\n" {
		t.Errorf("dirty file content changed: %s", string(content))
	}

	// Verify modified file was preserved
	newReadme := filepath.Join(mainWorktree, "README.md")
	content, err = os.ReadFile(newReadme)
	if err != nil {
		t.Fatalf("README should exist: %v", err)
	}
	if string(content) != "modified content\n" {
		t.Errorf("README content changed: %s", string(content))
	}
}

// TestMigrate_WithUntrackedFiles tests that untracked files are preserved.
//
// Scenario: User runs `wt migrate` on a repo with untracked files
// Expected: All untracked files are preserved in the new worktree
func TestMigrate_WithUntrackedFiles(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "untrackedrepo")

	// Create untracked files including nested directories
	untrackedDir := filepath.Join(repoPath, "untracked")
	os.MkdirAll(untrackedDir, 0755)
	if err := os.WriteFile(filepath.Join(untrackedDir, "file.txt"), []byte("untracked\n"), 0644); err != nil {
		t.Fatalf("failed to create untracked file: %v", err)
	}

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	// Verify untracked files were preserved
	mainWorktree := filepath.Join(repoPath, "main")
	untrackedFile := filepath.Join(mainWorktree, "untracked", "file.txt")
	content, err := os.ReadFile(untrackedFile)
	if err != nil {
		t.Fatalf("untracked file should exist: %v", err)
	}
	if string(content) != "untracked\n" {
		t.Errorf("untracked file content changed: %s", string(content))
	}
}

// TestMigrate_AlreadyBare tests that migrating an already bare repo fails.
//
// Scenario: User runs `wt migrate` on a repo already using bare-in-.git structure
// Expected: Command fails with appropriate error
func TestMigrate_AlreadyBare(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a source repo then clone it as bare
	sourceRepo := setupTestRepo(t, tmpDir, "source")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Clone as bare using wt clone
	ctx := testContext(t)
	cloneCmd := newCloneCmd()
	cloneCmd.SetContext(ctx)
	cloneCmd.SetArgs([]string{"file://" + sourceRepo, "barerepo"})

	if err := cloneCmd.Execute(); err != nil {
		t.Fatalf("clone command failed: %v", err)
	}

	// Try to migrate the already-bare repo
	migrateCmd := newMigrateCmd()
	migrateCmd.SetContext(ctx)
	migrateCmd.SetArgs([]string{filepath.Join(tmpDir, "barerepo")})

	err := migrateCmd.Execute()
	if err == nil {
		t.Error("expected error when migrating already-bare repo")
	}
}

// TestMigrate_DryRun tests that dry-run mode doesn't make changes.
//
// Scenario: User runs `wt migrate --dry-run`
// Expected: No changes are made, plan is displayed
func TestMigrate_DryRun(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "dryrunrepo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate dry-run failed: %v", err)
	}

	// Verify no changes were made
	// .git should still be a directory (not bare)
	gitDir := filepath.Join(repoPath, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		t.Fatalf("failed to stat .git: %v", err)
	}
	if !info.IsDir() {
		t.Error(".git should still be a directory")
	}

	// main worktree should not exist
	mainWorktree := filepath.Join(repoPath, "main")
	if _, err := os.Stat(mainWorktree); !os.IsNotExist(err) {
		t.Error("main worktree should not exist in dry-run mode")
	}

	// repo should not be registered
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(reg.Repos) != 0 {
		t.Error("repo should not be registered in dry-run mode")
	}
}

// TestMigrate_WithLabels tests that labels are preserved in registry.
//
// Scenario: User runs `wt migrate -l backend -l api`
// Expected: Repo is registered with specified labels
func TestMigrate_WithLabels(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "labeledrepo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-l", "backend", "-l", "api"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
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

// TestMigrate_WithCustomName tests that custom name is used in registry.
//
// Scenario: User runs `wt migrate -n myapp`
// Expected: Repo is registered with custom name
func TestMigrate_WithCustomName(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "actualdir")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath, "-n", "myapp"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if reg.Repos[0].Name != "myapp" {
		t.Errorf("expected name 'myapp', got %q", reg.Repos[0].Name)
	}
}

// TestMigrate_AlreadyRegistered tests that migrating an already registered repo fails.
//
// Scenario: User runs `wt migrate` on a repo that's already in the registry
// Expected: Command fails with appropriate error
func TestMigrate_AlreadyRegistered(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "registeredrepo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Pre-register the repo
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Path: repoPath, Name: "registeredrepo"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when migrating already registered repo")
	}
}

// TestMigrate_WithExistingWorktrees tests migrating a repo with existing worktrees.
//
// Scenario: User runs `wt migrate` on a repo that already has worktrees
// Expected: Worktrees are updated to work with new structure
func TestMigrate_WithExistingWorktrees(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "worktreerepo")

	// Create a worktree
	wtPath := createTestWorktree(t, repoPath, "feature")

	// Verify worktree was created
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist before migration: %s", wtPath)
	}

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	// Verify main worktree was created
	mainWorktree := filepath.Join(repoPath, "main")
	if _, err := os.Stat(mainWorktree); os.IsNotExist(err) {
		t.Error("main worktree should exist")
	}

	// Verify existing worktree still works (has .git file pointing to repo)
	gitFile := filepath.Join(wtPath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		t.Fatalf("worktree .git file should exist: %v", err)
	}
	if !strings.Contains(string(content), ".git/worktrees/") {
		t.Errorf("worktree .git should point to worktrees dir: %s", string(content))
	}
}

// TestMigrate_StripPrefix tests that worktree folders are renamed to strip repo prefix.
//
// Scenario: User migrates a repo with worktrees named "repo-feature"
// Expected: Worktree folder is renamed to "feature"
func TestMigrate_StripPrefix(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	// Create a worktree with repo prefix in name
	// The worktree will be created at tmpDir/myrepo-feature (sibling to repo)
	wtPath := filepath.Join(tmpDir, "myrepo-feature")
	gitCmd := "git"
	args := []string{"worktree", "add", wtPath, "-b", "feature"}
	cmd := exec.Command(gitCmd, args...)
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
	migrateCmd := newMigrateCmd()
	migrateCmd.SetContext(ctx)
	migrateCmd.SetArgs([]string{repoPath})

	if err := migrateCmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	// Verify worktree was renamed (prefix stripped)
	newWtPath := filepath.Join(tmpDir, "feature")
	if _, err := os.Stat(newWtPath); os.IsNotExist(err) {
		t.Error("worktree should be renamed to 'feature' (prefix stripped)")
	}

	// Verify old path no longer exists
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("old worktree path should not exist after prefix stripping")
	}
}

// TestMigrate_NotAGitRepo tests that migrating a non-git directory fails.
//
// Scenario: User runs `wt migrate` on a regular directory
// Expected: Command fails with appropriate error
func TestMigrate_NotAGitRepo(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a regular directory (not a git repo)
	notARepo := filepath.Join(tmpDir, "notarepo")
	os.MkdirAll(notARepo, 0755)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{notARepo})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when migrating non-git directory")
	}
}

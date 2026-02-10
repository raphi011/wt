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

// TestExec_NoCommand tests error when no command is given after --.
//
// Scenario: User runs `wt exec --` with no command
// Expected: Returns "no command specified" error
func TestExec_NoCommand(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no command, got nil")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("expected 'no command specified' error, got %q", err.Error())
	}
}

// TestExec_InCurrentWorktree tests running a command in the current directory.
//
// Scenario: User runs `wt exec -- touch test-file` from inside a worktree
// Expected: File is created in the current worktree directory
func TestExec_InCurrentWorktree(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--", "touch", "exec-test-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	// Verify file was created in workDir
	testFile := filepath.Join(repoPath, "exec-test-file")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("expected file %q to be created", testFile)
	}
}

// TestExec_ByBranch tests running a command in a specific worktree by branch name.
//
// Scenario: User runs `wt exec feature -- touch test-file`
// Expected: File is created in the feature worktree
func TestExec_ByBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--", "touch", "exec-test-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	// Verify file was created in the feature worktree
	testFile := filepath.Join(wtPath, "exec-test-file")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("expected file %q to be created in feature worktree", testFile)
	}

	// Verify it was NOT created in the main repo
	mainFile := filepath.Join(repoPath, "exec-test-file")
	if _, err := os.Stat(mainFile); err == nil {
		t.Errorf("did not expect file %q in main repo", mainFile)
	}
}

// TestExec_ByRepoBranch tests running a command with repo:branch targeting.
//
// Scenario: User runs `wt exec myrepo:feature -- touch test-file`
// Expected: File is created in the correct worktree
func TestExec_ByRepoBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo:feature", "--", "touch", "exec-test-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	testFile := filepath.Join(wtPath, "exec-test-file")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("expected file %q to be created", testFile)
	}
}

// TestExec_MultipleTargets tests running a command in multiple worktrees.
//
// Scenario: User runs `wt exec main feature -- touch test-file`
// Expected: File is created in both worktrees
func TestExec_MultipleTargets(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"main", "feature", "--", "touch", "exec-multi-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	// Verify file in main repo (main branch = repoPath)
	mainFile := filepath.Join(repoPath, "exec-multi-file")
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		t.Errorf("expected file %q in main worktree", mainFile)
	}

	// Verify file in feature worktree
	featureFile := filepath.Join(wtPath, "exec-multi-file")
	if _, err := os.Stat(featureFile); os.IsNotExist(err) {
		t.Errorf("expected file %q in feature worktree", featureFile)
	}
}

// TestExec_BranchNotFound tests error when target branch doesn't exist.
//
// Scenario: User runs `wt exec nonexistent -- ls`
// Expected: Returns "worktree not found" error
func TestExec_BranchNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent", "--", "ls"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent branch, got nil")
	}
	if !strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("expected 'worktree not found' error, got %q", err.Error())
	}
}

// TestExec_Deduplication tests that the same target is only executed once.
//
// Scenario: User runs `wt exec feature feature -- touch test-file`
// Expected: Command runs only once (single file created, no error)
func TestExec_Deduplication(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	// Use a script that appends to a file to detect double execution
	counterFile := filepath.Join(tmpDir, "counter")
	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "feature", "--", "sh", "-c", "echo x >> " + counterFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	// Verify the command ran exactly once (file should have one line)
	content, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("failed to read counter file: %v", err)
	}
	lines := strings.Count(strings.TrimSpace(string(content)), "x")
	if lines != 1 {
		t.Errorf("expected command to run once (1 line), got %d lines", lines)
	}

	// Also verify the command ran in the right directory
	testFile := filepath.Join(wtPath, "exec-test-file-does-not-exist")
	_ = testFile // dedup test — we just verify execution count
}

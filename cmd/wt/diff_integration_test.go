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

// setupDiffTestRepo creates a repo with an origin remote and a feature worktree
// that has a committed change (so git diff origin/main...HEAD shows something).
// Returns repoPath, worktreePath.
func setupDiffTestRepo(t *testing.T, tmpDir, repoName string) (string, string) {
	t.Helper()

	repoPath, _ := setupTestRepoWithOrigin(t, tmpDir, repoName)
	wtPath := createTestWorktree(t, repoPath, "feature")

	// Add a file in the feature worktree and commit it
	testFile := filepath.Join(wtPath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("feature content\n"), 0644); err != nil {
		t.Fatalf("failed to write feature file: %v", err)
	}

	cmds := [][]string{
		{"git", "add", "feature.txt"},
		{"git", "commit", "-m", "Add feature file"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = wtPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	return repoPath, wtPath
}

func setupDiffRegistry(t *testing.T, tmpDir string, repos []registry.Repo) string {
	t.Helper()

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: repos}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	return regFile
}

// TestDiff_CurrentWorktree tests diffing the current worktree with default (full diff) mode.
//
// Scenario: User runs `wt diff` from inside a feature worktree
// Expected: Succeeds (full diff output goes to terminal)
func TestDiff_CurrentWorktree(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	_, wtPath := setupDiffTestRepo(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: filepath.Join(tmpDir, "myrepo")},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command failed: %v", err)
	}
}

// TestDiff_ByBranch tests diffing a specific worktree by branch name.
//
// Scenario: User runs `wt diff feature --name-only` from the main worktree
// Expected: Shows the changed file in the feature worktree
func TestDiff_ByBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath, _ := setupDiffTestRepo(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: repoPath},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--name-only"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command failed: %v", err)
	}
}

// TestDiff_ScopedBranch tests diffing with repo:branch targeting.
//
// Scenario: User runs `wt diff myrepo:feature --name-only`
// Expected: Shows the changed file in the feature worktree
func TestDiff_ScopedBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath, _ := setupDiffTestRepo(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: repoPath},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo:feature", "--name-only"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command failed: %v", err)
	}
}

// TestDiff_StatFlag tests the --stat flag output.
//
// Scenario: User runs `wt diff --stat` in a worktree with changes
// Expected: Shows diffstat summary (file name with +/- indicators)
func TestDiff_StatFlag(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	_, wtPath := setupDiffTestRepo(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: filepath.Join(tmpDir, "myrepo")},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--stat"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command with --stat failed: %v", err)
	}
}

// TestDiff_NameOnlyFlag tests the --name-only flag output.
//
// Scenario: User runs `wt diff --name-only` in a worktree with changes
// Expected: Shows only the names of changed files
func TestDiff_NameOnlyFlag(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	_, wtPath := setupDiffTestRepo(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: filepath.Join(tmpDir, "myrepo")},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--name-only"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command with --name-only failed: %v", err)
	}
}

// TestDiff_WorkingFlag tests diffing uncommitted changes.
//
// Scenario: User runs `wt diff --working --name-only` in a worktree with uncommitted changes
// Expected: Shows the uncommitted file
func TestDiff_WorkingFlag(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	_, wtPath := setupDiffTestRepo(t, tmpDir, "myrepo")

	// Make an uncommitted change
	dirtyFile := filepath.Join(wtPath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty content\n"), 0644); err != nil {
		t.Fatalf("failed to write dirty file: %v", err)
	}

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: filepath.Join(tmpDir, "myrepo")},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--working", "--name-only"})

	// This should succeed — git diff HEAD shows the dirty file as untracked
	// (but git diff HEAD only shows tracked+modified, not untracked)
	// So let's stage it first to make it show up
	gitAdd := exec.Command("git", "add", "dirty.txt")
	gitAdd.Dir = wtPath
	if out, err := gitAdd.CombinedOutput(); err != nil {
		t.Fatalf("failed to stage dirty file: %v\n%s", err, out)
	}

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command with --working failed: %v", err)
	}
}

// TestDiff_CustomBase tests the --base flag.
//
// Scenario: User runs `wt diff --base HEAD~1 --name-only` in a worktree
// Expected: Shows diff against the custom base instead of origin/main
func TestDiff_CustomBase(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	_, wtPath := setupDiffTestRepo(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: filepath.Join(tmpDir, "myrepo")},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--base", "HEAD~1", "--name-only"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command with --base failed: %v", err)
	}
}

// TestDiff_NotInGitRepo tests error when running diff outside a git repo.
//
// Scenario: User runs `wt diff` from a non-git directory
// Expected: Returns "not in a git repository" error
func TestDiff_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{})

	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, nonGitDir)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestDiff_BranchNotFound tests error when target branch doesn't exist.
//
// Scenario: User runs `wt diff nonexistent`
// Expected: Returns "worktree not found" error
func TestDiff_BranchNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath, _ := setupTestRepoWithOrigin(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: repoPath},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent branch, got nil")
	}
	if !strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("expected 'worktree not found' error, got %q", err.Error())
	}
}

// TestDiff_InvalidBaseRef tests error when --base ref doesn't exist.
//
// Scenario: User runs `wt diff --base nonexistent/ref`
// Expected: Returns "base ref not found" error with --base hint
func TestDiff_InvalidBaseRef(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	_, wtPath := setupDiffTestRepo(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: filepath.Join(tmpDir, "myrepo")},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--base", "nonexistent/ref"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid base ref, got nil")
	}
	if !strings.Contains(err.Error(), "base ref") || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'base ref not found' error, got %q", err.Error())
	}
}

// TestDiff_BaseAndWorkingMutuallyExclusive tests that --base and --working cannot be combined.
//
// Scenario: User runs `wt diff --base HEAD~1 --working`
// Expected: Returns error about mutually exclusive flags
func TestDiff_BaseAndWorkingMutuallyExclusive(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	_, wtPath := setupDiffTestRepo(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: filepath.Join(tmpDir, "myrepo")},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--base", "HEAD~1", "--working"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --base + --working, got nil")
	}
}

// TestDiff_ToolFlag tests the --tool flag for pager override.
//
// Scenario: User runs `wt diff --tool cat --name-only`
// Expected: Succeeds (cat is used as pager instead of default)
func TestDiff_ToolFlag(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	_, wtPath := setupDiffTestRepo(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: filepath.Join(tmpDir, "myrepo")},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--tool", "cat", "--name-only"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command with --tool failed: %v", err)
	}
}

// TestDiff_NoChanges tests diffing a clean worktree that has no commits ahead of origin.
//
// Scenario: User runs `wt diff --name-only` in a worktree that has no commits beyond origin/main
// Expected: Succeeds with no output (no changed files)
func TestDiff_NoChanges(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	// setupTestRepoWithOrigin creates a repo with a real origin remote and
	// pushes main to it, so origin/main is up to date with main.
	repoPath, _ := setupTestRepoWithOrigin(t, tmpDir, "myrepo")

	regFile := setupDiffRegistry(t, tmpDir, []registry.Repo{
		{Name: "myrepo", Path: repoPath},
	})

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--name-only"})

	// Should succeed — no changes means empty diff output
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command on clean worktree failed: %v", err)
	}
}

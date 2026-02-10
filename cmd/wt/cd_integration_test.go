//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/registry"
)

// TestCd_BranchName tests resolving a worktree by branch name.
//
// Scenario: User runs `wt cd feature` with one repo containing that branch
// Expected: Prints the worktree path to stdout
func TestCd_BranchName(t *testing.T) {
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
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newCdCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cd command failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != wtPath {
		t.Errorf("expected path %q, got %q", wtPath, got)
	}
}

// TestCd_RepoBranch tests resolving a worktree by repo:branch format.
//
// Scenario: User runs `wt cd myrepo:feature`
// Expected: Prints the correct worktree path
func TestCd_RepoBranch(t *testing.T) {
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
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newCdCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo:feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cd command failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != wtPath {
		t.Errorf("expected path %q, got %q", wtPath, got)
	}
}

// TestCd_BranchNotFound tests error when branch doesn't exist.
//
// Scenario: User runs `wt cd nonexistent`
// Expected: Returns "worktree not found" error
func TestCd_BranchNotFound(t *testing.T) {
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

	cmd := newCdCmd()
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

// TestCd_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt cd nonexistent:feature`
// Expected: Returns "not found" error
func TestCd_RepoNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newCdCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent:feature"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %q", err.Error())
	}
}

// TestCd_AmbiguousBranch tests error when same branch exists in multiple repos.
//
// Scenario: Two repos both have a "feature" worktree, user runs `wt cd feature`
// Expected: Returns "exists in multiple repos" error
func TestCd_AmbiguousBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repo1Path := setupTestRepo(t, tmpDir, "repo1")
	repo2Path := setupTestRepo(t, tmpDir, "repo2")
	createTestWorktree(t, repo1Path, "feature")
	createTestWorktree(t, repo2Path, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

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
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newCdCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for ambiguous branch, got nil")
	}
	if !strings.Contains(err.Error(), "exists in multiple repos") {
		t.Errorf("expected 'exists in multiple repos' error, got %q", err.Error())
	}
}

// TestCd_NoArgs_NoHistory tests error when no args and no history.
//
// Scenario: User runs `wt cd` with empty history
// Expected: Returns "no worktree history" error
func TestCd_NoArgs_NoHistory(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	historyPath := filepath.Join(tmpDir, ".wt", "history.json")

	cfg := &config.Config{
		RegistryPath: regFile,
		HistoryPath:  historyPath,
	}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newCdCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no history, got nil")
	}
	if !strings.Contains(err.Error(), "no worktree history") {
		t.Errorf("expected 'no worktree history' error, got %q", err.Error())
	}
}

// TestCd_NoArgs_WithHistory tests returning the most recent worktree.
//
// Scenario: User previously cd'd to a worktree, then runs `wt cd` with no args
// Expected: Returns the most recently accessed worktree path
func TestCd_NoArgs_WithHistory(t *testing.T) {
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

	historyPath := filepath.Join(tmpDir, ".wt", "history.json")

	// Record a history entry
	if err := history.RecordAccess(wtPath, "myrepo", "feature", historyPath); err != nil {
		t.Fatalf("failed to record history: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		HistoryPath:  historyPath,
	}
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newCdCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cd command failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != wtPath {
		t.Errorf("expected path %q, got %q", wtPath, got)
	}
}

// TestCd_RecordsHistory tests that cd writes to history after resolving a worktree.
//
// Scenario: User runs `wt cd feature`
// Expected: History file is written with the accessed path
func TestCd_RecordsHistory(t *testing.T) {
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

	historyPath := filepath.Join(tmpDir, ".wt", "history.json")

	cfg := &config.Config{
		RegistryPath: regFile,
		HistoryPath:  historyPath,
	}
	ctx, _ := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newCdCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cd command failed: %v", err)
	}

	// Verify history was written
	hist, err := history.Load(historyPath)
	if err != nil {
		t.Fatalf("failed to load history: %v", err)
	}

	if len(hist.Entries) == 0 {
		t.Fatal("expected history entries, got none")
	}

	entry := hist.FindByPath(wtPath)
	if entry == nil {
		t.Fatalf("expected history entry for %q, not found", wtPath)
	}
	if entry.RepoName != "myrepo" {
		t.Errorf("expected repo name 'myrepo', got %q", entry.RepoName)
	}
	if entry.Branch != "feature" {
		t.Errorf("expected branch 'feature', got %q", entry.Branch)
	}
}

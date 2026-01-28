//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

// TestCd_ByWorktreeID verifies navigating to a worktree by its numeric ID.
//
// Scenario: User runs `wt cd -n 1`
// Expected: Outputs path to worktree with ID 1
func TestCd_ByWorktreeID(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache (required for ID lookup)
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		ID: 1, // First worktree
	}

	output, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt cd -n 1 failed: %v", err)
	}

	if strings.TrimSpace(output) != worktreePath {
		t.Errorf("expected path %s, got %s", worktreePath, strings.TrimSpace(output))
	}
}

// TestCd_ByWorktreeIDWithProjectFlag verifies -p flag returns main repo path
// instead of worktree path.
//
// Scenario: User runs `wt cd -n 1 -p`
// Expected: Outputs path to main repo, not the worktree
func TestCd_ByWorktreeIDWithProjectFlag(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache (required for ID lookup)
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		ID:      1, // First worktree
		Project: true,
	}

	output, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt cd -p -n 1 failed: %v", err)
	}

	// Should return main repo path, not worktree path
	if strings.TrimSpace(output) != repoPath {
		t.Errorf("expected main repo path %s, got %s", repoPath, strings.TrimSpace(output))
	}
}

// TestCd_ByRepoName verifies navigating to a repo by its name using -r flag.
//
// Scenario: User runs `wt cd -r myrepo`
// Expected: Outputs path to the main repository
func TestCd_ByRepoName(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		Repository: "myrepo",
	}

	output, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt cd -r myrepo failed: %v", err)
	}

	// Resolve output path for comparison (handles macOS symlinks)
	gotPath := resolvePath(t, strings.TrimSpace(output))
	if gotPath != repoPath {
		t.Errorf("expected repo path %s, got %s", repoPath, gotPath)
	}
}

// TestCd_ByLabel verifies navigating to a repo by its label using -l flag.
//
// Scenario: User runs `wt cd -l backend`
// Expected: Outputs path to the repo with "backend" label
func TestCd_ByLabel(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo with label
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	setRepoLabel(t, repoPath, "backend")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		Label: "backend",
	}

	output, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt cd -l backend failed: %v", err)
	}

	// Resolve output path for comparison (handles macOS symlinks)
	gotPath := resolvePath(t, strings.TrimSpace(output))
	if gotPath != repoPath {
		t.Errorf("expected repo path %s, got %s", repoPath, gotPath)
	}
}

// TestCd_MultipleWorktrees verifies ID-based navigation with multiple worktrees
// in the same repo.
//
// Scenario: User has 3 worktrees, navigates using IDs 1, 2, 3
// Expected: Each ID returns the correct worktree path
func TestCd_MultipleWorktrees(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with multiple worktrees
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	wt1 := filepath.Join(worktreeDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")

	wt2 := filepath.Join(worktreeDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	wt3 := filepath.Join(worktreeDir, "myrepo-feature3")
	setupWorktree(t, repoPath, wt3, "feature3")

	// Populate cache (required for ID lookup)
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Test accessing different IDs
	for id, expectedPath := range map[int]string{
		1: wt1,
		2: wt2,
		3: wt3,
	} {
		cmd := &CdCmd{ID: id}
		output, err := runCdCommand(t, worktreeDir, cfg, cmd)
		if err != nil {
			t.Fatalf("wt cd -n %d failed: %v", id, err)
		}

		if strings.TrimSpace(output) != expectedPath {
			t.Errorf("id %d: expected path %s, got %s", id, expectedPath, strings.TrimSpace(output))
		}
	}
}

// TestCd_ErrorNoTargetSpecified verifies error when no target (ID, repo, or
// label) is specified.
//
// Scenario: User runs `wt cd` without any flags
// Expected: Error "specify target"
func TestCd_ErrorNoTargetSpecified(t *testing.T) {
	t.Parallel()
	worktreeDir := t.TempDir()

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		// No target specified
	}

	_, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when no target specified")
	}
	if !strings.Contains(err.Error(), "specify target") {
		t.Errorf("expected 'specify target' error, got: %v", err)
	}
}

// TestCd_ErrorInvalidID verifies error when specified ID doesn't exist.
//
// Scenario: User runs `wt cd -n 999` when only 1 worktree exists
// Expected: Error for invalid ID
func TestCd_ErrorInvalidID(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with one worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache (required for ID lookup)
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		ID: 999, // Invalid ID
	}

	_, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

// TestCd_ErrorRepoNotFound verifies error when specified repo doesn't exist.
//
// Scenario: User runs `wt cd -r nonexistent-repo`
// Expected: Error mentioning the nonexistent repository
func TestCd_ErrorRepoNotFound(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create a different repo
	setupTestRepo(t, repoDir, "other-repo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		Repository: "nonexistent-repo",
	}

	_, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when repo not found")
	}
	if !strings.Contains(err.Error(), "nonexistent-repo") {
		t.Errorf("expected error to mention repo name, got: %v", err)
	}
}

// TestCd_ErrorLabelNotFound verifies error when no repos match the label.
//
// Scenario: User runs `wt cd -l nonexistent-label`
// Expected: Error mentioning the nonexistent label
func TestCd_ErrorLabelNotFound(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo without a label
	setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		Label: "nonexistent-label",
	}

	_, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when label not found")
	}
	if !strings.Contains(err.Error(), "nonexistent-label") {
		t.Errorf("expected error to mention label, got: %v", err)
	}
}

// TestCd_ErrorMultipleReposMatchLabel verifies error when label matches
// multiple repos (ambiguous target).
//
// Scenario: User runs `wt cd -l frontend` when 2 repos have "frontend" label
// Expected: Error "multiple repos"
func TestCd_ErrorMultipleReposMatchLabel(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create two repos with the same label
	repoA := setupTestRepo(t, repoDir, "repo-a")
	setRepoLabel(t, repoA, "frontend")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	setRepoLabel(t, repoB, "frontend")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		Label: "frontend",
	}

	_, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when multiple repos match label")
	}
	if !strings.Contains(err.Error(), "multiple repos") {
		t.Errorf("expected 'multiple repos' error, got: %v", err)
	}
}

// TestCd_NoHookFlag verifies --no-hook flag skips hook execution.
//
// Scenario: User runs `wt cd --no-hook -n 1`
// Expected: Path returned, hooks not executed
func TestCd_NoHookFlag(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache (required for ID lookup)
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		ID:     1,
		NoHook: true,
	}

	output, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt cd --no-hook -n 1 failed: %v", err)
	}

	if strings.TrimSpace(output) != worktreePath {
		t.Errorf("expected path %s, got %s", worktreePath, strings.TrimSpace(output))
	}
}

// TestCd_MultipleReposSameWorktreeDir verifies navigation works when multiple
// repos have worktrees in the same worktree_dir.
//
// Scenario: Two repos have worktrees in same directory, user navigates by ID and name
// Expected: Each target resolves to correct path
func TestCd_MultipleReposSameWorktreeDir(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create two repos with worktrees
	repoA := setupTestRepo(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Populate cache (required for ID lookup)
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Test by ID - should return worktrees in order
	cmd1 := &CdCmd{ID: 1}
	output1, err := runCdCommand(t, worktreeDir, cfg, cmd1)
	if err != nil {
		t.Fatalf("wt cd -n 1 failed: %v", err)
	}

	cmd2 := &CdCmd{ID: 2}
	output2, err := runCdCommand(t, worktreeDir, cfg, cmd2)
	if err != nil {
		t.Fatalf("wt cd -n 2 failed: %v", err)
	}

	// Both paths should be valid and different
	path1 := strings.TrimSpace(output1)
	path2 := strings.TrimSpace(output2)

	if path1 == path2 {
		t.Errorf("different IDs should return different paths")
	}

	// Test by repo name
	cmdA := &CdCmd{Repository: "repo-a"}
	outputA, err := runCdCommand(t, worktreeDir, cfg, cmdA)
	if err != nil {
		t.Fatalf("wt cd -r repo-a failed: %v", err)
	}

	gotA := resolvePath(t, strings.TrimSpace(outputA))
	if gotA != repoA {
		t.Errorf("expected repo-a path %s, got %s", repoA, gotA)
	}

	cmdB := &CdCmd{Repository: "repo-b"}
	outputB, err := runCdCommand(t, worktreeDir, cfg, cmdB)
	if err != nil {
		t.Fatalf("wt cd -r repo-b failed: %v", err)
	}

	gotB := resolvePath(t, strings.TrimSpace(outputB))
	if gotB != repoB {
		t.Errorf("expected repo-b path %s, got %s", repoB, gotB)
	}
}

// TestCd_WorktreeDirFromEnvOrConfig verifies worktree_dir config is used
// regardless of current working directory.
//
// Scenario: User runs `wt cd -n 1` from a different directory
// Expected: worktree_dir from config is used to resolve path
func TestCd_WorktreeDirFromEnvOrConfig(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache (required for ID lookup)
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir, // Set via config
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		ID: 1,
	}

	output, err := runCdCommand(t, repoDir, cfg, cmd) // Run from different dir
	if err != nil {
		t.Fatalf("wt cd failed: %v", err)
	}

	if strings.TrimSpace(output) != worktreePath {
		t.Errorf("expected path %s, got %s", worktreePath, strings.TrimSpace(output))
	}
}

// TestCd_BranchWithSlashesInName verifies navigation works for branches
// with slashes (e.g., feature/name).
//
// Scenario: User has worktree for feature/my-feature branch
// Expected: Worktree path returned correctly (slashes sanitized in dir name)
func TestCd_BranchWithSlashesInName(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree for branch with slashes
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature-my-feature")
	setupWorktree(t, repoPath, worktreePath, "feature/my-feature")

	// Populate cache (required for ID lookup)
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		ID: 1,
	}

	output, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt cd -n 1 failed: %v", err)
	}

	if strings.TrimSpace(output) != worktreePath {
		t.Errorf("expected path %s, got %s", worktreePath, strings.TrimSpace(output))
	}
}

// TestCd_RepoUsesWorktreeDirIfNoRepoDir verifies -r flag scans worktree_dir
// for repos when repo_dir is not configured.
//
// Scenario: No repo_dir set, user runs `wt cd -r myrepo`
// Expected: Repo found in worktree_dir
func TestCd_RepoUsesWorktreeDirIfNoRepoDir(t *testing.T) {
	t.Parallel()
	// When repo_dir not set, -r should scan worktree_dir for repos
	worktreeDir := t.TempDir()

	// Create repo directly in worktree_dir
	repoPath := setupTestRepo(t, worktreeDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        "", // Not set
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		Repository: "myrepo",
	}

	output, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt cd -r myrepo failed: %v", err)
	}

	// Resolve output path for comparison (handles macOS symlinks)
	gotPath := resolvePath(t, strings.TrimSpace(output))
	if gotPath != repoPath {
		t.Errorf("expected repo path %s, got %s", repoPath, gotPath)
	}
}

// TestCd_LabelUsesWorktreeDirIfNoRepoDir verifies -l flag scans worktree_dir
// for repos when repo_dir is not configured.
//
// Scenario: No repo_dir set, user runs `wt cd -l backend`
// Expected: Repo with label found in worktree_dir
func TestCd_LabelUsesWorktreeDirIfNoRepoDir(t *testing.T) {
	t.Parallel()
	// When repo_dir not set, -l should scan worktree_dir for repos
	worktreeDir := t.TempDir()

	// Create repo directly in worktree_dir with label
	repoPath := setupTestRepo(t, worktreeDir, "myrepo")
	setRepoLabel(t, repoPath, "backend")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        "", // Not set
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CdCmd{
		Label: "backend",
	}

	output, err := runCdCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt cd -l backend failed: %v", err)
	}

	// Resolve output path for comparison (handles macOS symlinks)
	gotPath := resolvePath(t, strings.TrimSpace(output))
	if gotPath != repoPath {
		t.Errorf("expected repo path %s, got %s", repoPath, gotPath)
	}
}

// runCdCommand runs wt cd with the given config and command in the specified directory.
// Returns the printed output (path).
func runCdCommand(t *testing.T, workDir string, cfg *config.Config, cmd *CdCmd) (string, error) {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx, out := testContextWithOutput(t)
	err := cmd.runCd(ctx)
	return out.String(), err
}

// Note: populateCache is defined in integration_test_helpers.go

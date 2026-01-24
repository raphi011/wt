//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

func TestExec_ByWorktreeID(t *testing.T) {
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

	// Create a marker file to verify command ran in correct directory
	cmd := &ExecCmd{
		ID:      []int{1},
		Command: []string{"touch", "exec-marker.txt"},
	}

	if err := runExecCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt exec -i 1 -- touch exec-marker.txt failed: %v", err)
	}

	// Verify marker file was created in worktree
	markerPath := filepath.Join(worktreePath, "exec-marker.txt")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Errorf("marker file not created in worktree at %s", markerPath)
	}
}

func TestExec_MultipleIDs(t *testing.T) {
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with multiple worktrees
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	wt1 := filepath.Join(worktreeDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")

	wt2 := filepath.Join(worktreeDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	// Populate cache (required for ID lookup)
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Execute in both worktrees
	cmd := &ExecCmd{
		ID:      []int{1, 2},
		Command: []string{"touch", "multi-exec-marker.txt"},
	}

	if err := runExecCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt exec -i 1 -i 2 -- touch multi-exec-marker.txt failed: %v", err)
	}

	// Verify marker file was created in both worktrees
	for _, wtPath := range []string{wt1, wt2} {
		markerPath := filepath.Join(wtPath, "multi-exec-marker.txt")
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			t.Errorf("marker file not created in worktree at %s", markerPath)
		}
	}
}

func TestExec_ByRepoName(t *testing.T) {
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

	// Execute in repo by name
	cmd := &ExecCmd{
		Repository: []string{"myrepo"},
		Command:    []string{"touch", "repo-exec-marker.txt"},
	}

	if err := runExecCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt exec -r myrepo -- touch repo-exec-marker.txt failed: %v", err)
	}

	// Verify marker file was created in main repo (not worktree)
	markerPath := filepath.Join(repoPath, "repo-exec-marker.txt")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Errorf("marker file not created in repo at %s", markerPath)
	}
}

func TestExec_ByLabel(t *testing.T) {
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

	// Execute by label
	cmd := &ExecCmd{
		Label:   []string{"backend"},
		Command: []string{"touch", "label-exec-marker.txt"},
	}

	if err := runExecCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt exec -l backend -- touch label-exec-marker.txt failed: %v", err)
	}

	// Verify marker file was created in repo
	markerPath := filepath.Join(repoPath, "label-exec-marker.txt")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Errorf("marker file not created in repo at %s", markerPath)
	}
}

func TestExec_MultipleRepos(t *testing.T) {
	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create multiple repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Execute in both repos
	cmd := &ExecCmd{
		Repository: []string{"repo-a", "repo-b"},
		Command:    []string{"touch", "multi-repo-marker.txt"},
	}

	if err := runExecCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt exec -r repo-a -r repo-b -- touch multi-repo-marker.txt failed: %v", err)
	}

	// Verify marker file was created in both repos
	for _, repoPath := range []string{repoA, repoB} {
		markerPath := filepath.Join(repoPath, "multi-repo-marker.txt")
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			t.Errorf("marker file not created in repo at %s", markerPath)
		}
	}
}

func TestExec_MultipleLabels(t *testing.T) {
	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repos with different labels
	repoA := setupTestRepo(t, repoDir, "repo-a")
	setRepoLabel(t, repoA, "frontend")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	setRepoLabel(t, repoB, "backend")

	repoC := setupTestRepo(t, repoDir, "repo-c")
	setRepoLabel(t, repoC, "infra")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Execute in frontend and backend labels (not infra)
	cmd := &ExecCmd{
		Label:   []string{"frontend", "backend"},
		Command: []string{"touch", "label-multi-marker.txt"},
	}

	if err := runExecCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt exec -l frontend -l backend -- touch label-multi-marker.txt failed: %v", err)
	}

	// Verify marker file was created in frontend and backend repos
	for _, repoPath := range []string{repoA, repoB} {
		markerPath := filepath.Join(repoPath, "label-multi-marker.txt")
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			t.Errorf("marker file not created in repo at %s", markerPath)
		}
	}

	// Verify marker file was NOT created in infra repo
	infraMarker := filepath.Join(repoC, "label-multi-marker.txt")
	if _, err := os.Stat(infraMarker); !os.IsNotExist(err) {
		t.Errorf("marker file should NOT be created in infra repo at %s", infraMarker)
	}
}

func TestExec_CombineRepoAndLabel(t *testing.T) {
	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")
	setRepoLabel(t, repoB, "backend")
	repoC := setupTestRepo(t, repoDir, "repo-c")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Use both -r and -l
	cmd := &ExecCmd{
		Repository: []string{"repo-a"},
		Label:      []string{"backend"},
		Command:    []string{"touch", "combined-marker.txt"},
	}

	if err := runExecCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt exec -r repo-a -l backend -- touch combined-marker.txt failed: %v", err)
	}

	// Verify marker file was created in repo-a (from -r) and repo-b (from -l backend)
	for _, repoPath := range []string{repoA, repoB} {
		markerPath := filepath.Join(repoPath, "combined-marker.txt")
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			t.Errorf("marker file not created in repo at %s", markerPath)
		}
	}

	// Verify marker file was NOT created in repo-c
	markerC := filepath.Join(repoC, "combined-marker.txt")
	if _, err := os.Stat(markerC); !os.IsNotExist(err) {
		t.Errorf("marker file should NOT be created in repo-c at %s", markerC)
	}
}

func TestExec_ErrorNoTargetSpecified(t *testing.T) {
	worktreeDir := t.TempDir()

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ExecCmd{
		// No target specified
		Command: []string{"echo", "hello"},
	}

	err := runExecCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when no target specified")
	}
	if !strings.Contains(err.Error(), "specify target") {
		t.Errorf("expected 'specify target' error, got: %v", err)
	}
}

func TestExec_ErrorNoCommandSpecified(t *testing.T) {
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ExecCmd{
		ID:      []int{1},
		Command: []string{}, // No command
	}

	err := runExecCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when no command specified")
	}
	if !strings.Contains(err.Error(), "no command") {
		t.Errorf("expected 'no command' error, got: %v", err)
	}
}

func TestExec_ErrorInvalidID(t *testing.T) {
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ExecCmd{
		ID:      []int{999}, // Invalid ID
		Command: []string{"echo", "hello"},
	}

	err := runExecCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

func TestExec_ErrorRepoNotFound(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create a different repo
	setupTestRepo(t, repoDir, "other-repo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ExecCmd{
		Repository: []string{"nonexistent-repo"},
		Command:    []string{"echo", "hello"},
	}

	err := runExecCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when repo not found")
	}
	if !strings.Contains(err.Error(), "nonexistent-repo") {
		t.Errorf("expected error to mention repo name, got: %v", err)
	}
}

func TestExec_ErrorLabelNotFound(t *testing.T) {
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo without label
	setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ExecCmd{
		Label:   []string{"nonexistent-label"},
		Command: []string{"echo", "hello"},
	}

	err := runExecCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when label not found")
	}
	if !strings.Contains(err.Error(), "nonexistent-label") {
		t.Errorf("expected error to mention label, got: %v", err)
	}
}

func TestExec_CommandFails(t *testing.T) {
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Use a command that will fail
	cmd := &ExecCmd{
		ID:      []int{1},
		Command: []string{"false"}, // 'false' always exits with 1
	}

	err := runExecCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when command fails")
	}
}

func TestExec_PartialFailure(t *testing.T) {
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with multiple worktrees
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	wt1 := filepath.Join(worktreeDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")

	wt2 := filepath.Join(worktreeDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Create a file that exists only in wt1
	testFile := filepath.Join(wt1, "only-in-wt1.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Run a command that will succeed in wt1 but fail in wt2
	// 'cat only-in-wt1.txt' will work in wt1, fail in wt2
	cmd := &ExecCmd{
		ID:      []int{1, 2},
		Command: []string{"cat", "only-in-wt1.txt"},
	}

	err := runExecCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error for partial failure")
	}

	// Error should mention the failure
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected error to mention failure, got: %v", err)
	}
}

func TestExec_CommandWithArguments(t *testing.T) {
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Use a command with multiple arguments
	cmd := &ExecCmd{
		ID:      []int{1},
		Command: []string{"sh", "-c", "echo 'test content' > args-marker.txt"},
	}

	if err := runExecCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt exec with args failed: %v", err)
	}

	// Verify file was created with correct content
	markerPath := filepath.Join(worktreePath, "args-marker.txt")
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Errorf("failed to read marker file: %v", err)
	}
	if !strings.Contains(string(content), "test content") {
		t.Errorf("expected 'test content' in marker file, got: %s", content)
	}
}

func TestExec_StripLeadingDashes(t *testing.T) {
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := t.TempDir()

	// Create repo with worktree
	repoPath := setupTestRepo(t, repoDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Simulate kong passthrough which includes "--"
	cmd := &ExecCmd{
		ID:      []int{1},
		Command: []string{"--", "touch", "dashes-marker.txt"},
	}

	if err := runExecCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt exec with leading -- failed: %v", err)
	}

	// Verify marker file was created (-- should be stripped)
	markerPath := filepath.Join(worktreePath, "dashes-marker.txt")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Errorf("marker file not created at %s", markerPath)
	}
}

// runExecCommand runs wt exec with the given config and command in the specified directory.
func runExecCommand(t *testing.T, workDir string, cfg *config.Config, cmd *ExecCmd) error {
	t.Helper()
	return runExec(cmd, cfg, workDir)
}

// Note: populateCache is defined in integration_test_helpers.go

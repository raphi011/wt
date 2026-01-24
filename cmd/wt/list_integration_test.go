//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

func TestList_InsideRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

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

	cmd := &ListCmd{}

	// Run from inside the worktree (which is part of myrepo)
	output, err := runListCommand(t, worktreePath, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list failed: %v", err)
	}

	// Should show the worktree
	if !strings.Contains(output, "feature") {
		t.Errorf("expected output to contain 'feature', got: %s", output)
	}
}

func TestList_OutsideRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

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

	cmd := &ListCmd{}

	// Run from outside any repo (use worktreeDir itself, which is not a git repo)
	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list failed: %v", err)
	}

	// Should show all worktrees since we're not in any repo
	if !strings.Contains(output, "feature") {
		t.Errorf("expected output to contain 'feature', got: %s", output)
	}
}

func TestList_Global(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos with worktrees
	repoA := setupTestRepo(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Global: true,
	}

	// Run from inside repo-a worktree
	output, err := runListCommand(t, wtA, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -g failed: %v", err)
	}

	// Should show both worktrees
	if !strings.Contains(output, "repo-a") {
		t.Errorf("expected output to contain 'repo-a', got: %s", output)
	}
	if !strings.Contains(output, "repo-b") {
		t.Errorf("expected output to contain 'repo-b', got: %s", output)
	}
}

func TestList_NoWorktrees(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	worktreeDir := resolvePath(t, t.TempDir())

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{}

	// Run from empty worktree directory
	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list failed: %v", err)
	}

	// Should not contain any worktree entries (table headers might appear)
	// An empty list should not contain "ID" column header if using JSON
	// For non-JSON, it's acceptable to show no output or a message
	// Just verify no error occurred
	_ = output
}

func TestList_MultipleWorktreesSameRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with multiple worktrees
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	wt1 := filepath.Join(worktreeDir, "myrepo-feature1")
	setupWorktree(t, repoPath, wt1, "feature1")

	wt2 := filepath.Join(worktreeDir, "myrepo-feature2")
	setupWorktree(t, repoPath, wt2, "feature2")

	wt3 := filepath.Join(worktreeDir, "myrepo-feature3")
	setupWorktree(t, repoPath, wt3, "feature3")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{}

	output, err := runListCommand(t, wt1, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list failed: %v", err)
	}

	// Should show all three worktrees
	if !strings.Contains(output, "feature1") {
		t.Errorf("expected output to contain 'feature1', got: %s", output)
	}
	if !strings.Contains(output, "feature2") {
		t.Errorf("expected output to contain 'feature2', got: %s", output)
	}
	if !strings.Contains(output, "feature3") {
		t.Errorf("expected output to contain 'feature3', got: %s", output)
	}
}

func TestList_FilterByRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos with worktrees
	repoA := setupTestRepo(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Repository: []string{"repo-a"},
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -r repo-a failed: %v", err)
	}

	// Should show only repo-a worktree
	if !strings.Contains(output, "repo-a") {
		t.Errorf("expected output to contain 'repo-a', got: %s", output)
	}
	// repo-b should not appear (except possibly in the "Listing worktrees" header count)
	// Use JSON to verify precisely
}

func TestList_FilterByMultipleRepos(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create three repos with worktrees
	repoA := setupTestRepo(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	repoC := setupTestRepo(t, repoDir, "repo-c")
	wtC := filepath.Join(worktreeDir, "repo-c-feature")
	setupWorktree(t, repoC, wtC, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Repository: []string{"repo-a", "repo-b"},
		JSON:       true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -r repo-a -r repo-b --json failed: %v", err)
	}

	// Parse JSON to verify
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(worktrees) != 2 {
		t.Errorf("expected 2 worktrees, got %d", len(worktrees))
	}

	// Verify repo-a and repo-b are included, repo-c is not
	repoNames := make(map[string]bool)
	for _, wt := range worktrees {
		if name, ok := wt["repo_name"].(string); ok {
			repoNames[name] = true
		}
	}

	if !repoNames["repo-a"] {
		t.Error("expected repo-a to be in output")
	}
	if !repoNames["repo-b"] {
		t.Error("expected repo-b to be in output")
	}
	if repoNames["repo-c"] {
		t.Error("repo-c should not be in output")
	}
}

func TestList_FilterByRepoOverridesCurrentRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos with worktrees
	repoA := setupTestRepo(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Repository: []string{"repo-b"},
		JSON:       true,
	}

	// Run from inside repo-a worktree but filter by repo-b
	output, err := runListCommand(t, wtA, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -r repo-b --json failed: %v", err)
	}

	// Parse JSON to verify
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Should only show repo-b, not repo-a (current repo)
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}

	if len(worktrees) > 0 {
		repoName := worktrees[0]["repo_name"].(string)
		if repoName != "repo-b" {
			t.Errorf("expected repo-b, got %s", repoName)
		}
	}
}

func TestList_FilterByRepoNotFound(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create a repo
	setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Repository: []string{"nonexistent-repo"},
	}

	// Should produce a warning (to stderr) but not fail
	// Note: warnings go to stderr which we don't capture in parallel-safe tests
	_, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -r nonexistent-repo failed: %v", err)
	}
	// Command should succeed even with nonexistent repo filter
}

func TestList_FilterByLabel(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos with labels
	repoA := setupTestRepo(t, repoDir, "repo-a")
	setRepoLabel(t, repoA, "frontend")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	setRepoLabel(t, repoB, "backend")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Label: []string{"frontend"},
		JSON:  true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -l frontend --json failed: %v", err)
	}

	// Parse JSON to verify
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Should only show repo-a (frontend label)
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}

	if len(worktrees) > 0 {
		repoName := worktrees[0]["repo_name"].(string)
		if repoName != "repo-a" {
			t.Errorf("expected repo-a (frontend), got %s", repoName)
		}
	}
}

func TestList_FilterByMultipleLabels(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create three repos with different labels
	repoA := setupTestRepo(t, repoDir, "repo-a")
	setRepoLabel(t, repoA, "frontend")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	setRepoLabel(t, repoB, "backend")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	repoC := setupTestRepo(t, repoDir, "repo-c")
	setRepoLabel(t, repoC, "infra")
	wtC := filepath.Join(worktreeDir, "repo-c-feature")
	setupWorktree(t, repoC, wtC, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Label: []string{"frontend", "backend"},
		JSON:  true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -l frontend -l backend --json failed: %v", err)
	}

	// Parse JSON to verify
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Should show repo-a (frontend) and repo-b (backend), not repo-c (infra)
	if len(worktrees) != 2 {
		t.Errorf("expected 2 worktrees, got %d", len(worktrees))
	}

	repoNames := make(map[string]bool)
	for _, wt := range worktrees {
		if name, ok := wt["repo_name"].(string); ok {
			repoNames[name] = true
		}
	}

	if !repoNames["repo-a"] {
		t.Error("expected repo-a (frontend) to be in output")
	}
	if !repoNames["repo-b"] {
		t.Error("expected repo-b (backend) to be in output")
	}
	if repoNames["repo-c"] {
		t.Error("repo-c (infra) should not be in output")
	}
}

func TestList_FilterByLabelOverridesCurrentRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	// repo-a has no label
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	setRepoLabel(t, repoB, "backend")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Label: []string{"backend"},
		JSON:  true,
	}

	// Run from inside repo-a worktree but filter by backend label
	output, err := runListCommand(t, wtA, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -l backend --json failed: %v", err)
	}

	// Parse JSON to verify
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Should only show repo-b (backend), not repo-a (current repo)
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}

	if len(worktrees) > 0 {
		repoName := worktrees[0]["repo_name"].(string)
		if repoName != "repo-b" {
			t.Errorf("expected repo-b (backend), got %s", repoName)
		}
	}
}

func TestList_FilterByLabelNotFound(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create a repo without matching label
	repoA := setupTestRepo(t, repoDir, "repo-a")
	setRepoLabel(t, repoA, "frontend")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Label: []string{"nonexistent-label"},
	}

	// Should produce a warning (to stderr) but not fail
	// Note: warnings go to stderr which we don't capture in parallel-safe tests
	_, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -l nonexistent-label failed: %v", err)
	}
	// Command should succeed even with nonexistent label filter
}

func TestList_CombineRepoAndLabel(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create three repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	// repo-a has no label
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	setRepoLabel(t, repoB, "backend")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	repoC := setupTestRepo(t, repoDir, "repo-c")
	// repo-c has no label
	wtC := filepath.Join(worktreeDir, "repo-c-feature")
	setupWorktree(t, repoC, wtC, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Use both -r repo-a and -l backend (repo-b)
	cmd := &ListCmd{
		Repository: []string{"repo-a"},
		Label:      []string{"backend"},
		JSON:       true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -r repo-a -l backend --json failed: %v", err)
	}

	// Parse JSON to verify
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Should show repo-a (from -r) and repo-b (from -l backend), not repo-c
	if len(worktrees) != 2 {
		t.Errorf("expected 2 worktrees, got %d", len(worktrees))
	}

	repoNames := make(map[string]bool)
	for _, wt := range worktrees {
		if name, ok := wt["repo_name"].(string); ok {
			repoNames[name] = true
		}
	}

	if !repoNames["repo-a"] {
		t.Error("expected repo-a to be in output")
	}
	if !repoNames["repo-b"] {
		t.Error("expected repo-b (backend) to be in output")
	}
	if repoNames["repo-c"] {
		t.Error("repo-c should not be in output")
	}
}

func TestList_FilterFromMultipleRepos(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos with multiple worktrees each
	repoA := setupTestRepo(t, repoDir, "repo-a")
	wtA1 := filepath.Join(worktreeDir, "repo-a-feature1")
	setupWorktree(t, repoA, wtA1, "feature1")
	wtA2 := filepath.Join(worktreeDir, "repo-a-feature2")
	setupWorktree(t, repoA, wtA2, "feature2")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	wtB1 := filepath.Join(worktreeDir, "repo-b-feature1")
	setupWorktree(t, repoB, wtB1, "feature1")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Repository: []string{"repo-a", "repo-b"},
		JSON:       true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -r repo-a -r repo-b --json failed: %v", err)
	}

	// Parse JSON to verify
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Should show all 3 worktrees (2 from repo-a, 1 from repo-b)
	if len(worktrees) != 3 {
		t.Errorf("expected 3 worktrees, got %d", len(worktrees))
	}
}

func TestList_SortByID(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with multiple worktrees
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	wt1 := filepath.Join(worktreeDir, "myrepo-aaa")
	setupWorktree(t, repoPath, wt1, "aaa")

	wt2 := filepath.Join(worktreeDir, "myrepo-bbb")
	setupWorktree(t, repoPath, wt2, "bbb")

	wt3 := filepath.Join(worktreeDir, "myrepo-ccc")
	setupWorktree(t, repoPath, wt3, "ccc")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Sort: "id",
		JSON: true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -s id --json failed: %v", err)
	}

	// Parse JSON to verify order
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(worktrees) < 2 {
		t.Fatalf("expected at least 2 worktrees, got %d", len(worktrees))
	}

	// Verify IDs are in ascending order
	prevID := 0
	for _, wt := range worktrees {
		id := int(wt["id"].(float64))
		if id < prevID {
			t.Errorf("IDs not in ascending order: %d came after %d", id, prevID)
		}
		prevID = id
	}
}

func TestList_SortByRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repos with names in non-alphabetical order
	repoC := setupTestRepo(t, repoDir, "repo-c")
	wtC := filepath.Join(worktreeDir, "repo-c-feature")
	setupWorktree(t, repoC, wtC, "feature")

	repoA := setupTestRepo(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Sort: "repo",
		JSON: true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -s repo --json failed: %v", err)
	}

	// Parse JSON to verify order
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(worktrees) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(worktrees))
	}

	// Verify repos are in alphabetical order
	expectedOrder := []string{"repo-a", "repo-b", "repo-c"}
	for i, wt := range worktrees {
		repoName := wt["repo_name"].(string)
		if repoName != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], repoName)
		}
	}
}

func TestList_SortByBranch(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with branches in non-alphabetical order
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	wt3 := filepath.Join(worktreeDir, "myrepo-zeta")
	setupWorktree(t, repoPath, wt3, "zeta")

	wt1 := filepath.Join(worktreeDir, "myrepo-alpha")
	setupWorktree(t, repoPath, wt1, "alpha")

	wt2 := filepath.Join(worktreeDir, "myrepo-beta")
	setupWorktree(t, repoPath, wt2, "beta")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Sort: "branch",
		JSON: true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -s branch --json failed: %v", err)
	}

	// Parse JSON to verify order
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(worktrees) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(worktrees))
	}

	// Verify branches are in alphabetical order
	expectedOrder := []string{"alpha", "beta", "zeta"}
	for i, wt := range worktrees {
		branch := wt["branch"].(string)
		if branch != expectedOrder[i] {
			t.Errorf("position %d: expected %s, got %s", i, expectedOrder[i], branch)
		}
	}
}

func TestList_SortByCommit(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create repo with worktrees
	repoPath := setupTestRepo(t, repoDir, "myrepo")

	wt1 := filepath.Join(worktreeDir, "myrepo-old")
	setupWorktree(t, repoPath, wt1, "old")

	wt2 := filepath.Join(worktreeDir, "myrepo-middle")
	setupWorktree(t, repoPath, wt2, "middle")

	wt3 := filepath.Join(worktreeDir, "myrepo-recent")
	setupWorktree(t, repoPath, wt3, "recent")

	// Make commits with explicit different timestamps (recent should be last)
	// Using dates that are days apart to ensure clear ordering
	makeCommitInWorktreeWithDate(t, wt1, "old-file.txt", "2024-01-01T12:00:00Z")
	makeCommitInWorktreeWithDate(t, wt2, "middle-file.txt", "2024-01-15T12:00:00Z")
	makeCommitInWorktreeWithDate(t, wt3, "recent-file.txt", "2024-01-30T12:00:00Z")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		Sort: "commit",
		JSON: true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list -s commit --json failed: %v", err)
	}

	// Parse JSON to verify order
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(worktrees) != 3 {
		t.Fatalf("expected 3 worktrees, got %d", len(worktrees))
	}

	// Most recent should be first (descending order)
	firstBranch := worktrees[0]["branch"].(string)
	if firstBranch != "recent" {
		t.Errorf("expected 'recent' branch first, got %s", firstBranch)
	}
}

func TestList_JSONOutput(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

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

	cmd := &ListCmd{
		JSON: true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list --json failed: %v", err)
	}

	// Parse JSON to verify structure
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktrees))
	}

	wt := worktrees[0]

	// Verify required fields exist
	requiredFields := []string{"id", "path", "branch", "origin_url"}
	for _, field := range requiredFields {
		if _, ok := wt[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	// Verify field values
	if wt["branch"].(string) != "feature" {
		t.Errorf("expected branch 'feature', got %s", wt["branch"])
	}

	if wt["path"].(string) != worktreePath {
		t.Errorf("expected path %s, got %s", worktreePath, wt["path"])
	}

	// ID should be a number > 0
	id := int(wt["id"].(float64))
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}
}

func TestList_JSONOutputMultiple(t *testing.T) {
	t.Parallel()
	// Setup temp directories (resolve symlinks for macOS compatibility)
	worktreeDir := resolvePath(t, t.TempDir())
	repoDir := resolvePath(t, t.TempDir())

	// Create two repos with worktrees
	repoA := setupTestRepo(t, repoDir, "repo-a")
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	setupWorktree(t, repoA, wtA, "feature")

	repoB := setupTestRepo(t, repoDir, "repo-b")
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	setupWorktree(t, repoB, wtB, "feature")

	// Populate cache
	populateCache(t, worktreeDir)

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &ListCmd{
		JSON: true,
	}

	output, err := runListCommand(t, worktreeDir, cfg, cmd)
	if err != nil {
		t.Fatalf("wt list --json failed: %v", err)
	}

	// Parse JSON to verify structure
	var worktrees []map[string]interface{}
	if err := json.Unmarshal([]byte(output), &worktrees); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}

	// Verify each worktree has unique ID
	ids := make(map[int]bool)
	for _, wt := range worktrees {
		id := int(wt["id"].(float64))
		if ids[id] {
			t.Errorf("duplicate ID found: %d", id)
		}
		ids[id] = true
	}
}

// runListCommand runs wt list with the given config and command in the specified directory.
// Returns the stdout output.
func runListCommand(t *testing.T, workDir string, cfg *config.Config, cmd *ListCmd) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := runList(cmd, cfg, workDir, &buf)
	return buf.String(), err
}

// makeCommitInWorktree creates a commit in a worktree directory.
func makeCommitInWorktree(t *testing.T, worktreePath, filename string) {
	t.Helper()

	filePath := filepath.Join(worktreePath, filename)
	if err := os.WriteFile(filePath, []byte("content for "+filename+"\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	runGitCommand(t, worktreePath, "git", "add", filename)
	runGitCommand(t, worktreePath, "git", "commit", "-m", "Add "+filename)
}

// makeCommitInWorktreeWithDate creates a commit with a specific date.
// The date should be in RFC 3339 format (e.g., "2024-01-15T12:00:00Z").
func makeCommitInWorktreeWithDate(t *testing.T, worktreePath, filename, date string) {
	t.Helper()

	filePath := filepath.Join(worktreePath, filename)
	if err := os.WriteFile(filePath, []byte("content for "+filename+"\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	runGitCommand(t, worktreePath, "git", "add", filename)

	// Use GIT_COMMITTER_DATE and --date to set both dates
	cmd := exec.Command("git", "commit", "-m", "Add "+filename, "--date", date)
	cmd.Dir = worktreePath
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to run git commit: %v\n%s", err, out)
	}
}

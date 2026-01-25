//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

// --- Basic Listing Tests ---

func TestRepos_ListsReposInDirectory(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	setupTestRepo(t, repoDir, "repo-a")
	setupTestRepo(t, repoDir, "repo-b")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos failed: %v", err)
	}

	if !strings.Contains(output, "repo-a") {
		t.Errorf("expected output to contain 'repo-a', got: %s", output)
	}
	if !strings.Contains(output, "repo-b") {
		t.Errorf("expected output to contain 'repo-b', got: %s", output)
	}
}

func TestRepos_SingleRepo(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos failed: %v", err)
	}

	if !strings.Contains(output, "myrepo") {
		t.Errorf("expected output to contain 'myrepo', got: %s", output)
	}
	if !strings.Contains(output, "(1)") {
		t.Errorf("expected output to show count (1), got: %s", output)
	}
}

func TestRepos_NoReposFound(t *testing.T) {
	t.Parallel()
	emptyDir := t.TempDir()
	emptyDir = resolvePath(t, emptyDir)

	cfg := &config.Config{RepoDir: emptyDir}
	cmd := &ReposCmd{}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos failed: %v", err)
	}

	if !strings.Contains(output, "No repositories found") {
		t.Errorf("expected 'No repositories found' message, got: %s", output)
	}
}

// --- Label Filter Tests ---

func TestRepos_FilterByLabel(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	repoA := setupTestRepo(t, repoDir, "repo-a")
	setupTestRepo(t, repoDir, "repo-b")
	repoC := setupTestRepo(t, repoDir, "repo-c")

	// Set labels
	setRepoLabel(t, repoA, "backend")
	setRepoLabel(t, repoC, "backend")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{Label: "backend"}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos -l backend failed: %v", err)
	}

	if !strings.Contains(output, "repo-a") {
		t.Errorf("expected output to contain 'repo-a' (has backend label), got: %s", output)
	}
	if !strings.Contains(output, "repo-c") {
		t.Errorf("expected output to contain 'repo-c' (has backend label), got: %s", output)
	}
	if strings.Contains(output, "repo-b") {
		t.Errorf("expected output NOT to contain 'repo-b' (no backend label), got: %s", output)
	}
}

func TestRepos_FilterByLabel_NoMatch(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	setupTestRepo(t, repoDir, "repo-a")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{Label: "nonexistent"}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos -l nonexistent failed: %v", err)
	}

	if !strings.Contains(output, "No repositories found with label") {
		t.Errorf("expected 'No repositories found with label' message, got: %s", output)
	}
}

// --- Sort Tests ---

func TestRepos_SortByName(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	setupTestRepo(t, repoDir, "zebra")
	setupTestRepo(t, repoDir, "alpha")
	setupTestRepo(t, repoDir, "beta")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{Sort: "name", JSON: true}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos --sort=name --json failed: %v", err)
	}

	var repos []RepoInfo
	if err := json.Unmarshal([]byte(output), &repos); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}

	// Should be sorted alphabetically
	if repos[0].Name != "alpha" {
		t.Errorf("expected first repo to be 'alpha', got '%s'", repos[0].Name)
	}
	if repos[1].Name != "beta" {
		t.Errorf("expected second repo to be 'beta', got '%s'", repos[1].Name)
	}
	if repos[2].Name != "zebra" {
		t.Errorf("expected third repo to be 'zebra', got '%s'", repos[2].Name)
	}
}

func TestRepos_SortByBranch(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")
	repoC := setupTestRepo(t, repoDir, "repo-c")

	// Change branches to different names
	runGitCommand(t, repoA, "git", "checkout", "-b", "zebra-branch")
	runGitCommand(t, repoB, "git", "checkout", "-b", "alpha-branch")
	runGitCommand(t, repoC, "git", "checkout", "-b", "beta-branch")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{Sort: "branch", JSON: true}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos --sort=branch --json failed: %v", err)
	}

	var repos []RepoInfo
	if err := json.Unmarshal([]byte(output), &repos); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}

	// Should be sorted by branch name alphabetically
	if repos[0].Branch != "alpha-branch" {
		t.Errorf("expected first repo branch to be 'alpha-branch', got '%s'", repos[0].Branch)
	}
	if repos[1].Branch != "beta-branch" {
		t.Errorf("expected second repo branch to be 'beta-branch', got '%s'", repos[1].Branch)
	}
	if repos[2].Branch != "zebra-branch" {
		t.Errorf("expected third repo branch to be 'zebra-branch', got '%s'", repos[2].Branch)
	}
}

func TestRepos_SortByWorktrees(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)
	worktreeDir = resolvePath(t, worktreeDir)

	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")
	repoC := setupTestRepo(t, repoDir, "repo-c")

	// Create different numbers of worktrees
	// repo-a: 2 worktrees
	setupWorktree(t, repoA, worktreeDir+"/repo-a-wt1", "feature1")
	setupWorktree(t, repoA, worktreeDir+"/repo-a-wt2", "feature2")
	// repo-b: 0 worktrees
	_ = repoB
	// repo-c: 1 worktree
	setupWorktree(t, repoC, worktreeDir+"/repo-c-wt1", "feature1")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{Sort: "worktrees", JSON: true}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos --sort=worktrees --json failed: %v", err)
	}

	var repos []RepoInfo
	if err := json.Unmarshal([]byte(output), &repos); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}

	// Should be sorted by worktree count descending
	if repos[0].WorktreeCount != 2 {
		t.Errorf("expected first repo to have 2 worktrees, got %d", repos[0].WorktreeCount)
	}
	if repos[1].WorktreeCount != 1 {
		t.Errorf("expected second repo to have 1 worktree, got %d", repos[1].WorktreeCount)
	}
	if repos[2].WorktreeCount != 0 {
		t.Errorf("expected third repo to have 0 worktrees, got %d", repos[2].WorktreeCount)
	}
}

func TestRepos_SortByLabel(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")
	repoC := setupTestRepo(t, repoDir, "repo-c")
	repoD := setupTestRepo(t, repoDir, "repo-d")

	// Set labels (some with, some without)
	setRepoLabel(t, repoA, "zebra")
	setRepoLabel(t, repoB, "alpha")
	// repoC has no label
	_ = repoC
	setRepoLabel(t, repoD, "beta")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{Sort: "label", JSON: true}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos --sort=label --json failed: %v", err)
	}

	var repos []RepoInfo
	if err := json.Unmarshal([]byte(output), &repos); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(repos) != 4 {
		t.Fatalf("expected 4 repos, got %d", len(repos))
	}

	// Should be sorted by label alphabetically, unlabeled last
	if len(repos[0].Labels) == 0 || repos[0].Labels[0] != "alpha" {
		t.Errorf("expected first repo label to be 'alpha', got %v", repos[0].Labels)
	}
	if len(repos[1].Labels) == 0 || repos[1].Labels[0] != "beta" {
		t.Errorf("expected second repo label to be 'beta', got %v", repos[1].Labels)
	}
	if len(repos[2].Labels) == 0 || repos[2].Labels[0] != "zebra" {
		t.Errorf("expected third repo label to be 'zebra', got %v", repos[2].Labels)
	}
	// Last repo should be unlabeled (repo-c)
	if len(repos[3].Labels) != 0 {
		t.Errorf("expected fourth repo to have no labels, got %v", repos[3].Labels)
	}
}

// --- JSON Output Tests ---

func TestRepos_JSONOutput(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)
	worktreeDir = resolvePath(t, worktreeDir)

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	setRepoLabel(t, repoPath, "backend")
	setupWorktree(t, repoPath, worktreeDir+"/myrepo-feature", "feature")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{JSON: true}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos --json failed: %v", err)
	}

	var repos []RepoInfo
	if err := json.Unmarshal([]byte(output), &repos); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	repo := repos[0]
	if repo.Name != "myrepo" {
		t.Errorf("expected name 'myrepo', got '%s'", repo.Name)
	}
	if repo.Branch != "main" {
		t.Errorf("expected branch 'main', got '%s'", repo.Branch)
	}
	if len(repo.Labels) != 1 || repo.Labels[0] != "backend" {
		t.Errorf("expected labels ['backend'], got %v", repo.Labels)
	}
	if repo.WorktreeCount != 1 {
		t.Errorf("expected 1 worktree, got %d", repo.WorktreeCount)
	}
	if repo.Path != repoPath {
		t.Errorf("expected path '%s', got '%s'", repoPath, repo.Path)
	}
	if !strings.Contains(repo.OriginURL, "myrepo") {
		t.Errorf("expected origin URL to contain 'myrepo', got '%s'", repo.OriginURL)
	}
}

func TestRepos_JSONOutputEmpty(t *testing.T) {
	t.Parallel()
	emptyDir := t.TempDir()
	emptyDir = resolvePath(t, emptyDir)

	cfg := &config.Config{RepoDir: emptyDir}
	cmd := &ReposCmd{JSON: true}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos --json failed: %v", err)
	}

	var repos []RepoInfo
	if err := json.Unmarshal([]byte(output), &repos); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	if repos == nil {
		t.Error("expected empty array [], got nil")
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

// --- Worktree Count Tests ---

func TestRepos_ShowsCorrectWorktreeCount(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)
	worktreeDir = resolvePath(t, worktreeDir)

	repoPath := setupTestRepo(t, repoDir, "myrepo")

	// Create 3 worktrees
	setupWorktree(t, repoPath, worktreeDir+"/myrepo-wt1", "feature1")
	setupWorktree(t, repoPath, worktreeDir+"/myrepo-wt2", "feature2")
	setupWorktree(t, repoPath, worktreeDir+"/myrepo-wt3", "feature3")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{JSON: true}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos --json failed: %v", err)
	}

	var repos []RepoInfo
	if err := json.Unmarshal([]byte(output), &repos); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	// Should show 3 worktrees (excluding main worktree)
	if repos[0].WorktreeCount != 3 {
		t.Errorf("expected 3 worktrees, got %d", repos[0].WorktreeCount)
	}
}

func TestRepos_ZeroWorktreesForRepoWithoutWorktrees(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{JSON: true}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos --json failed: %v", err)
	}

	var repos []RepoInfo
	if err := json.Unmarshal([]byte(output), &repos); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	if repos[0].WorktreeCount != 0 {
		t.Errorf("expected 0 worktrees, got %d", repos[0].WorktreeCount)
	}
}

// --- Labels Display Tests ---

func TestRepos_ShowsMultipleLabels(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	setRepoLabels(t, repoPath, "backend", "api", "go")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{JSON: true}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos --json failed: %v", err)
	}

	var repos []RepoInfo
	if err := json.Unmarshal([]byte(output), &repos); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	if len(repos[0].Labels) != 3 {
		t.Errorf("expected 3 labels, got %d: %v", len(repos[0].Labels), repos[0].Labels)
	}
}

func TestRepos_TableShowsLabels(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	repoDir = resolvePath(t, repoDir)

	repoPath := setupTestRepo(t, repoDir, "myrepo")
	setRepoLabel(t, repoPath, "backend")

	cfg := &config.Config{RepoDir: repoDir}
	cmd := &ReposCmd{}

	output, err := runReposCommand(t, cfg, cmd)
	if err != nil {
		t.Fatalf("wt repos failed: %v", err)
	}

	if !strings.Contains(output, "backend") {
		t.Errorf("expected output to contain label 'backend', got: %s", output)
	}
}

// --- Helper Functions ---

func runReposCommand(t *testing.T, cfg *config.Config, cmd *ReposCmd) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := runRepos(cmd, cfg, &buf)
	return buf.String(), err
}

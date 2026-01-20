package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMainRepoPath(t *testing.T) {
	// Create a temp directory to simulate a worktree
	tmpDir := t.TempDir()

	// Test valid .git file
	gitContent := "gitdir: /path/to/repo/.git/worktrees/test-branch"
	gitFile := filepath.Join(tmpDir, ".git")
	if err := os.WriteFile(gitFile, []byte(gitContent), 0644); err != nil {
		t.Fatal(err)
	}

	path, err := GetMainRepoPath(tmpDir)
	if err != nil {
		t.Errorf("GetMainRepoPath failed: %v", err)
	}
	if path != "/path/to/repo" {
		t.Errorf("expected /path/to/repo, got %s", path)
	}

	// Test invalid .git file format
	invalidDir := t.TempDir()
	invalidGitFile := filepath.Join(invalidDir, ".git")
	if err := os.WriteFile(invalidGitFile, []byte("invalid content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = GetMainRepoPath(invalidDir)
	if err == nil {
		t.Error("expected error for invalid .git file format")
	}

	// Test missing .git file
	emptyDir := t.TempDir()
	_, err = GetMainRepoPath(emptyDir)
	if err == nil {
		t.Error("expected error for missing .git file")
	}
}

func TestGetDefaultBranch(t *testing.T) {
	// This test requires a real git repo, so we skip in CI
	// but the function should return "main" or "master" as fallback
	result := GetDefaultBranch("/nonexistent/path")
	if result != "main" && result != "master" {
		t.Errorf("expected main or master as fallback, got %s", result)
	}
}

func TestWorktreeStruct(t *testing.T) {
	wt := Worktree{
		Path:        "/test/path",
		Branch:      "feature-branch",
		MainRepo:    "/test/main",
		RepoName:    "test-repo",
		IsMerged:    true,
		CommitCount: 5,
		IsDirty:     false,
		LastCommit:  "2 days ago",
	}

	if wt.Path != "/test/path" {
		t.Errorf("unexpected path: %s", wt.Path)
	}
	if wt.Branch != "feature-branch" {
		t.Errorf("unexpected branch: %s", wt.Branch)
	}
	if !wt.IsMerged {
		t.Error("expected IsMerged to be true")
	}
}

func TestCreateWorktreeResult(t *testing.T) {
	result := &CreateWorktreeResult{
		Path:          "/test/worktree",
		AlreadyExists: true,
	}

	if result.Path != "/test/worktree" {
		t.Errorf("unexpected path: %s", result.Path)
	}
	if !result.AlreadyExists {
		t.Error("expected AlreadyExists to be true")
	}
}

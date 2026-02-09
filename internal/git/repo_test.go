package git

import (
	"context"
	"path/filepath"
	"testing"
)

func TestGetMainRepoPath(t *testing.T) {
	// Create a real git repo with a worktree to test
	tmpDir := t.TempDir()
	// Resolve symlinks for macOS (/var -> /private/var)
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)
	repoPath := filepath.Join(tmpDir, "test-repo")
	wtPath := filepath.Join(tmpDir, "test-worktree")

	// Initialize a git repo
	ctx := context.Background()
	if err := runGit(ctx, "", "init", repoPath); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	// Configure git user for CI environment
	if err := runGit(ctx, repoPath, "config", "user.email", "test@test.com"); err != nil {
		t.Fatalf("failed to set git email: %v", err)
	}
	if err := runGit(ctx, repoPath, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("failed to set git name: %v", err)
	}

	// Create an initial commit (required for worktrees)
	if err := runGit(ctx, repoPath, "commit", "--allow-empty", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	// Create a worktree
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "test-branch", wtPath); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Test from the worktree
	mainPath, err := GetMainRepoPath(wtPath)
	if err != nil {
		t.Errorf("GetMainRepoPath from worktree failed: %v", err)
	}
	if mainPath != repoPath {
		t.Errorf("expected %s, got %s", repoPath, mainPath)
	}

	// Test from the main repo
	mainPathFromRepo, err := GetMainRepoPath(repoPath)
	if err != nil {
		t.Errorf("GetMainRepoPath from main repo failed: %v", err)
	}
	if mainPathFromRepo != repoPath {
		t.Errorf("expected %s, got %s", repoPath, mainPathFromRepo)
	}

	// Test from non-git directory
	emptyDir := t.TempDir()
	_, err = GetMainRepoPath(emptyDir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestGetDefaultBranch(t *testing.T) {
	// This test requires a real git repo, so we skip in CI
	// but the function should return "main" or "master" as fallback
	result := GetDefaultBranch(context.Background(), "/nonexistent/path")
	if result != "main" && result != "master" {
		t.Errorf("expected main or master as fallback, got %s", result)
	}
}

func TestWorktreeStruct(t *testing.T) {
	wt := Worktree{
		Path:     "/test/path",
		Branch:   "feature-branch",
		RepoPath: "/test/main",
		RepoName: "test-repo",
		PRState:  "MERGED",
	}

	if wt.Path != "/test/path" {
		t.Errorf("unexpected path: %s", wt.Path)
	}
	if wt.Branch != "feature-branch" {
		t.Errorf("unexpected branch: %s", wt.Branch)
	}
	if wt.PRState != "MERGED" {
		t.Errorf("expected PRState to be MERGED, got %s", wt.PRState)
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

func TestParseRemoteRef(t *testing.T) {
	// Create a git repo with origin remote
	tmpDir := t.TempDir()
	tmpDir, _ = filepath.EvalSymlinks(tmpDir)
	repoPath := filepath.Join(tmpDir, "test-repo")
	ctx := context.Background()

	if err := runGit(ctx, "", "init", repoPath); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	// Add origin remote
	if err := runGit(ctx, repoPath, "remote", "add", "origin", "https://github.com/test/repo.git"); err != nil {
		t.Fatalf("failed to add origin: %v", err)
	}
	// Add upstream remote
	if err := runGit(ctx, repoPath, "remote", "add", "upstream", "https://github.com/upstream/repo.git"); err != nil {
		t.Fatalf("failed to add upstream: %v", err)
	}

	tests := []struct {
		ref          string
		wantRemote   string
		wantBranch   string
		wantIsRemote bool
	}{
		{"main", "", "main", false},                             // Simple branch, no remote
		{"feature/test", "", "feature/test", false},             // Branch with slash, no matching remote
		{"origin/main", "origin", "main", true},                 // Origin remote ref
		{"origin/feature/test", "origin", "feature/test", true}, // Origin with nested branch
		{"upstream/develop", "upstream", "develop", true},       // Upstream remote ref
		{"nonexistent/branch", "", "nonexistent/branch", false}, // Non-existent remote
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			remote, branch, isRemote := ParseRemoteRef(ctx, repoPath, tt.ref)
			if remote != tt.wantRemote {
				t.Errorf("remote: got %q, want %q", remote, tt.wantRemote)
			}
			if branch != tt.wantBranch {
				t.Errorf("branch: got %q, want %q", branch, tt.wantBranch)
			}
			if isRemote != tt.wantIsRemote {
				t.Errorf("isRemote: got %v, want %v", isRemote, tt.wantIsRemote)
			}
		})
	}
}

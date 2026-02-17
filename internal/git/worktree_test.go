package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateWorktree(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	// Create a branch first
	if err := runGit(ctx, repoPath, "branch", "existing-branch"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	wtPath := filepath.Join(tmpDir, "wt-existing")
	gitDir := filepath.Join(repoPath, ".git")

	if err := CreateWorktree(ctx, gitDir, wtPath, "existing-branch"); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir should exist: %v", err)
	}

	// Verify branch
	branch, err := GetCurrentBranch(ctx, wtPath)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "existing-branch" {
		t.Errorf("branch = %q, want existing-branch", branch)
	}
}

func TestCreateWorktreeNewBranch(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	wtPath := filepath.Join(tmpDir, "wt-new-branch")
	gitDir := filepath.Join(repoPath, ".git")

	if err := CreateWorktreeNewBranch(ctx, gitDir, wtPath, "new-feature", "main"); err != nil {
		t.Fatalf("CreateWorktreeNewBranch failed: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir should exist: %v", err)
	}

	// Verify branch
	branch, err := GetCurrentBranch(ctx, wtPath)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if branch != "new-feature" {
		t.Errorf("branch = %q, want new-feature", branch)
	}
}

func TestCreateWorktreeOrphan(t *testing.T) {
	t.Parallel()

	// Create an empty repo (no commits)
	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}
	repoPath := filepath.Join(resolved, "empty-repo")
	ctx := context.Background()

	if err := runGit(ctx, "", "init", "-b", "main", repoPath); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	wtPath := filepath.Join(resolved, "wt-orphan")
	gitDir := filepath.Join(repoPath, ".git")

	if err := CreateWorktreeOrphan(ctx, gitDir, wtPath, "orphan-branch"); err != nil {
		t.Fatalf("CreateWorktreeOrphan failed: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir should exist: %v", err)
	}
}

func TestRemoveWorktree(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	wtPath := filepath.Join(tmpDir, "wt-to-remove")
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "remove-me", wtPath); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	wt := Worktree{
		Path:     wtPath,
		RepoPath: repoPath,
	}

	if err := RemoveWorktree(ctx, wt, false); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree dir should be removed")
	}
}

func TestPruneWorktrees(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	tmpDir := filepath.Dir(repoPath)
	ctx := context.Background()

	// Create a worktree then manually rm -rf the directory
	wtPath := filepath.Join(tmpDir, "wt-to-prune")
	if err := runGit(ctx, repoPath, "worktree", "add", "-b", "prune-me", wtPath); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Manually remove the directory (simulating it being deleted outside of git)
	if err := os.RemoveAll(wtPath); err != nil {
		t.Fatalf("failed to remove worktree dir: %v", err)
	}

	// Prune should clean up the stale reference
	if err := PruneWorktrees(ctx, repoPath); err != nil {
		t.Fatalf("PruneWorktrees failed: %v", err)
	}

	// After prune, listing should not include the stale worktree
	wts, err := ListWorktreesFromRepo(ctx, repoPath)
	if err != nil {
		t.Fatalf("ListWorktreesFromRepo failed: %v", err)
	}

	for _, wt := range wts {
		if wt.Branch == "prune-me" {
			t.Error("pruned worktree should not appear in list")
		}
	}
}

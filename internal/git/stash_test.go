package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStashAndPop(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Create a dirty working tree (untracked file)
	dirtyFile := filepath.Join(repoPath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted changes\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Stash changes
	n, err := Stash(ctx, repoPath)
	if err != nil {
		t.Fatalf("Stash failed: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 stashed file, got %d", n)
	}

	// Working tree should be clean (dirty.txt gone since it was untracked and stashed with -u)
	if _, err := os.Stat(dirtyFile); !os.IsNotExist(err) {
		t.Error("dirty.txt should not exist after stash")
	}

	// Pop stash
	if err := StashPop(ctx, repoPath); err != nil {
		t.Fatalf("StashPop failed: %v", err)
	}

	// File should be restored
	content, err := os.ReadFile(dirtyFile)
	if err != nil {
		t.Fatalf("dirty.txt should exist after pop: %v", err)
	}
	if string(content) != "uncommitted changes\n" {
		t.Errorf("content = %q, want 'uncommitted changes\\n'", content)
	}
}

func TestStash_NoChanges(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Stash on clean worktree — git stash push succeeds (exit 0) but creates no entry
	n, err := Stash(ctx, repoPath)
	if err != nil {
		t.Fatalf("Stash on clean worktree should not error, got: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 stashed files on clean worktree, got %d", n)
	}
}

func TestStash_StagedAndUntracked(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Create a staged file
	stagedFile := filepath.Join(repoPath, "staged.txt")
	if err := os.WriteFile(stagedFile, []byte("staged content\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGit(ctx, repoPath, "add", "staged.txt"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	// Create an untracked file
	untrackedFile := filepath.Join(repoPath, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked content\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Stash should capture both
	n, err := Stash(ctx, repoPath)
	if err != nil {
		t.Fatalf("Stash failed: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 stashed files, got %d", n)
	}

	// Both files should be gone
	if _, err := os.Stat(stagedFile); !os.IsNotExist(err) {
		t.Error("staged.txt should not exist after stash")
	}
	if _, err := os.Stat(untrackedFile); !os.IsNotExist(err) {
		t.Error("untracked.txt should not exist after stash")
	}
}

func TestStashPop_NoStash(t *testing.T) {
	t.Parallel()

	repoPath := setupTestRepo(t)
	ctx := context.Background()

	// Pop with no stash entries should fail
	if err := StashPop(ctx, repoPath); err == nil {
		t.Error("expected error when popping with no stash entries")
	}
}

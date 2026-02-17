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

	// Create a dirty working tree
	dirtyFile := filepath.Join(repoPath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted changes\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Stash changes
	if err := Stash(ctx, repoPath); err != nil {
		t.Fatalf("Stash failed: %v", err)
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

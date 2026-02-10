package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupNotesTestRepo creates a git repo with a branch for testing notes.
func setupNotesTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for %s: %v", tmpDir, err)
	}
	tmpDir = resolved
	repoPath := filepath.Join(tmpDir, "test-repo")

	ctx := context.Background()
	if err := runGit(ctx, "", "init", "-b", "main", repoPath); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	cmds := [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test User"},
		{"config", "commit.gpgsign", "false"},
	}

	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run git %v: %v\n%s", args, err, out)
		}
	}

	// Create initial commit
	readme := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGit(ctx, repoPath, "add", "README.md"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if err := runGit(ctx, repoPath, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create a feature branch
	if err := runGit(ctx, repoPath, "branch", "feature"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	return repoPath
}

func TestSetBranchNote(t *testing.T) {
	t.Parallel()
	repoPath := setupNotesTestRepo(t)
	ctx := context.Background()

	if err := SetBranchNote(ctx, repoPath, "feature", "WIP: fixing tests"); err != nil {
		t.Fatalf("SetBranchNote failed: %v", err)
	}

	// Verify via direct git config read
	cmd := exec.Command("git", "config", "branch.feature.description")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to read git config: %v", err)
	}
	got := string(out)
	if got != "WIP: fixing tests\n" {
		t.Errorf("expected 'WIP: fixing tests', got %q", got)
	}
}

func TestGetBranchNote(t *testing.T) {
	t.Parallel()
	repoPath := setupNotesTestRepo(t)
	ctx := context.Background()

	// Set note first
	if err := SetBranchNote(ctx, repoPath, "feature", "review needed"); err != nil {
		t.Fatalf("SetBranchNote failed: %v", err)
	}

	note, err := GetBranchNote(ctx, repoPath, "feature")
	if err != nil {
		t.Fatalf("GetBranchNote failed: %v", err)
	}
	if note != "review needed" {
		t.Errorf("expected 'review needed', got %q", note)
	}
}

func TestGetBranchNote_NoNote(t *testing.T) {
	t.Parallel()
	repoPath := setupNotesTestRepo(t)
	ctx := context.Background()

	note, err := GetBranchNote(ctx, repoPath, "feature")
	if err != nil {
		t.Fatalf("GetBranchNote failed: %v", err)
	}
	if note != "" {
		t.Errorf("expected empty note, got %q", note)
	}
}

func TestClearBranchNote(t *testing.T) {
	t.Parallel()
	repoPath := setupNotesTestRepo(t)
	ctx := context.Background()

	// Set and then clear
	if err := SetBranchNote(ctx, repoPath, "feature", "WIP"); err != nil {
		t.Fatalf("SetBranchNote failed: %v", err)
	}
	if err := ClearBranchNote(ctx, repoPath, "feature"); err != nil {
		t.Fatalf("ClearBranchNote failed: %v", err)
	}

	note, err := GetBranchNote(ctx, repoPath, "feature")
	if err != nil {
		t.Fatalf("GetBranchNote failed: %v", err)
	}
	if note != "" {
		t.Errorf("expected empty note after clear, got %q", note)
	}
}

func TestClearBranchNote_NoNote(t *testing.T) {
	t.Parallel()
	repoPath := setupNotesTestRepo(t)
	ctx := context.Background()

	// Clearing a non-existent note should not error
	if err := ClearBranchNote(ctx, repoPath, "feature"); err != nil {
		t.Fatalf("ClearBranchNote on unset note should not fail: %v", err)
	}
}

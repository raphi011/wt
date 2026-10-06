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

func TestHasUncommittedChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(t *testing.T, repoPath string)
		want  bool
	}{
		{
			name:  "clean",
			setup: func(t *testing.T, repoPath string) {},
			want:  false,
		},
		{
			name: "modified tracked file",
			setup: func(t *testing.T, repoPath string) {
				if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# changed\n"), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
			},
			want: true,
		},
		{
			name: "staged file",
			setup: func(t *testing.T, repoPath string) {
				if err := os.WriteFile(filepath.Join(repoPath, "staged.txt"), []byte("staged\n"), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
				if err := runGit(context.Background(), repoPath, "add", "staged.txt"); err != nil {
					t.Fatalf("failed to stage file: %v", err)
				}
			},
			want: true,
		},
		{
			name: "untracked file",
			setup: func(t *testing.T, repoPath string) {
				if err := os.WriteFile(filepath.Join(repoPath, "untracked.txt"), []byte("untracked\n"), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
			},
			want: true,
		},
		{
			name: "ignored file only",
			setup: func(t *testing.T, repoPath string) {
				ctx := context.Background()
				if err := os.WriteFile(filepath.Join(repoPath, ".gitignore"), []byte("build/\n"), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
				if err := runGit(ctx, repoPath, "add", ".gitignore"); err != nil {
					t.Fatalf("failed to stage file: %v", err)
				}
				if err := runGit(ctx, repoPath, "commit", "-m", "Add gitignore"); err != nil {
					t.Fatalf("failed to commit: %v", err)
				}
				if err := os.MkdirAll(filepath.Join(repoPath, "build"), 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(repoPath, "build", "out.bin"), []byte("artifact\n"), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repoPath := setupTestRepo(t)
			tt.setup(t, repoPath)

			got, err := HasUncommittedChanges(context.Background(), repoPath)
			if err != nil {
				t.Fatalf("HasUncommittedChanges failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("HasUncommittedChanges = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasUncommittedChanges_NotARepo(t *testing.T) {
	t.Parallel()

	if _, err := HasUncommittedChanges(context.Background(), resolveTempDir(t)); err == nil {
		t.Error("HasUncommittedChanges on a non-repo directory returned no error")
	}
}

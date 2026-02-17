package git

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLoadWorktreesForRepos(t *testing.T) {
	t.Parallel()

	// Set up two repos with worktrees
	repo1 := setupTestRepo(t)
	repo2 := setupTestRepo(t)

	ctx := context.Background()

	// Add worktrees to repo1
	wt1 := filepath.Join(filepath.Dir(repo1), "wt-load-1")
	if err := runGit(ctx, repo1, "worktree", "add", "-b", "load-branch-1", wt1); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Add worktrees to repo2
	wt2 := filepath.Join(filepath.Dir(repo2), "wt-load-2")
	if err := runGit(ctx, repo2, "worktree", "add", "-b", "load-branch-2", wt2); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	repos := []RepoRef{
		{Name: "repo1", Path: repo1},
		{Name: "repo2", Path: repo2},
	}

	worktrees, warnings := LoadWorktreesForRepos(ctx, repos)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	// Each repo has main + 1 extra worktree = 4 total
	if len(worktrees) != 4 {
		t.Errorf("got %d worktrees, want 4", len(worktrees))
	}

	// Verify metadata
	for _, wt := range worktrees {
		if wt.RepoName == "" {
			t.Error("RepoName should be set")
		}
		if wt.RepoPath == "" {
			t.Error("RepoPath should be set")
		}
		if wt.Path == "" {
			t.Error("Path should be set")
		}
		if wt.Branch == "" {
			t.Error("Branch should be set")
		}
	}
}

func TestLoadWorktreesForRepos_BadRepo(t *testing.T) {
	t.Parallel()

	goodRepo := setupTestRepo(t)
	ctx := context.Background()

	repos := []RepoRef{
		{Name: "bad-repo", Path: "/nonexistent/path"},
		{Name: "good-repo", Path: goodRepo},
	}

	worktrees, warnings := LoadWorktreesForRepos(ctx, repos)

	// Bad repo should produce a warning
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
	if len(warnings) > 0 && warnings[0].RepoName != "bad-repo" {
		t.Errorf("warning repo = %q, want bad-repo", warnings[0].RepoName)
	}

	// Good repo should still load (main worktree)
	if len(worktrees) < 1 {
		t.Error("good repo worktrees should still load")
	}

	hasGoodRepo := false
	for _, wt := range worktrees {
		if wt.RepoName == "good-repo" {
			hasGoodRepo = true
			break
		}
	}
	if !hasGoodRepo {
		t.Error("should have worktrees from good-repo")
	}
}

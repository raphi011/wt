//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/prstatus"
)

// TestRemoveWorktree tests the teardown shared by prune and pr merge.
//
// Scenario: A "feature" worktree with a commit that is not on main is torn
// down, clean or with an untracked file, with and without force and branch deletion
// Expected: Without force a dirty worktree is kept together with its PR cache
// entry and branch; the branch is only deleted when asked, and an unmerged
// branch only when the forge confirmed the merge
func TestRemoveWorktree(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dirty       bool
		prState     string
		opts        teardownOpts
		wantErr     bool
		wantRemoved bool
		wantBranch  bool
	}{
		{name: "clean", wantRemoved: true, wantBranch: true},
		{name: "dirty without force", dirty: true, opts: teardownOpts{DeleteBranch: true}, prState: forge.PRStateMerged, wantErr: true, wantBranch: true},
		{name: "dirty with force", dirty: true, opts: teardownOpts{Force: true}, wantRemoved: true, wantBranch: true},
		{name: "delete branch, forge confirmed merge", prState: forge.PRStateMerged, opts: teardownOpts{DeleteBranch: true}, wantRemoved: true},
		{name: "delete branch, not merged", opts: teardownOpts{DeleteBranch: true}, wantRemoved: true, wantBranch: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, repoPath, wtPath := setupMergedWorktree(t)
			if tt.dirty {
				writeUntrackedFile(t, wtPath)
			}
			cacheKey := prcache.CacheKey(repoPath, "feature")
			cache := prcache.LoadFrom(filepath.Join(filepath.Dir(cfg.RegistryPath), "prs.json"))
			if cache.Get(cacheKey) == nil {
				t.Fatal("expected a PR cache entry before teardown")
			}

			ctx := testContextWithConfig(t, cfg, repoPath)
			wt := git.Worktree{Path: wtPath, RepoPath: repoPath, Branch: "feature"}
			status, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{})
			if err != nil {
				t.Fatalf("load PR status: %v", err)
			}
			tt.opts.PRStatus = status
			tt.opts.PR = forge.PRInfo{State: tt.prState}
			err = removeWorktree(ctx, wt, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("removeWorktree error = %v, wantErr %v", err, tt.wantErr)
			}

			if _, err := os.Stat(wtPath); os.IsNotExist(err) != tt.wantRemoved {
				t.Errorf("worktree removed = %v, want %v", os.IsNotExist(err), tt.wantRemoved)
			}
			if cached := prcache.LoadFrom(filepath.Join(filepath.Dir(cfg.RegistryPath), "prs.json")).Get(cacheKey) != nil; cached == tt.wantRemoved {
				t.Errorf("PR cache entry present = %v, want %v", cached, !tt.wantRemoved)
			}
			_, err = runGitCommand(repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/feature")
			if exists := err == nil; exists != tt.wantBranch {
				t.Errorf("branch exists = %v, want %v", exists, tt.wantBranch)
			}
		})
	}
}

// TestForgetWorktrees tests the cleanup after worktrees were removed.
//
// Scenario: History has entries for a removed worktree and for the main repo
// Expected: Only the entry of the removed worktree is dropped
func TestForgetWorktrees(t *testing.T) {
	t.Parallel()

	cfg, repoPath, wtPath := setupMergedWorktree(t)
	cfg.HistoryPath = filepath.Join(t.TempDir(), "history.json")
	if err := history.RecordAccess(wtPath, "test-repo", "feature", cfg.HistoryPath); err != nil {
		t.Fatalf("failed to record history: %v", err)
	}
	if err := history.RecordAccess(repoPath, "test-repo", "main", cfg.HistoryPath); err != nil {
		t.Fatalf("failed to record history: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	forgetWorktrees(ctx, []git.Worktree{{Path: wtPath, RepoPath: repoPath, Branch: "feature"}})

	hist, err := history.Load(cfg.HistoryPath)
	if err != nil {
		t.Fatalf("failed to load history: %v", err)
	}
	if hist.FindByPath(wtPath) != nil {
		t.Error("removed worktree should be removed from history")
	}
	if hist.FindByPath(repoPath) == nil {
		t.Error("history entry of the main repo should be kept")
	}
}

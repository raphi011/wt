//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

// setupEnsureWorktreeRepo registers a repo with a "feature" branch and
// configures one hook per checkout subtype, each touching a marker file.
func setupEnsureWorktreeRepo(t *testing.T) (cfg *config.Config, repo registry.Repo, tmpDir string) {
	t.Helper()

	tmpDir = resolvePath(t, t.TempDir())
	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
	repo = registry.Repo{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}
	if err := saveRegistry(&registry.Registry{Repos: []registry.Repo{repo}}, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg = &config.Config{
		RegistryPath: regFile,
		HistoryPath:  filepath.Join(tmpDir, "history.json"),
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"on-create": {Command: "touch " + filepath.Join(tmpDir, "create-hook-ran"), On: []string{"checkout:create"}},
				"on-open":   {Command: "touch " + filepath.Join(tmpDir, "open-hook-ran"), On: []string{"checkout:open"}},
				"on-pr":     {Command: "touch " + filepath.Join(tmpDir, "pr-hook-ran"), On: []string{"checkout:pr"}},
			},
		},
	}
	return cfg, repo, tmpDir
}

// TestEnsureWorktree tests the worktree sequence shared by the checkout commands.
//
// Scenario: A worktree is ensured for a branch without a worktree, for a
// branch that already has one, and for a new branch; with a note and as a
// PR branch
// Expected: The worktree exists afterwards and is reported as created or
// opened; history is recorded, the note is set and the hook of the matching
// subtype runs
func TestEnsureWorktree(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		branch       string
		existingWt   bool
		opts         checkoutOpts
		wantCreated  bool
		wantHook     string
		wantNoHook   string
		wantBranchAt string // worktree dir relative to tmpDir, empty: the existing worktree
	}{
		{name: "branch without worktree", branch: "feature", wantCreated: true, wantHook: "open-hook-ran", wantNoHook: "create-hook-ran", wantBranchAt: "test-repo-feature"},
		{name: "branch with worktree", branch: "feature", existingWt: true, wantHook: "open-hook-ran", wantNoHook: "create-hook-ran"},
		{name: "new branch", branch: "brand-new", opts: checkoutOpts{NewBranch: true}, wantCreated: true, wantHook: "create-hook-ran", wantNoHook: "open-hook-ran", wantBranchAt: "test-repo-brand-new"},
		{name: "note, branch without worktree", branch: "feature", opts: checkoutOpts{Note: "wip"}, wantCreated: true, wantHook: "open-hook-ran", wantNoHook: "create-hook-ran", wantBranchAt: "test-repo-feature"},
		{name: "note, branch with worktree", branch: "feature", existingWt: true, opts: checkoutOpts{Note: "wip"}, wantHook: "open-hook-ran", wantNoHook: "create-hook-ran"},
		{name: "PR branch without worktree", branch: "feature", opts: checkoutOpts{PR: &prIntent{Number: 7}}, wantCreated: true, wantHook: "pr-hook-ran", wantNoHook: "open-hook-ran", wantBranchAt: "test-repo-feature"},
		{name: "PR branch with worktree", branch: "feature", existingWt: true, opts: checkoutOpts{PR: &prIntent{Number: 7}}, wantHook: "pr-hook-ran", wantNoHook: "open-hook-ran"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, repo, tmpDir := setupEnsureWorktreeRepo(t)
			wantPath := filepath.Join(tmpDir, tt.wantBranchAt)
			if tt.existingWt {
				wantPath = createTestWorktree(t, repo.Path, tt.branch)
			}

			ctx := testContextWithConfig(t, cfg, repo.Path)
			got, err := ensureWorktree(ctx, repo, tt.branch, tt.opts)
			if err != nil {
				t.Fatalf("ensureWorktree failed: %v", err)
			}

			if got.Path != wantPath {
				t.Errorf("Path = %q, want %q", got.Path, wantPath)
			}
			if got.Created != tt.wantCreated {
				t.Errorf("Created = %v, want %v", got.Created, tt.wantCreated)
			}
			if _, err := os.Stat(wantPath); err != nil {
				t.Errorf("worktree should exist: %v", err)
			}
			hist, err := history.Load(cfg.HistoryPath)
			if err != nil {
				t.Fatalf("failed to load history: %v", err)
			}
			if hist.FindByPath(wantPath) == nil {
				t.Error("worktree should be recorded in history")
			}
			note, err := git.GetBranchNote(ctx, repo.Path, tt.branch)
			if err != nil {
				t.Fatalf("failed to read note: %v", err)
			}
			if note != tt.opts.Note {
				t.Errorf("note = %q, want %q", note, tt.opts.Note)
			}
			if _, err := os.Stat(filepath.Join(tmpDir, tt.wantHook)); err != nil {
				t.Errorf("hook marker %s should exist: %v", tt.wantHook, err)
			}
			if _, err := os.Stat(filepath.Join(tmpDir, tt.wantNoHook)); err == nil {
				t.Errorf("hook marker %s should not exist", tt.wantNoHook)
			}
		})
	}
}

// TestResolveCheckoutRepos_ExistingWorktreeHasNoSideEffects tests that
// resolving the repos of a checkout does not open worktrees.
//
// Scenario: The target branch already has a worktree, given unscoped inside
// the repo, unscoped outside a repo and repo-scoped
// Expected: The repo is returned; no hook runs and no history is recorded
func TestResolveCheckoutRepos_ExistingWorktreeHasNoSideEffects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		scoped  bool
		outside bool
	}{
		{name: "unscoped in repo"},
		{name: "unscoped outside repo", outside: true},
		{name: "repo scope", scoped: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, repo, tmpDir := setupEnsureWorktreeRepo(t)
			wtPath := createTestWorktree(t, repo.Path, "feature")

			workDir := repo.Path
			if tt.outside {
				workDir = tmpDir
			}
			parsed := ScopedTargetResult{Branch: "feature"}
			if tt.scoped {
				parsed.Repos = []registry.Repo{repo}
			}

			ctx := testContextWithConfig(t, cfg, workDir)
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				t.Fatalf("failed to load registry: %v", err)
			}
			repos, err := resolveCheckoutRepos(ctx, log.FromContext(ctx), reg, parsed, false, false, false)
			if err != nil {
				t.Fatalf("resolveCheckoutRepos failed: %v", err)
			}

			if len(repos) != 1 || repos[0].Path != repo.Path {
				t.Errorf("repos = %v, want only %s", repos, repo.Path)
			}
			if _, err := os.Stat(filepath.Join(tmpDir, "open-hook-ran")); err == nil {
				t.Error("resolving should not run hooks")
			}
			hist, err := history.Load(cfg.HistoryPath)
			if err != nil {
				t.Fatalf("failed to load history: %v", err)
			}
			if hist.FindByPath(wtPath) != nil {
				t.Error("resolving should not record history")
			}
		})
	}
}

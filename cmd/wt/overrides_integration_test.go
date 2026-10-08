//go:build integration

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
)

// TestCheckout_AutoFetchOverrides tests config precedence using a newer remote commit.
//
// Scenario: User runs `wt checkout -b feature` with global/local auto_fetch and optional --fetch overrides
// Expected: The new branch includes the remote commit only when effective auto_fetch is true; invalid local config warns and falls back
func TestCheckout_AutoFetchOverrides(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		global      bool
		local       string
		flag        string
		wantFetch   bool
		wantWarning bool
	}{
		{name: "global true", global: true, wantFetch: true},
		{name: "local false", global: true, local: "[checkout]\nauto_fetch = false\n"},
		{name: "local true", local: "[checkout]\nauto_fetch = true\n", wantFetch: true},
		{name: "explicit false", global: true, local: "[checkout]\nauto_fetch = true\n", flag: "--fetch=false"},
		{name: "explicit true", local: "[checkout]\nauto_fetch = false\n", flag: "--fetch", wantFetch: true},
		{name: "invalid local false override", global: true, local: "invalid [[[", flag: "--fetch=false", wantWarning: true},
		{name: "invalid local true override", local: "invalid [[[", flag: "--fetch", wantFetch: true, wantWarning: true},
		{name: "invalid local global fallback", global: true, local: "invalid [[[", wantFetch: true, wantWarning: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := resolvePath(t, t.TempDir())
			repoPath, originPath := setupTestRepoWithOrigin(t, tmpDir, "test-repo")
			clonePath := filepath.Join(tmpDir, "origin-clone")
			if out, err := runGitCommand(tmpDir, "clone", originPath, clonePath); err != nil {
				t.Fatalf("clone origin: %v\n%s", err, out)
			}
			for _, args := range [][]string{{"config", "user.email", "test@test.com"}, {"config", "user.name", "Test User"}, {"config", "commit.gpgsign", "false"}} {
				if out, err := runGitCommand(clonePath, args...); err != nil {
					t.Fatalf("configure clone: %v\n%s", err, out)
				}
			}
			addCommit(t, clonePath, "origin-only.txt", "New remote commit")
			if out, err := runGitCommand(clonePath, "push", "origin", "main"); err != nil {
				t.Fatalf("push remote commit: %v\n%s", err, out)
			}
			regFile := overrideTestRegistry(t, tmpDir, repoPath)
			if tc.local != "" {
				if err := os.WriteFile(filepath.Join(repoPath, config.LocalConfigFileName), []byte(tc.local), 0644); err != nil {
					t.Fatalf("write local config: %v", err)
				}
			}
			cfg := &config.Config{RegistryPath: regFile, Checkout: config.CheckoutConfig{AutoFetch: tc.global, WorktreeFormat: "../{repo}-{branch}"}}
			var diagnostics strings.Builder
			ctx := log.WithLogger(testContextWithConfig(t, cfg, repoPath), log.New(&diagnostics, false, false))
			cmd := newCheckoutCmd()
			cmd.SetContext(ctx)
			args := []string{"-b", "feature"}
			if tc.flag != "" {
				args = append(args, tc.flag)
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("checkout command: %v", err)
			}
			wtPath := filepath.Join(tmpDir, "test-repo-feature")
			if _, err := os.Stat(wtPath); err != nil {
				t.Fatalf("created worktree: %v", err)
			}
			_, err := os.Stat(filepath.Join(wtPath, "origin-only.txt"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatalf("inspect fetched commit: %v", err)
			}
			if got := err == nil; got != tc.wantFetch {
				t.Errorf("new branch includes remote commit = %t, want %t", got, tc.wantFetch)
			}
			if tc.wantWarning && (!strings.Contains(diagnostics.String(), "Warning: failed to load local config") || !strings.Contains(diagnostics.String(), filepath.Join(repoPath, config.LocalConfigFileName))) {
				t.Errorf("missing malformed local config warning: %s", diagnostics.String())
			}
		})
	}
}

// TestPrune_DeleteBranchOverrides tests precedence in both targeted and automatic pruning.
//
// Scenario: User prunes a merged worktree with global/local delete_local_branches and optional boolean flag overrides
// Expected: Worktree removal always succeeds; branch retention follows effective config, and malformed local config warns and falls back
func TestPrune_DeleteBranchOverrides(t *testing.T) {
	t.Parallel()

	for _, targeted := range []bool{false, true} {
		t.Run(fmt.Sprintf("targeted=%t", targeted), func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name        string
				global      bool
				local       string
				flag        string
				wantDelete  bool
				wantWarning bool
			}{
				{name: "local false", global: true, local: "[prune]\ndelete_local_branches = false\n"},
				{name: "local true", local: "[prune]\ndelete_local_branches = true\n", wantDelete: true},
				{name: "explicit false", global: true, local: "[prune]\ndelete_local_branches = true\n", flag: "--delete-branches=false"},
				{name: "negative flag", global: true, local: "[prune]\ndelete_local_branches = true\n", flag: "--no-delete-branches"},
				{name: "negative flag false still suppresses", global: true, local: "[prune]\ndelete_local_branches = true\n", flag: "--no-delete-branches=false"},
				{name: "explicit true", local: "[prune]\ndelete_local_branches = false\n", flag: "--delete-branches", wantDelete: true},
				{name: "invalid local false override", global: true, local: "invalid [[[", flag: "--delete-branches=false", wantWarning: true},
				{name: "invalid local true override", local: "invalid [[[", flag: "--delete-branches", wantDelete: true, wantWarning: true},
				{name: "invalid local global fallback", global: true, local: "invalid [[[", wantDelete: true, wantWarning: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					tmpDir := resolvePath(t, t.TempDir())
					repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})
					wtPath := createTestWorktree(t, repoPath, "feature")
					regFile := overrideTestRegistry(t, tmpDir, repoPath)
					if err := os.WriteFile(filepath.Join(repoPath, config.LocalConfigFileName), []byte(tc.local), 0644); err != nil {
						t.Fatalf("write local config: %v", err)
					}
					cache := prcache.LoadFrom(filepath.Join(filepath.Dir(regFile), "prs.json"))
					cache.Set(prcache.CacheKey(repoPath, "feature"), &forge.PRInfo{Number: 1, State: forge.PRStateMerged, Fetched: true})
					if err := cache.Save(); err != nil {
						t.Fatalf("seed merged PR cache: %v", err)
					}
					cfg := &config.Config{RegistryPath: regFile, Prune: config.PruneConfig{DeleteLocalBranches: tc.global}}
					var diagnostics strings.Builder
					ctx := log.WithLogger(testContextWithConfig(t, cfg, repoPath), log.New(&diagnostics, false, false))
					cmd := newPruneCmd()
					cmd.SetContext(ctx)
					var args []string
					if targeted {
						args = append(args, "feature")
					}
					if tc.flag != "" {
						args = append(args, tc.flag)
					}
					cmd.SetArgs(args)
					if err := cmd.Execute(); err != nil {
						t.Fatalf("prune command: %v", err)
					}
					if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
						t.Fatalf("worktree should be removed, stat error = %v", err)
					}
					branches, err := runGitCommand(repoPath, "branch", "--list", "feature")
					if err != nil {
						t.Fatalf("list branches: %v", err)
					}
					if got := strings.TrimSpace(branches) == ""; got != tc.wantDelete {
						t.Errorf("branch deleted = %t, want %t", got, tc.wantDelete)
					}
					if tc.wantWarning && (!strings.Contains(diagnostics.String(), "Warning: failed to load local config") || !strings.Contains(diagnostics.String(), filepath.Join(repoPath, config.LocalConfigFileName))) {
						t.Errorf("missing malformed local config warning: %s", diagnostics.String())
					}
				})
			}
		})
	}
}

// TestPrMerge_StrategyOverrides tests merge strategy precedence.
//
// Scenario: User runs `wt pr merge` with global/local merge.strategy and optional --strategy
// Expected: The forge merges with the flag, else the local, else the global strategy
func TestPrMerge_StrategyOverrides(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		global string
		local  string
		args   []string
		want   string
	}{
		{name: "unset"},
		{name: "global", global: "squash", want: "squash"},
		{name: "local overrides global", global: "squash", local: "[merge]\nstrategy = \"rebase\"\n", want: "rebase"},
		{name: "flag overrides local", global: "squash", local: "[merge]\nstrategy = \"rebase\"\n", args: []string{"--strategy", "merge"}, want: "merge"},
		{name: "flag overrides global", global: "squash", args: []string{"--strategy", "rebase"}, want: "rebase"},
		{name: "invalid local flag override", global: "squash", local: "invalid [[[", args: []string{"--strategy", "rebase"}, want: "rebase"},
		{name: "invalid local global fallback", global: "squash", local: "invalid [[[", want: "squash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, repoPath, wtPath := setupMergedWorktree(t)
			if tc.local != "" {
				if err := os.WriteFile(filepath.Join(repoPath, config.LocalConfigFileName), []byte(tc.local), 0644); err != nil {
					t.Fatalf("write local config: %v", err)
				}
			}
			cfg.Merge.Strategy = tc.global
			ctx, _ := testContextWithConfigAndOutput(t, cfg, wtPath)
			ctx, fake := withRepoPR(t, ctx, repoPath, forge.PRStateOpen)
			cmd := newPrMergeCmd()
			cmd.SetContext(ctx)
			cmd.SetArgs(append([]string{"--keep"}, tc.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("pr merge command: %v", err)
			}
			merges := fake.Merges()
			if len(merges) != 1 || merges[0].Strategy != tc.want {
				t.Errorf("merge calls = %+v, want one merge with strategy %q", merges, tc.want)
			}
		})
	}
}

// TestList_SortOverrides tests sort precedence.
//
// Scenario: User runs `wt list` with default_sort in config and optional --sort
// Expected: Worktrees are ordered by the flag, else by default_sort
func TestList_SortOverrides(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		global     string
		args       []string
		wantByRepo bool
	}{
		{name: "config repo", global: "repo", wantByRepo: true},
		{name: "config branch", global: "branch"},
		{name: "flag overrides config branch", global: "branch", args: []string{"--sort", "repo"}, wantByRepo: true},
		{name: "flag overrides config repo", global: "repo", args: []string{"-s", "branch"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := resolvePath(t, t.TempDir())
			// By repo "one" (zzz-last) sorts first; by branch "two" (aaa-first) does
			onePath := setupTestRepoWithBranches(t, tmpDir, "one", []string{"zzz-last"})
			createTestWorktree(t, onePath, "zzz-last")
			twoPath := setupTestRepoWithBranches(t, tmpDir, "two", []string{"aaa-first"})
			createTestWorktree(t, twoPath, "aaa-first")
			regFile := filepath.Join(tmpDir, ".wt", "repos.json")
			if err := saveRegistry(&registry.Registry{Repos: []registry.Repo{{Name: "one", Path: onePath}, {Name: "two", Path: twoPath}}}, regFile); err != nil {
				t.Fatalf("save registry: %v", err)
			}
			otherDir := filepath.Join(tmpDir, "other")
			if err := os.MkdirAll(otherDir, 0755); err != nil {
				t.Fatalf("create directory: %v", err)
			}
			ctx, out := testContextWithConfigAndOutput(t, &config.Config{RegistryPath: regFile, DefaultSort: tc.global}, otherDir)
			cmd := newListCmd()
			cmd.SetContext(ctx)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("list command: %v", err)
			}
			lastIdx := strings.Index(out.String(), "zzz-last")
			firstIdx := strings.Index(out.String(), "aaa-first")
			if lastIdx == -1 || firstIdx == -1 {
				t.Fatalf("expected both branches in output, got %q", out.String())
			}
			if got := lastIdx < firstIdx; got != tc.wantByRepo {
				t.Errorf("sorted by repo = %t, want %t\n%s", got, tc.wantByRepo, out.String())
			}
		})
	}
}

// TestRepoClone_CloneModeOverrides tests clone mode precedence.
//
// Scenario: User runs `wt repo clone <url>` with clone.mode in config and optional --clone-mode
// Expected: The clone is bare when the flag, else clone.mode, says so; an invalid flag value is rejected
func TestRepoClone_CloneModeOverrides(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		global   string
		args     []string
		wantBare bool
		wantErr  bool
	}{
		{name: "unset"},
		{name: "config bare", global: "bare", wantBare: true},
		{name: "config regular", global: "regular"},
		{name: "flag overrides config bare", global: "bare", args: []string{"--clone-mode", "regular"}},
		{name: "flag overrides config regular", global: "regular", args: []string{"--clone-mode", "bare"}, wantBare: true},
		{name: "invalid flag", global: "bare", args: []string{"--clone-mode", "shallow"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := resolvePath(t, t.TempDir())
			sourceRepo := setupTestRepo(t, tmpDir, "source-repo")
			cfg := testConfig()
			cfg.RegistryPath = filepath.Join(tmpDir, ".wt", "repos.json")
			cfg.Clone.Mode = tc.global
			cmd := newRepoCloneCmd()
			cmd.SetContext(testContextWithConfig(t, cfg, tmpDir))
			cmd.SetArgs(append([]string{"file://" + sourceRepo, "cloned-repo"}, tc.args...))
			err := cmd.Execute()
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "clone-mode") {
					t.Fatalf("clone error = %v, want invalid clone-mode", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("clone command: %v", err)
			}
			bare, err := runGitCommand(filepath.Join(tmpDir, "cloned-repo", ".git"), "rev-parse", "--is-bare-repository")
			if err != nil {
				t.Fatalf("inspect clone: %v\n%s", err, bare)
			}
			if got := strings.TrimSpace(bare) == "true"; got != tc.wantBare {
				t.Errorf("bare clone = %t, want %t", got, tc.wantBare)
			}
		})
	}
}

// TestCheckout_WorktreeFormatOverrides tests worktree format precedence.
//
// Scenario: User runs `wt checkout -b feature` with worktree_format in the registry, .wt.toml and global config
// Expected: The worktree is created at the registered format, else the local, else the global one
func TestCheckout_WorktreeFormatOverrides(t *testing.T) {
	t.Parallel()

	const localConfig = "[checkout]\nworktree_format = \"../local-{branch}\"\n"
	for _, tc := range []struct {
		name       string
		registered string
		local      string
		want       string
	}{
		{name: "global", want: "global-feature"},
		{name: "local overrides global", local: localConfig, want: "local-feature"},
		{name: "registered overrides local", registered: "../registered-{branch}", local: localConfig, want: "registered-feature"},
		{name: "registered overrides global", registered: "../registered-{branch}", want: "registered-feature"},
		{name: "invalid local registered override", registered: "../registered-{branch}", local: "invalid [[[", want: "registered-feature"},
		{name: "invalid local global fallback", local: "invalid [[[", want: "global-feature"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := resolvePath(t, t.TempDir())
			repoPath := setupTestRepo(t, tmpDir, "test-repo")
			regFile := filepath.Join(tmpDir, ".wt", "repos.json")
			if err := saveRegistry(&registry.Registry{Repos: []registry.Repo{{Name: "test-repo", Path: repoPath, WorktreeFormat: tc.registered}}}, regFile); err != nil {
				t.Fatalf("save registry: %v", err)
			}
			if tc.local != "" {
				if err := os.WriteFile(filepath.Join(repoPath, config.LocalConfigFileName), []byte(tc.local), 0644); err != nil {
					t.Fatalf("write local config: %v", err)
				}
			}
			cfg := &config.Config{RegistryPath: regFile, Checkout: config.CheckoutConfig{WorktreeFormat: "../global-{branch}"}}
			cmd := newCheckoutCmd()
			cmd.SetContext(testContextWithConfig(t, cfg, repoPath))
			cmd.SetArgs([]string{"-b", "feature"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("checkout command: %v", err)
			}
			if _, err := os.Stat(filepath.Join(tmpDir, tc.want)); err != nil {
				t.Errorf("worktree at %s: %v", tc.want, err)
			}
		})
	}
}

func overrideTestRegistry(t *testing.T, tmpDir, repoPath string) string {
	t.Helper()
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := saveRegistry(&registry.Registry{Repos: []registry.Repo{{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"}}}, regFile); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	return regFile
}

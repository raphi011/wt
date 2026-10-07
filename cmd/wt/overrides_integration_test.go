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

func overrideTestRegistry(t *testing.T, tmpDir, repoPath string) string {
	t.Helper()
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := saveRegistry(&registry.Registry{Repos: []registry.Repo{{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"}}}, regFile); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	return regFile
}

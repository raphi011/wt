//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/prcache"
)

// TestPRCache_CorruptListAndReset tests displaying, refreshing, and explicitly resetting corrupt PR data.
//
// Scenario: User lists a repo with a corrupt PR cache, or explicitly resets it with `wt list --reset-cache`.
// Expected: Normal listing preserves the bytes and warns, live status remains available, and only a successful explicit reset replaces the file.
func TestPRCache_CorruptListAndReset(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		refresh  bool
		reset    bool
		failSave bool
	}{
		{name: "read only"},
		{name: "live refresh", refresh: true},
		{name: "explicit reset", reset: true},
		{name: "failed reset", reset: true, failSave: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, repoPath, _ := setupMergedWorktree(t)
			for _, args := range [][]string{
				{"config", "branch.feature.remote", "origin"},
				{"config", "branch.feature.merge", "refs/heads/feature"},
			} {
				if output, err := runGitCommand(repoPath, args...); err != nil {
					t.Fatalf("configure upstream: %v\n%s", err, output)
				}
			}
			path, err := cfg.GetPRCachePath()
			if err != nil {
				t.Fatalf("get cache path: %v", err)
			}
			corrupt := []byte("{original corrupt PR cache")
			if err := os.WriteFile(path, corrupt, 0600); err != nil {
				t.Fatalf("write corrupt cache: %v", err)
			}
			if tc.failSave {
				if err := os.Remove(path + ".lock"); err != nil {
					t.Fatalf("remove cache lock file: %v", err)
				}
				if err := os.Mkdir(path+".lock", 0755); err != nil {
					t.Fatalf("block cache lock: %v", err)
				}
			}
			ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)
			var logs strings.Builder
			ctx = log.WithLogger(ctx, log.New(&logs, false, false))
			ctx, _ = withRepoPR(t, ctx, repoPath, forge.PRStateOpen)
			cmd := newListCmd()
			args := []string{"--json"}
			if tc.refresh {
				args = append(args, "-R")
			}
			if tc.reset {
				args = append(args, "--reset-cache")
			}
			cmd.SetContext(ctx)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("list with corrupt cache: %v", err)
			}
			if tc.reset && !tc.failSave {
				cache := prcache.LoadFrom(path)
				if cache.LoadError() != nil || len(cache.PRs) != 0 {
					t.Errorf("explicit reset did not persist a valid empty cache: %+v", cache)
				}
				if !strings.Contains(logs.String(), "Cache reset: PR info cleared") || strings.Contains(logs.String(), "failed to load PR cache") {
					t.Errorf("successful reset diagnostics = %q", logs.String())
				}
			} else {
				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read preserved cache: %v", err)
				}
				if string(contents) != string(corrupt) {
					t.Errorf("original cache bytes changed: %q", contents)
				}
				if !strings.Contains(logs.String(), "failed to load PR cache") || !strings.Contains(logs.String(), "wt list --reset-cache") {
					t.Errorf("missing recovery warning: %q", logs.String())
				}
				if strings.Contains(logs.String(), "Cache reset: PR info cleared") {
					t.Errorf("unsuccessful reset reported success: %q", logs.String())
				}
			}
			var rows []map[string]any
			if err := json.Unmarshal([]byte(out.String()), &rows); err != nil {
				t.Fatalf("decode list output: %v", err)
			}
			found := false
			for _, row := range rows {
				if row["branch"] != "feature" {
					continue
				}
				found = true
				if tc.refresh {
					if row["pr_number"] != float64(1) || row["pr_state"] != forge.PRStateOpen {
						t.Errorf("live feature status missing: %v", row)
					}
				} else if _, ok := row["pr_number"]; ok {
					t.Errorf("corrupt cache produced a PR number: %v", row)
				}
			}
			if !found {
				t.Fatal("list output omitted feature worktree")
			}
		})
	}
}

// TestPrCommands_CorruptPRCache verifies command success without destroying corrupt PR data.
//
// Scenario: User checks out or merges an open PR while its PR cache contains corrupt bytes.
// Expected: The requested worktree action succeeds, the cache stays intact, and a recovery warning is shown.
func TestPrCommands_CorruptPRCache(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"checkout", "merge"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			var cfg *config.Config
			var repoPath, wtPath string
			if command == "checkout" {
				var tmpDir string
				cfg, repoPath, tmpDir = setupPrCheckoutRepo(t)
				wtPath = filepath.Join(tmpDir, "test-repo-feature")
			} else {
				cfg, repoPath, wtPath = setupMergedWorktree(t)
			}
			path, err := cfg.GetPRCachePath()
			if err != nil {
				t.Fatalf("get cache path: %v", err)
			}
			corrupt := []byte("{original command cache")
			if err := os.WriteFile(path, corrupt, 0600); err != nil {
				t.Fatalf("write corrupt cache: %v", err)
			}
			workDir := wtPath
			if command == "checkout" {
				workDir = repoPath
			}
			ctx, logs := testContextWithLog(t, cfg, workDir)
			ctx, fake := withRepoPR(t, ctx, repoPath, forge.PRStateOpen)
			switch command {
			case "checkout":
				cmd := newPrCheckoutCmd()
				cmd.SetContext(ctx)
				cmd.SetArgs([]string{"1", "--forge", "github"})
				if err := cmd.Execute(); err != nil {
					t.Fatalf("checkout with corrupt cache: %v", err)
				}
				branch, err := git.GetCurrentBranch(ctx, wtPath)
				if err != nil || branch != "feature" {
					t.Errorf("checkout branch = %q, error = %v", branch, err)
				}
			case "merge":
				cmd := newPrMergeCmd()
				cmd.SetContext(ctx)
				if err := cmd.Execute(); err != nil {
					t.Fatalf("merge with corrupt cache: %v", err)
				}
				merges := fake.Merges()
				if len(merges) != 1 || merges[0].Number != 1 {
					t.Errorf("merge calls = %+v, want PR 1 merged once", merges)
				}
				if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
					t.Errorf("merge did not remove worktree: %v", err)
				}
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read preserved cache: %v", err)
			}
			if string(contents) != string(corrupt) {
				t.Errorf("command changed corrupt cache bytes: %q", contents)
			}
			if !strings.Contains(logs.String(), "failed to load PR cache") || !strings.Contains(logs.String(), "wt list --reset-cache") {
				t.Errorf("command omitted cache recovery warning: %q", logs.String())
			}
		})
	}
}

//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/forge/forgetest"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
)

type prBranchFixture struct {
	ctx      context.Context
	out      *strings.Builder
	fake     *forgetest.Forge
	repoPath string
	wtPath   string
	origin   string
	cache    string
}

func setupPRBranchFixture(t *testing.T, host string, upstream bool) prBranchFixture {
	t.Helper()
	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"local-feature"})
	wtPath := createTestWorktree(t, repoPath, "local-feature")
	origin := "https://" + host + "/test/test-repo.git"
	if output, err := runGitCommand(repoPath, "remote", "set-url", "origin", origin); err != nil {
		t.Fatalf("set origin URL: %v\n%s", err, output)
	}
	if upstream {
		for _, args := range [][]string{
			{"config", "branch.local-feature.merge", "refs/heads/remote-source"},
			{"config", "branch.local-feature.remote", "origin"},
		} {
			if output, err := runGitCommand(repoPath, args...); err != nil {
				t.Fatalf("configure upstream: %v\n%s", err, output)
			}
		}
	}
	cfg := &config.Config{
		RegistryPath: filepath.Join(tmpDir, "repos.json"),
		HistoryPath:  filepath.Join(tmpDir, "history.json"),
		Merge:        config.MergeConfig{Strategy: "squash"},
	}
	reg := &registry.Registry{Repos: []registry.Repo{{Name: "test-repo", Path: repoPath}}}
	if err := saveRegistry(reg, cfg.RegistryPath); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	fake := forgetest.New()
	localNumber, remoteNumber := 99, 207
	if !upstream {
		localNumber, remoteNumber = 207, 99
	}
	fake.SetPR(origin, "local-feature", forge.PRInfo{Number: localNumber, State: forge.PRStateOpen, URL: "https://" + host + "/test/test-repo/pull/local"})
	fake.SetPR(origin, "remote-source", forge.PRInfo{Number: remoteNumber, State: forge.PRStateOpen, URL: "https://" + host + "/test/test-repo/pull/remote"})
	ctx, out := testContextWithConfigAndOutput(t, cfg, wtPath)
	ctx = forge.WithResolver(ctx, func(repoURL, name string, hosts map[string]string, forgeCfg *config.ForgeConfig) forge.Forge {
		return fake
	})
	return prBranchFixture{ctx: ctx, out: out, fake: fake, repoPath: repoPath, wtPath: wtPath, origin: origin, cache: filepath.Join(tmpDir, "prs.json")}
}

// TestPrMerge_UsesUpstreamBranch selects the PR source branch while preserving local identity.
//
// Scenario: User runs `wt pr merge --keep` on a differently named local branch, or without an upstream.
// Expected: The source PR is merged, the worktree remains, and merged cache status uses the local branch key.
func TestPrMerge_UsesUpstreamBranch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		host     string
		upstream bool
	}{
		{"github/upstream", "github.com", true},
		{"gitlab/upstream", "gitlab.com", true},
		{"github/local-fallback", "github.com", false},
		{"gitlab/local-fallback", "gitlab.com", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := setupPRBranchFixture(t, tc.host, tc.upstream)
			cmd := newPrMergeCmd()
			cmd.SetContext(fixture.ctx)
			cmd.SetArgs([]string{"--keep"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("merge PR: %v", err)
			}
			merges := fixture.fake.Merges()
			if len(merges) != 1 || merges[0].Number != 207 || merges[0].RepoURL != fixture.origin || merges[0].Strategy != "squash" {
				t.Errorf("merge history = %+v, want PR 207 with configured squash strategy", merges)
			}
			if fixture.out.String() != "Merged PR #207\n" {
				t.Errorf("merge output = %q, want merged PR 207", fixture.out.String())
			}
			cache := prcache.LoadFrom(fixture.cache)
			pr := cache.Get(prcache.CacheKey(fixture.repoPath, "local-feature"))
			if pr == nil || pr.Number != 207 || pr.State != forge.PRStateMerged || !pr.Fetched {
				t.Errorf("local cache entry = %+v, want fetched merged PR 207", pr)
			}
			if pr := cache.Get(prcache.CacheKey(fixture.repoPath, "remote-source")); pr != nil {
				t.Errorf("source branch created an extra cache entry: %+v", pr)
			}
			if _, err := os.Stat(fixture.wtPath); err != nil {
				t.Errorf("kept worktree missing: %v", err)
			}
			decoyBranch := "local-feature"
			if !tc.upstream {
				decoyBranch = "remote-source"
			}
			decoy, err := fixture.fake.GetPRForBranch(fixture.ctx, fixture.origin, decoyBranch)
			if err != nil {
				t.Fatalf("read decoy PR: %v", err)
			}
			if decoy.Number != 99 || decoy.State != forge.PRStateOpen {
				t.Errorf("decoy PR = %+v, want open PR 99", decoy)
			}
		})
	}
}

// TestPrView_UsesUpstreamBranch displays the PR for the configured source branch.
//
// Scenario: User runs `wt pr view` on a differently named local branch, or without an upstream.
// Expected: The source PR is displayed, with the local PR used only when no upstream is configured.
func TestPrView_UsesUpstreamBranch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		host     string
		upstream bool
	}{
		{"github/upstream", "github.com", true},
		{"gitlab/upstream", "gitlab.com", true},
		{"github/local-fallback", "github.com", false},
		{"gitlab/local-fallback", "gitlab.com", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := setupPRBranchFixture(t, tc.host, tc.upstream)
			cmd := newPrViewCmd()
			cmd.SetContext(fixture.ctx)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("view PR: %v", err)
			}
			urlSuffix := "remote"
			if !tc.upstream {
				urlSuffix = "local"
			}
			want := "PR #207\nState: " + forge.PRStateOpen + "\nURL: https://" + tc.host + "/test/test-repo/pull/" + urlSuffix + "\n"
			if fixture.out.String() != want {
				t.Errorf("view output = %q, want %q", fixture.out.String(), want)
			}
		})
	}
}

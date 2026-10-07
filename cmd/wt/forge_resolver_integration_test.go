//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/forge/forgetest"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
)

func setupForgeResolverRepo(t *testing.T) (context.Context, *config.Config, *registry.Registry, string) {
	t.Helper()
	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "resolver-repo")
	if err := os.WriteFile(filepath.Join(repoPath, ".wt.toml"), []byte("[forge]\ndefault = \"gitlab\"\n"), 0644); err != nil {
		t.Fatalf("write local config: %v", err)
	}
	reg := &registry.Registry{Repos: []registry.Repo{{Name: "resolver-repo", Path: repoPath}}}
	cfg := &config.Config{
		RegistryPath: filepath.Join(tmpDir, "repos.json"),
		HistoryPath:  filepath.Join(tmpDir, "history.json"),
		Hosts:        map[string]string{"forge.example": "gitlab"},
		Forge: config.ForgeConfig{
			Default:    "github",
			DefaultOrg: "test",
			Rules:      []config.ForgeRule{{Pattern: "test/*", User: "fixture-user"}},
		},
	}
	if err := saveRegistry(reg, cfg.RegistryPath); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	return testContextWithConfig(t, cfg, repoPath), cfg, reg, repoPath
}

// TestResolveRepoForge_InjectedResolver preserves effective repository settings.
//
// Scenario: A PR command resolves a registered repository with local forge settings.
// Expected: The injected adapter receives local defaults and inherited global settings.
func TestResolveRepoForge_InjectedResolver(t *testing.T) {
	t.Parallel()
	ctx, cfg, _, repoPath := setupForgeResolverRepo(t)
	fake := forgetest.New()
	var gotURL, gotName string
	var gotHosts map[string]string
	var gotConfig *config.ForgeConfig
	ctx = forge.WithResolver(ctx, func(repoURL, name string, hosts map[string]string, forgeCfg *config.ForgeConfig) forge.Forge {
		gotURL, gotName, gotHosts, gotConfig = repoURL, name, hosts, forgeCfg
		return fake
	})

	result, err := resolveRepoForge(ctx, "resolver-repo")
	if err != nil {
		t.Fatalf("resolve repo forge: %v", err)
	}
	if result.forge != fake {
		t.Fatalf("adapter = %T, want injected fake", result.forge)
	}
	if gotURL != "https://github.com/test/resolver-repo.git" || gotName != "" {
		t.Errorf("resolver target = (%q, %q), want repository origin and automatic forge", gotURL, gotName)
	}
	if !reflect.DeepEqual(gotHosts, cfg.Hosts) {
		t.Errorf("resolver hosts = %v, want %v", gotHosts, cfg.Hosts)
	}
	wantForge := cfg.Forge
	wantForge.Default = "gitlab"
	if !reflect.DeepEqual(gotConfig, &wantForge) {
		t.Errorf("resolver forge config = %+v, want %+v", gotConfig, wantForge)
	}
	if result.repo.Path != repoPath || result.branch != "main" || result.effCfg.Forge.Default != "gitlab" {
		t.Errorf("resolved repository = %+v, branch = %q, default forge = %q", result.repo, result.branch, result.effCfg.Forge.Default)
	}
}

// TestPrCheckoutWizardParams_InjectedResolver forwards the explicit forge selection.
//
// Scenario: User fetches PR choices with an explicit forge during interactive checkout.
// Expected: The fetcher uses the injected adapter with the explicit name and local config.
func TestPrCheckoutWizardParams_InjectedResolver(t *testing.T) {
	t.Parallel()
	ctx, cfg, reg, repoPath := setupForgeResolverRepo(t)
	originURL := "https://github.com/test/resolver-repo.git"
	fake := forgetest.New()
	fake.SetPR(originURL, "feature", forge.PRInfo{Number: 207, State: forge.PRStateOpen})
	var gotURL, gotName string
	var gotConfig *config.ForgeConfig
	calls := 0
	ctx = forge.WithResolver(ctx, func(repoURL, name string, hosts map[string]string, forgeCfg *config.ForgeConfig) forge.Forge {
		calls++
		gotURL, gotName, gotConfig = repoURL, name, forgeCfg
		return fake
	})

	params, err := prCheckoutWizardParams(ctx, reg, "resolver-repo", "github", hookFlags{})
	if err != nil {
		t.Fatalf("create wizard params: %v", err)
	}
	if calls != 0 {
		t.Fatal("wizard resolved a forge before deferred PR fetching")
	}
	prs, err := params.FetchPRs(ctx, repoPath)
	if err != nil {
		t.Fatalf("fetch wizard PRs: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 207 || prs[0].Branch != "feature" {
		t.Errorf("wizard PRs = %+v, want PR 207 on feature", prs)
	}
	if calls != 1 || gotURL != originURL || gotName != "github" {
		t.Errorf("resolver calls = %d, target = (%q, %q), want one call with origin and explicit github", calls, gotURL, gotName)
	}
	wantForge := cfg.Forge
	wantForge.Default = "gitlab"
	if !reflect.DeepEqual(gotConfig, &wantForge) {
		t.Errorf("resolver forge config = %+v, want %+v", gotConfig, wantForge)
	}
}

// TestRefreshPRs_InjectedResolver uses the injected adapter through a forge session.
//
// Scenario: PR status refresh fetches two worktrees using their upstream branch names.
// Expected: The context resolver supplies the adapter and both results enter the PR cache.
func TestRefreshPRs_InjectedResolver(t *testing.T) {
	t.Parallel()
	ctx, cfg, _, repoPath := setupForgeResolverRepo(t)
	originURL := "https://github.com/test/resolver-repo.git"
	fake := forgetest.New()
	fake.SetPR(originURL, "remote-feature", forge.PRInfo{Number: 207, State: forge.PRStateOpen, Fetched: true})
	fake.SetPR(originURL, "second", forge.PRInfo{Number: 208, State: forge.PRStateClosed, Fetched: true})
	var mu sync.Mutex
	calls := 0
	var gotURL, gotName string
	var gotHosts map[string]string
	var gotConfig *config.ForgeConfig
	ctx = forge.WithResolver(ctx, func(repoURL, name string, hosts map[string]string, forgeCfg *config.ForgeConfig) forge.Forge {
		mu.Lock()
		defer mu.Unlock()
		calls++
		gotURL, gotName, gotHosts, gotConfig = repoURL, name, hosts, forgeCfg
		return fake
	})
	worktrees := []git.Worktree{
		{RepoPath: repoPath, Branch: "local-feature", OriginURL: originURL, HasUpstream: true, UpstreamBranch: "remote-feature"},
		{RepoPath: repoPath, Branch: "second", OriginURL: originURL, HasUpstream: true},
	}
	cache := prcache.New()

	if failed := refreshPRs(ctx, worktrees, cache, cfg.Hosts, &cfg.Forge); len(failed) != 0 {
		t.Fatalf("PR refresh failed for branches: %v", failed)
	}
	for branch, number := range map[string]int{"local-feature": 207, "second": 208} {
		pr := cache.Get(prcache.CacheKey(repoPath, branch))
		if pr == nil || pr.Number != number || !pr.Fetched {
			t.Errorf("cached PR for %q = %+v, want fetched PR %d", branch, pr, number)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != len(worktrees) || gotURL != originURL || gotName != "" {
		t.Errorf("resolver calls = %d, target = (%q, %q), want one call per worktree for origin", calls, gotURL, gotName)
	}
	if !reflect.DeepEqual(gotHosts, cfg.Hosts) || gotConfig != &cfg.Forge {
		t.Errorf("resolver settings = (%v, %+v), want supplied refresh settings", gotHosts, gotConfig)
	}
}

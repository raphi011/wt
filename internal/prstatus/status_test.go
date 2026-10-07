package prstatus_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/forge/forgetest"
	"github.com/raphi011/wt/internal/fs"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/prstatus"
)

func fixture(t *testing.T) (context.Context, *config.Config, *forgetest.Forge, git.Worktree) {
	t.Helper()
	dir := fs.ResolvePath(t.TempDir())
	cfg := &config.Config{RegistryPath: filepath.Join(dir, "repos.json")}
	f := forgetest.New()
	ctx := forge.WithResolver(context.Background(), func(string, string, map[string]string, *config.ForgeConfig) forge.Forge { return f })
	wt := git.Worktree{RepoPath: filepath.Join(dir, "repo"), Branch: "local", UpstreamBranch: "remote", HasUpstream: true, OriginURL: "https://example.test/org/repo"}
	return ctx, cfg, f, wt
}

func TestRefreshPersistsBeforeReturning(t *testing.T) {
	t.Parallel()
	ctx, cfg, f, wt := fixture(t)
	f.SetPR(wt.OriginURL, wt.UpstreamBranch, forge.PRInfo{Number: 207, State: forge.PRStateOpen, IsDraft: true, URL: "https://example.test/207"})
	var progress [][3]int
	result, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{Refresh: true, Progress: func(done, total, failed int) { progress = append(progress, [3]int{done, total, failed}) }})
	if err != nil {
		t.Fatal(err)
	}
	if result.SaveError != nil || len(result.FailedBranches) != 0 {
		t.Fatalf("refresh: %+v", result)
	}
	if pr := result.For(wt); pr.Number != 207 || !pr.IsDraft || pr.State != forge.PRStateOpen {
		t.Fatalf("lookup: %+v", pr)
	}
	path, err := cfg.GetPRCachePath()
	if err != nil {
		t.Fatal(err)
	}
	if pr := prcache.LoadFrom(path).Get(prcache.CacheKey(wt.RepoPath, wt.Branch)); pr == nil || pr.Number != 207 {
		t.Fatalf("refresh was not saved: %+v", pr)
	}
	if len(progress) != 2 || progress[0] != [3]int{0, 1, 0} || progress[1] != [3]int{1, 1, 0} {
		t.Fatalf("progress: %v", progress)
	}
	copy := result.For(wt)
	copy.State = forge.PRStateClosed
	if result.For(wt).State != forge.PRStateOpen {
		t.Fatal("lookup mutates snapshot")
	}
}

func TestCachedLookupResetAndMutations(t *testing.T) {
	t.Parallel()
	ctx, cfg, _, wt := fixture(t)
	result, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.For(wt).Fetched {
		t.Fatal("missing PR reported fetched")
	}
	if err := result.Record(wt, &forge.PRInfo{Number: 42, State: forge.PRStateMerged, Fetched: true}); err != nil {
		t.Fatal(err)
	}
	result, err = prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.For(wt).Number != 42 {
		t.Fatal("record was not persisted")
	}
	if err := result.Forget(wt); err != nil {
		t.Fatal(err)
	}
	if result.For(wt).Number != 42 {
		t.Fatal("forget changed immutable display snapshot")
	}
	next, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if next.For(wt).Number != 0 {
		t.Fatal("forget was not persisted")
	}
	if err := next.Record(wt, &forge.PRInfo{Number: 43}); err != nil {
		t.Fatal(err)
	}
	reset, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{Reset: true})
	if err != nil {
		t.Fatal(err)
	}
	if reset.SaveError != nil || reset.For(wt).Number != 0 {
		t.Fatalf("reset: %+v", reset)
	}
	next, err = prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if next.For(wt).Number != 0 {
		t.Fatal("reset was not persisted")
	}
}

type failingForge struct{ *forgetest.Forge }

func (f failingForge) GetPRForBranch(context.Context, string, string) (*forge.PRInfo, error) {
	return nil, errors.New("offline")
}

func TestFailedRefreshKeepsCachedStatus(t *testing.T) {
	t.Parallel()
	ctx, cfg, f, wt := fixture(t)
	seed, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Record(wt, &forge.PRInfo{Number: 42, State: forge.PRStateOpen, Fetched: true}); err != nil {
		t.Fatal(err)
	}
	ctx = forge.WithResolver(ctx, func(string, string, map[string]string, *config.ForgeConfig) forge.Forge { return failingForge{f} })
	result, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{Refresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.FailedBranches) != 1 || result.FailedBranches[0] != wt.Branch || result.For(wt).Number != 42 {
		t.Fatalf("failed refresh: %+v, status %+v", result, result.For(wt))
	}
}

func TestRefreshEligibility(t *testing.T) {
	t.Parallel()
	ctx, cfg, f, wt := fixture(t)
	f.SetPR(wt.OriginURL, wt.UpstreamBranch, forge.PRInfo{Number: 99, State: forge.PRStateOpen})
	seed, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Record(wt, &forge.PRInfo{Number: 42, State: forge.PRStateMerged, Fetched: true}); err != nil {
		t.Fatal(err)
	}
	noUpstream, noOrigin := wt, wt
	noUpstream.Branch = "no-upstream"
	noUpstream.HasUpstream = false
	noOrigin.Branch = "no-origin"
	noOrigin.OriginURL = ""
	result, err := prstatus.Load(ctx, []git.Worktree{wt, noUpstream, noOrigin}, cfg, prstatus.Options{Refresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.For(wt).Number != 42 || result.For(noUpstream).Fetched || result.For(noOrigin).Fetched {
		t.Fatal("refresh eligibility changed")
	}
}

func TestSaveFailureStillReturnsFetchedStatus(t *testing.T) {
	t.Parallel()
	ctx, cfg, f, wt := fixture(t)
	blocked := filepath.Join(filepath.Dir(cfg.RegistryPath), "blocked")
	if err := os.WriteFile(blocked, []byte("file instead of directory"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.RegistryPath = filepath.Join(blocked, "repos.json")
	f.SetPR(wt.OriginURL, wt.UpstreamBranch, forge.PRInfo{Number: 207, State: forge.PRStateOpen})
	result, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{Refresh: true})
	if err != nil || result.SaveError == nil || result.For(wt).Number != 207 {
		t.Fatalf("save failure: result=%+v, error=%v", result, err)
	}
}

func TestLookupCanonicalizesRepoPaths(t *testing.T) {
	t.Parallel()
	ctx, cfg, _, wt := fixture(t)
	if err := os.Mkdir(wt.RepoPath, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(cfg.RegistryPath), "alias")
	if err := os.Symlink(wt.RepoPath, alias); err != nil {
		t.Fatal(err)
	}
	seed, err := prstatus.Load(ctx, nil, cfg, prstatus.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Record(wt, &forge.PRInfo{Number: 42, Fetched: true}); err != nil {
		t.Fatal(err)
	}
	aliasWT := wt
	aliasWT.RepoPath = alias
	result, err := prstatus.Load(ctx, []git.Worktree{aliasWT}, cfg, prstatus.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.For(wt).Number != 42 || result.For(aliasWT).Number != 42 {
		t.Fatal("symlink alias did not share PR status")
	}
}

func TestCachedLookupNamespacesReposAndSkipsUnfetched(t *testing.T) {
	t.Parallel()
	ctx, cfg, _, wt := fixture(t)
	other, unfetched := wt, wt
	other.RepoPath = filepath.Join(filepath.Dir(cfg.RegistryPath), "other-repo")
	unfetched.Branch = "unfetched"
	path, err := cfg.GetPRCachePath()
	if err != nil {
		t.Fatal(err)
	}
	cache := prcache.LoadFrom(path)
	first := forge.PRInfo{Number: 42, State: forge.PRStateOpen, URL: "https://example.test/42", IsDraft: true, Fetched: true}
	second := forge.PRInfo{Number: 99, State: forge.PRStateMerged, URL: "https://example.test/99", Fetched: true}
	cache.Set(prcache.CacheKey(wt.RepoPath, wt.Branch), &first)
	cache.Set(prcache.CacheKey(other.RepoPath, other.Branch), &second)
	cache.Set(prcache.CacheKey(unfetched.RepoPath, unfetched.Branch), &forge.PRInfo{Number: 10, State: forge.PRStateOpen, IsDraft: true})
	if err := cache.Save(); err != nil {
		t.Fatal(err)
	}
	ctx = forge.WithResolver(ctx, func(string, string, map[string]string, *config.ForgeConfig) forge.Forge {
		t.Error("cached lookup queried a forge")
		return forgetest.New()
	})
	result, err := prstatus.Load(ctx, []git.Worktree{wt, other, unfetched}, cfg, prstatus.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.For(wt) != first || result.For(other) != second || result.For(unfetched) != (forge.PRInfo{}) {
		t.Fatalf("cached lookup = %+v, %+v, %+v", result.For(wt), result.For(other), result.For(unfetched))
	}
}

func TestCorruptCachePreservesBytesAndReturnsLiveStatus(t *testing.T) {
	t.Parallel()
	ctx, cfg, f, wt := fixture(t)
	path, err := cfg.GetPRCachePath()
	if err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("{original corrupted cache")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	f.SetPR(wt.OriginURL, wt.UpstreamBranch, forge.PRInfo{Number: 209, State: forge.PRStateOpen})
	result, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{Refresh: true})
	if err != nil {
		t.Fatalf("cache corruption should be nonfatal: %v", err)
	}
	if result.LoadError == nil || result.SaveError == nil || len(result.FailedBranches) != 0 {
		t.Errorf("cache diagnostics = %+v, want load and save errors with successful fetching", result)
	}
	if pr := result.For(wt); pr.Number != 209 || pr.State != forge.PRStateOpen || !pr.Fetched {
		t.Errorf("live PR status = %+v, want fetched open PR 209", pr)
	}
	if err := result.Record(wt, &forge.PRInfo{Number: 210, State: forge.PRStateMerged, Fetched: true}); err == nil {
		t.Error("Record must refuse to overwrite a corrupt cache")
	}
	if err := result.Forget(wt); err == nil {
		t.Error("Forget must refuse to overwrite a corrupt cache")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(corrupt) {
		t.Errorf("corrupt cache bytes changed: %q", contents)
	}
}

func TestResetCorruptCacheReportsPersistenceOutcome(t *testing.T) {
	t.Parallel()
	for _, failSave := range []bool{false, true} {
		name := "successful reset"
		if failSave {
			name = "blocked reset"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cfg, _, wt := fixture(t)
			path, err := cfg.GetPRCachePath()
			if err != nil {
				t.Fatal(err)
			}
			corrupt := []byte("{corrupt cache")
			if err := os.WriteFile(path, corrupt, 0600); err != nil {
				t.Fatal(err)
			}
			if failSave {
				if err := os.Mkdir(path+".lock", 0755); err != nil {
					t.Fatal(err)
				}
			}
			result, err := prstatus.Load(ctx, []git.Worktree{wt}, cfg, prstatus.Options{Reset: true})
			if err != nil {
				t.Fatalf("reset cache: %v", err)
			}
			if failSave {
				if result.LoadError == nil || result.SaveError == nil {
					t.Errorf("failed reset lost its diagnostics: %+v", result)
				}
				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(contents) != string(corrupt) {
					t.Errorf("failed reset changed corrupt bytes: %q", contents)
				}
				return
			}
			if result.LoadError != nil || result.SaveError != nil {
				t.Errorf("successful reset retained cache diagnostics: %+v", result)
			}
			recovered := prcache.LoadFrom(path)
			if recovered.LoadError() != nil || len(recovered.PRs) != 0 {
				t.Errorf("reset cache should be valid and empty: %+v", recovered)
			}
		})
	}
}

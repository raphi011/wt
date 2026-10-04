package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
	"github.com/spf13/cobra"
)

// performanceTools uses isolated fake executables; no network or real auth is used.
func performanceTools(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	t.Setenv("WT_PERF_CALLS", logPath)
	t.Setenv("WT_PERF_FAIL", "")
	t.Setenv("WT_PERF_EVENTS", "")
	t.Setenv("WT_PERF_JITTER", "")
	t.Setenv("WT_PERF_VERIFY_TOKEN", "")
	t.Setenv("WT_PERF_MULTI", "")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := `#!/bin/sh
printf '%s %s\n' "${0##*/}" "$* host=${GH_HOST:-${GITLAB_HOST:-}}" >> "$WT_PERF_CALLS"
if [ -n "$WT_PERF_EVENTS" ]; then
 printf 'start\n' >> "$WT_PERF_EVENTS"
 trap 'printf "end\n" >> "$WT_PERF_EVENTS"' EXIT
fi
sleep 0.01
case "$WT_PERF_JITTER:$PWD" in 1:*[02468]) sleep 0.02 ;; esac
if [ "$WT_PERF_FAIL" = auth ]; then
 case "$*" in 'auth status'*) exit 1 ;; esac
fi
case "$PWD" in *missing*) exit 1 ;; esac
case "${0##*/} $*" in
 'git worktree list --porcelain -z') printf 'worktree %s\0HEAD 0123456789\0branch refs/heads/main\0\0' "$PWD"
 if [ "$WT_PERF_MULTI" = 1 ]; then
  printf 'worktree %s/one\0HEAD 0123456789\0branch refs/heads/feature.one\0\0worktree %s/two\0HEAD 0123456789\0branch refs/heads/feature.two\0\0' "$PWD" "$PWD"
 fi ;;
 'git branch --format=%(refname:short)') printf 'main\nalpha\nzeta\n' ;;
 'git remote get-url origin') printf 'https://github.com/org/repo.git\n' ;;
 'git show '*) printf '0123456789|1760000000|1 day ago\n' ;;
 'git config --get-regexp '*) printf 'branch.main.merge refs/heads/remote-main\nbranch.feature.one.merge refs/heads/remote-one\nbranch.feature.two.merge refs/heads/remote-two\n' ;;
 'git config '*) printf 'refs/heads/main\n' ;;
 *'auth token'*) printf 'fixture-%s\n' "$4" ;;
 *'auth status'*) exit 0 ;;
 *'pr list'*|*'mr list'*)
 if [ "$WT_PERF_VERIFY_TOKEN" = 1 ] && [ "${0##*/}" = gh ]; then
  case "$4" in alice/*) expected=fixture-alice ;; bob/*) expected=fixture-bob ;; *) expected=fixture-fixture ;; esac
  [ "$GH_TOKEN" = "$expected" ] || exit 1
  if [ "$GH_HOST" != github.com ]; then [ "$GH_ENTERPRISE_TOKEN" = "$expected" ] || exit 1; fi
 fi
 if [ "$WT_PERF_FAIL" = query ]; then case "$*" in *remote-1*) exit 1 ;; esac; fi
 printf '[]\n' ;;

 *) exit 1 ;;
esac
`
	for _, name := range []string{"git", "gh", "glab"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return logPath
}

// TestPerformanceMeasurements records deterministic command latency separately from network timing.
func TestPerformanceMeasurements(t *testing.T) {
	if os.Getenv("WT_PERF_MEASURE") == "" {
		t.Skip("set WT_PERF_MEASURE=1 for 20-sample median/p95 measurements")
	}
	calls := performanceTools(t)
	for _, n := range []int{1, 10, 50} {
		reg := &registry.Registry{}
		wts := make([]git.Worktree, n)
		for i := range n {
			path := filepath.Join(t.TempDir(), fmt.Sprintf("repo-%d", i))
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatal(err)
			}
			reg.Repos = append(reg.Repos, registry.Repo{Name: fmt.Sprintf("repo-%d", i), Path: path, Labels: []string{"group"}})
			wts[i] = git.Worktree{RepoPath: path, Branch: "main", OriginURL: "https://github.com/org/repo.git", HasUpstream: true}
		}
		regPath := filepath.Join(t.TempDir(), "registry.json")
		if err := reg.Save(regPath); err != nil {
			t.Fatal(err)
		}
		ctx := config.WithConfig(context.Background(), &config.Config{RegistryPath: regPath})
		cmd := &cobra.Command{}
		cmd.SetContext(ctx)
		checkout := &cobra.Command{}
		checkout.SetContext(ctx)
		registerCheckoutCompletions(checkout)
		for _, mode := range []string{"cd", "checkout", "refresh"} {
			var times []time.Duration
			if err := os.WriteFile(calls, nil, 0600); err != nil {
				t.Fatal(err)
			}
			for range 20 {
				start := time.Now()
				switch mode {
				case "cd":
					got, _ := completeScopedWorktreeArg(cmd, nil, "group:")
					if !slices.Equal(got, []string{"group:main"}) {
						t.Fatalf("cd contents: %v", got)
					}
				case "checkout":
					got, _ := checkout.ValidArgsFunction(checkout, nil, "group:")
					slices.Sort(got)
					if !slices.Equal(got, []string{"group:alpha", "group:zeta"}) {
						t.Fatalf("checkout contents: %v", got)
					}
				case "refresh":
					if failed := refreshPRs(ctx, wts, prcache.New(), nil, &config.ForgeConfig{Rules: []config.ForgeRule{{Pattern: "org/*", User: "fixture"}}}); len(failed) != 0 {
						t.Fatalf("refresh failed: %v", failed)
					}
				}
				times = append(times, time.Since(start))
			}
			slices.Sort(times)
			data, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s repos=%d median=%s p95=%s subprocesses/run=%d", mode, n, times[10], times[18], len(strings.Split(strings.TrimSpace(string(data)), "\n"))/20)
		}
	}
}

func readCalls(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRefreshSessionSubprocessCounts(t *testing.T) {
	calls := performanceTools(t)
	t.Setenv("WT_PERF_VERIFY_TOKEN", "1")
	events := filepath.Join(t.TempDir(), "events")
	t.Setenv("WT_PERF_EVENTS", events)
	cfg := &config.ForgeConfig{Rules: []config.ForgeRule{{Pattern: "alice/*", User: "alice"}, {Pattern: "bob/*", User: "bob"}}}
	var wts []git.Worktree
	for i, url := range []string{
		"https://github.com/alice/a.git", "git@github.com-personal:alice/b.git",
		"https://github.com/bob/a.git", "https://enterprise.example/alice/a.git",
		"https://gitlab.com/team/a.git", "https://gitlab.com/team/b.git",
		"https://lab.example/team/a.git",
	} {
		for j := range 3 {
			wts = append(wts, git.Worktree{RepoPath: fmt.Sprintf("/repo/%d", i), Branch: fmt.Sprintf("local-%d", j), UpstreamBranch: fmt.Sprintf("remote-%d", j), OriginURL: url, HasUpstream: true})
		}
	}
	cache := prcache.New()
	cache.Set(prcache.CacheKey(wts[0].RepoPath, wts[0].Branch), &forge.PRInfo{Fetched: true, State: forge.PRStateMerged})
	wts = append(wts, git.Worktree{Branch: "no-origin", HasUpstream: true}, git.Worktree{Branch: "no-upstream", OriginURL: wts[0].OriginURL})
	failed := refreshPRs(context.Background(), wts, cache, map[string]string{"lab.example": "gitlab"}, cfg)
	if len(failed) != 0 {
		t.Fatalf("failed: %v", failed)
	}
	data := readCalls(t, calls)
	for fragment, want := range map[string]int{"auth status": 5, "auth token": 3, "gh pr list": 11, "glab mr list": 9, "git ": 0, "--head remote-": 11, "--source-branch remote-": 9, "--hostname enterprise.example": 2, "--hostname lab.example": 1} {
		if got := strings.Count(data, fragment); got != want {
			t.Errorf("%q count=%d want=%d", fragment, got, want)
		}
	}
	active, peak := 0, 0
	for event := range strings.FieldsSeq(readCalls(t, events)) {
		if event == "start" {
			active++
			peak = max(peak, active)
		} else {
			active--
		}
	}
	if active != 0 || peak > forge.MaxConcurrentFetches {
		t.Fatalf("refresh active=%d peak=%d", active, peak)
	}
	if len(cache.PRs) != 21 {
		t.Errorf("cache size=%d want=21", len(cache.PRs))
	}
	// A new invocation must retry authentication, rather than retain stale tokens.
	if err := os.WriteFile(calls, nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WT_PERF_FAIL", "auth")
	cache = prcache.New()
	failed = refreshPRs(context.Background(), wts, cache, map[string]string{"lab.example": "gitlab"}, cfg)
	data = readCalls(t, calls)
	if len(failed) != 21 || len(cache.PRs) != 0 || strings.Count(data, "auth status") != 5 || strings.Contains(data, "pr list") || strings.Contains(data, "mr list") {
		t.Fatalf("failure caching: failed=%d cache=%d calls=%s", len(failed), len(cache.PRs), data)
	}
	if !slices.IsSorted(failed) {
		t.Fatal("failures are not sorted")
	}
	t.Setenv("WT_PERF_FAIL", "query")
	cache = prcache.New()
	failed = refreshPRs(context.Background(), wts, cache, map[string]string{"lab.example": "gitlab"}, cfg)
	if len(failed) != 7 || len(cache.PRs) != 14 {
		t.Fatalf("partial query failure: failed=%d cache=%d", len(failed), len(cache.PRs))
	}
}

func TestLabelCompletionBoundedAndStable(t *testing.T) {
	calls := performanceTools(t)
	t.Setenv("WT_PERF_JITTER", "1")
	events := filepath.Join(t.TempDir(), "events")
	t.Setenv("WT_PERF_EVENTS", events)
	reg := &registry.Registry{}
	for i := range 50 {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("repo-%d", i))
		if i == 3 {
			path += "-missing"
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		reg.Repos = append(reg.Repos, registry.Repo{Name: fmt.Sprintf("repo-%d", i), Path: path, Labels: []string{"group"}})
	}
	regPath := filepath.Join(t.TempDir(), "registry.json")
	if err := reg.Save(regPath); err != nil {
		t.Fatal(err)
	}
	ctx := config.WithConfig(context.Background(), &config.Config{RegistryPath: regPath})
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	checkout := &cobra.Command{}
	checkout.SetContext(ctx)
	registerCheckoutCompletions(checkout)
	for _, mode := range []string{"cd", "checkout"} {
		for range 3 {
			rand.Shuffle(len(reg.Repos), func(i, j int) { reg.Repos[i], reg.Repos[j] = reg.Repos[j], reg.Repos[i] })
			if err := reg.Save(regPath); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(events, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(calls, nil, 0600); err != nil {
				t.Fatal(err)
			}
			var got, want []string
			if mode == "cd" {
				got, _ = completeScopedWorktreeArg(cmd, nil, "group:")
				want = []string{"group:main"}
			} else {
				got, _ = checkout.ValidArgsFunction(checkout, nil, "group:")
				want = []string{"group:alpha", "group:zeta"}
			}
			if !slices.Equal(got, want) {
				t.Fatalf("%s got=%v want=%v", mode, got, want)
			}
			active, peak := 0, 0
			for event := range strings.FieldsSeq(readCalls(t, events)) {
				if event == "start" {
					active++
					peak = max(peak, active)
				} else {
					active--
				}
			}
			if active != 0 || peak > 8 || peak < 2 {
				t.Fatalf("%s active=%d peak=%d", mode, active, peak)
			}
			data := readCalls(t, calls)
			count := strings.Count(data, "git ")
			expected := 50
			if mode == "checkout" {
				expected = 99
			}
			if count != expected || strings.Contains(data, "config") {
				t.Fatalf("%s subprocess count=%d want=%d", mode, count, expected)
			}
		}
	}
	// A cancelled invocation must launch no subprocesses.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	cmd.SetContext(cancelled)
	if err := os.WriteFile(calls, nil, 0600); err != nil {
		t.Fatal(err)
	}
	got, _ := completeScopedWorktreeArg(cmd, nil, "group:")
	if len(got) != 0 || readCalls(t, calls) != "" {
		t.Fatal("cancelled completion scheduled work")
	}
	refreshPRs(cancelled, []git.Worktree{{RepoPath: "/repo", Branch: "main", OriginURL: "https://github.com/org/repo", HasUpstream: true}}, prcache.New(), nil, nil)
	if readCalls(t, calls) != "" {
		t.Fatal("cancelled refresh scheduled work")
	}
}

func TestRefreshMissingTool(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	wts := []git.Worktree{{RepoPath: "/repo", Branch: "main", OriginURL: "https://github.com/org/repo", HasUpstream: true}, {RepoPath: "/lab", Branch: "feature", OriginURL: "https://gitlab.com/org/repo", HasUpstream: true}}
	cache := prcache.New()
	if failed := refreshPRs(context.Background(), wts, cache, nil, nil); !slices.Equal(failed, []string{"feature", "main"}) || len(cache.PRs) != 0 {
		t.Fatalf("failed=%v cache=%v", failed, cache.PRs)
	}
}

func TestPerformanceCancellationStopsScheduling(t *testing.T) {
	calls := performanceTools(t)
	for _, mode := range []string{"worktrees", "branches", "refresh"} {
		t.Run(mode, func(t *testing.T) {
			if err := os.WriteFile(calls, nil, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var refs []git.RepoRef
			var wts []git.Worktree
			for i := range 50 {
				path := t.TempDir()
				refs = append(refs, git.RepoRef{Name: fmt.Sprint(i), Path: path})
				wts = append(wts, git.Worktree{RepoPath: path, Branch: "main", OriginURL: "https://github.com/org/repo", HasUpstream: true})
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				switch mode {
				case "worktrees":
					git.ListWorktreesForRepos(ctx, refs)
				case "branches":
					git.ListAvailableBranchesForRepos(ctx, refs)
				case "refresh":
					refreshPRs(ctx, wts, prcache.New(), nil, nil)
				}
			}()
			deadline := time.Now().Add(3 * time.Second)
			for {
				data, err := os.ReadFile(calls)
				if err != nil {
					t.Fatal(err)
				}
				if len(data) > 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("subprocess never started")
				}
				time.Sleep(time.Millisecond)
			}
			cancel()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("cancellation did not stop loading")
			}
			count := len(strings.Split(strings.TrimSpace(readCalls(t, calls)), "\n"))
			if count >= 50 {
				t.Fatalf("cancelled %s still scheduled %d subprocesses", mode, count)
			}
		})
	}
}

func TestRefreshUpstreamReadsScaleWithRepos(t *testing.T) {
	calls := performanceTools(t)
	t.Setenv("WT_PERF_MULTI", "1")
	var refs []git.RepoRef
	for i := range 10 {
		refs = append(refs, git.RepoRef{Name: fmt.Sprint(i), Path: t.TempDir()})
	}
	ctx := context.Background()
	wts, warnings := git.LoadWorktreesForRepos(ctx, refs)
	if len(warnings) != 0 || len(wts) != 30 {
		t.Fatalf("worktrees=%d warnings=%v", len(wts), warnings)
	}
	if failed := refreshPRs(ctx, wts, prcache.New(), nil, nil); len(failed) != 0 {
		t.Fatalf("failed: %v", failed)
	}
	data := readCalls(t, calls)
	for fragment, want := range map[string]int{"git config --get-regexp": 10, "git config branch.": 0, "--head remote-main": 10, "--head remote-one": 10, "--head remote-two": 10, "auth status": 1, "gh pr list": 30} {
		if got := strings.Count(data, fragment); got != want {
			t.Errorf("%q count=%d want=%d", fragment, got, want)
		}
	}
}

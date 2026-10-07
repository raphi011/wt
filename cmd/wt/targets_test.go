package main

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

// assertError checks that err matches wantErr. Returns true if an error was expected
// (so the caller can return early).
func assertError(t *testing.T, err error, wantErr string) bool {
	t.Helper()
	if wantErr != "" {
		if err == nil {
			t.Fatalf("expected error %q, got nil", wantErr)
		}
		if err.Error() != wantErr {
			t.Fatalf("error = %q, want %q", err.Error(), wantErr)
		}
		return true
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return false
}

func TestReposToRefs(t *testing.T) {
	t.Parallel()

	t.Run("empty slice", func(t *testing.T) {
		t.Parallel()
		refs := reposToRefs(nil)
		if len(refs) != 0 {
			t.Errorf("reposToRefs(nil) returned %d refs, want 0", len(refs))
		}
	})

	t.Run("multiple repos", func(t *testing.T) {
		t.Parallel()
		repos := []registry.Repo{
			{Name: "repo-a", Path: "/tmp/repo-a"},
			{Name: "repo-b", Path: "/tmp/repo-b"},
			{Name: "repo-c", Path: "/tmp/repo-c"},
		}

		refs := reposToRefs(repos)
		if len(refs) != 3 {
			t.Fatalf("got %d refs, want 3", len(refs))
		}

		for i, ref := range refs {
			want := git.RepoRef{Name: repos[i].Name, Path: repos[i].Path}
			if ref != want {
				t.Errorf("refs[%d] = %+v, want %+v", i, ref, want)
			}
		}
	})
}

func newTestRegistry() *registry.Registry {
	return &registry.Registry{Repos: []registry.Repo{
		{Name: "repo-a", Path: "/tmp/repo-a", Labels: []string{"backend"}},
		{Name: "repo-b", Path: "/tmp/repo-b", Labels: []string{"backend", "frontend"}},
		{Name: "repo-c", Path: "/tmp/repo-c", Labels: []string{"frontend"}},
	}}
}

func TestParseScopedTarget(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()

	tests := []struct {
		name       string
		target     string
		wantRepos  int
		wantLabel  bool
		wantBranch string
		wantErr    string
	}{
		{
			name:       "no scope",
			target:     "feature",
			wantRepos:  0,
			wantBranch: "feature",
		},
		{
			name:       "repo scope",
			target:     "repo-a:feature",
			wantRepos:  1,
			wantLabel:  false,
			wantBranch: "feature",
		},
		{
			name:       "label scope matches multiple",
			target:     "backend:feature",
			wantRepos:  2,
			wantLabel:  true,
			wantBranch: "feature",
		},
		{
			name:       "label scope matches one",
			target:     "frontend:bugfix",
			wantRepos:  2,
			wantLabel:  true,
			wantBranch: "bugfix",
		},
		{
			name:    "unknown scope",
			target:  "nonexistent:feature",
			wantErr: "no repo or label found: nonexistent",
		},
		{
			name:    "scope with no branch",
			target:  "repo-a:",
			wantErr: `branch name required after "repo-a:"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseScopedTarget(reg, tt.target)

			if assertError(t, err, tt.wantErr) {
				return
			}
			if len(result.Repos) != tt.wantRepos {
				t.Errorf("got %d repos, want %d", len(result.Repos), tt.wantRepos)
			}
			if result.IsLabel != tt.wantLabel {
				t.Errorf("IsLabel = %v, want %v", result.IsLabel, tt.wantLabel)
			}
			if result.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", result.Branch, tt.wantBranch)
			}
		})
	}

	// Repo name takes precedence over label when both match
	t.Run("repo takes precedence over label", func(t *testing.T) {
		t.Parallel()
		reg := &registry.Registry{Repos: []registry.Repo{
			{Name: "backend", Path: "/tmp/backend"},
			{Name: "other", Path: "/tmp/other", Labels: []string{"backend"}},
		}}
		result, err := parseScopedTarget(reg, "backend:feat")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsLabel {
			t.Error("expected repo match (IsLabel=false), got label match")
		}
		if len(result.Repos) != 1 || result.Repos[0].Name != "backend" {
			t.Errorf("expected repo 'backend', got %v", result.Repos)
		}
	})
}

func TestResolveScopedRepos(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()

	tests := []struct {
		name      string
		scope     string
		wantNames []string
		wantErr   string
	}{
		{
			name:    "empty scope",
			scope:   "",
			wantErr: "repo or label required",
		},
		{
			name:      "repo name",
			scope:     "repo-a",
			wantNames: []string{"repo-a"},
		},
		{
			name:      "label",
			scope:     "backend",
			wantNames: []string{"repo-a", "repo-b"},
		},
		{
			name:    "unknown scope",
			scope:   "nonexistent",
			wantErr: "no repo or label found: nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repos, err := resolveScopedRepos(reg, tt.scope)

			if assertError(t, err, tt.wantErr) {
				return
			}

			var gotNames []string
			for _, r := range repos {
				gotNames = append(gotNames, r.Name)
			}

			if len(gotNames) != len(tt.wantNames) {
				t.Fatalf("got repos %v, want %v", gotNames, tt.wantNames)
			}
			for i, name := range gotNames {
				if name != tt.wantNames[i] {
					t.Errorf("repos[%d] = %q, want %q", i, name, tt.wantNames[i])
				}
			}
		})
	}
}

func TestResolveScopeArgs(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()

	t.Run("single scope", func(t *testing.T) {
		t.Parallel()
		repos, err := resolveScopeArgs(reg, []string{"repo-a"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repos) != 1 || repos[0].Name != "repo-a" {
			t.Errorf("got %v, want [repo-a]", repos)
		}
	})

	t.Run("multiple scopes", func(t *testing.T) {
		t.Parallel()
		repos, err := resolveScopeArgs(reg, []string{"repo-a", "repo-c"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repos) != 2 {
			t.Fatalf("got %d repos, want 2", len(repos))
		}
	})

	t.Run("dedup same repo via name and label", func(t *testing.T) {
		t.Parallel()
		// "repo-a" by name + "backend" label (includes repo-a and repo-b)
		repos, err := resolveScopeArgs(reg, []string{"repo-a", "backend"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// repo-a appears via name and via backend label — should be deduped
		if len(repos) != 2 {
			names := make([]string, len(repos))
			for i, r := range repos {
				names[i] = r.Name
			}
			t.Errorf("expected 2 repos (repo-a deduped), got %d: %v", len(repos), names)
		}
	})

	t.Run("unknown scope error", func(t *testing.T) {
		t.Parallel()
		_, err := resolveScopeArgs(reg, []string{"nonexistent"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestResolveWorktreeTargetsIn(t *testing.T) {
	t.Parallel()

	reg := newTestRegistry()
	repoA, repoB := reg.Repos[0], reg.Repos[1]

	worktrees := map[string][]git.WorktreeInfo{
		"/tmp/repo-a": {{Path: "/wt/a-main", Branch: "main"}, {Path: "/wt/a-feature", Branch: "feature"}},
		"/tmp/repo-b": {{Path: "/wt/b-main", Branch: "main"}, {Path: "/wt/b-feature", Branch: "feature"}},
		"/tmp/repo-c": {{Path: "/wt/c-main", Branch: "main"}, {Path: "/wt/c-only", Branch: "only-c"}},
	}
	inRepoA := func() ([]registry.Repo, bool, error) { return []registry.Repo{repoA}, true, nil }
	allRepos := func() ([]registry.Repo, bool, error) { return reg.Repos, false, nil }

	tests := []struct {
		name     string
		targets  []string
		opts     targetOpts
		unscoped func() ([]registry.Repo, bool, error)
		broken   string // repo path where listing worktrees fails
		want     []string
		wantErr  string
		wantLog  string
	}{
		{name: "repo scope", targets: []string{"repo-b:feature"}, want: []string{"/wt/b-feature"}},
		{name: "repo scope, no worktree", targets: []string{"repo-a:only-c"}, wantErr: "worktree not found: repo-a:only-c"},
		{name: "repo scope, git fails", targets: []string{"repo-a:feature"}, broken: "/tmp/repo-a", wantErr: "repo-a: boom"},
		{name: "label scope, single-target is ambiguous", targets: []string{"backend:feature"}, wantErr: `branch "feature" exists in multiple repos: repo-a:feature, repo-b:feature (use repo:branch to pick one)`},
		{name: "label scope, multi-target fans out", targets: []string{"backend:feature"}, opts: targetOpts{Multi: true}, want: []string{"/wt/a-feature", "/wt/b-feature"}},
		{name: "label scope, repos without the branch are skipped", targets: []string{"frontend:feature"}, want: []string{"/wt/b-feature"}, wantLog: "Skipped (no worktree for feature): repo-c\n"},
		{name: "label scope, failing repo is skipped", targets: []string{"backend:feature"}, broken: "/tmp/repo-a", want: []string{"/wt/b-feature"}, wantLog: "Warning: repo-a: boom\n"},
		{name: "label scope, no worktree", targets: []string{"backend:only-c"}, wantErr: "worktree not found: backend:only-c (label matched 2 repos)"},
		{name: "unknown scope", targets: []string{"nope:feature"}, wantErr: "no repo or label found: nope"},
		{name: "unscoped, current repo", targets: []string{"feature"}, unscoped: inRepoA, want: []string{"/wt/a-feature"}},
		{name: "unscoped, current repo, multi-target stays in it", targets: []string{"feature"}, opts: targetOpts{Multi: true}, unscoped: inRepoA, want: []string{"/wt/a-feature"}},
		{name: "unscoped, current repo, no worktree", targets: []string{"only-c"}, unscoped: inRepoA, wantErr: "worktree not found in repo-a: only-c (use -g to search all repos, or repo:branch)"},
		{name: "unscoped, current repo, git fails", targets: []string{"feature"}, unscoped: inRepoA, broken: "/tmp/repo-a", wantErr: "repo-a: boom"},
		{name: "unscoped, all repos, unique", targets: []string{"only-c"}, unscoped: allRepos, want: []string{"/wt/c-only"}},
		{name: "unscoped, all repos, no worktree", targets: []string{"nope"}, unscoped: allRepos, wantErr: "worktree not found: nope"},
		{name: "unscoped, all repos, single-target is ambiguous", targets: []string{"feature"}, opts: targetOpts{Global: true}, unscoped: allRepos, wantErr: `branch "feature" exists in multiple repos: repo-a:feature, repo-b:feature (use repo:branch to pick one)`},
		{name: "unscoped, all repos, multi-target needs -g", targets: []string{"feature"}, opts: targetOpts{Multi: true}, unscoped: allRepos, wantErr: `branch "feature" exists in multiple repos: repo-a:feature, repo-b:feature (use -g to target all of them, or repo:branch to pick one)`},
		{name: "unscoped, all repos, -g multi-target fans out", targets: []string{"feature"}, opts: targetOpts{Global: true, Multi: true}, unscoped: allRepos, want: []string{"/wt/a-feature", "/wt/b-feature"}},
		{name: "unscoped, all repos, failing repo is skipped", targets: []string{"feature"}, unscoped: allRepos, broken: "/tmp/repo-a", want: []string{"/wt/b-feature"}, wantLog: "Warning: repo-a: boom\n"},
		{name: "unscoped, current repo lookup fails", targets: []string{"feature"}, unscoped: func() ([]registry.Repo, bool, error) { return nil, false, errors.New("update registry: denied") }, wantErr: "update registry: denied"},
		{name: "several targets, same worktree once", targets: []string{"repo-a:feature", "backend:feature", "main"}, opts: targetOpts{Multi: true}, unscoped: inRepoA, want: []string{"/wt/a-feature", "/wt/b-feature", "/wt/a-main"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := targetEnv{
				unscopedRepos: func() ([]registry.Repo, bool, error) {
					if tt.unscoped == nil {
						t.Fatal("scoped target must not look up the current repo")
					}
					return tt.unscoped()
				},
				worktrees: func(repoPath string) ([]git.WorktreeInfo, error) {
					if repoPath == tt.broken {
						return nil, errors.New("boom")
					}
					return worktrees[repoPath], nil
				},
			}
			var logs bytes.Buffer
			got, err := resolveWorktreeTargetsIn(log.New(&logs, false, false), reg, tt.targets, tt.opts, env)

			if assertError(t, err, tt.wantErr) {
				return
			}
			var paths []string
			for _, wt := range got {
				paths = append(paths, wt.Path)
			}
			if !slices.Equal(paths, tt.want) {
				t.Errorf("resolved %v, want %v", paths, tt.want)
			}
			if logs.String() != tt.wantLog {
				t.Errorf("log = %q, want %q", logs.String(), tt.wantLog)
			}
		})
	}

	t.Run("target carries repo and branch", func(t *testing.T) {
		t.Parallel()
		env := targetEnv{worktrees: func(repoPath string) ([]git.WorktreeInfo, error) { return worktrees[repoPath], nil }}
		got, err := resolveWorktreeTargetsIn(log.New(io.Discard, false, false), reg, []string{"repo-b:feature"}, targetOpts{}, env)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := WorktreeTarget{RepoName: repoB.Name, RepoPath: repoB.Path, Branch: "feature", Path: "/wt/b-feature"}
		if len(got) != 1 || got[0] != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

package main

import (
	"testing"

	"github.com/raphi011/wt/internal/git"
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
		name      string
		target    string
		wantRepos int
		wantLabel bool
		wantBranch string
		wantErr   string
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

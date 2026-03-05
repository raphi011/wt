package main

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func TestFilterOrphanedRepos(t *testing.T) {
	t.Parallel()

	t.Run("keeps repos with existing paths", func(t *testing.T) {
		t.Parallel()

		dir1, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatalf("EvalSymlinks: %v", err)
		}
		dir2, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatalf("EvalSymlinks: %v", err)
		}

		l := log.New(io.Discard, false, true)
		repos := []registry.Repo{
			{Name: "repo-a", Path: dir1},
			{Name: "repo-b", Path: dir2},
		}

		got := filterOrphanedRepos(l, repos)

		if len(got) != 2 {
			t.Fatalf("len(got) = %d, want 2", len(got))
		}
		if got[0].Name != "repo-a" {
			t.Errorf("got[0].Name = %q, want %q", got[0].Name, "repo-a")
		}
		if got[1].Name != "repo-b" {
			t.Errorf("got[1].Name = %q, want %q", got[1].Name, "repo-b")
		}
	})

	t.Run("filters repos with missing paths", func(t *testing.T) {
		t.Parallel()

		existingDir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatalf("EvalSymlinks: %v", err)
		}

		l := log.New(io.Discard, false, true)
		repos := []registry.Repo{
			{Name: "exists", Path: existingDir},
			{Name: "missing-1", Path: "/nonexistent/path/abc123"},
			{Name: "missing-2", Path: "/nonexistent/path/def456"},
		}

		got := filterOrphanedRepos(l, repos)

		if len(got) != 1 {
			t.Fatalf("len(got) = %d, want 1", len(got))
		}
		if got[0].Name != "exists" {
			t.Errorf("got[0].Name = %q, want %q", got[0].Name, "exists")
		}
	})

	t.Run("returns nil for all missing", func(t *testing.T) {
		t.Parallel()

		l := log.New(io.Discard, false, true)
		repos := []registry.Repo{
			{Name: "gone-1", Path: "/nonexistent/aaa"},
			{Name: "gone-2", Path: "/nonexistent/bbb"},
		}

		got := filterOrphanedRepos(l, repos)

		if got != nil {
			t.Errorf("got = %v, want nil", got)
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Parallel()

		l := log.New(io.Discard, false, true)

		got := filterOrphanedRepos(l, []registry.Repo{})

		if got != nil {
			t.Errorf("got = %v, want nil", got)
		}
	})
}

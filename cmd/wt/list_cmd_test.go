package main

import (
	"testing"
	"time"

	"github.com/raphi011/wt/internal/git"
)

func TestSortWorktreesByMode(t *testing.T) {
	t.Parallel()

	now := time.Now()

	base := []git.Worktree{
		{RepoName: "bravo", Branch: "feature", CommitDate: now.Add(-2 * time.Hour)},
		{RepoName: "alpha", Branch: "zebra", CommitDate: now.Add(-1 * time.Hour)},
		{RepoName: "alpha", Branch: "apple", CommitDate: now},
	}

	copyWts := func() []git.Worktree {
		wts := make([]git.Worktree, len(base))
		copy(wts, base)
		return wts
	}

	t.Run("date sorts newest first", func(t *testing.T) {
		t.Parallel()
		wts := copyWts()
		if err := sortWorktreesByMode(wts, "date"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"apple", "zebra", "feature"}
		for i, wt := range wts {
			if wt.Branch != want[i] {
				t.Errorf("[%d] Branch = %q, want %q", i, wt.Branch, want[i])
			}
		}
	})

	t.Run("repo sorts by repo then branch", func(t *testing.T) {
		t.Parallel()
		wts := copyWts()
		if err := sortWorktreesByMode(wts, "repo"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"apple", "zebra", "feature"}
		for i, wt := range wts {
			if wt.Branch != want[i] {
				t.Errorf("[%d] Branch = %q, want %q", i, wt.Branch, want[i])
			}
		}
	})

	t.Run("branch sorts alphabetically", func(t *testing.T) {
		t.Parallel()
		wts := copyWts()
		if err := sortWorktreesByMode(wts, "branch"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"apple", "feature", "zebra"}
		for i, wt := range wts {
			if wt.Branch != want[i] {
				t.Errorf("[%d] Branch = %q, want %q", i, wt.Branch, want[i])
			}
		}
	})

	t.Run("invalid mode returns error", func(t *testing.T) {
		t.Parallel()
		wts := copyWts()
		err := sortWorktreesByMode(wts, "invalid")
		if err == nil {
			t.Fatal("expected error for invalid mode")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()
		if err := sortWorktreesByMode(nil, "date"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

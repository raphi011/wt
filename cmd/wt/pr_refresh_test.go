package main

import (
	"testing"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/prcache"
)

func TestPopulatePRFields(t *testing.T) {
	t.Parallel()

	t.Run("populates PR fields from cache", func(t *testing.T) {
		t.Parallel()

		worktrees := []git.Worktree{
			{RepoPath: "/repo/a", Branch: "feat-1"},
			{RepoPath: "/repo/b", Branch: "feat-2"},
		}

		cache := prcache.New()
		cache.Set(prcache.CacheKey("/repo/a", "feat-1"), &forge.PRInfo{
			Number:  42,
			State:   "OPEN",
			URL:     "https://github.com/org/a/pull/42",
			IsDraft: true,
			Fetched: true,
		})
		cache.Set(prcache.CacheKey("/repo/b", "feat-2"), &forge.PRInfo{
			Number:  99,
			State:   "MERGED",
			URL:     "https://github.com/org/b/pull/99",
			IsDraft: false,
			Fetched: true,
		})

		populatePRFields(worktrees, cache)

		if worktrees[0].PRNumber != 42 {
			t.Errorf("worktrees[0].PRNumber = %d, want 42", worktrees[0].PRNumber)
		}
		if worktrees[0].PRState != "OPEN" {
			t.Errorf("worktrees[0].PRState = %q, want %q", worktrees[0].PRState, "OPEN")
		}
		if worktrees[0].PRURL != "https://github.com/org/a/pull/42" {
			t.Errorf("worktrees[0].PRURL = %q, want %q", worktrees[0].PRURL, "https://github.com/org/a/pull/42")
		}
		if !worktrees[0].PRDraft {
			t.Error("worktrees[0].PRDraft = false, want true")
		}

		if worktrees[1].PRNumber != 99 {
			t.Errorf("worktrees[1].PRNumber = %d, want 99", worktrees[1].PRNumber)
		}
		if worktrees[1].PRState != "MERGED" {
			t.Errorf("worktrees[1].PRState = %q, want %q", worktrees[1].PRState, "MERGED")
		}
		if worktrees[1].PRURL != "https://github.com/org/b/pull/99" {
			t.Errorf("worktrees[1].PRURL = %q, want %q", worktrees[1].PRURL, "https://github.com/org/b/pull/99")
		}
		if worktrees[1].PRDraft {
			t.Error("worktrees[1].PRDraft = true, want false")
		}
	})

	t.Run("skips unfetched cache entries", func(t *testing.T) {
		t.Parallel()

		worktrees := []git.Worktree{
			{RepoPath: "/repo/a", Branch: "feat-1"},
		}

		cache := prcache.New()
		cache.Set(prcache.CacheKey("/repo/a", "feat-1"), &forge.PRInfo{
			Number:  10,
			State:   "OPEN",
			URL:     "https://github.com/org/a/pull/10",
			IsDraft: true,
			Fetched: false,
		})

		populatePRFields(worktrees, cache)

		if worktrees[0].PRNumber != 0 {
			t.Errorf("worktrees[0].PRNumber = %d, want 0", worktrees[0].PRNumber)
		}
		if worktrees[0].PRState != "" {
			t.Errorf("worktrees[0].PRState = %q, want %q", worktrees[0].PRState, "")
		}
		if worktrees[0].PRURL != "" {
			t.Errorf("worktrees[0].PRURL = %q, want %q", worktrees[0].PRURL, "")
		}
		if worktrees[0].PRDraft {
			t.Error("worktrees[0].PRDraft = true, want false")
		}
	})

	t.Run("skips cache miss", func(t *testing.T) {
		t.Parallel()

		worktrees := []git.Worktree{
			{RepoPath: "/repo/a", Branch: "no-match"},
		}

		cache := prcache.New()

		populatePRFields(worktrees, cache)

		if worktrees[0].PRNumber != 0 {
			t.Errorf("worktrees[0].PRNumber = %d, want 0", worktrees[0].PRNumber)
		}
		if worktrees[0].PRState != "" {
			t.Errorf("worktrees[0].PRState = %q, want %q", worktrees[0].PRState, "")
		}
		if worktrees[0].PRURL != "" {
			t.Errorf("worktrees[0].PRURL = %q, want %q", worktrees[0].PRURL, "")
		}
		if worktrees[0].PRDraft {
			t.Error("worktrees[0].PRDraft = true, want false")
		}
	})

	t.Run("handles empty worktrees", func(t *testing.T) {
		t.Parallel()

		cache := prcache.New()
		cache.Set(prcache.CacheKey("/repo/a", "feat-1"), &forge.PRInfo{
			Number:  1,
			Fetched: true,
		})

		// Should not panic
		populatePRFields(nil, cache)
		populatePRFields([]git.Worktree{}, cache)
	})
}

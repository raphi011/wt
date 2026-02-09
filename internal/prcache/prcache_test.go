package prcache

import (
	"testing"
	"time"

	"github.com/raphi011/wt/internal/forge"
)

func TestFromForge(t *testing.T) {
	t.Parallel()

	now := time.Now()
	src := &forge.PRInfo{
		Number:       42,
		State:        forge.PRStateMerged,
		IsDraft:      false,
		URL:          "https://github.com/org/repo/pull/42",
		Author:       "alice",
		CommentCount: 3,
		HasReviews:   true,
		IsApproved:   true,
		CachedAt:     now,
		Fetched:      true,
	}

	got := FromForge(src)

	if got.Number != src.Number {
		t.Errorf("Number = %d, want %d", got.Number, src.Number)
	}
	if got.State != src.State {
		t.Errorf("State = %q, want %q", got.State, src.State)
	}
	if got.IsDraft != src.IsDraft {
		t.Errorf("IsDraft = %v, want %v", got.IsDraft, src.IsDraft)
	}
	if got.URL != src.URL {
		t.Errorf("URL = %q, want %q", got.URL, src.URL)
	}
	if got.Author != src.Author {
		t.Errorf("Author = %q, want %q", got.Author, src.Author)
	}
	if got.CommentCount != src.CommentCount {
		t.Errorf("CommentCount = %d, want %d", got.CommentCount, src.CommentCount)
	}
	if got.HasReviews != src.HasReviews {
		t.Errorf("HasReviews = %v, want %v", got.HasReviews, src.HasReviews)
	}
	if got.IsApproved != src.IsApproved {
		t.Errorf("IsApproved = %v, want %v", got.IsApproved, src.IsApproved)
	}
	if !got.CachedAt.Equal(src.CachedAt) {
		t.Errorf("CachedAt = %v, want %v", got.CachedAt, src.CachedAt)
	}
	if got.Fetched != src.Fetched {
		t.Errorf("Fetched = %v, want %v", got.Fetched, src.Fetched)
	}
}

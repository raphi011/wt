package prcache

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/storage"
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

func TestFromForge_Nil(t *testing.T) {
	t.Parallel()

	got := FromForge(nil)
	if got != nil {
		t.Errorf("FromForge(nil) = %v, want nil", got)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.PRs == nil {
		t.Fatal("New().PRs is nil, want initialized map")
	}
	if len(c.PRs) != 0 {
		t.Errorf("New().PRs has %d entries, want 0", len(c.PRs))
	}
}

func TestCacheKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		repo   string
		branch string
		want   string
	}{
		{"normal", "/path/to/repo", "feature", "/path/to/repo:feature"},
		{"empty repo", "", "feature", ":feature"},
		{"empty branch", "/path/to/repo", "", "/path/to/repo:"},
		{"branch with slashes", "/repo", "feature/sub/deep", "/repo:feature/sub/deep"},
		{"branch with colons", "/repo", "fix:thing", "/repo:fix:thing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CacheKey(tt.repo, tt.branch)
			if got != tt.want {
				t.Errorf("CacheKey(%q, %q) = %q, want %q", tt.repo, tt.branch, got, tt.want)
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cachedAt time.Time
		want     bool
	}{
		{"zero time is stale", time.Time{}, true},
		{"1h ago is fresh", time.Now().Add(-1 * time.Hour), false},
		{"25h ago is stale", time.Now().Add(-25 * time.Hour), true},
		{"exactly at boundary is fresh", time.Now().Add(-CacheMaxAge + time.Second), false},
		{"just past boundary is stale", time.Now().Add(-CacheMaxAge - time.Second), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pr := &PRInfo{CachedAt: tt.cachedAt}
			got := pr.IsStale()
			if got != tt.want {
				t.Errorf("IsStale() = %v, want %v (cachedAt: %v)", got, tt.want, tt.cachedAt)
			}
		})
	}
}

func TestSetGet(t *testing.T) {
	t.Parallel()

	c := New()
	key := "/repo:feature"
	pr := &PRInfo{
		Number: 42,
		State:  "OPEN",
		URL:    "https://github.com/org/repo/pull/42",
	}

	c.Set(key, pr)

	got := c.Get(key)
	if got == nil {
		t.Fatal("Get returned nil after Set")
	}
	if got.Number != 42 {
		t.Errorf("Number = %d, want 42", got.Number)
	}
	if got.State != "OPEN" {
		t.Errorf("State = %q, want OPEN", got.State)
	}

	// Nonexistent key returns nil
	if c.Get("nonexistent") != nil {
		t.Error("Get(nonexistent) should return nil")
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	c := New()
	key := "/repo:feature"
	c.Set(key, &PRInfo{Number: 1})

	c.Delete(key)
	if c.Get(key) != nil {
		t.Error("Get after Delete should return nil")
	}

	// Delete nonexistent key doesn't panic
	c.Delete("nonexistent")
}

func TestReset(t *testing.T) {
	t.Parallel()

	c := New()
	c.Set("key1", &PRInfo{Number: 1})
	c.Set("key2", &PRInfo{Number: 2})

	c.Reset()

	if c.Get("key1") != nil {
		t.Error("Get(key1) after Reset should return nil")
	}
	if c.Get("key2") != nil {
		t.Error("Get(key2) after Reset should return nil")
	}
	if c.PRs == nil {
		t.Error("PRs map should be non-nil after Reset")
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "prs.json")

	now := time.Now().Truncate(time.Second) // JSON loses sub-second precision

	original := &Cache{
		PRs: map[string]*PRInfo{
			"/repo:main": {
				Number:       10,
				State:        "MERGED",
				IsDraft:      false,
				URL:          "https://github.com/org/repo/pull/10",
				Author:       "bob",
				CommentCount: 5,
				HasReviews:   true,
				IsApproved:   true,
				CachedAt:     now,
				Fetched:      true,
			},
			"/repo:feature": {
				Number:  20,
				State:   "OPEN",
				IsDraft: true,
				Fetched: true,
			},
		},
	}

	if err := storage.SaveJSON(path, original); err != nil {
		t.Fatalf("SaveJSON failed: %v", err)
	}

	var loaded Cache
	if err := storage.LoadJSON(path, &loaded); err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}

	if len(loaded.PRs) != 2 {
		t.Fatalf("loaded %d PRs, want 2", len(loaded.PRs))
	}

	pr := loaded.PRs["/repo:main"]
	if pr == nil {
		t.Fatal("missing /repo:main entry")
	}
	if pr.Number != 10 {
		t.Errorf("Number = %d, want 10", pr.Number)
	}
	if pr.State != "MERGED" {
		t.Errorf("State = %q, want MERGED", pr.State)
	}
	if pr.Author != "bob" {
		t.Errorf("Author = %q, want bob", pr.Author)
	}
	if !pr.CachedAt.Equal(now) {
		t.Errorf("CachedAt = %v, want %v", pr.CachedAt, now)
	}

	pr2 := loaded.PRs["/repo:feature"]
	if pr2 == nil {
		t.Fatal("missing /repo:feature entry")
	}
	if !pr2.IsDraft {
		t.Error("IsDraft should be true")
	}
}

func TestPath(t *testing.T) {
	t.Parallel()

	p := Path()
	if p == "" {
		t.Fatal("Path() returned empty string")
	}
	if filepath.Base(p) != "prs.json" {
		t.Errorf("Path() = %q, want base name prs.json", p)
	}
	if filepath.Base(filepath.Dir(p)) != ".wt" {
		t.Errorf("Path() parent = %q, want .wt", filepath.Dir(p))
	}
}

func TestLoadSave(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "prs.json")

	// Create cache, set data, save to explicit path
	c := New()
	c.Set("/repo:main", &PRInfo{Number: 1, State: "OPEN", Fetched: true})
	c.Set("/repo:feature", &PRInfo{Number: 2, State: "MERGED", Fetched: true})

	if err := storage.SaveJSON(path, c); err != nil {
		t.Fatalf("SaveJSON failed: %v", err)
	}

	// Load back
	var loaded Cache
	if err := storage.LoadJSON(path, &loaded); err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}
	if loaded.PRs == nil {
		t.Fatal("loaded PRs map is nil")
	}
	if len(loaded.PRs) != 2 {
		t.Fatalf("loaded %d PRs, want 2", len(loaded.PRs))
	}
	if loaded.PRs["/repo:main"].Number != 1 {
		t.Errorf("PR number = %d, want 1", loaded.PRs["/repo:main"].Number)
	}
}

func TestDirtyFlag(t *testing.T) {
	t.Parallel()

	c := New()

	// New cache is not dirty — SaveIfDirty should be a no-op
	if err := c.SaveIfDirty(); err != nil {
		t.Fatalf("SaveIfDirty on clean cache: %v", err)
	}

	// After Set, cache is dirty
	c.Set("key", &PRInfo{Number: 1})
	if !c.dirty {
		t.Error("dirty should be true after Set")
	}

	c.dirty = false

	// After Delete, dirty again
	c.Delete("key")
	if !c.dirty {
		t.Error("dirty should be true after Delete")
	}

	c.dirty = false

	// After Reset, dirty again
	c.Reset()
	if !c.dirty {
		t.Error("dirty should be true after Reset")
	}
}

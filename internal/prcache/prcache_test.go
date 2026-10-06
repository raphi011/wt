package prcache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/fs"
)

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

func TestSetGet(t *testing.T) {
	t.Parallel()

	c := New()
	key := "/repo:feature"
	pr := &forge.PRInfo{
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
	c.Set(key, &forge.PRInfo{Number: 1})

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
	c.Set("key1", &forge.PRInfo{Number: 1})
	c.Set("key2", &forge.PRInfo{Number: 2})

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
		PRs: map[string]*forge.PRInfo{
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

	if err := fs.SaveJSON(path, original); err != nil {
		t.Fatalf("SaveJSON failed: %v", err)
	}

	var loaded Cache
	if err := fs.LoadJSON(path, &loaded); err != nil {
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

func TestSaveWithoutPath(t *testing.T) {
	t.Parallel()

	c := New()
	c.Set("/repo:main", &forge.PRInfo{Number: 1, State: "OPEN"})

	if err := c.Save(); err == nil {
		t.Fatal("Save on a cache without a path should fail")
	}
}

func TestLoadSave(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "prs.json")

	// Create cache, set data, save to explicit path
	c := New()
	c.Set("/repo:main", &forge.PRInfo{Number: 1, State: "OPEN", Fetched: true})
	c.Set("/repo:feature", &forge.PRInfo{Number: 2, State: "MERGED", Fetched: true})

	if err := fs.SaveJSON(path, c); err != nil {
		t.Fatalf("SaveJSON failed: %v", err)
	}

	// Load back
	var loaded Cache
	if err := fs.LoadJSON(path, &loaded); err != nil {
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

func TestSave_NoChanges(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prs.json")

	if err := LoadFrom(path).Save(); err != nil {
		t.Fatalf("Save without changes: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Save without changes should not write the file, stat error: %v", err)
	}
}

func TestLoadFrom(t *testing.T) {
	t.Parallel()

	t.Run("loads valid cache", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "prs.json")

		original := LoadFrom(path)
		original.Set("/repo:main", &forge.PRInfo{Number: 42, State: "OPEN", Fetched: true})
		if err := original.Save(); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		loaded := LoadFrom(path)
		if loaded.PRs == nil {
			t.Fatal("loaded PRs map is nil")
		}
		pr := loaded.Get("/repo:main")
		if pr == nil {
			t.Fatal("expected /repo:main entry")
		}
		if pr.Number != 42 {
			t.Errorf("Number = %d, want 42", pr.Number)
		}
	})

	t.Run("returns empty cache for missing file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.json")

		loaded := LoadFrom(path)
		if loaded == nil {
			t.Fatal("LoadFrom returned nil")
		}
		if loaded.PRs == nil {
			t.Fatal("PRs map should be initialized")
		}
		if len(loaded.PRs) != 0 {
			t.Errorf("expected 0 entries, got %d", len(loaded.PRs))
		}
	})

	t.Run("returns empty cache for corrupted JSON", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "bad.json")

		if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
			t.Fatalf("setup: write failed: %v", err)
		}

		loaded := LoadFrom(path)
		if loaded == nil {
			t.Fatal("LoadFrom returned nil")
		}
		if loaded.PRs == nil {
			t.Fatal("PRs map should be initialized")
		}
		if len(loaded.PRs) != 0 {
			t.Errorf("expected 0 entries, got %d", len(loaded.PRs))
		}
	})

	t.Run("initializes nil PRs map", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "empty.json")

		// Write valid JSON with null prs field
		if err := os.WriteFile(path, []byte(`{"prs":null}`), 0644); err != nil {
			t.Fatalf("setup: write failed: %v", err)
		}

		loaded := LoadFrom(path)
		if loaded.PRs == nil {
			t.Fatal("PRs map should be initialized even when null in JSON")
		}
	})

	t.Run("round-trip Save then LoadFrom", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "rt.json")

		now := time.Now().Truncate(time.Second)
		original := LoadFrom(path)
		original.Set("/repo:feat", &forge.PRInfo{
			Number:   99,
			State:    "MERGED",
			Author:   "alice",
			CachedAt: now,
			Fetched:  true,
		})

		if err := original.Save(); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		loaded := LoadFrom(path)
		pr := loaded.Get("/repo:feat")
		if pr == nil {
			t.Fatal("missing /repo:feat entry after round-trip")
		}
		if pr.Number != 99 {
			t.Errorf("Number = %d, want 99", pr.Number)
		}
		if pr.State != "MERGED" {
			t.Errorf("State = %q, want MERGED", pr.State)
		}
		if pr.Author != "alice" {
			t.Errorf("Author = %q, want alice", pr.Author)
		}
		if !pr.CachedAt.Equal(now) {
			t.Errorf("CachedAt = %v, want %v", pr.CachedAt, now)
		}
	})
}

func TestSave_WritesChanges(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".wt", "prs.json")

	c := LoadFrom(path)
	c.Set("/repo:main", &forge.PRInfo{Number: 1, State: "OPEN"})

	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file was actually written
	var loaded Cache
	if err := fs.LoadJSON(path, &loaded); err != nil {
		t.Fatalf("failed to load saved cache: %v", err)
	}
	if loaded.PRs["/repo:main"] == nil {
		t.Error("saved cache should contain /repo:main entry")
	}
}

func TestSave_Concurrent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prs.json")

	// Every writer loads before any of them saves, as concurrent wt processes do
	const writers = 20
	caches := make([]*Cache, writers)
	for i := range writers {
		caches[i] = LoadFrom(path)
		caches[i].Set(fmt.Sprintf("/repo:branch-%d", i), &forge.PRInfo{Number: i, Fetched: true})
	}

	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for _, c := range caches {
		wg.Go(func() {
			errs <- c.Save()
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("Save failed: %v", err)
		}
	}

	if got := len(LoadFrom(path).PRs); got != writers {
		t.Errorf("expected %d PRs, got %d", writers, got)
	}
}

func TestSave_KeepsKeysSavedByOthers(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prs.json")

	seed := LoadFrom(path)
	seed.Set("/repo:stale", &forge.PRInfo{Number: 1})
	seed.Set("/repo:kept", &forge.PRInfo{Number: 2})
	if err := seed.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	c := LoadFrom(path)

	other := LoadFrom(path)
	other.Set("/repo:other", &forge.PRInfo{Number: 3})
	if err := other.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	c.Delete("/repo:stale")
	c.Set("/repo:new", &forge.PRInfo{Number: 4})
	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	saved := LoadFrom(path)
	if saved.Get("/repo:stale") != nil {
		t.Error("deleted key should be gone")
	}
	for _, key := range []string{"/repo:kept", "/repo:other", "/repo:new"} {
		if saved.Get(key) == nil {
			t.Errorf("missing %s", key)
		}
	}
}

func TestSave_ResetClearsKeysSavedByOthers(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prs.json")

	c := LoadFrom(path)

	other := LoadFrom(path)
	other.Set("/repo:other", &forge.PRInfo{Number: 1})
	if err := other.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	c.Set("/repo:before-reset", &forge.PRInfo{Number: 2})
	c.Reset()
	c.Set("/repo:after-reset", &forge.PRInfo{Number: 3})
	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	saved := LoadFrom(path)
	if len(saved.PRs) != 1 || saved.Get("/repo:after-reset") == nil {
		t.Errorf("expected only /repo:after-reset, got %v", saved.PRs)
	}
}

func TestSave_ChangesSavedOnce(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prs.json")

	c := LoadFrom(path)
	c.Set("/repo:main", &forge.PRInfo{Number: 1})
	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	other := LoadFrom(path)
	other.Delete("/repo:main")
	if err := other.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// The Set was already saved, a second Save must not bring the key back
	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if LoadFrom(path).Get("/repo:main") != nil {
		t.Error("second Save replayed an already saved change")
	}
}

func TestSave_CorruptFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prs.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("setup: write failed: %v", err)
	}

	c := LoadFrom(path)
	c.Set("/repo:main", &forge.PRInfo{Number: 1})
	if err := c.Save(); err != nil {
		t.Fatalf("Save over a corrupted file failed: %v", err)
	}

	saved := LoadFrom(path)
	if len(saved.PRs) != 1 || saved.Get("/repo:main") == nil {
		t.Errorf("expected only /repo:main, got %v", saved.PRs)
	}
}

func TestSave_FailureKeepsChanges(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "prs.json")

	// A directory at the lock path can't be opened for writing
	if err := os.Mkdir(path+".lock", 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	c := LoadFrom(path)
	c.Set("/repo:main", &forge.PRInfo{Number: 1})
	if err := c.Save(); err == nil {
		t.Fatal("expected error when the lock can't be taken, got nil")
	}

	if err := os.Remove(path + ".lock"); err != nil {
		t.Fatalf("failed to remove directory: %v", err)
	}
	if err := c.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if LoadFrom(path).Get("/repo:main") == nil {
		t.Error("changes from the failed save should be saved by the retry")
	}
}

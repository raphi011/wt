package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordAccess(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	if err := RecordAccess("/path/to/worktree", "myrepo", "main", historyFile); err != nil {
		t.Fatalf("RecordAccess failed: %v", err)
	}

	h, err := Load(historyFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(h.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(h.Entries))
	}
	e := h.Entries[0]
	if e.Path != "/path/to/worktree" {
		t.Errorf("Path = %q, want %q", e.Path, "/path/to/worktree")
	}
	if e.RepoName != "myrepo" {
		t.Errorf("RepoName = %q, want %q", e.RepoName, "myrepo")
	}
	if e.Branch != "main" {
		t.Errorf("Branch = %q, want %q", e.Branch, "main")
	}
	if e.AccessCount != 1 {
		t.Errorf("AccessCount = %d, want 1", e.AccessCount)
	}
	if e.LastAccess.IsZero() {
		t.Error("LastAccess should not be zero")
	}
}

func TestRecordAccess_IncrementExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	if err := RecordAccess("/wt/one", "repo", "feat", historyFile); err != nil {
		t.Fatalf("first RecordAccess failed: %v", err)
	}

	// Small sleep to ensure LastAccess changes
	time.Sleep(10 * time.Millisecond)

	if err := RecordAccess("/wt/one", "repo", "feat", historyFile); err != nil {
		t.Fatalf("second RecordAccess failed: %v", err)
	}

	h, err := Load(historyFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(h.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(h.Entries))
	}
	if h.Entries[0].AccessCount != 2 {
		t.Errorf("AccessCount = %d, want 2", h.Entries[0].AccessCount)
	}
}

func TestRecordAccess_MultipleEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	if err := RecordAccess("/wt/a", "repo-a", "main", historyFile); err != nil {
		t.Fatalf("RecordAccess /wt/a failed: %v", err)
	}
	if err := RecordAccess("/wt/b", "repo-b", "feat", historyFile); err != nil {
		t.Fatalf("RecordAccess /wt/b failed: %v", err)
	}

	h, err := Load(historyFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(h.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(h.Entries))
	}
}

func TestRecordAccess_MaxCap(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	// Create history at the cap
	h := &History{}
	base := time.Now().Add(-time.Hour)
	for i := range maxEntries {
		h.Entries = append(h.Entries, Entry{
			Path:        filepath.Join("/wt", "entry", string(rune('a'+i%26))+filepath.Join("x", string(rune('0'+i/26)))),
			RepoName:    "repo",
			Branch:      "b",
			AccessCount: 1,
			LastAccess:  base.Add(time.Duration(i) * time.Second),
		})
	}
	if err := h.Save(historyFile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Add one more — should evict the oldest
	if err := RecordAccess("/wt/new", "repo", "new", historyFile); err != nil {
		t.Fatalf("RecordAccess failed: %v", err)
	}

	h, err := Load(historyFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(h.Entries) != maxEntries {
		t.Errorf("expected %d entries, got %d", maxEntries, len(h.Entries))
	}

	// The new entry should be present
	found := false
	for _, e := range h.Entries {
		if e.Path == "/wt/new" {
			found = true
			break
		}
	}
	if !found {
		t.Error("new entry not found after cap eviction")
	}
}

func TestGetMostRecent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	// Record two paths
	if err := RecordAccess("/wt/old", "repo", "old", historyFile); err != nil {
		t.Fatalf("RecordAccess failed: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := RecordAccess("/wt/new", "repo", "new", historyFile); err != nil {
		t.Fatalf("RecordAccess failed: %v", err)
	}

	mostRecent, err := GetMostRecent(historyFile)
	if err != nil {
		t.Fatalf("GetMostRecent failed: %v", err)
	}
	if mostRecent != "/wt/new" {
		t.Errorf("expected %q, got %q", "/wt/new", mostRecent)
	}
}

func TestGetMostRecent_NoHistory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "nonexistent.json")

	mostRecent, err := GetMostRecent(historyFile)
	if err != nil {
		t.Fatalf("GetMostRecent failed: %v", err)
	}
	if mostRecent != "" {
		t.Errorf("expected empty string, got %q", mostRecent)
	}
}

func TestRemoveStale(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a real directory to serve as a valid path
	validPath := filepath.Join(tmpDir, "valid-wt")
	if err := os.MkdirAll(validPath, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	h := &History{
		Entries: []Entry{
			{Path: validPath, RepoName: "repo", Branch: "a", AccessCount: 1, LastAccess: time.Now()},
			{Path: "/nonexistent/path", RepoName: "repo", Branch: "b", AccessCount: 1, LastAccess: time.Now()},
		},
	}

	removed := h.RemoveStale()
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if len(h.Entries) != 1 {
		t.Fatalf("expected 1 entry remaining, got %d", len(h.Entries))
	}
	if h.Entries[0].Path != validPath {
		t.Errorf("expected valid path to remain, got %q", h.Entries[0].Path)
	}
}

func TestRemoveByPath(t *testing.T) {
	t.Parallel()

	h := &History{
		Entries: []Entry{
			{Path: "/wt/a", RepoName: "repo", Branch: "a"},
			{Path: "/wt/b", RepoName: "repo", Branch: "b"},
			{Path: "/wt/c", RepoName: "repo", Branch: "c"},
		},
	}

	if !h.RemoveByPath("/wt/b") {
		t.Error("expected RemoveByPath to return true for existing entry")
	}
	if len(h.Entries) != 2 {
		t.Fatalf("expected 2 entries after removal, got %d", len(h.Entries))
	}
	if h.FindByPath("/wt/b") != nil {
		t.Error("removed entry should not be findable")
	}

	if h.RemoveByPath("/wt/nonexistent") {
		t.Error("expected RemoveByPath to return false for nonexistent entry")
	}
}

func TestFindByPath(t *testing.T) {
	t.Parallel()

	h := &History{
		Entries: []Entry{
			{Path: "/wt/a", RepoName: "repo", Branch: "a"},
			{Path: "/wt/b", RepoName: "repo", Branch: "b"},
		},
	}

	entry := h.FindByPath("/wt/b")
	if entry == nil {
		t.Fatal("expected to find entry, got nil")
	}
	if entry.Branch != "b" {
		t.Errorf("Branch = %q, want %q", entry.Branch, "b")
	}

	if h.FindByPath("/wt/nonexistent") != nil {
		t.Error("expected nil for nonexistent path")
	}
}

func TestLoad_OldFormat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	// Write old format: {"most_recent": "/foo"}
	if err := os.WriteFile(historyFile, []byte(`{"most_recent":"/foo"}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	h, err := Load(historyFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// Old format should result in empty entries (Go ignores unknown fields)
	if len(h.Entries) != 0 {
		t.Errorf("expected 0 entries for old format, got %d", len(h.Entries))
	}
}

func TestLoad_NoFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "nonexistent.json")

	h, err := Load(historyFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(h.Entries) != 0 {
		t.Errorf("expected 0 entries for missing file, got %d", len(h.Entries))
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	if err := os.WriteFile(historyFile, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, err := Load(historyFile)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "subdir", "history.json")

	h := &History{
		Entries: []Entry{
			{Path: "/some/path", RepoName: "repo", Branch: "main", AccessCount: 1, LastAccess: time.Now()},
		},
	}
	if err := h.Save(historyFile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Error("expected history file to be created")
	}
}

func TestDefaultPath(t *testing.T) {
	t.Parallel()

	path := DefaultPath()
	if path == "" {
		t.Error("DefaultPath returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("DefaultPath should return absolute path, got %q", path)
	}
	if filepath.Base(path) != "history.json" {
		t.Errorf("expected filename 'history.json', got %q", filepath.Base(path))
	}
}

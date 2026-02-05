package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordAccess(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	// Record a worktree path
	wtPath := "/path/to/worktree"
	if err := RecordAccess(wtPath, historyFile); err != nil {
		t.Fatalf("RecordAccess failed: %v", err)
	}

	// Verify the file was created and contains correct data
	h, err := Load(historyFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if h.MostRecent != wtPath {
		t.Errorf("expected MostRecent %q, got %q", wtPath, h.MostRecent)
	}
}

func TestRecordAccess_Overwrites(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	// Record first path
	if err := RecordAccess("/first/path", historyFile); err != nil {
		t.Fatalf("first RecordAccess failed: %v", err)
	}

	// Record second path - should overwrite
	if err := RecordAccess("/second/path", historyFile); err != nil {
		t.Fatalf("second RecordAccess failed: %v", err)
	}

	// Verify only second path is stored
	h, err := Load(historyFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if h.MostRecent != "/second/path" {
		t.Errorf("expected MostRecent %q, got %q", "/second/path", h.MostRecent)
	}
}

func TestGetMostRecent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	// Record a path
	wtPath := "/my/worktree"
	if err := RecordAccess(wtPath, historyFile); err != nil {
		t.Fatalf("RecordAccess failed: %v", err)
	}

	// Get most recent
	mostRecent, err := GetMostRecent(historyFile)
	if err != nil {
		t.Fatalf("GetMostRecent failed: %v", err)
	}
	if mostRecent != wtPath {
		t.Errorf("expected %q, got %q", wtPath, mostRecent)
	}
}

func TestGetMostRecent_NoHistory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "nonexistent.json")

	// Get most recent when no history exists
	mostRecent, err := GetMostRecent(historyFile)
	if err != nil {
		t.Fatalf("GetMostRecent failed: %v", err)
	}
	if mostRecent != "" {
		t.Errorf("expected empty string for no history, got %q", mostRecent)
	}
}

func TestLoad_NoFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "nonexistent.json")

	// Load should return empty history when file doesn't exist
	h, err := Load(historyFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if h.MostRecent != "" {
		t.Errorf("expected empty MostRecent for missing file, got %q", h.MostRecent)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "history.json")

	// Write invalid JSON
	if err := os.WriteFile(historyFile, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid file: %v", err)
	}

	// Load should return error for invalid JSON
	_, err := Load(historyFile)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "subdir", "history.json")

	h := &History{MostRecent: "/some/path"}
	if err := h.Save(historyFile); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
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

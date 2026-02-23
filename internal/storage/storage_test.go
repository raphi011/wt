package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadJSON_Roundtrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	type Data struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	original := Data{Name: "test", Count: 42}

	if err := SaveJSON(path, original); err != nil {
		t.Fatalf("SaveJSON failed: %v", err)
	}

	var loaded Data
	if err := LoadJSON(path, &loaded); err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}

	if loaded.Name != original.Name || loaded.Count != original.Count {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", loaded, original)
	}
}

func TestLoadJSON_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.json")

	var data map[string]any
	err := LoadJSON(path, &data)
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist error, got %v", err)
	}
}

func TestSaveJSON_CreatesDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Nested path that doesn't exist yet
	path := filepath.Join(tmpDir, "a", "b", "c", "data.json")

	data := map[string]string{"key": "value"}

	if err := SaveJSON(path, data); err != nil {
		t.Fatalf("SaveJSON failed to create directories: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}

	// Verify content
	var loaded map[string]string
	if err := LoadJSON(path, &loaded); err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}
	if loaded["key"] != "value" {
		t.Errorf("expected key=value, got key=%s", loaded["key"])
	}
}

func TestWtDir(t *testing.T) {
	t.Parallel()

	dir, err := WtDir()
	if err != nil {
		t.Fatalf("WtDir() error: %v", err)
	}

	// Should end with .wt
	if filepath.Base(dir) != ".wt" {
		t.Errorf("WtDir() = %q, want base dir .wt", dir)
	}

	// Directory should exist
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("WtDir directory does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("WtDir path is not a directory")
	}
}

func TestLoadJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(path, []byte(`{not valid json}`), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	var data map[string]any
	err := LoadJSON(path, &data)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestSaveJSON_MarshalError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")

	// Channels can't be marshaled to JSON
	err := SaveJSON(path, make(chan int))
	if err == nil {
		t.Fatal("expected error for unmarshalable data, got nil")
	}
}

func TestSaveJSON_Atomic(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "atomic.json")

	// Save initial data
	if err := SaveJSON(path, map[string]int{"v": 1}); err != nil {
		t.Fatalf("SaveJSON failed: %v", err)
	}

	// Overwrite with new data
	if err := SaveJSON(path, map[string]int{"v": 2}); err != nil {
		t.Fatalf("SaveJSON overwrite failed: %v", err)
	}

	// Verify no temp file left behind
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("temp file should not exist after successful save")
	}

	// Verify updated content
	var loaded map[string]int
	if err := LoadJSON(path, &loaded); err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}
	if loaded["v"] != 2 {
		t.Errorf("expected v=2, got v=%d", loaded["v"])
	}
}

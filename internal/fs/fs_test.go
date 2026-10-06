package fs

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
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

func TestResolvePath_Symlink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	realDir := filepath.Join(resolved, "real")
	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	linkDir := filepath.Join(resolved, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	got := ResolvePath(linkDir)
	if got != realDir {
		t.Errorf("ResolvePath(symlink) = %q, want %q", got, realDir)
	}
}

func TestResolvePath_NonExistent(t *testing.T) {
	t.Parallel()

	// Non-existent path should be returned unchanged
	path := "/nonexistent/path/that/does/not/exist"
	got := ResolvePath(path)
	if got != path {
		t.Errorf("ResolvePath(nonexistent) = %q, want %q (unchanged)", got, path)
	}
}

func TestResolvePath_AlreadyCanonical(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	got := ResolvePath(resolved)
	if got != resolved {
		t.Errorf("ResolvePath(canonical) = %q, want %q (unchanged)", got, resolved)
	}
}

func TestSaveJSON_Concurrent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	const writers = 50
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			errs <- SaveJSON(path, map[string]int{"writer": i})
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("SaveJSON failed: %v", err)
		}
	}

	var loaded map[string]int
	if err := LoadJSON(path, &loaded); err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected only test.json, found %d files", len(entries))
	}
}

func TestUpdateJSON_Concurrent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "sub", "counter.json")

	type counter struct {
		N int `json:"n"`
	}

	const writers = 50
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() {
			errs <- UpdateJSON(path, func(c *counter) error {
				c.N++
				return nil
			})
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("UpdateJSON failed: %v", err)
		}
	}

	var got counter
	if err := LoadJSON(path, &got); err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}
	if got.N != writers {
		t.Errorf("expected n=%d, got n=%d", writers, got.N)
	}
}

func TestUpdateJSON_FnError(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.json")
	if err := SaveJSON(path, map[string]int{"v": 1}); err != nil {
		t.Fatalf("SaveJSON failed: %v", err)
	}

	wantErr := errors.New("abort")
	err := UpdateJSON(path, func(m *map[string]int) error {
		(*m)["v"] = 2
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected fn error, got %v", err)
	}

	var loaded map[string]int
	if err := LoadJSON(path, &loaded); err != nil {
		t.Fatalf("LoadJSON failed: %v", err)
	}
	if loaded["v"] != 1 {
		t.Errorf("expected v=1 after failed update, got v=%d", loaded["v"])
	}
}

func TestUpdateJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	called := false
	err := UpdateJSON(path, func(m *map[string]int) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if called {
		t.Error("fn should not run when the file cannot be read")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != "not json" {
		t.Errorf("file was overwritten: %q", data)
	}
}

func TestUpdateJSONLenient_InvalidJSON(t *testing.T) {
	t.Parallel()

	type data struct {
		A int `json:"a"`
		B int `json:"b"`
	}

	tests := []struct {
		name    string
		content string
	}{
		{"syntax error", "not json"},
		{"empty file", ""},
		// "a" decodes before "b" fails
		{"type mismatch", `{"a": 1, "b": "x"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "test.json")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}

			err := UpdateJSONLenient(path, func(d *data) error {
				if *d != (data{}) {
					t.Errorf("fn should receive the zero value, got %+v", *d)
				}
				d.B = 2
				return nil
			})
			if err != nil {
				t.Fatalf("UpdateJSONLenient failed: %v", err)
			}

			var got data
			if err := LoadJSON(path, &got); err != nil {
				t.Fatalf("LoadJSON failed: %v", err)
			}
			if got != (data{B: 2}) {
				t.Errorf("expected {A:0 B:2}, got %+v", got)
			}
		})
	}
}

func TestUpdateJSONLenient_ReadError(t *testing.T) {
	t.Parallel()

	// A directory at the file path can't be read
	path := filepath.Join(t.TempDir(), "test.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	called := false
	err := UpdateJSONLenient(path, func(m *map[string]int) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error when the file can't be read, got nil")
	}
	if called {
		t.Error("fn should not run when the file cannot be read")
	}
}

func TestSaveJSON_RenameError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	// A directory at the target path makes the final rename fail
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	if err := SaveJSON(path, map[string]int{"v": 1}); err == nil {
		t.Fatal("expected error when target is a directory, got nil")
	}

	// Verify no temp file left behind
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected only the target directory, got %d entries", len(entries))
	}
}

func TestUpdateJSON_LockError(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "test.json")

	// A directory at the lock path can't be opened for writing
	if err := os.Mkdir(path+".lock", 0o755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	called := false
	err := UpdateJSON(path, func(m *map[string]int) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error when the lock can't be taken, got nil")
	}
	if called {
		t.Error("fn should not run without the lock")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should not be created without the lock, stat error: %v", err)
	}
}

func TestUpdateJSON_ParentNotDirectory(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "parent")
	if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	called := false
	err := UpdateJSON(filepath.Join(parent, "test.json"), func(m *map[string]int) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error when the parent is a file, got nil")
	}
	if called {
		t.Error("fn should not run when the directory can't be created")
	}
}

package preserve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchesPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		relPath  string
		patterns []string
		exclude  []string
		want     bool
	}{
		{
			name:     "exact basename match",
			relPath:  ".env",
			patterns: []string{".env"},
			want:     true,
		},
		{
			name:     "glob pattern match",
			relPath:  ".env.local",
			patterns: []string{".env.*"},
			want:     true,
		},
		{
			name:     "nested file matches basename",
			relPath:  "config/.env",
			patterns: []string{".env"},
			want:     true,
		},
		{
			name:     "no match",
			relPath:  "main.go",
			patterns: []string{".env", ".envrc"},
			want:     false,
		},
		{
			name:     "excluded path segment",
			relPath:  "node_modules/.env",
			patterns: []string{".env"},
			exclude:  []string{"node_modules"},
			want:     false,
		},
		{
			name:     "deeply nested excluded segment",
			relPath:  "packages/app/node_modules/.cache/.env",
			patterns: []string{".env"},
			exclude:  []string{"node_modules"},
			want:     false,
		},
		{
			name:     "exclude does not match basename",
			relPath:  ".env",
			patterns: []string{".env"},
			exclude:  []string{"vendor"},
			want:     true,
		},
		{
			name:     "multiple patterns first matches",
			relPath:  ".envrc",
			patterns: []string{".env", ".envrc", "docker-compose.override.yml"},
			want:     true,
		},
		{
			name:     "docker-compose override",
			relPath:  "docker-compose.override.yml",
			patterns: []string{"docker-compose.override.yml"},
			want:     true,
		},
		{
			name:     "empty patterns",
			relPath:  ".env",
			patterns: nil,
			want:     false,
		},
		{
			name:     "invalid glob pattern does not match",
			relPath:  ".env",
			patterns: []string{"[invalid"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := matchesPattern(tt.relPath, tt.patterns, tt.exclude)
			if got != tt.want {
				t.Errorf("matchesPattern(%q, %v, %v) = %v, want %v",
					tt.relPath, tt.patterns, tt.exclude, got, tt.want)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	t.Run("copies file with contents", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src", ".env")
		dst := filepath.Join(tmpDir, "dst", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("SECRET=abc123\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		copied, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}
		if !copied {
			t.Error("CopyFile() should return true for new file")
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read dst: %v", err)
		}
		if string(got) != "SECRET=abc123\n" {
			t.Errorf("got %q, want %q", got, "SECRET=abc123\n")
		}
	})

	t.Run("skips existing file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src", ".env")
		dst := filepath.Join(tmpDir, "dst", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("NEW_CONTENT\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}
		if err := os.WriteFile(dst, []byte("EXISTING_CONTENT\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		copied, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile() should not error on existing: %v", err)
		}
		if copied {
			t.Error("CopyFile() should return false for existing file")
		}

		// Existing content should be preserved
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read dst: %v", err)
		}
		if string(got) != "EXISTING_CONTENT\n" {
			t.Errorf("existing file was overwritten: got %q, want %q", got, "EXISTING_CONTENT\n")
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src", "deep", "nested", ".env")
		dst := filepath.Join(tmpDir, "dst", "deep", "nested", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("NESTED=true\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		copied, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}
		if !copied {
			t.Error("CopyFile() should return true for new file")
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read dst: %v", err)
		}
		if string(got) != "NESTED=true\n" {
			t.Errorf("got %q, want %q", got, "NESTED=true\n")
		}
	})

	t.Run("preserves file permissions", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src", "script.sh")
		dst := filepath.Join(tmpDir, "dst", "script.sh")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		if _, err := CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}

		info, err := os.Stat(dst)
		if err != nil {
			t.Fatalf("failed to stat dst: %v", err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("permissions = %o, want %o", info.Mode().Perm(), 0755)
		}
	})

	t.Run("returns error for non-existent source", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "does-not-exist")
		dst := filepath.Join(tmpDir, "dst", "file")

		_, err := CopyFile(src, dst)
		if err == nil {
			t.Fatal("CopyFile() should return error for non-existent source")
		}
	})

	t.Run("skips symlinks", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		// Create a real file and a symlink to it
		realFile := filepath.Join(tmpDir, "real.txt")
		if err := os.WriteFile(realFile, []byte("content\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		src := filepath.Join(tmpDir, "link.txt")
		if err := os.Symlink(realFile, src); err != nil {
			t.Fatalf("setup: symlink failed: %v", err)
		}

		dst := filepath.Join(tmpDir, "dst", "link.txt")

		copied, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}
		if copied {
			t.Error("CopyFile() should return false for symlink")
		}

		// dst should not exist
		if _, err := os.Stat(dst); !os.IsNotExist(err) {
			t.Error("dst should not exist when source is a symlink")
		}
	})
}

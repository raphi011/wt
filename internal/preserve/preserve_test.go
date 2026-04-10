//go:build !integration

package preserve

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
)

func TestLinkFile(t *testing.T) {
	t.Parallel()

	t.Run("creates relative symlink", func(t *testing.T) {
		t.Parallel()
		tmpDir := resolveTempDir(t)

		src := filepath.Join(tmpDir, "repo", ".env")
		dst := filepath.Join(tmpDir, "worktree", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("SECRET=abc123\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		linked, err := LinkFile(src, dst)
		if err != nil {
			t.Fatalf("LinkFile() error = %v", err)
		}
		if !linked {
			t.Error("LinkFile() should return true for new link")
		}

		// Verify it's a symlink
		info, err := os.Lstat(dst)
		if err != nil {
			t.Fatalf("failed to lstat dst: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Error("dst should be a symlink")
		}

		// Verify symlink target is relative
		target, err := os.Readlink(dst)
		if err != nil {
			t.Fatalf("failed to readlink dst: %v", err)
		}
		if filepath.IsAbs(target) {
			t.Errorf("symlink target should be relative, got %q", target)
		}

		// Verify content is accessible through symlink
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read through symlink: %v", err)
		}
		if string(got) != "SECRET=abc123\n" {
			t.Errorf("got %q, want %q", got, "SECRET=abc123\n")
		}
	})

	t.Run("skips when destination exists", func(t *testing.T) {
		t.Parallel()
		tmpDir := resolveTempDir(t)

		src := filepath.Join(tmpDir, "repo", ".env")
		dst := filepath.Join(tmpDir, "worktree", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("NEW\n"), 0644); err != nil {
			t.Fatalf("setup: write src failed: %v", err)
		}
		if err := os.WriteFile(dst, []byte("EXISTING\n"), 0644); err != nil {
			t.Fatalf("setup: write dst failed: %v", err)
		}

		linked, err := LinkFile(src, dst)
		if !errors.Is(err, ErrDestExists) {
			t.Fatalf("LinkFile() error = %v, want ErrDestExists", err)
		}
		if linked {
			t.Error("LinkFile() should return false when destination exists")
		}

		// Existing content should be preserved
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read dst: %v", err)
		}
		if string(got) != "EXISTING\n" {
			t.Errorf("existing file was modified: got %q, want %q", got, "EXISTING\n")
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		t.Parallel()
		tmpDir := resolveTempDir(t)

		src := filepath.Join(tmpDir, "repo", "config", "deep", ".env")
		dst := filepath.Join(tmpDir, "worktree", "config", "deep", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("NESTED=true\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		linked, err := LinkFile(src, dst)
		if err != nil {
			t.Fatalf("LinkFile() error = %v", err)
		}
		if !linked {
			t.Error("LinkFile() should return true for new link")
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read through symlink: %v", err)
		}
		if string(got) != "NESTED=true\n" {
			t.Errorf("got %q, want %q", got, "NESTED=true\n")
		}
	})

	t.Run("returns false when source does not exist", func(t *testing.T) {
		t.Parallel()
		tmpDir := resolveTempDir(t)

		src := filepath.Join(tmpDir, "does-not-exist")
		dst := filepath.Join(tmpDir, "dst", "file")

		linked, err := LinkFile(src, dst)
		if err != nil {
			t.Fatalf("LinkFile() should not error for missing source: %v", err)
		}
		if linked {
			t.Error("LinkFile() should return false for missing source")
		}
	})

	t.Run("edits propagate through symlink", func(t *testing.T) {
		t.Parallel()
		tmpDir := resolveTempDir(t)

		src := filepath.Join(tmpDir, "repo", ".env")
		dst := filepath.Join(tmpDir, "worktree", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("ORIGINAL\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		if _, err := LinkFile(src, dst); err != nil {
			t.Fatalf("LinkFile() error = %v", err)
		}

		// Edit via symlink
		if err := os.WriteFile(dst, []byte("MODIFIED\n"), 0644); err != nil {
			t.Fatalf("failed to write through symlink: %v", err)
		}

		// Source should reflect the change
		got, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("failed to read source: %v", err)
		}
		if string(got) != "MODIFIED\n" {
			t.Errorf("source not updated: got %q, want %q", got, "MODIFIED\n")
		}
	})
}

func TestPreserveFiles(t *testing.T) {
	t.Parallel()

	t.Run("symlinks listed paths", func(t *testing.T) {
		t.Parallel()
		tmpDir := resolveTempDir(t)
		ctx := testContext()

		sourceDir := filepath.Join(tmpDir, "repo")
		targetDir := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}

		// Create source files
		if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("SECRET=abc\n"), 0644); err != nil {
			t.Fatalf("setup: write .env failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourceDir, ".envrc"), []byte("dotenv\n"), 0644); err != nil {
			t.Fatalf("setup: write .envrc failed: %v", err)
		}

		cfg := config.PreserveConfig{
			Paths: []string{".env", ".envrc"},
		}

		linked, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error = %v", err)
		}
		if len(linked) != 2 {
			t.Errorf("PreserveFiles() linked = %v, want 2 files", linked)
		}

		// Verify symlinks were created
		for _, path := range []string{".env", ".envrc"} {
			dst := filepath.Join(targetDir, path)
			info, err := os.Lstat(dst)
			if err != nil {
				t.Errorf("symlink %s should exist: %v", path, err)
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("%s should be a symlink", path)
			}
		}
	})

	t.Run("skips missing source files silently", func(t *testing.T) {
		t.Parallel()
		tmpDir := resolveTempDir(t)
		ctx := testContext()

		sourceDir := filepath.Join(tmpDir, "repo")
		targetDir := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}

		cfg := config.PreserveConfig{
			Paths: []string{".env", ".envrc"}, // neither exists
		}

		linked, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error = %v", err)
		}
		if len(linked) != 0 {
			t.Errorf("PreserveFiles() linked = %v, want empty", linked)
		}
	})

	t.Run("skips existing destination with warning", func(t *testing.T) {
		t.Parallel()
		tmpDir := resolveTempDir(t)
		ctx := testContext()

		sourceDir := filepath.Join(tmpDir, "repo")
		targetDir := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}

		if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("SOURCE\n"), 0644); err != nil {
			t.Fatalf("setup: write source failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, ".env"), []byte("EXISTING\n"), 0644); err != nil {
			t.Fatalf("setup: write target failed: %v", err)
		}

		cfg := config.PreserveConfig{
			Paths: []string{".env"},
		}

		linked, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error = %v", err)
		}
		if len(linked) != 0 {
			t.Errorf("PreserveFiles() linked = %v, want empty", linked)
		}

		// Existing content should be untouched
		got, err := os.ReadFile(filepath.Join(targetDir, ".env"))
		if err != nil {
			t.Fatalf("failed to read target: %v", err)
		}
		if string(got) != "EXISTING\n" {
			t.Errorf("existing file was modified: got %q, want %q", got, "EXISTING\n")
		}
	})

	t.Run("symlinks nested paths", func(t *testing.T) {
		t.Parallel()
		tmpDir := resolveTempDir(t)
		ctx := testContext()

		sourceDir := filepath.Join(tmpDir, "repo")
		targetDir := filepath.Join(tmpDir, "worktree")

		// Create nested source
		nestedDir := filepath.Join(sourceDir, "config")
		if err := os.MkdirAll(nestedDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(nestedDir, ".env"), []byte("NESTED=true\n"), 0644); err != nil {
			t.Fatalf("setup: write failed: %v", err)
		}

		cfg := config.PreserveConfig{
			Paths: []string{"config/.env"},
		}

		linked, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error = %v", err)
		}
		if len(linked) != 1 || linked[0] != "config/.env" {
			t.Errorf("PreserveFiles() linked = %v, want [config/.env]", linked)
		}

		got, err := os.ReadFile(filepath.Join(targetDir, "config", ".env"))
		if err != nil {
			t.Fatalf("failed to read through symlink: %v", err)
		}
		if string(got) != "NESTED=true\n" {
			t.Errorf("got %q, want %q", got, "NESTED=true\n")
		}
	})
}

func resolveTempDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for %s: %v", tmpDir, err)
	}
	return resolved
}

func testContext() context.Context {
	l := log.New(io.Discard, false, true)
	return log.WithLogger(context.Background(), l)
}

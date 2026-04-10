//go:build integration

package preserve

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
)

// resolvePath resolves symlinks (needed on macOS where /var -> /private/var).
func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("failed to resolve path %s: %v", path, err)
	}
	return resolved
}

// testContext returns a context with a discarded logger and stderr output writer.
func testContext() context.Context {
	ctx := context.Background()
	ctx = log.WithLogger(ctx, log.New(io.Discard, false, true))
	ctx = output.WithPrinter(ctx, os.Stderr)
	return ctx
}

func TestPreserveFiles(t *testing.T) {
	t.Parallel()

	t.Run("symlinks listed files from source to target", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		ctx := testContext()

		sourceDir := filepath.Join(tmpDir, "repo")
		targetDir := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			t.Fatalf("mkdir source: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}

		// Create source files
		if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("DB_HOST=localhost\n"), 0o644); err != nil {
			t.Fatalf("write .env: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sourceDir, ".envrc"), []byte("dotenv\n"), 0o644); err != nil {
			t.Fatalf("write .envrc: %v", err)
		}

		cfg := config.PreserveConfig{
			Paths: []string{".env", ".envrc"},
		}

		linked, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error: %v", err)
		}
		if len(linked) != 2 {
			t.Fatalf("expected 2 linked files, got %d: %v", len(linked), linked)
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

		// Verify content is accessible
		data, err := os.ReadFile(filepath.Join(targetDir, ".env"))
		if err != nil {
			t.Fatalf("read .env through symlink: %v", err)
		}
		if string(data) != "DB_HOST=localhost\n" {
			t.Errorf(".env content = %q, want %q", data, "DB_HOST=localhost\n")
		}
	})

	t.Run("skips missing source files silently", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		ctx := testContext()

		sourceDir := filepath.Join(tmpDir, "repo")
		targetDir := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			t.Fatalf("mkdir source: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}

		cfg := config.PreserveConfig{
			Paths: []string{".env"}, // doesn't exist in source
		}

		linked, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error: %v", err)
		}
		if len(linked) != 0 {
			t.Errorf("expected 0 linked files, got %d: %v", len(linked), linked)
		}
	})

	t.Run("skips files that already exist in target", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		ctx := testContext()

		sourceDir := filepath.Join(tmpDir, "repo")
		targetDir := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			t.Fatalf("mkdir source: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}

		// Create source file
		if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("SOURCE=1\n"), 0o644); err != nil {
			t.Fatalf("write source .env: %v", err)
		}

		// Create existing file in target with different content
		if err := os.WriteFile(filepath.Join(targetDir, ".env"), []byte("TARGET=1\n"), 0o644); err != nil {
			t.Fatalf("write target .env: %v", err)
		}

		cfg := config.PreserveConfig{
			Paths: []string{".env"},
		}

		linked, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error: %v", err)
		}

		if len(linked) != 0 {
			t.Errorf("expected 0 linked files (existing), got %d: %v", len(linked), linked)
		}

		// Target content should be unchanged
		data, err := os.ReadFile(filepath.Join(targetDir, ".env"))
		if err != nil {
			t.Fatalf("read target .env: %v", err)
		}
		if string(data) != "TARGET=1\n" {
			t.Errorf("target .env was overwritten: got %q", data)
		}
	})

	t.Run("edits propagate through symlink", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		ctx := testContext()

		sourceDir := filepath.Join(tmpDir, "repo")
		targetDir := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			t.Fatalf("mkdir source: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}

		if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("ORIGINAL\n"), 0o644); err != nil {
			t.Fatalf("write .env: %v", err)
		}

		cfg := config.PreserveConfig{
			Paths: []string{".env"},
		}

		if _, err := PreserveFiles(ctx, cfg, sourceDir, targetDir); err != nil {
			t.Fatalf("PreserveFiles() error: %v", err)
		}

		// Edit via symlink in target
		if err := os.WriteFile(filepath.Join(targetDir, ".env"), []byte("MODIFIED\n"), 0o644); err != nil {
			t.Fatalf("write through symlink: %v", err)
		}

		// Source should reflect the change
		data, err := os.ReadFile(filepath.Join(sourceDir, ".env"))
		if err != nil {
			t.Fatalf("read source .env: %v", err)
		}
		if string(data) != "MODIFIED\n" {
			t.Errorf("edit did not propagate: source = %q, want %q", data, "MODIFIED\n")
		}
	})
}

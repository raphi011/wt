//go:build darwin

package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePath_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve tmpDir: %v", err)
	}

	// Create a directory with known casing
	trueCase := filepath.Join(resolved, "TrueCase")
	if err := os.Mkdir(trueCase, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Access with wrong casing — macOS allows this on case-insensitive APFS
	wrongCase := filepath.Join(resolved, "truecase")

	got := ResolvePath(wrongCase)
	if got != trueCase {
		t.Errorf("ResolvePath(%q) = %q, want %q", wrongCase, got, trueCase)
	}
}

func TestResolvePath_SymlinkAndCase(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve tmpDir: %v", err)
	}

	// Create real dir and symlink
	realDir := filepath.Join(resolved, "RealDir")
	if err := os.Mkdir(realDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	linkDir := filepath.Join(resolved, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	// Create a nested dir with known casing
	nested := filepath.Join(realDir, "NestedDir")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Access via symlink + wrong case
	wrongCase := filepath.Join(linkDir, "nesteddir")

	got := ResolvePath(wrongCase)
	want := filepath.Join(realDir, "NestedDir")
	if got != want {
		t.Errorf("ResolvePath(%q) = %q, want %q", wrongCase, got, want)
	}
}

func TestResolvePath_DeepCaseNormalization(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve tmpDir: %v", err)
	}

	// Create a/B/c with specific casing
	dir := filepath.Join(resolved, "Alpha", "BETA", "gamma")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdirall failed: %v", err)
	}

	// Access with all-lowercase
	wrongCase := filepath.Join(resolved, "alpha", "beta", "gamma")

	got := ResolvePath(wrongCase)
	want := filepath.Join(resolved, "Alpha", "BETA", "gamma")
	if got != want {
		t.Errorf("ResolvePath(%q) = %q, want %q", wrongCase, got, want)
	}
}

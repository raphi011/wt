//go:build darwin

package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindByPath_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	// Create directory with specific casing
	repoPath := filepath.Join(resolved, "MyRepo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	reg := &Registry{
		Repos: []Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}

	// Look up via wrong casing — should find the repo on macOS
	wrongCase := filepath.Join(resolved, "myrepo")
	repo, err := reg.FindByPath(wrongCase)
	if err != nil {
		t.Fatalf("FindByPath(wrong case) failed: %v", err)
	}
	if repo.Name != "myrepo" {
		t.Errorf("expected myrepo, got %s", repo.Name)
	}
}

func TestAdd_CaseInsensitiveDuplicate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	// Create directory with specific casing
	repoPath := filepath.Join(resolved, "MyRepo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	reg := &Registry{Repos: []Repo{}}

	// Register at canonical path
	if err := reg.Add(Repo{Name: "myrepo", Path: repoPath}); err != nil {
		t.Fatalf("Add(canonical) failed: %v", err)
	}

	// Try to add again via wrong-cased path — should detect duplicate
	wrongCase := filepath.Join(resolved, "myrepo")
	err = reg.Add(Repo{Name: "myrepo-alias", Path: wrongCase})
	if err == nil {
		t.Error("expected error adding duplicate via wrong-cased path")
	}
}

func TestAdd_StoresCanonicalCase(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	// Create directory with specific casing
	repoPath := filepath.Join(resolved, "MyRepo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	reg := &Registry{Repos: []Repo{}}

	// Add via wrong-cased path
	wrongCase := filepath.Join(resolved, "myrepo")
	if err := reg.Add(Repo{Name: "myrepo", Path: wrongCase}); err != nil {
		t.Fatalf("Add(wrong case) failed: %v", err)
	}

	// Stored path should be the canonical (true-cased) path
	if reg.Repos[0].Path != repoPath {
		t.Errorf("stored path = %q, want canonical %q", reg.Repos[0].Path, repoPath)
	}
}

func TestRemove_CaseInsensitive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks: %v", err)
	}

	// Create directory with specific casing
	repoPath := filepath.Join(resolved, "MyRepo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	reg := &Registry{
		Repos: []Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}

	// Remove via wrong-cased path — should find and remove
	wrongCase := filepath.Join(resolved, "myrepo")
	if err := reg.Remove(wrongCase); err != nil {
		t.Fatalf("Remove(wrong case) failed: %v", err)
	}
	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos after remove, got %d", len(reg.Repos))
	}
}

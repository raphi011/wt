//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/registry"
)

// TestRemove_UnregisterRepo tests unregistering a repo.
//
// Scenario: User runs `wt remove testrepo`
// Expected: Repo is removed from registry
func TestRemove_UnregisterRepo(t *testing.T) {
	// Don't run in parallel - modifies HOME env var

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "remove-test")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	// Set HOME BEFORE saving registry
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Add the repo via registry
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "remove-test", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Now remove it
	ctx := testContext(t)
	cmd := newRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"remove-test"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("remove command failed: %v", err)
	}

	// Verify repo was removed
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(reg.Repos))
	}

	// Files should still exist
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Error("repo files should not be deleted without --delete flag")
	}
}

// TestRemove_NonExistent tests removing a non-existent repo.
//
// Scenario: User runs `wt remove nonexistent`
// Expected: Command fails with error
func TestRemove_NonExistent(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newRemoveCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-existent repo")
	}
}

//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestMigrate_ImportRepos tests importing repos from a directory.
//
// Scenario: User runs `wt migrate ~/Git`
// Expected: All git repos in directory are registered
func TestMigrate_ImportRepos(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create directory with repos
	reposDir := filepath.Join(tmpDir, "Git")
	os.MkdirAll(reposDir, 0755)

	// Create two repos
	setupTestRepo(t, reposDir, "repo1")
	setupTestRepo(t, reposDir, "repo2")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Start with empty registry
	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{reposDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(reg.Repos))
	}

	// Verify both repos were imported
	names := make(map[string]bool)
	for _, r := range reg.Repos {
		names[r.Name] = true
	}

	if !names["repo1"] {
		t.Error("repo1 should be imported")
	}
	if !names["repo2"] {
		t.Error("repo2 should be imported")
	}
}

// TestMigrate_SkipsExisting tests that migrate skips already registered repos.
//
// Scenario: User runs `wt migrate ~/Git` with some repos already registered
// Expected: Only new repos are imported
func TestMigrate_SkipsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	reposDir := filepath.Join(tmpDir, "Git")
	os.MkdirAll(reposDir, 0755)

	repo1Path := setupTestRepo(t, reposDir, "repo1")
	setupTestRepo(t, reposDir, "repo2")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Start with repo1 already registered
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{reposDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(reg.Repos))
	}
}

// TestMigrate_DryRun tests dry-run mode.
//
// Scenario: User runs `wt migrate ~/Git -d`
// Expected: Shows what would be imported without importing
func TestMigrate_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	reposDir := filepath.Join(tmpDir, "Git")
	os.MkdirAll(reposDir, 0755)

	setupTestRepo(t, reposDir, "repo1")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Start with empty registry
	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{reposDir, "-d"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate command failed: %v", err)
	}

	// Reload registry - should still be empty
	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 0 {
		t.Errorf("expected 0 repos in dry-run, got %d", len(reg.Repos))
	}
}

// TestMigrate_NonExistentDir tests migrating from non-existent directory.
//
// Scenario: User runs `wt migrate /nonexistent`
// Expected: Command fails with error
func TestMigrate_NonExistentDir(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldCfg := cfg
	cfg = &config.Config{}
	defer func() { cfg = oldCfg }()

	ctx := testContext(t)
	cmd := newMigrateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"/nonexistent/path"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-existent directory")
	}
}

//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestFindOrRegisterCurrentRepo_RegisteredMeanwhile tests auto-registration
// when another process registered the repo after the registry was loaded.
//
// Scenario: Registry is loaded, another process registers the current repo,
// then the command auto-registers it
// Expected: The existing entry is returned and not duplicated
func TestFindOrRegisterCurrentRepo_RegisteredMeanwhile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	otherPath := setupTestRepo(t, tmpDir, "other")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	// Snapshot taken before the other process wrote
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	other := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"theirs"}},
			{Name: "other", Path: otherPath},
		},
	}
	if err := saveRegistry(other, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	repo, err := findOrRegisterCurrentRepo(ctx, reg, cfg)
	if err != nil {
		t.Fatalf("findOrRegisterCurrentRepo failed: %v", err)
	}
	if !repo.HasLabel("theirs") {
		t.Errorf("expected the entry registered by the other process, got %+v", repo)
	}

	// The snapshot is refreshed with the saved state
	if _, err := reg.FindByName("other"); err != nil {
		t.Errorf("expected snapshot to include repos added meanwhile: %v", err)
	}

	loaded, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(loaded.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(loaded.Repos))
	}
}

// TestFindOrRegisterCurrentRepo_AutoRegisters tests auto-registration of an
// unregistered repo.
//
// Scenario: Command runs inside a git repo that is not in the registry
// Expected: The repo is registered with the default labels and saved
func TestFindOrRegisterCurrentRepo_AutoRegisters(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile, DefaultLabels: []string{"auto"}}
	ctx := testContextWithConfig(t, cfg, repoPath)

	repo, err := findOrRegisterCurrentRepo(ctx, reg, cfg)
	if err != nil {
		t.Fatalf("findOrRegisterCurrentRepo failed: %v", err)
	}
	if repo.Path != repoPath || !repo.HasLabel("auto") {
		t.Errorf("unexpected repo: %+v", repo)
	}

	loaded, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if _, err := loaded.FindByPath(repoPath); err != nil {
		t.Errorf("repo was not saved: %v", err)
	}
}

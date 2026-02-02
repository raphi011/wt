//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestPrCheckout_InvalidPRNumber tests error when first arg is not a valid PR number.
//
// Scenario: User runs `wt pr checkout notanumber`
// Expected: Returns error about invalid PR number
func TestPrCheckout_InvalidPRNumber(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"notanumber"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid PR number, got nil")
	}
	if !strings.Contains(err.Error(), "invalid PR number") {
		t.Errorf("expected error about invalid PR number, got %q", err.Error())
	}
}

// TestPrCheckout_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt pr checkout nonexistent 123`
// Expected: Returns error about repo not found in registry
func TestPrCheckout_RepoNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Work from a non-repo directory
	otherDir := filepath.Join(tmpDir, "other")
	os.MkdirAll(otherDir, 0755)

	ctx := testContextWithConfig(t, cfg, otherDir)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found in registry") {
		t.Errorf("expected error about repo not found in registry, got %q", err.Error())
	}
}

// TestPrCheckout_InvalidPRNumberWithRepo tests error when second arg is not a valid PR number.
//
// Scenario: User runs `wt pr checkout myrepo notanumber`
// Expected: Returns error about invalid PR number
func TestPrCheckout_InvalidPRNumberWithRepo(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo", "notanumber"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid PR number, got nil")
	}
	if !strings.Contains(err.Error(), "invalid PR number") {
		t.Errorf("expected error about invalid PR number, got %q", err.Error())
	}
}

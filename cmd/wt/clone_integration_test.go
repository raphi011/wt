//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestRepoClone_BareRepo tests cloning a repository as bare (default behavior).
//
// Scenario: User runs `wt repo clone file:///path/to/repo`
// Expected: Bare repo is cloned into .git directory and registered in registry
func TestRepoClone_BareRepo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a source repo to clone from
	sourceRepo := setupTestRepo(t, tmpDir, "source-repo")

	// Setup registry file
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newRepoCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"file://" + sourceRepo, "cloned-repo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("clone command failed: %v", err)
	}

	// Verify repo was cloned
	clonedPath := filepath.Join(tmpDir, "cloned-repo")
	if _, err := os.Stat(clonedPath); os.IsNotExist(err) {
		t.Error("cloned repo directory should exist")
	}

	// Verify .git directory exists
	gitDir := filepath.Join(clonedPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("cloned repo should have .git directory")
	}

	// Verify it's a bare repo inside .git (has HEAD file directly in .git)
	if _, err := os.Stat(filepath.Join(gitDir, "HEAD")); os.IsNotExist(err) {
		t.Error(".git should contain bare repo with HEAD file")
	}

	// Verify repo was registered
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(reg.Repos))
	}

	if reg.Repos[0].Name != "cloned-repo" {
		t.Errorf("expected name 'cloned-repo', got %q", reg.Repos[0].Name)
	}
}

// TestRepoClone_WithLabels tests cloning with labels.
//
// Scenario: User runs `wt repo clone file:///repo -l backend -l api`
// Expected: Repo is cloned and registered with labels
func TestRepoClone_WithLabels(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	sourceRepo := setupTestRepo(t, tmpDir, "source-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newRepoCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"file://" + sourceRepo, "labeled-repo", "-l", "backend", "-l", "api"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("clone command failed: %v", err)
	}

	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if len(reg.Repos[0].Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(reg.Repos[0].Labels))
	}

	hasBackend := false
	hasAPI := false
	for _, l := range reg.Repos[0].Labels {
		if l == "backend" {
			hasBackend = true
		}
		if l == "api" {
			hasAPI = true
		}
	}

	if !hasBackend || !hasAPI {
		t.Errorf("expected labels [backend, api], got %v", reg.Repos[0].Labels)
	}
}

// TestRepoClone_WithCustomName tests cloning with a custom display name.
//
// Scenario: User runs `wt repo clone file:///repo --name my-app`
// Expected: Repo is cloned and registered with custom name
func TestRepoClone_WithCustomName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	sourceRepo := setupTestRepo(t, tmpDir, "source-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newRepoCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"file://" + sourceRepo, "actual-dir", "--name", "my-app"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("clone command failed: %v", err)
	}

	// Directory name should be actual-dir
	if _, err := os.Stat(filepath.Join(tmpDir, "actual-dir")); os.IsNotExist(err) {
		t.Error("directory should be named 'actual-dir'")
	}

	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	// Registry name should be the custom name
	if reg.Repos[0].Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", reg.Repos[0].Name)
	}
}

// TestRepoClone_DestinationExists tests that cloning to an existing path fails.
//
// Scenario: User runs `wt repo clone file:///repo existing-dir`
// Expected: Command fails with error
func TestRepoClone_DestinationExists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	sourceRepo := setupTestRepo(t, tmpDir, "source-repo")

	// Create destination directory
	existingDir := filepath.Join(tmpDir, "existing-dir")
	os.MkdirAll(existingDir, 0755)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newRepoCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"file://" + sourceRepo, "existing-dir"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when destination exists")
	}
}

// TestRepoClone_AutoName tests cloning without destination extracts name from URL.
//
// Scenario: User runs `wt repo clone file:///path/to/myrepo`
// Expected: Clones to ./myrepo
func TestRepoClone_AutoName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create source repo with specific name
	sourceRepo := setupTestRepo(t, tmpDir, "my-project")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	// Create a work subdirectory for cloning
	otherDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(otherDir, 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, otherDir)

	cmd := newRepoCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"file://" + sourceRepo})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("clone command failed: %v", err)
	}

	// Should clone to ./my-project
	if _, err := os.Stat(filepath.Join(otherDir, "my-project")); os.IsNotExist(err) {
		t.Error("expected repo to be cloned to 'my-project'")
	}

	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if reg.Repos[0].Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", reg.Repos[0].Name)
	}
}

// TestRepoClone_ShortFormWithoutDefaultOrg tests that short-form without org fails.
//
// Scenario: User runs `wt repo clone myrepo` without default_org configured
// Expected: Command fails with error about missing org
func TestRepoClone_ShortFormWithoutDefaultOrg(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	// Config without default_org
	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newRepoCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo"}) // short-form without org/

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for short-form without default_org")
	}

	expectedMsg := "no organization specified and forge.default_org not configured"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestRepoClone_ShortFormAutoExtractRepoName tests that short-form extracts repo name.
//
// Scenario: User runs `wt repo clone org/repo`
// Expected: Destination is named "repo"
func TestRepoClone_ShortFormAutoExtractRepoName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newRepoCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"org/myrepo"})

	// This will fail because gh CLI won't be properly authenticated in test,
	// but we can still verify the input parsing by checking the destination check
	// happens before the clone (destination would be "myrepo")
	err := cmd.Execute()

	// The error should be from gh CLI, not about "destination already exists"
	// which would indicate we're correctly parsing "org/myrepo" -> "myrepo"
	if err == nil {
		t.Skip("gh CLI available and authenticated - skipping error path test")
	}

	// Verify it's a forge/clone error, not a dest parsing error
	if err.Error() == "destination already exists: "+filepath.Join(tmpDir, "myrepo") {
		t.Error("unexpected destination exists error - indicates parsing issue")
	}
}

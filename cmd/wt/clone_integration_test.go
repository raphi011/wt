//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/registry"
)

// TestClone_BareRepo tests cloning a repository as bare (default behavior).
//
// Scenario: User runs `wt clone file:///path/to/repo`
// Expected: Bare repo is cloned into .git directory and registered in registry
func TestClone_BareRepo(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a source repo to clone from
	sourceRepo := setupTestRepo(t, tmpDir, "source-repo")

	// Setup registry directory
	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCloneCmd()
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
	reg, err := registry.Load()
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

// TestClone_WithLabels tests cloning with labels.
//
// Scenario: User runs `wt clone file:///repo -l backend -l api`
// Expected: Repo is cloned and registered with labels
func TestClone_WithLabels(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	sourceRepo := setupTestRepo(t, tmpDir, "source-repo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"file://" + sourceRepo, "labeled-repo", "-l", "backend", "-l", "api"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("clone command failed: %v", err)
	}

	reg, err := registry.Load()
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

// TestClone_WithCustomName tests cloning with a custom display name.
//
// Scenario: User runs `wt clone file:///repo --name my-app`
// Expected: Repo is cloned and registered with custom name
func TestClone_WithCustomName(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	sourceRepo := setupTestRepo(t, tmpDir, "source-repo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"file://" + sourceRepo, "actual-dir", "--name", "my-app"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("clone command failed: %v", err)
	}

	// Directory name should be actual-dir
	if _, err := os.Stat(filepath.Join(tmpDir, "actual-dir")); os.IsNotExist(err) {
		t.Error("directory should be named 'actual-dir'")
	}

	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	// Registry name should be the custom name
	if reg.Repos[0].Name != "my-app" {
		t.Errorf("expected name 'my-app', got %q", reg.Repos[0].Name)
	}
}

// TestClone_DestinationExists tests that cloning to an existing path fails.
//
// Scenario: User runs `wt clone file:///repo existing-dir`
// Expected: Command fails with error
func TestClone_DestinationExists(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	sourceRepo := setupTestRepo(t, tmpDir, "source-repo")

	// Create destination directory
	existingDir := filepath.Join(tmpDir, "existing-dir")
	os.MkdirAll(existingDir, 0755)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"file://" + sourceRepo, "existing-dir"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error when destination exists")
	}
}

// TestClone_AutoName tests cloning without destination extracts name from URL.
//
// Scenario: User runs `wt clone file:///path/to/myrepo`
// Expected: Clones to ./myrepo
func TestClone_AutoName(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create source repo with specific name
	sourceRepo := setupTestRepo(t, tmpDir, "my-project")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	// Create a work subdirectory for cloning
	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldDir, _ := os.Getwd()
	os.Chdir(workDir)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newCloneCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"file://" + sourceRepo})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("clone command failed: %v", err)
	}

	// Should clone to ./my-project
	if _, err := os.Stat(filepath.Join(workDir, "my-project")); os.IsNotExist(err) {
		t.Error("expected repo to be cloned to 'my-project'")
	}

	reg, err := registry.Load()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	if reg.Repos[0].Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", reg.Repos[0].Name)
	}
}

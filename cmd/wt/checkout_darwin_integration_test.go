//go:build integration && darwin

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestCheckout_NewBranchViaCaseInsensitivePath tests that checkout works
// when the workDir has different casing than the registered repo path.
//
// Scenario: Repo registered at /tmp/.../MyRepo, user cds into /tmp/.../myrepo
// Expected: Checkout detects the repo via case-normalized path matching
func TestCheckout_NewBranchViaCaseInsensitivePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a parent dir with known casing, then create the repo inside it.
	// We use the parent dir casing trick because setupTestRepo calls
	// resolvePath which would normalize the repo path itself.
	parentDir := filepath.Join(tmpDir, "Projects")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}

	repoPath := setupTestRepo(t, parentDir, "myrepo")

	// Register repo at canonical path (true casing: .../Projects/myrepo)
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
	}

	// Use wrong-cased parent dir in workDir — this is the key:
	// "projects" instead of "Projects"
	wrongCasedWorkDir := filepath.Join(tmpDir, "projects", "myrepo")

	ctx := testContextWithConfig(t, cfg, wrongCasedWorkDir)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "case-test-branch"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout -b via wrong-cased path failed: %v", err)
	}

	// Verify worktree was created (at canonical path's sibling)
	wtPath := filepath.Join(parentDir, "myrepo-case-test-branch")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Errorf("worktree should exist at %s", wtPath)
	}

	// Verify it's on the correct branch
	branch := getGitBranch(t, wtPath)
	if branch != "case-test-branch" {
		t.Errorf("expected branch 'case-test-branch', got %q", branch)
	}
}

// TestRepoAdd_CaseInsensitiveDuplicate tests that `wt repo add` detects
// a duplicate when the path differs only in casing.
//
// Scenario: Repo registered at /tmp/.../MyRepo, user tries to add /tmp/.../myrepo
// Expected: Error about duplicate repo
func TestRepoAdd_CaseInsensitiveDuplicate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create a directory with known casing
	repoDir := filepath.Join(tmpDir, "CasedDir")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create cased dir: %v", err)
	}

	repoPath := setupTestRepo(t, repoDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// First: add the repo at its canonical path
	ctx := testContextWithConfig(t, cfg, tmpDir)
	cmd := newRepoAddCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{repoPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("first repo add failed: %v", err)
	}

	// Second: try to add via wrong-cased parent dir
	wrongCasedPath := filepath.Join(tmpDir, "caseddir", "myrepo")

	ctx2 := testContextWithConfig(t, cfg, tmpDir)
	cmd2 := newRepoAddCmd()
	cmd2.SetContext(ctx2)
	cmd2.SetArgs([]string{wrongCasedPath})

	err := cmd2.Execute()
	if err == nil {
		// Load registry to check if duplicate was added
		reg, loadErr := registry.Load(regFile)
		if loadErr != nil {
			t.Fatalf("failed to load registry: %v", loadErr)
		}
		if len(reg.Repos) > 1 {
			t.Errorf("expected 1 repo (duplicate should be detected), got %d", len(reg.Repos))
		}
	}
	// err != nil is acceptable — means duplicate was properly detected
}

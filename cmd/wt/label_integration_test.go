//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestLabel_Add tests adding a label to a repo.
//
// Scenario: User runs `wt label add backend myrepo`
// Expected: Label is added to the repo
func TestLabel_Add(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"add", "backend", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label add command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, err := reg.FindByName("myrepo")
	if err != nil {
		t.Fatalf("failed to find repo: %v", err)
	}

	hasLabel := false
	for _, l := range repo.Labels {
		if l == "backend" {
			hasLabel = true
			break
		}
	}

	if !hasLabel {
		t.Errorf("expected repo to have label 'backend', got %v", repo.Labels)
	}
}

// TestLabel_Remove tests removing a label from a repo.
//
// Scenario: User runs `wt label remove backend myrepo`
// Expected: Label is removed from the repo
func TestLabel_Remove(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend", "api"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"remove", "backend", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label remove command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, err := reg.FindByName("myrepo")
	if err != nil {
		t.Fatalf("failed to find repo: %v", err)
	}

	for _, l := range repo.Labels {
		if l == "backend" {
			t.Errorf("expected label 'backend' to be removed, but found it in %v", repo.Labels)
		}
	}

	// Should still have api label
	hasAPI := false
	for _, l := range repo.Labels {
		if l == "api" {
			hasAPI = true
		}
	}
	if !hasAPI {
		t.Errorf("expected label 'api' to remain, got %v", repo.Labels)
	}
}

// TestLabel_List tests listing labels for a repo.
//
// Scenario: User runs `wt label list myrepo`
// Expected: Labels are listed
func TestLabel_List(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend", "api"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithOutput(t)
	ctx = config.WithConfig(ctx, cfg)
	ctx = config.WithWorkDir(ctx, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"list", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label list command failed: %v", err)
	}

	output := out.String()
	if output == "" {
		t.Error("expected some output")
	}
}

// TestLabel_Clear tests clearing all labels from a repo.
//
// Scenario: User runs `wt label clear myrepo`
// Expected: All labels are removed
func TestLabel_Clear(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend", "api"}},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newLabelCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"clear", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("label clear command failed: %v", err)
	}

	// Reload registry and verify
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	repo, err := reg.FindByName("myrepo")
	if err != nil {
		t.Fatalf("failed to find repo: %v", err)
	}

	if len(repo.Labels) != 0 {
		t.Errorf("expected 0 labels after clear, got %d: %v", len(repo.Labels), repo.Labels)
	}
}

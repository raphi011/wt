//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/registry"
)

// TestRepos_ListEmpty tests listing repos when none are registered.
//
// Scenario: User runs `wt repos` with no registered repos
// Expected: Shows "No repos registered" message
func TestRepos_ListEmpty(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir, err := os.MkdirTemp("", "wt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	ctx := testContext(t)
	cmd := newReposCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos command failed: %v", err)
	}
}

// TestRepos_ListRepos tests listing registered repos.
//
// Scenario: User runs `wt repos` with registered repos
// Expected: Shows all registered repos
func TestRepos_ListRepos(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create registry with repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: "/tmp/repo1", Labels: []string{"backend"}},
			{Name: "repo2", Path: "/tmp/repo2", Labels: []string{"frontend"}},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContext(t)
	cmd := newReposCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos command failed: %v", err)
	}
}

// TestRepos_FilterByLabel tests filtering repos by label.
//
// Scenario: User runs `wt repos -l backend`
// Expected: Shows only repos with the backend label
func TestRepos_FilterByLabel(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create registry with repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "backend-api", Path: "/tmp/backend-api", Labels: []string{"backend"}},
			{Name: "frontend-app", Path: "/tmp/frontend-app", Labels: []string{"frontend"}},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContext(t)
	cmd := newReposCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-l", "backend"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos command failed: %v", err)
	}
}

// TestRepos_JSON tests JSON output.
//
// Scenario: User runs `wt repos --json`
// Expected: Outputs repos in JSON format
func TestRepos_JSON(t *testing.T) {
	// Not parallel - modifies HOME

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create registry with repos
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: "/tmp/test-repo"},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// The JSON output goes through the output.Printer, so use testContextWithOutput
	ctx, out := testContextWithOutput(t)
	cmd := newReposCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("repos command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "test-repo") {
		t.Errorf("expected output to contain 'test-repo', got: %s", output)
	}
}

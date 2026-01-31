//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestHook_RunHook tests running a configured hook.
//
// Scenario: User runs `wt hook myhook -r myrepo`
// Expected: Hook command is executed in repo directory
func TestHook_RunHook(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Create a marker file that the hook will create
	markerPath := filepath.Join(tmpDir, "hook-ran")

	oldCfg := cfg
	cfg = &config.Config{
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "touch " + markerPath,
					Description: "Test hook",
				},
			},
		},
	}
	defer func() { cfg = oldCfg }()

	ctx := testContext(t)
	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myhook", "-r", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook command failed: %v", err)
	}

	// Verify hook ran by checking marker file
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		t.Error("hook should have created marker file")
	}
}

// TestHook_UnknownHook tests running an unknown hook.
//
// Scenario: User runs `wt hook nonexistent`
// Expected: Command fails with error
func TestHook_UnknownHook(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	oldCfg := cfg
	cfg = &config.Config{
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{},
		},
	}
	defer func() { cfg = oldCfg }()

	oldDir, _ := os.Getwd()
	os.Chdir(repoPath)
	defer os.Chdir(oldDir)

	ctx := testContext(t)
	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for unknown hook")
	}
}

// TestHook_DryRun tests dry-run mode.
//
// Scenario: User runs `wt hook myhook -d -r myrepo`
// Expected: Hook command is printed but not executed
func TestHook_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "hook-ran")

	oldCfg := cfg
	cfg = &config.Config{
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "touch " + markerPath,
					Description: "Test hook",
				},
			},
		},
	}
	defer func() { cfg = oldCfg }()

	ctx := testContext(t)
	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myhook", "-r", "myrepo", "-d"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook command failed: %v", err)
	}

	// Hook should NOT have run (dry-run)
	if _, err := os.Stat(markerPath); err == nil {
		t.Error("hook should NOT have run in dry-run mode")
	}
}

// TestHook_WithEnvVar tests hook with environment variable.
//
// Scenario: User runs `wt hook myhook -a VAR=value -r myrepo`
// Expected: Hook command has access to the variable
func TestHook_WithEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regPath := filepath.Join(tmpDir, ".wt")
	os.MkdirAll(regPath, 0755)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "output.txt")

	oldCfg := cfg
	cfg = &config.Config{
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "echo {myvar} > " + outputPath,
					Description: "Test hook with var",
				},
			},
		},
	}
	defer func() { cfg = oldCfg }()

	ctx := testContext(t)
	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myhook", "-r", "myrepo", "-a", "myvar=hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook command failed: %v", err)
	}

	// Verify the variable was substituted
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if string(content) != "hello\n" {
		t.Errorf("expected 'hello', got %q", string(content))
	}
}

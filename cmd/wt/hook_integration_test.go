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

// TestHook_RunHook tests running a configured hook.
//
// Scenario: User runs `wt hook myhook` from inside a repo
// Expected: Hook command is executed in repo directory
func TestHook_RunHook(t *testing.T) {
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

	// Create a marker file that the hook will create
	markerPath := filepath.Join(tmpDir, "hook-ran")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "touch " + markerPath,
					Description: "Test hook",
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myhook"})

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

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown hook")
	}
	if !strings.Contains(err.Error(), `unknown hook "nonexistent"`) {
		t.Errorf("error should mention unknown hook name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "no hooks configured") {
		t.Errorf("error should mention no hooks configured, got: %v", err)
	}
}

// TestHook_DryRun tests dry-run mode.
//
// Scenario: User runs `wt hook myhook -d` from inside a repo
// Expected: Hook command is printed but not executed
func TestHook_DryRun(t *testing.T) {
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

	markerPath := filepath.Join(tmpDir, "hook-ran")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "touch " + markerPath,
					Description: "Test hook",
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myhook", "-d"})

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
// Scenario: User runs `wt hook myhook -a myvar=value` from inside a repo
// Expected: Hook command has access to the variable
func TestHook_WithEnvVar(t *testing.T) {
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

	outputPath := filepath.Join(tmpDir, "output.txt")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "echo {myvar} > " + outputPath,
					Description: "Test hook with var",
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myhook", "-a", "myvar=hello"})

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

// TestHook_WithRepoBranchFormat tests hook with repo:branch format.
//
// Scenario: User has two repos with same branch name, runs `wt hook repo1:feature myhook`
// Expected: Hook runs only in the specified repo's worktree
func TestHook_WithRepoBranchFormat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	// Create two repos with the same branch name
	repo1Path := setupTestRepoWithBranches(t, tmpDir, "repo1", []string{"feature"})
	repo2Path := setupTestRepoWithBranches(t, tmpDir, "repo2", []string{"feature"})

	// Create worktrees in both repos
	wt1Path := createTestWorktree(t, repo1Path, "feature")
	createTestWorktree(t, repo2Path, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path},
			{Name: "repo2", Path: repo2Path},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Create marker file that will record the worktree path
	markerPath := filepath.Join(tmpDir, "hook-ran-in")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					// Write the worktree dir to marker file (uses hyphen not underscore)
					Command:     "echo {worktree-dir} > " + markerPath,
					Description: "Test hook",
				},
			},
		},
	}

	// Work from a different directory
	otherDir := filepath.Join(tmpDir, "other")
	os.MkdirAll(otherDir, 0755)

	ctx := testContextWithConfig(t, cfg, otherDir)
	cmd := newHookCmd()
	cmd.SetContext(ctx)
	// Use repo:branch hookname format to target only repo1's worktree
	cmd.SetArgs([]string{"repo1:feature", "myhook"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook command failed: %v", err)
	}

	// Verify hook ran in repo1's worktree
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("failed to read marker: %v", err)
	}

	worktreePath := strings.TrimSpace(string(content))
	if worktreePath != wt1Path {
		t.Errorf("hook should run in repo1's worktree %q, but ran in %q", wt1Path, worktreePath)
	}
}

// TestHook_RepoBranchFormat_BranchNotFound tests error when branch in repo:branch format is not found.
//
// Scenario: User runs `wt hook myrepo:nonexistent myhook`
// Expected: Command fails with informative error
func TestHook_RepoBranchFormat_BranchNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

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

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "echo test",
					Description: "Test hook",
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newHookCmd()
	cmd.SetContext(ctx)
	// Use nonexistent branch name with target hookname format
	cmd.SetArgs([]string{"myrepo:nonexistent", "myhook"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent branch")
	}

	// Error should mention the branch
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention nonexistent branch, got: %v", err)
	}
}

// TestHook_BareBranchTarget tests hook with unscoped branch as target.
//
// Scenario: User runs `wt hook feature myhook` (no repo: prefix)
// Expected: Hook runs in the worktree matching the branch across repos
func TestHook_BareBranchTarget(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "hook-ran-in")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "echo {worktree-dir} > " + markerPath,
					Description: "Test hook",
				},
			},
		},
	}

	// Work from a different directory (not inside any repo)
	otherDir := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create other dir: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, otherDir)
	cmd := newHookCmd()
	cmd.SetContext(ctx)
	// Bare branch (no scope prefix) as target
	cmd.SetArgs([]string{"feature", "myhook"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook command failed: %v", err)
	}

	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("failed to read marker: %v", err)
	}

	worktreePath := strings.TrimSpace(string(content))
	if worktreePath != wtPath {
		t.Errorf("hook should run in worktree %q, but ran in %q", wtPath, worktreePath)
	}
}

// TestHook_UnknownHookWithTarget tests unknown hook error via the target code path.
//
// Scenario: User runs `wt hook myrepo:feature nonexistent`
// Expected: Error mentions repo:branch context and unknown hook name
func TestHook_UnknownHookWithTarget(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command:     "echo test",
					Description: "Test hook",
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo:feature", "nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown hook")
	}
	if !strings.Contains(err.Error(), "myrepo") {
		t.Errorf("error should mention repo name, got: %v", err)
	}
	if !strings.Contains(err.Error(), `unknown hook "nonexistent"`) {
		t.Errorf("error should mention unknown hook name, got: %v", err)
	}
}

// TestHook_ActionPhasePlaceholders tests that manual hook gets correct action/phase/trigger values.
//
// Scenario: User runs `wt hook myhook` which writes {action} {phase} {trigger} to a file
// Expected: File contains "manual after run"
func TestHook_ActionPhasePlaceholders(t *testing.T) {
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

	outputPath := filepath.Join(tmpDir, "placeholders.txt")

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"myhook": {
					Command: "echo {action} {phase} {trigger} > " + outputPath,
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myhook"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook command failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	got := strings.TrimSpace(string(content))
	if got != "manual after run" {
		t.Errorf("expected 'manual after run', got %q", got)
	}
}

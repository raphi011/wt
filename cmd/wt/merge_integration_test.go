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

// TestMerge_RegularMerge tests a standard git merge via wt merge.
//
// Scenario: Feature branch has one commit ahead of main
// Expected: Merge succeeds, output contains "Merged", wt-merged config set with "merge:main@..."
func TestMerge_RegularMerge(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")
	addCommit(t, wtPath, "feature.txt", "add feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("merge command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Merged") {
		t.Errorf("expected output to contain 'Merged', got %q", output)
	}

	// Verify wt-merged config
	merged, err := runGitCommand(repoPath, "config", "branch.feature.wt-merged")
	if err != nil {
		t.Fatalf("failed to read wt-merged config: %v", err)
	}
	merged = strings.TrimSpace(merged)
	if !strings.HasPrefix(merged, "merge:main@") {
		t.Errorf("expected wt-merged to start with 'merge:main@', got %q", merged)
	}
}

// TestMerge_SquashMerge tests a squash merge via wt merge.
//
// Scenario: Feature branch has one commit, merged with --squash
// Expected: Merge succeeds, wt-merged config set with "squash:main@..."
func TestMerge_SquashMerge(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")
	addCommit(t, wtPath, "feature.txt", "add feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--", "--squash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("merge command failed: %v", err)
	}

	_ = out.String() // consume output

	// Verify wt-merged config
	merged, err := runGitCommand(repoPath, "config", "branch.feature.wt-merged")
	if err != nil {
		t.Fatalf("failed to read wt-merged config: %v", err)
	}
	merged = strings.TrimSpace(merged)
	if !strings.HasPrefix(merged, "squash:main@") {
		t.Errorf("expected wt-merged to start with 'squash:main@', got %q", merged)
	}
}

// TestMerge_FFOnlyMerge tests a fast-forward only merge.
//
// Scenario: Feature branch is ahead of main with no divergence (ff possible)
// Expected: Merge succeeds, wt-merged config set with "ff:main@..."
func TestMerge_FFOnlyMerge(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")
	addCommit(t, wtPath, "feature.txt", "add feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--", "--ff-only"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("merge command failed: %v", err)
	}

	_ = out.String()

	// Verify wt-merged config
	merged, err := runGitCommand(repoPath, "config", "branch.feature.wt-merged")
	if err != nil {
		t.Fatalf("failed to read wt-merged config: %v", err)
	}
	merged = strings.TrimSpace(merged)
	if !strings.HasPrefix(merged, "ff:main@") {
		t.Errorf("expected wt-merged to start with 'ff:main@', got %q", merged)
	}
}

// TestMerge_MarkOnly tests marking a branch as merged without running git merge.
//
// Scenario: Branch already merged externally, user marks it retroactively
// Expected: wt-merged config set with specified strategy
func TestMerge_MarkOnly(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--mark", "feature", "--strategy", "squash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("merge --mark command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Marked") {
		t.Errorf("expected output to contain 'Marked', got %q", output)
	}

	// Verify wt-merged config
	merged, err := runGitCommand(repoPath, "config", "branch.feature.wt-merged")
	if err != nil {
		t.Fatalf("failed to read wt-merged config: %v", err)
	}
	merged = strings.TrimSpace(merged)
	if !strings.HasPrefix(merged, "squash:main@") {
		t.Errorf("expected wt-merged to start with 'squash:main@', got %q", merged)
	}
}

// TestMerge_DryRun tests that dry-run mode shows what would happen without doing it.
//
// Scenario: Feature branch with commit, run merge with --dry-run
// Expected: Output contains "Would merge", wt-merged config NOT set
func TestMerge_DryRun(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")
	addCommit(t, wtPath, "feature.txt", "add feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--dry-run", "--", "--squash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("merge --dry-run command failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Would merge") {
		t.Errorf("expected output to contain 'Would merge', got %q", output)
	}

	// Verify wt-merged config is NOT set
	merged, _ := runGitCommand(repoPath, "config", "branch.feature.wt-merged")
	if strings.TrimSpace(merged) != "" {
		t.Errorf("expected wt-merged config to be unset in dry-run, got %q", strings.TrimSpace(merged))
	}
}

// TestMerge_AlreadyMerged tests that merging an already-marked branch is a no-op.
//
// Scenario: Branch already has wt-merged config set
// Expected: Output contains "already marked", no error
func TestMerge_AlreadyMerged(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	// Pre-set wt-merged config
	if _, err := runGitCommand(repoPath, "config", "branch.feature.wt-merged", "merge:main@2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("failed to set wt-merged config: %v", err)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("merge command should not fail for already merged branch: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "already marked") {
		t.Errorf("expected output to contain 'already marked', got %q", output)
	}
}

// TestMerge_DirtyTarget tests that merge fails when target worktree has uncommitted changes.
//
// Scenario: Main worktree has uncommitted changes
// Expected: Error containing "uncommitted changes"
func TestMerge_DirtyTarget(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")
	addCommit(t, wtPath, "feature.txt", "add feature")

	// Make main worktree dirty
	dirtyFile := filepath.Join(repoPath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted\n"), 0644); err != nil {
		t.Fatalf("failed to write dirty file: %v", err)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
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

	cmd := newMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected merge to fail with dirty target worktree")
	}
	if !strings.Contains(err.Error(), "uncommitted changes") {
		t.Errorf("expected error about uncommitted changes, got %q", err.Error())
	}
}

// TestMerge_SameBranch tests that merging a branch into itself fails.
//
// Scenario: User tries to merge main into main
// Expected: Error containing "same"
func TestMerge_SameBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
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

	cmd := newMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"main"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected merge to fail when source and target are same branch")
	}
	if !strings.Contains(err.Error(), "same") {
		t.Errorf("expected error about same branch, got %q", err.Error())
	}
}

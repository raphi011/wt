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

// TestNoteSet_CurrentBranch tests setting a note on the current branch.
//
// Scenario: User is in a worktree and runs `wt note set "WIP"`
// Expected: Note is stored in git config
func TestNoteSet_CurrentBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"set", "WIP"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("note set command failed: %v", err)
	}

	// Verify note was stored by reading it back
	note, err := runGitCommand(repoPath, "config", "branch.feature.description")
	if err != nil {
		t.Fatalf("failed to read git config: %v", err)
	}
	if strings.TrimSpace(note) != "WIP" {
		t.Errorf("expected note 'WIP', got %q", strings.TrimSpace(note))
	}
}

// TestNoteGet_CurrentBranch tests getting a note from the current branch.
//
// Scenario: Note is set on branch, user runs `wt note get`
// Expected: Prints the note to stdout
func TestNoteGet_CurrentBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")

	// Set the note via git config directly
	if _, err := runGitCommand(repoPath, "config", "branch.feature.description", "WIP"); err != nil {
		t.Fatalf("failed to set git config: %v", err)
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
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, wtPath)

	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("note get command failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "WIP" {
		t.Errorf("expected note 'WIP', got %q", got)
	}
}

// TestNoteGet_NoNote tests getting a note when none is set.
//
// Scenario: No note set on branch, user runs `wt note get`
// Expected: Empty stdout (no error)
func TestNoteGet_NoNote(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, wtPath)

	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("note get command failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "" {
		t.Errorf("expected empty output for no note, got %q", got)
	}
}

// TestNoteClear_CurrentBranch tests clearing a note from the current branch.
//
// Scenario: Note is set, user runs `wt note clear`
// Expected: Note is removed from git config
func TestNoteClear_CurrentBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")

	// Set a note first
	if _, err := runGitCommand(repoPath, "config", "branch.feature.description", "WIP"); err != nil {
		t.Fatalf("failed to set git config: %v", err)
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
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, wtPath)

	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"clear"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("note clear command failed: %v", err)
	}

	// Verify note was cleared (git config returns error for unset keys)
	note, _ := runGitCommand(repoPath, "config", "branch.feature.description")
	if strings.TrimSpace(note) != "" {
		t.Errorf("expected empty note after clear, got %q", strings.TrimSpace(note))
	}
}

// TestNoteSet_ExplicitRepoBranch tests setting a note via repo:branch target.
//
// Scenario: User runs `wt note set "TODO" myrepo:feature`
// Expected: Note is set on the correct branch in the correct repo
func TestNoteSet_ExplicitRepoBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	createTestWorktree(t, repoPath, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"set", "TODO", "myrepo:feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("note set command failed: %v", err)
	}

	// Verify
	note, err := runGitCommand(repoPath, "config", "branch.feature.description")
	if err != nil {
		t.Fatalf("failed to read git config: %v", err)
	}
	if strings.TrimSpace(note) != "TODO" {
		t.Errorf("expected note 'TODO', got %q", strings.TrimSpace(note))
	}
}

// TestNoteGet_ExplicitBranch tests getting a note via repo:branch target.
//
// Scenario: User runs `wt note get myrepo:feature`
// Expected: Prints the note for the specified branch
func TestNoteGet_ExplicitBranch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	createTestWorktree(t, repoPath, "feature")

	// Set note via git config
	if _, err := runGitCommand(repoPath, "config", "branch.feature.description", "review needed"); err != nil {
		t.Fatalf("failed to set git config: %v", err)
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
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, out := testContextWithConfigAndOutput(t, cfg, tmpDir)

	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"get", "myrepo:feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("note get command failed: %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != "review needed" {
		t.Errorf("expected note 'review needed', got %q", got)
	}
}

// TestNote_BranchNotFound tests error when target branch doesn't exist.
//
// Scenario: User runs `wt note get nonexistent`
// Expected: Returns error about worktree not found
func TestNote_BranchNotFound(t *testing.T) {
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
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"get", "nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent branch, got nil")
	}
	if !strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("expected 'worktree not found' error, got %q", err.Error())
	}
}

// TestNote_NotInGitRepo tests error when no target is given outside a git repo.
//
// Scenario: User runs `wt note get` outside a git repository
// Expected: Returns error about not being in a git repository
func TestNote_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"get"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error outside a git repository, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestNoteSet_LabelScope tests setting a note on all repos matching a label.
//
// Scenario: Two repos have label "backend", user runs `wt note set "WIP" backend:feature`
// Expected: Note is set on both repos' feature branches
func TestNoteSet_LabelScope(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repo1Path := setupTestRepo(t, tmpDir, "svc-a")
	repo2Path := setupTestRepo(t, tmpDir, "svc-b")
	createTestWorktree(t, repo1Path, "feature")
	createTestWorktree(t, repo2Path, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "svc-a", Path: repo1Path, Labels: []string{"backend"}},
			{Name: "svc-b", Path: repo2Path, Labels: []string{"backend"}},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"set", "WIP", "backend:feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("note set command failed: %v", err)
	}

	// Verify note set on both repos
	for _, rp := range []string{repo1Path, repo2Path} {
		note, err := runGitCommand(rp, "config", "branch.feature.description")
		if err != nil {
			t.Fatalf("failed to read git config for %s: %v", rp, err)
		}
		if strings.TrimSpace(note) != "WIP" {
			t.Errorf("expected note 'WIP' in %s, got %q", rp, strings.TrimSpace(note))
		}
	}
}

// TestNote_UnscopedGitErrorWarns tests that a git failure during an unscoped
// note target search is reported as a warning.
//
// Scenario: User runs `wt note get feature` where `git worktree list` fails in one registered repo
// Expected: The target from the working repo is returned and a warning names the failing repo
func TestNote_UnscopedGitErrorWarns(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	brokenPath := setupBrokenRepoDir(t, tmpDir, "broken")
	createTestWorktree(t, repoPath, "feature")

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
			{Name: "broken", Path: brokenPath},
		},
	}

	cfg := &config.Config{}
	ctx, logs := testContextWithLog(t, cfg, tmpDir)

	targets, err := resolveNoteTargets(ctx, cfg, tmpDir, reg, []string{"feature"})
	if err != nil {
		t.Fatalf("resolveNoteTargets failed: %v", err)
	}
	if len(targets) != 1 || targets[0].RepoPath != repoPath {
		t.Errorf("expected target in %s, got %+v", repoPath, targets)
	}
	if !strings.Contains(logs.String(), "Warning: broken: failed to list worktrees") {
		t.Errorf("expected warning for failing repo, got log: %q", logs.String())
	}
}

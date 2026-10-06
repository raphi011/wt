//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

// TestFindOrRegisterCurrentRepo_RegisteredMeanwhile tests auto-registration
// when another process registered the repo after the registry was loaded.
//
// Scenario: Registry is loaded, another process registers the current repo,
// then the command auto-registers it
// Expected: The existing entry is returned and not duplicated
func TestFindOrRegisterCurrentRepo_RegisteredMeanwhile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	otherPath := setupTestRepo(t, tmpDir, "other")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	// Snapshot taken before the other process wrote
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	other := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"theirs"}},
			{Name: "other", Path: otherPath},
		},
	}
	if err := saveRegistry(other, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	repo, err := findOrRegisterCurrentRepo(ctx, reg, cfg)
	if err != nil {
		t.Fatalf("findOrRegisterCurrentRepo failed: %v", err)
	}
	if !repo.HasLabel("theirs") {
		t.Errorf("expected the entry registered by the other process, got %+v", repo)
	}

	// The snapshot is refreshed with the saved state
	if _, err := reg.FindByName("other"); err != nil {
		t.Errorf("expected snapshot to include repos added meanwhile: %v", err)
	}

	loaded, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(loaded.Repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(loaded.Repos))
	}
}

// TestFindOrRegisterCurrentRepo_AutoRegisters tests auto-registration of an
// unregistered repo.
//
// Scenario: Command runs inside a git repo that is not in the registry
// Expected: The repo is registered with the default labels and saved
func TestFindOrRegisterCurrentRepo_AutoRegisters(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile, DefaultLabels: []string{"auto"}}
	ctx := testContextWithConfig(t, cfg, repoPath)

	repo, err := findOrRegisterCurrentRepo(ctx, reg, cfg)
	if err != nil {
		t.Fatalf("findOrRegisterCurrentRepo failed: %v", err)
	}
	if repo.Path != repoPath || !repo.HasLabel("auto") {
		t.Errorf("unexpected repo: %+v", repo)
	}

	loaded, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if _, err := loaded.FindByPath(repoPath); err != nil {
		t.Errorf("repo was not saved: %v", err)
	}
}

// setupBrokenRepoDir creates a directory that is not a git repository,
// so that git commands run in it fail.
func setupBrokenRepoDir(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	return path
}

// testContextWithLog creates a context with config and workDir set and returns
// the log output for assertions.
func testContextWithLog(t *testing.T, cfg *config.Config, workDir string) (context.Context, *strings.Builder) {
	t.Helper()
	var logs strings.Builder
	ctx := testContextWithConfig(t, cfg, workDir)
	ctx = log.WithLogger(ctx, log.New(&logs, false, false))
	return ctx, &logs
}

// TestResolveWorktreeTargets_ScopedGitError tests that a git failure in a
// repo-scoped target is returned as an error.
//
// Scenario: User targets `broken:feature` where `git worktree list` fails in the repo
// Expected: The git error is returned instead of "worktree not found"
func TestResolveWorktreeTargets_ScopedGitError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	brokenPath := setupBrokenRepoDir(t, tmpDir, "broken")

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "broken", Path: brokenPath},
		},
	}

	ctx := testContextWithConfig(t, &config.Config{}, tmpDir)

	_, err := resolveWorktreeTargets(ctx, reg, []string{"broken:feature"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("git failure reported as not found: %v", err)
	}
	if !strings.Contains(err.Error(), "failed to list worktrees") {
		t.Errorf("expected the git error, got: %v", err)
	}
}

// TestResolveWorktreeTargets_LabelPartialMatch tests that a label target
// reports the repos that have no matching worktree.
//
// Scenario: User targets `team:feature` where only one of two labelled repos has the worktree
// Expected: The matching worktree is returned and the other repo is reported as skipped
func TestResolveWorktreeTargets_LabelPartialMatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	withPath := setupTestRepo(t, tmpDir, "with-feature")
	withoutPath := setupTestRepo(t, tmpDir, "without-feature")
	wtPath := createTestWorktree(t, withPath, "feature")

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "with-feature", Path: withPath, Labels: []string{"team"}},
			{Name: "without-feature", Path: withoutPath, Labels: []string{"team"}},
		},
	}

	ctx, logs := testContextWithLog(t, &config.Config{}, tmpDir)

	targets, err := resolveWorktreeTargets(ctx, reg, []string{"team:feature"})
	if err != nil {
		t.Fatalf("resolveWorktreeTargets failed: %v", err)
	}
	if len(targets) != 1 || targets[0].Path != wtPath {
		t.Errorf("expected worktree %s, got %+v", wtPath, targets)
	}
	if !strings.Contains(logs.String(), "without-feature") {
		t.Errorf("expected skipped repo to be reported, got log: %q", logs.String())
	}
}

// TestResolveWorktreeTargets_LabelGitErrorWarns tests that a git failure in
// one repo of a label target is reported as a warning.
//
// Scenario: User targets `team:feature` where `git worktree list` fails in one labelled repo
// Expected: The worktree from the working repo is returned and a warning names the failing repo
func TestResolveWorktreeTargets_LabelGitErrorWarns(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	brokenPath := setupBrokenRepoDir(t, tmpDir, "broken")
	wtPath := createTestWorktree(t, repoPath, "feature")

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"team"}},
			{Name: "broken", Path: brokenPath, Labels: []string{"team"}},
		},
	}

	ctx, logs := testContextWithLog(t, &config.Config{}, tmpDir)

	targets, err := resolveWorktreeTargets(ctx, reg, []string{"team:feature"})
	if err != nil {
		t.Fatalf("resolveWorktreeTargets failed: %v", err)
	}
	if len(targets) != 1 || targets[0].Path != wtPath {
		t.Errorf("expected worktree %s, got %+v", wtPath, targets)
	}
	if !strings.Contains(logs.String(), "Warning: broken: failed to list worktrees") {
		t.Errorf("expected warning for failing repo, got log: %q", logs.String())
	}
}

// TestResolveWorktreeTargets_UnscopedGitErrorWarns tests that a git failure
// during an unscoped search is reported as a warning.
//
// Scenario: User targets `feature` where `git worktree list` fails in one registered repo
// Expected: The worktree from the working repo is returned and a warning names the failing repo
func TestResolveWorktreeTargets_UnscopedGitErrorWarns(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	brokenPath := setupBrokenRepoDir(t, tmpDir, "broken")
	wtPath := createTestWorktree(t, repoPath, "feature")

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
			{Name: "broken", Path: brokenPath},
		},
	}

	ctx, logs := testContextWithLog(t, &config.Config{}, tmpDir)

	targets, err := resolveWorktreeTargets(ctx, reg, []string{"feature"})
	if err != nil {
		t.Fatalf("resolveWorktreeTargets failed: %v", err)
	}
	if len(targets) != 1 || targets[0].Path != wtPath {
		t.Errorf("expected worktree %s, got %+v", wtPath, targets)
	}
	if !strings.Contains(logs.String(), "Warning: broken: failed to list worktrees") {
		t.Errorf("expected warning for failing repo, got log: %q", logs.String())
	}
}

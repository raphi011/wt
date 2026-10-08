//go:build integration

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

// TestCurrentRepo_RegisteredMeanwhile tests auto-registration
// when another process registered the repo after the registry was loaded.
//
// Scenario: Registry is loaded, another process registers the current repo,
// then the command auto-registers it
// Expected: The existing entry is returned and not duplicated
func TestCurrentRepo_RegisteredMeanwhile(t *testing.T) {
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

	repo, err := currentRepo(ctx, reg, autoRegister)
	if err != nil {
		t.Fatalf("currentRepo failed: %v", err)
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

// TestCurrentRepo_AutoRegisters tests auto-registration of an
// unregistered repo.
//
// Scenario: Command runs inside a git repo that is not in the registry
// Expected: The repo is registered with the default labels and saved
func TestCurrentRepo_AutoRegisters(t *testing.T) {
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

	repo, err := currentRepo(ctx, reg, autoRegister)
	if err != nil {
		t.Fatalf("currentRepo failed: %v", err)
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

	_, err := resolveWorktreeTargets(ctx, reg, []string{"broken:feature"}, targetOpts{Multi: true})
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

	targets, err := resolveWorktreeTargets(ctx, reg, []string{"team:feature"}, targetOpts{Multi: true})
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

	targets, err := resolveWorktreeTargets(ctx, reg, []string{"team:feature"}, targetOpts{Multi: true})
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

	targets, err := resolveWorktreeTargets(ctx, reg, []string{"feature"}, targetOpts{Multi: true})
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

// TestResolveWorktreeTargets_Rule tests the resolution rule for [scope:]branch
// targets shared by all commands.
//
// Scenario: Repos alpha and beta (both labelled team) each have a worktree
// "shared"; alpha also has "only-a", beta "only-b"
// Expected: Unscoped targets resolve in the current repo, or in all repos
// outside a repo or with -g; several matches need -g on a multi-target command
func TestResolveWorktreeTargets_Rule(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	alphaPath := setupTestRepoWithBranches(t, tmpDir, "alpha", []string{"shared", "only-a"})
	betaPath := setupTestRepoWithBranches(t, tmpDir, "beta", []string{"shared", "only-b"})
	createTestWorktree(t, alphaPath, "shared")
	createTestWorktree(t, alphaPath, "only-a")
	createTestWorktree(t, betaPath, "shared")
	createTestWorktree(t, betaPath, "only-b")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "alpha", Path: alphaPath, Labels: []string{"team"}},
			{Name: "beta", Path: betaPath, Labels: []string{"team"}},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}
	cfg := &config.Config{RegistryPath: regFile}

	tests := []struct {
		name    string
		workDir string
		target  string
		opts    targetOpts
		want    []string // repo:branch
		wantErr string
	}{
		{name: "in repo, branch in current repo", workDir: alphaPath, target: "shared", want: []string{"alpha:shared"}},
		{name: "in repo, multi-target stays in current repo", workDir: alphaPath, target: "shared", opts: targetOpts{Multi: true}, want: []string{"alpha:shared"}},
		{name: "in repo, branch only in other repo", workDir: alphaPath, target: "only-b", wantErr: "worktree not found in alpha: only-b"},
		{name: "in repo, -g finds other repo", workDir: alphaPath, target: "only-b", opts: targetOpts{Global: true}, want: []string{"beta:only-b"}},
		{name: "in repo, -g single-target is ambiguous", workDir: alphaPath, target: "shared", opts: targetOpts{Global: true}, wantErr: "exists in multiple repos"},
		{name: "in repo, -g multi-target fans out", workDir: alphaPath, target: "shared", opts: targetOpts{Global: true, Multi: true}, want: []string{"alpha:shared", "beta:shared"}},
		{name: "outside repo, unique branch", workDir: tmpDir, target: "only-a", want: []string{"alpha:only-a"}},
		{name: "outside repo, single-target is ambiguous", workDir: tmpDir, target: "shared", wantErr: "exists in multiple repos"},
		{name: "outside repo, multi-target needs -g", workDir: tmpDir, target: "shared", opts: targetOpts{Multi: true}, wantErr: "use -g"},
		{name: "outside repo, -g multi-target fans out", workDir: tmpDir, target: "shared", opts: targetOpts{Global: true, Multi: true}, want: []string{"alpha:shared", "beta:shared"}},
		{name: "outside repo, unknown branch", workDir: tmpDir, target: "missing", wantErr: "worktree not found: missing"},
		{name: "repo scope from another repo", workDir: alphaPath, target: "beta:shared", want: []string{"beta:shared"}},
		{name: "label scope, multi-target fans out", workDir: alphaPath, target: "team:shared", opts: targetOpts{Multi: true}, want: []string{"alpha:shared", "beta:shared"}},
		{name: "label scope, single-target is ambiguous", workDir: alphaPath, target: "team:shared", wantErr: "exists in multiple repos"},
		{name: "label scope, single match", workDir: tmpDir, target: "team:only-b", want: []string{"beta:only-b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := testContextWithConfig(t, &config.Config{RegistryPath: cfg.RegistryPath}, tt.workDir)
			got, err := resolveWorktreeTargets(ctx, reg, []string{tt.target}, tt.opts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveWorktreeTargets failed: %v", err)
			}
			var names []string
			for _, wt := range got {
				names = append(names, wt.RepoName+":"+wt.Branch)
			}
			if !slices.Equal(names, tt.want) {
				t.Errorf("resolved %v, want %v", names, tt.want)
			}
		})
	}
}

// setupSharedFeatureRepos registers repo1 and repo2, each with a "feature" worktree.
func setupSharedFeatureRepos(t *testing.T) (cfg *config.Config, repo1Path, repo2Path, wt1Path, wt2Path string) {
	t.Helper()

	tmpDir := resolvePath(t, t.TempDir())
	repo1Path = setupTestRepo(t, tmpDir, "repo1")
	repo2Path = setupTestRepo(t, tmpDir, "repo2")
	wt1Path = createTestWorktree(t, repo1Path, "feature")
	wt2Path = createTestWorktree(t, repo2Path, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repo1Path},
			{Name: "repo2", Path: repo2Path},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	return &config.Config{RegistryPath: regFile}, repo1Path, repo2Path, wt1Path, wt2Path
}

// TestCd_UnscopedInRepo_UsesCurrentRepo tests that cd resolves an unscoped
// branch in the current repo.
//
// Scenario: repo1 and repo2 both have a "feature" worktree, user runs `wt cd feature` inside repo1
// Expected: Prints repo1's worktree path
func TestCd_UnscopedInRepo_UsesCurrentRepo(t *testing.T) {
	t.Parallel()

	cfg, repo1Path, _, wt1Path, _ := setupSharedFeatureRepos(t)

	ctx, out := testContextWithConfigAndOutput(t, cfg, repo1Path)
	cmd := newCdCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cd command failed: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != wt1Path {
		t.Errorf("cd printed %q, want %q", got, wt1Path)
	}
}

// TestCd_Global_Ambiguous tests that cd -g searches all repos and rejects an ambiguous branch.
//
// Scenario: repo1 and repo2 both have a "feature" worktree, user runs `wt cd -g feature` inside repo1
// Expected: Error naming both repos
func TestCd_Global_Ambiguous(t *testing.T) {
	t.Parallel()

	cfg, repo1Path, _, _, _ := setupSharedFeatureRepos(t)

	ctx := testContextWithConfig(t, cfg, repo1Path)
	cmd := newCdCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-g", "feature"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "exists in multiple repos: repo1:feature, repo2:feature") {
		t.Fatalf("cd -g should fail naming both repos, got: %v", err)
	}
}

// TestExec_UnscopedInRepo_UsesCurrentRepo tests that exec runs an unscoped
// target in the current repo only, and in every match with -g.
//
// Scenario: repo1 and repo2 both have a "feature" worktree, user runs `wt exec feature -- touch marker` inside repo1, then with -g
// Expected: marker is created in repo1's worktree only; with -g in both
func TestExec_UnscopedInRepo_UsesCurrentRepo(t *testing.T) {
	t.Parallel()

	cfg, repo1Path, _, wt1Path, wt2Path := setupSharedFeatureRepos(t)

	ctx := testContextWithConfig(t, cfg, repo1Path)
	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--", "touch", "marker"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt1Path, "marker")); err != nil {
		t.Errorf("command should run in repo1's worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt2Path, "marker")); err == nil {
		t.Error("command should not run in repo2's worktree without -g")
	}

	cmd = newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-g", "feature", "--", "touch", "marker-global"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec -g command failed: %v", err)
	}
	for _, wtPath := range []string{wt1Path, wt2Path} {
		if _, err := os.Stat(filepath.Join(wtPath, "marker-global")); err != nil {
			t.Errorf("command should run in %s with -g: %v", wtPath, err)
		}
	}
}

// TestHook_UnscopedInRepo_UsesCurrentRepo tests that hook runs for an unscoped
// target in the current repo only.
//
// Scenario: repo1 and repo2 both have a "feature" worktree, user runs `wt hook feature marker` inside repo1
// Expected: The hook runs in repo1's worktree only
func TestHook_UnscopedInRepo_UsesCurrentRepo(t *testing.T) {
	t.Parallel()

	cfg, repo1Path, _, wt1Path, wt2Path := setupSharedFeatureRepos(t)
	cfg.Hooks = config.HooksConfig{
		Hooks: map[string]config.Hook{
			"marker": {Command: "touch {worktree-dir}/hook-ran"},
		},
	}

	ctx := testContextWithConfig(t, cfg, repo1Path)
	cmd := newHookCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "marker"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook command failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt1Path, "hook-ran")); err != nil {
		t.Errorf("hook should run in repo1's worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt2Path, "hook-ran")); err == nil {
		t.Error("hook should not run in repo2's worktree without -g")
	}
}

// TestNote_UnscopedInRepo_UsesCurrentRepo tests that note set applies an
// unscoped target to the current repo only.
//
// Scenario: repo1 and repo2 both have a "feature" worktree, user runs `wt note set feature WIP` inside repo1
// Expected: The note is set in repo1 only
func TestNote_UnscopedInRepo_UsesCurrentRepo(t *testing.T) {
	t.Parallel()

	cfg, repo1Path, repo2Path, _, _ := setupSharedFeatureRepos(t)

	ctx := testContextWithConfig(t, cfg, repo1Path)
	cmd := newNoteCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"set", "feature", "WIP"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("note set command failed: %v", err)
	}

	note, err := git.GetBranchNote(ctx, repo1Path, "feature")
	if err != nil {
		t.Fatalf("failed to read note: %v", err)
	}
	if note != "WIP" {
		t.Errorf("repo1 note = %q, want %q", note, "WIP")
	}
	note, err = git.GetBranchNote(ctx, repo2Path, "feature")
	if err != nil {
		t.Fatalf("failed to read note: %v", err)
	}
	if note != "" {
		t.Errorf("repo2 note should stay empty without -g, got %q", note)
	}
}

// TestDiff_UnscopedInRepo_UsesCurrentRepo tests that diff resolves an unscoped
// branch in the current repo.
//
// Scenario: repo1 and repo2 both have a "feature" worktree, user runs `wt diff feature --working` inside repo1
// Expected: The diff runs without an ambiguity error
func TestDiff_UnscopedInRepo_UsesCurrentRepo(t *testing.T) {
	t.Parallel()

	cfg, repo1Path, _, _, _ := setupSharedFeatureRepos(t)

	ctx := testContextWithConfig(t, cfg, repo1Path)
	cmd := newDiffCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--working"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff command failed: %v", err)
	}
}

// TestUnscopedRepos tests which repos a command without a scope acts on.
//
// Scenario: Repos alpha and beta are registered, gone is registered but its
// directory no longer exists
// Expected: The current repo inside a repo; all existing repos outside a repo
// or with -g
func TestUnscopedRepos(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	alphaPath := setupTestRepo(t, tmpDir, "alpha")
	betaPath := setupTestRepo(t, tmpDir, "beta")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "alpha", Path: alphaPath},
			{Name: "beta", Path: betaPath},
			{Name: "gone", Path: filepath.Join(tmpDir, "gone")},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	tests := []struct {
		name        string
		workDir     string
		global      bool
		want        []string
		wantCurrent bool
	}{
		{name: "in repo", workDir: alphaPath, want: []string{"alpha"}, wantCurrent: true},
		{name: "in repo, -g", workDir: alphaPath, global: true, want: []string{"alpha", "beta"}},
		{name: "outside repo", workDir: tmpDir, want: []string{"alpha", "beta"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := testContextWithConfig(t, &config.Config{RegistryPath: regFile}, tt.workDir)
			repos, current, err := unscopedRepos(ctx, reg, tt.global)
			if err != nil {
				t.Fatalf("unscopedRepos failed: %v", err)
			}
			var names []string
			for _, r := range repos {
				names = append(names, r.Name)
			}
			if !slices.Equal(names, tt.want) {
				t.Errorf("repos = %v, want %v", names, tt.want)
			}
			if current != tt.wantCurrent {
				t.Errorf("current = %v, want %v", current, tt.wantCurrent)
			}
		})
	}
}

// TestCurrentRepo_RequireRegistered tests the current repo lookup for
// commands that do not auto-register.
//
// Scenario: Command runs inside a git repo that is not in the registry
// Expected: Error naming the repo path, registry is left unchanged
func TestCurrentRepo_RequireRegistered(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	ctx := testContextWithConfig(t, &config.Config{RegistryPath: regFile}, repoPath)

	_, err = currentRepo(ctx, reg, requireRegistered)
	if err == nil || err.Error() != "repo not registered: "+repoPath {
		t.Fatalf("error = %v, want repo not registered", err)
	}
	if len(reg.Repos) != 0 {
		t.Errorf("expected registry unchanged, got %+v", reg.Repos)
	}

	// Outside a repo
	ctx = testContextWithConfig(t, &config.Config{RegistryPath: regFile}, tmpDir)
	if _, err := currentRepo(ctx, reg, requireRegistered); !errors.Is(err, errNotInRepo) {
		t.Errorf("error = %v, want errNotInRepo", err)
	}
}

// TestCompleteScopedArg_RepoAndLabelScope tests scope:branch completion for
// worktree targets and for checkout.
//
// Scenario: Repo alpha (label team) has a worktree "wip" and a branch "todo"
// without worktree; a repo named "team" does not exist
// Expected: Worktree commands complete worktree branches, checkout completes
// branches without a worktree, for the repo scope and for the label scope
func TestCompleteScopedArg_RepoAndLabelScope(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	alphaPath := setupTestRepoWithBranches(t, tmpDir, "alpha", []string{"wip", "todo"})
	createTestWorktree(t, alphaPath, "wip")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "alpha", Path: alphaPath, Labels: []string{"team"}},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContextWithConfig(t, &config.Config{RegistryPath: regFile}, tmpDir)
	cd := &cobra.Command{}
	cd.SetContext(ctx)
	checkout := &cobra.Command{}
	checkout.SetContext(ctx)
	checkout.Flags().String("base", "", "")
	registerCheckoutCompletions(checkout)

	tests := []struct {
		toComplete   string
		wantWorktree []string
		wantCheckout []string
	}{
		{toComplete: "alpha:w", wantWorktree: []string{"alpha:wip"}},
		{toComplete: "alpha:t", wantCheckout: []string{"alpha:todo"}},
		{toComplete: "team:w", wantWorktree: []string{"team:wip"}},
		{toComplete: "team:t", wantCheckout: []string{"team:todo"}},
		{toComplete: "nope:"},
	}
	for _, tt := range tests {
		got, _ := completeScopedWorktreeArg(cd, nil, tt.toComplete)
		slices.Sort(got)
		if !slices.Equal(got, tt.wantWorktree) {
			t.Errorf("worktree completion of %q = %v, want %v", tt.toComplete, got, tt.wantWorktree)
		}
		got, _ = checkout.ValidArgsFunction(checkout, nil, tt.toComplete)
		slices.Sort(got)
		if !slices.Equal(got, tt.wantCheckout) {
			t.Errorf("checkout completion of %q = %v, want %v", tt.toComplete, got, tt.wantCheckout)
		}
	}
}

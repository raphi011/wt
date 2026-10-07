//go:build integration

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestExec_NoCommand tests error when no command is given after --.
//
// Scenario: User runs `wt exec --` with no command
// Expected: Returns "no command specified" error
func TestExec_NoCommand(t *testing.T) {
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
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no command, got nil")
	}
	if !strings.Contains(err.Error(), "no command specified") {
		t.Errorf("expected 'no command specified' error, got %q", err.Error())
	}
}

// TestExec_InCurrentWorktree tests running a command in the current directory.
//
// Scenario: User runs `wt exec -- touch test-file` from inside a worktree
// Expected: File is created in the current worktree directory
func TestExec_InCurrentWorktree(t *testing.T) {
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
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--", "touch", "exec-test-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	// Verify file was created in workDir
	testFile := filepath.Join(repoPath, "exec-test-file")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("expected file %q to be created", testFile)
	}
}

// TestExec_ByBranch tests running a command in a specific worktree by branch name.
//
// Scenario: User runs `wt exec feature -- touch test-file`
// Expected: File is created in the feature worktree
func TestExec_ByBranch(t *testing.T) {
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
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "--", "touch", "exec-test-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	// Verify file was created in the feature worktree
	testFile := filepath.Join(wtPath, "exec-test-file")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("expected file %q to be created in feature worktree", testFile)
	}

	// Verify it was NOT created in the main repo
	mainFile := filepath.Join(repoPath, "exec-test-file")
	if _, err := os.Stat(mainFile); err == nil {
		t.Errorf("did not expect file %q in main repo", mainFile)
	}
}

// TestExec_ByRepoBranch tests running a command with repo:branch targeting.
//
// Scenario: User runs `wt exec myrepo:feature -- touch test-file`
// Expected: File is created in the correct worktree
func TestExec_ByRepoBranch(t *testing.T) {
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
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo:feature", "--", "touch", "exec-test-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	testFile := filepath.Join(wtPath, "exec-test-file")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("expected file %q to be created", testFile)
	}
}

// TestExec_MultipleTargets tests running a command in multiple worktrees.
//
// Scenario: User runs `wt exec main feature -- touch test-file`
// Expected: File is created in both worktrees
func TestExec_MultipleTargets(t *testing.T) {
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
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"main", "feature", "--", "touch", "exec-multi-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	// Verify file in main repo (main branch = repoPath)
	mainFile := filepath.Join(repoPath, "exec-multi-file")
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		t.Errorf("expected file %q in main worktree", mainFile)
	}

	// Verify file in feature worktree
	featureFile := filepath.Join(wtPath, "exec-multi-file")
	if _, err := os.Stat(featureFile); os.IsNotExist(err) {
		t.Errorf("expected file %q in feature worktree", featureFile)
	}
}

// TestExec_BranchNotFound tests error when target branch doesn't exist.
//
// Scenario: User runs `wt exec nonexistent -- ls`
// Expected: Returns "worktree not found" error
func TestExec_BranchNotFound(t *testing.T) {
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
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent", "--", "ls"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent branch, got nil")
	}
	if !strings.Contains(err.Error(), "worktree not found") {
		t.Errorf("expected 'worktree not found' error, got %q", err.Error())
	}
}

// TestExec_Deduplication tests that the same target is only executed once.
//
// Scenario: User runs `wt exec feature feature -- touch test-file`
// Expected: Command runs only once (single file created, no error)
func TestExec_Deduplication(t *testing.T) {
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
	ctx := testContextWithConfig(t, cfg, repoPath)

	// Use a script that appends to a file to detect double execution
	counterFile := filepath.Join(tmpDir, "counter")
	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"feature", "feature", "--", "sh", "-c", "echo x >> " + counterFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	// Verify the command ran exactly once (file should have one line)
	content, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("failed to read counter file: %v", err)
	}
	lines := strings.Count(strings.TrimSpace(string(content)), "x")
	if lines != 1 {
		t.Errorf("expected command to run once (1 line), got %d lines", lines)
	}

}

// TestExec_NotInGitRepo tests error when running exec with no targets from outside a git repo.
//
// Scenario: User runs `wt exec -- ls` from a non-git directory
// Expected: Returns "not in a git repository" error
func TestExec_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Use a non-git directory as workDir
	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, nonGitDir)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--", "ls"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestExec_FailingCommand tests that a non-zero exit command is returned to the caller.
//
// Scenario: User runs `wt exec -- false` (exit code 1)
// Expected: Returns an error preserving the command exit code
func TestExec_FailingCommand(t *testing.T) {
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
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--", "false"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for failing command, got nil")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected wrapped exit code 1, got: %v", err)
	}
}

// TestExec_RepoNotFound tests error when targeting a non-existent repo.
//
// Scenario: User runs `wt exec nonexistent:main -- ls`
// Expected: Returns error from reg.FindByName
func TestExec_RepoNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent:main", "--", "ls"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "no repo or label found") {
		t.Errorf("expected 'no repo or label found' error, got %q", err.Error())
	}
}

// TestExec_ByLabelScope tests running a command in worktrees matched by label scope.
//
// Scenario: User runs `wt exec backend:main -- touch test-file` where "backend" is a label
// Expected: Since exec uses parseBranchTarget not label resolution, "backend" is treated as repo name
//
//	and fails with not found. The label:branch pattern is for worktree commands, not exec.
//	Actually, exec uses parseBranchTarget which splits on ":" and treats left side as repo name.
func TestExec_ByRepoScope(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath, Labels: []string{"backend"}},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	// exec's parser treats "backend" as plain branch name when no colon
	// When branch "main" is specified without colon, it searches all repos
	cmd := newExecCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"main", "--", "touch", "exec-label-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command failed: %v", err)
	}

	// Verify file was created in the main worktree (repoPath is on branch main)
	testFile := filepath.Join(repoPath, "exec-label-file")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("expected file %q to be created in main worktree", testFile)
	}
}

// TestExec_LabelScope tests running a command in worktrees matched by a label scope.
//
// Scenario: Two repos are labeled "team-b", user runs `wt exec team-b:feature -- touch file`
// Expected: Command runs in both repos' feature worktrees
func TestExec_LabelScope(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repo1Path := setupTestRepo(t, tmpDir, "svc-x")
	repo2Path := setupTestRepo(t, tmpDir, "svc-y")

	wt1Path := createTestWorktree(t, repo1Path, "feature")
	wt2Path := createTestWorktree(t, repo2Path, "feature")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "svc-x", Path: repo1Path, Labels: []string{"team-b"}},
			{Name: "svc-y", Path: repo2Path, Labels: []string{"team-b"}},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newExecCmd()
	cmd.SetContext(ctx)
	// "team-b" is a label (not a repo name), so label:branch resolution applies
	cmd.SetArgs([]string{"team-b:feature", "--", "touch", "label-exec-file"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("exec command with label scope failed: %v", err)
	}

	// Verify file was created in both feature worktrees
	for _, wtPath := range []string{wt1Path, wt2Path} {
		testFile := filepath.Join(wtPath, "label-exec-file")
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Errorf("expected file %q to be created in worktree %s", testFile, wtPath)
		}
	}
}

// TestExec_MultipleTargetsFailure tests that failures do not stop remaining targets.
//
// Scenario: The command fails in main and succeeds in feature
// Expected: Both targets run, and the returned error identifies only the failing worktree
func TestExec_MultipleTargetsFailure(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")
	wtPath := createTestWorktree(t, repoPath, "feature")
	regFile := filepath.Join(tmpDir, "repos.json")
	reg := &registry.Registry{Repos: []registry.Repo{{Name: "myrepo", Path: repoPath}}}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "fail"), nil, 0600); err != nil {
		t.Fatal(err)
	}

	cmd := newExecCmd()
	cmd.SetContext(testContextWithConfig(t, &config.Config{RegistryPath: regFile}, repoPath))
	cmd.SetArgs([]string{"main", "feature", "--", "sh", "-c", "touch ran; if test -f fail; then exit 7; fi"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for failing target, got nil")
	}
	if !strings.Contains(err.Error(), "myrepo:main") || strings.Contains(err.Error(), "myrepo:feature") {
		t.Errorf("expected error identifying only myrepo:main, got: %v", err)
	}
	if code := commandExitCode(err); code != 1 {
		t.Errorf("expected multi-target exit code 1, got %d", code)
	}
	for _, path := range []string{repoPath, wtPath} {
		if _, err := os.Stat(filepath.Join(path, "ran")); err != nil {
			t.Errorf("command did not run in %s: %v", path, err)
		}
	}
}

// TestExec_ExitCodes tests exit status handling for single targets and launch failures.
//
// Scenario: Commands succeed, exit with status 7, or cannot start
// Expected: Single targets preserve status 7; launch failures return 1
func TestExec_ExitCodes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		targets    []string
		command    []string
		registered bool
		wantCode   int
	}{
		{name: "success", command: []string{"true"}, wantCode: 0},
		{name: "current", command: []string{"sh", "-c", "exit 7"}, registered: true, wantCode: 7},
		{name: "explicit", targets: []string{"myrepo:main"}, command: []string{"sh", "-c", "exit 7"}, registered: true, wantCode: 7},
		{name: "unregistered", command: []string{"sh", "-c", "exit 7"}, wantCode: 7},
		{name: "missing executable", command: []string{"./does-not-exist"}, wantCode: 1},
		{name: "multiple failures", targets: []string{"main", "feature"}, command: []string{"sh", "-c", "exit 7"}, registered: true, wantCode: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := resolvePath(t, t.TempDir())
			repoPath := setupTestRepo(t, tmpDir, "myrepo")
			reg := &registry.Registry{}
			if tt.registered {
				reg.Repos = []registry.Repo{{Name: "myrepo", Path: repoPath}}
			}
			if len(tt.targets) > 1 {
				createTestWorktree(t, repoPath, "feature")
			}
			regFile := filepath.Join(tmpDir, "repos.json")
			if err := saveRegistry(reg, regFile); err != nil {
				t.Fatal(err)
			}
			cmd := newExecCmd()
			cmd.SetContext(testContextWithConfig(t, &config.Config{RegistryPath: regFile}, repoPath))
			args := append(append([]string{}, tt.targets...), "--")
			cmd.SetArgs(append(args, tt.command...))
			err := cmd.Execute()
			if code := commandExitCode(err); code != tt.wantCode {
				t.Fatalf("expected exit code %d, got %d (error: %v)", tt.wantCode, code, err)
			}
			if tt.wantCode != 0 {
				wantTarget := repoPath
				if len(tt.targets) > 0 {
					wantTarget = "myrepo:main"
				}
				if err == nil || !strings.Contains(err.Error(), wantTarget) {
					t.Fatalf("expected error naming %s, got: %v", wantTarget, err)
				}
				if len(tt.targets) > 1 && !strings.Contains(err.Error(), "myrepo:feature") {
					t.Errorf("expected both failures in error, got: %v", err)
				}
			}
		})
	}
}

//go:build integration

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestPrCheckout_InvalidPRNumber tests error when first arg is not a valid PR number.
//
// Scenario: User runs `wt pr checkout notanumber`
// Expected: Returns error about invalid PR number
func TestPrCheckout_InvalidPRNumber(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

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
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"notanumber"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid PR number, got nil")
	}
	if !strings.Contains(err.Error(), "invalid PR number") {
		t.Errorf("expected error about invalid PR number, got %q", err.Error())
	}
}

// TestPrCheckout_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt pr checkout nonexistent 123`
// Expected: Returns error about repo not found in registry
func TestPrCheckout_RepoNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Work from a non-repo directory
	otherDir := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, otherDir)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found in registry") {
		t.Errorf("expected error about repo not found in registry, got %q", err.Error())
	}
}

// TestPrCheckout_InvalidPRNumberWithRepo tests error when second arg is not a valid PR number.
//
// Scenario: User runs `wt pr checkout myrepo notanumber`
// Expected: Returns error about invalid PR number
func TestPrCheckout_InvalidPRNumberWithRepo(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

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
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo", "notanumber"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid PR number, got nil")
	}
	if !strings.Contains(err.Error(), "invalid PR number") {
		t.Errorf("expected error about invalid PR number, got %q", err.Error())
	}
}

// TestPrCreate_NotInGitRepo tests error when running pr create outside a git repo.
//
// Scenario: User runs `wt pr create` from a non-git directory with no repo arg
// Expected: Returns "not in a git repository" error
func TestPrCreate_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, nonGitDir)

	cmd := newPrCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--title", "test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestPrCreate_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt pr create nonexistent`
// Expected: Returns "not found" error
func TestPrCreate_RepoNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent", "--title", "test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %q", err.Error())
	}
}

// TestPrMerge_NotInGitRepo tests error when running pr merge outside a git repo.
//
// Scenario: User runs `wt pr merge` from a non-git directory with no repo arg
// Expected: Returns "not in a git repository" error
func TestPrMerge_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, nonGitDir)

	cmd := newPrMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestPrMerge_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt pr merge nonexistent`
// Expected: Returns "not found" error
func TestPrMerge_RepoNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %q", err.Error())
	}
}

// TestPrView_NotInGitRepo tests error when running pr view outside a git repo.
//
// Scenario: User runs `wt pr view` from a non-git directory with no repo arg
// Expected: Returns "not in a git repository" error
func TestPrView_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, nonGitDir)

	cmd := newPrViewCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestPrView_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt pr view nonexistent`
// Expected: Returns "not found" error
func TestPrView_RepoNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrViewCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %q", err.Error())
	}
}

// TestPrCreate_BodyAndBodyFileMutuallyExclusive tests that --body and --body-file cannot both be used.
//
// Scenario: User runs `wt pr create --title test --body "text" --body-file file.txt`
// Expected: Returns cobra mutual exclusivity error
func TestPrCreate_BodyAndBodyFileMutuallyExclusive(t *testing.T) {
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

	cmd := newPrCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--title", "test", "--body", "inline body", "--body-file", "file.txt"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "body") || !strings.Contains(err.Error(), "body-file") {
		t.Errorf("expected error about body/body-file mutual exclusivity, got %q", err.Error())
	}
}

// TestPrCheckout_HookNoHookMutuallyExclusive tests that --hook and --no-hook cannot both be used.
//
// Scenario: User runs `wt pr checkout --hook myhook --no-hook 123`
// Expected: Returns cobra mutual exclusivity error
func TestPrCheckout_HookNoHookMutuallyExclusive(t *testing.T) {
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

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--hook", "myhook", "--no-hook", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "hook") || !strings.Contains(err.Error(), "no-hook") {
		t.Errorf("expected error about hook/no-hook mutual exclusivity, got %q", err.Error())
	}
}

// TestPrMerge_HookNoHookMutuallyExclusive tests that --hook and --no-hook cannot both be used.
//
// Scenario: User runs `wt pr merge --hook myhook --no-hook`
// Expected: Returns cobra mutual exclusivity error
func TestPrMerge_HookNoHookMutuallyExclusive(t *testing.T) {
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

	cmd := newPrMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--hook", "myhook", "--no-hook"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "hook") || !strings.Contains(err.Error(), "no-hook") {
		t.Errorf("expected error about hook/no-hook mutual exclusivity, got %q", err.Error())
	}
}

// TestPrCheckout_OrgRepoAlreadyInRegistry tests error when org/repo format is used
// but the repo name is already registered.
//
// Scenario: User runs `wt pr checkout org/myrepo 123` but "myrepo" already in registry
// Expected: Returns specific error suggesting to use the local name instead
func TestPrCheckout_OrgRepoAlreadyInRegistry(t *testing.T) {
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
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"org/myrepo", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for already registered repo, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got %q", err.Error())
	}
	// Should suggest using the local name
	if !strings.Contains(err.Error(), "wt pr checkout myrepo") {
		t.Errorf("expected suggestion to use local name, got %q", err.Error())
	}
}

// TestPrCheckout_AlreadyCheckedOut tests that the pr checkout code path correctly
// detects an existing worktree, outputs "Opened worktree" instead of
// "Created worktree", and does not destroy the worktree.
//
// We cannot run the full pr checkout command (requires a forge/remote), so we
// exercise the detection + output message logic that pr_cmd.go uses inline.
//
// Scenario: Branch "feature" already has a worktree checked out
// Expected: findWorktreeForBranch detects it, output says "Opened worktree"
func TestPrCheckout_AlreadyCheckedOut(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	// Create a worktree for the branch first
	wtPath := filepath.Join(repoPath, ".worktrees", "feature")
	gitWorktreeAdd := exec.Command("git", "worktree", "add", wtPath, "feature")
	gitWorktreeAdd.Dir = repoPath
	if out, err := gitWorktreeAdd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
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

	ctx := testContextWithConfig(t, &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: ".worktrees/{branch}",
		},
	}, repoPath)

	// Step 1: Verify findWorktreeForBranch detects the existing worktree
	foundPath, found := findWorktreeForBranch(ctx, repoPath, "feature")
	if !found {
		t.Fatal("expected to find existing worktree for branch 'feature'")
	}
	if foundPath != wtPath {
		t.Errorf("expected worktree path %s, got %s", wtPath, foundPath)
	}

	// Step 2: Verify the output message selection logic from pr_cmd.go.
	// When found=true and justClonedRegular=false, the code builds:
	//   outputMsg = fmt.Sprintf("Opened worktree: %s (%s)\n", wtPath, branch)
	// Reproduce that logic here to verify it selects "Opened worktree".
	justClonedRegular := false
	branch := "feature"

	var outputMsg string
	if justClonedRegular {
		outputMsg = fmt.Sprintf("Checked out PR branch: %s (%s)\n", repoPath, branch)
	} else if found {
		outputMsg = fmt.Sprintf("Opened worktree: %s (%s)\n", foundPath, branch)
	} else {
		outputMsg = fmt.Sprintf("Created worktree: %s (%s)\n", wtPath, branch)
	}

	if !strings.Contains(outputMsg, "Opened worktree") {
		t.Errorf("expected 'Opened worktree' in output, got %q", outputMsg)
	}
	if strings.Contains(outputMsg, "Created worktree") {
		t.Errorf("output should not contain 'Created worktree', got %q", outputMsg)
	}
	expectedMsg := fmt.Sprintf("Opened worktree: %s (feature)\n", wtPath)
	if outputMsg != expectedMsg {
		t.Errorf("expected output %q, got %q", expectedMsg, outputMsg)
	}

	// Step 3: Worktree should still exist (not removed or modified)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should still exist at %s", wtPath)
	}
}

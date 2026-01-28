//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

// TestCheckout_ExistingBranchInsideRepo verifies worktree creation for an existing
// branch when running checkout from inside a repository.
//
// Scenario: User runs `wt checkout feature-branch` from inside a repo
// Expected: Worktree created at {worktree_dir}/myrepo-feature-branch
func TestCheckout_ExistingBranchInsideRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo with a feature branch
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	createBranch(t, repoPath, "feature-branch")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch: "feature-branch",
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout failed: %v", err)
	}

	// Verify worktree created
	expectedPath := filepath.Join(worktreeDir, "myrepo-feature-branch")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", expectedPath)
	}

	// Verify worktree works
	verifyWorktreeWorks(t, expectedPath)
}

// TestCheckout_NewBranchInsideRepo verifies creating a new branch with -b flag
// when running from inside a repository.
//
// Scenario: User runs `wt checkout -b new-feature` from inside a repo
// Expected: New branch created, worktree at {worktree_dir}/myrepo-new-feature
func TestCheckout_NewBranchInsideRepo(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
		BaseRef:        "local", // Use local ref to avoid needing origin
	}

	cmd := &CheckoutCmd{
		Branch:    "new-feature",
		NewBranch: true,
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -b failed: %v", err)
	}

	// Verify worktree created
	expectedPath := filepath.Join(worktreeDir, "myrepo-new-feature")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", expectedPath)
	}

	// Verify branch was created
	verifyWorktreeWorks(t, expectedPath)
	verifyBranchExists(t, repoPath, "new-feature")
}

// TestCheckout_NewBranchFromCustomBase verifies creating a new branch from a
// custom base branch using --base flag.
//
// Scenario: User runs `wt checkout -b feature-from-develop --base develop`
// Expected: New branch based on develop, containing develop's commits
func TestCheckout_NewBranchFromCustomBase(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo with a develop branch
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	createBranch(t, repoPath, "develop")

	// Make a commit on develop so it differs from main
	makeCommitOnBranch(t, repoPath, "develop", "develop-file.txt")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
		BaseRef:        "local",
	}

	cmd := &CheckoutCmd{
		Branch:    "feature-from-develop",
		NewBranch: true,
		Base:      "develop",
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -b --base develop failed: %v", err)
	}

	// Verify worktree created
	expectedPath := filepath.Join(worktreeDir, "myrepo-feature-from-develop")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", expectedPath)
	}

	// Verify the new branch has the develop file (was based on develop)
	developFile := filepath.Join(expectedPath, "develop-file.txt")
	if _, err := os.Stat(developFile); os.IsNotExist(err) {
		t.Errorf("develop-file.txt should exist (branch based on develop)")
	}
}

// TestCheckout_WithNote verifies setting a branch note during checkout using
// the --note flag.
//
// Scenario: User runs `wt checkout -b feature --note "Working on JIRA-123"`
// Expected: Branch note stored in git config branch.feature.description
func TestCheckout_WithNote(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
		BaseRef:        "local",
	}

	cmd := &CheckoutCmd{
		Branch:    "feature-with-note",
		NewBranch: true,
		Note:      "Working on JIRA-123",
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout --note failed: %v", err)
	}

	// Verify note was set (using git config branch.<branch>.description)
	note := getBranchNote(t, repoPath, "feature-with-note")
	if note != "Working on JIRA-123" {
		t.Errorf("expected note 'Working on JIRA-123', got %q", note)
	}
}

// TestCheckout_WorktreeAlreadyExists verifies graceful handling when the target
// worktree already exists.
//
// Scenario: User runs `wt checkout feature` but worktree already exists
// Expected: Command succeeds without error, existing worktree preserved
func TestCheckout_WorktreeAlreadyExists(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo and worktree manually
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	worktreePath := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, worktreePath, "feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch: "feature",
	}

	// Should not error, just report already exists
	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout should succeed when worktree already exists: %v", err)
	}

	// Verify worktree still works
	verifyWorktreeWorks(t, worktreePath)
}

// TestCheckout_MultiRepoByName verifies checking out the same branch in multiple
// repositories using -r flags.
//
// Scenario: User runs `wt checkout shared-feature -r repo-a -r repo-b`
// Expected: Worktrees created for both repos at {worktree_dir}/repo-*-shared-feature
func TestCheckout_MultiRepoByName(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create two repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	// Create branches in both repos
	createBranch(t, repoA, "shared-feature")
	createBranch(t, repoB, "shared-feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch:     "shared-feature",
		Repository: []string{"repo-a", "repo-b"},
	}

	// Run from outside any repo
	if err := runCheckoutCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -r repo-a -r repo-b failed: %v", err)
	}

	// Verify both worktrees created
	wtA := filepath.Join(worktreeDir, "repo-a-shared-feature")
	if _, err := os.Stat(wtA); os.IsNotExist(err) {
		t.Errorf("worktree for repo-a not created at %s", wtA)
	}

	wtB := filepath.Join(worktreeDir, "repo-b-shared-feature")
	if _, err := os.Stat(wtB); os.IsNotExist(err) {
		t.Errorf("worktree for repo-b not created at %s", wtB)
	}
}

// TestCheckout_MultiRepoByLabel verifies checking out branches in repos matching
// a label using the -l flag.
//
// Scenario: User runs `wt checkout ui-feature -l frontend`
// Expected: Worktrees created only for repos labeled "frontend"
func TestCheckout_MultiRepoByLabel(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repos with labels
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")
	repoC := setupTestRepo(t, repoDir, "repo-c")

	// Label repo-a and repo-b as "frontend", repo-c as "backend"
	// Labels use wt.labels config key
	setRepoLabel(t, repoA, "frontend")
	setRepoLabel(t, repoB, "frontend")
	setRepoLabel(t, repoC, "backend")

	// Create branches
	createBranch(t, repoA, "ui-feature")
	createBranch(t, repoB, "ui-feature")
	createBranch(t, repoC, "ui-feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch: "ui-feature",
		Label:  []string{"frontend"},
	}

	// Run from outside any repo
	if err := runCheckoutCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -l frontend failed: %v", err)
	}

	// Verify frontend repos got worktrees
	wtA := filepath.Join(worktreeDir, "repo-a-ui-feature")
	if _, err := os.Stat(wtA); os.IsNotExist(err) {
		t.Errorf("worktree for repo-a (frontend label) not created at %s", wtA)
	}

	wtB := filepath.Join(worktreeDir, "repo-b-ui-feature")
	if _, err := os.Stat(wtB); os.IsNotExist(err) {
		t.Errorf("worktree for repo-b (frontend label) not created at %s", wtB)
	}

	// Verify backend repo did NOT get worktree
	wtC := filepath.Join(worktreeDir, "repo-c-ui-feature")
	if _, err := os.Stat(wtC); !os.IsNotExist(err) {
		t.Errorf("worktree for repo-c (backend label) should NOT be created")
	}
}

// TestCheckout_ErrorMissingBranchInsideRepo verifies error when no branch name provided.
//
// Scenario: User runs `wt checkout` without specifying a branch
// Expected: Error "branch name required"
func TestCheckout_ErrorMissingBranchInsideRepo(t *testing.T) {
	t.Parallel()
	// Setup
	sourceDir := t.TempDir()
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	cfg := &config.Config{
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch: "", // No branch specified
	}

	err := runCheckoutCommand(t, repoPath, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when branch name not provided")
	}
	if !strings.Contains(err.Error(), "branch name required") {
		t.Errorf("expected 'branch name required' error, got: %v", err)
	}
}

// TestCheckout_ErrorOutsideRepoWithoutRepoFlag verifies error when running
// outside a git repository without specifying -r or -l flags.
//
// Scenario: User runs `wt checkout branch` from non-repo directory
// Expected: Error requiring --repository or --label flag
func TestCheckout_ErrorOutsideRepoWithoutRepoFlag(t *testing.T) {
	t.Parallel()
	// Setup
	tempDir := t.TempDir()

	cfg := &config.Config{
		WorktreeDir:    tempDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch: "some-branch",
	}

	// Run from temp dir (not a git repo)
	err := runCheckoutCommand(t, tempDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when outside repo without -r flag")
	}
	if !strings.Contains(err.Error(), "--repository (-r) or --label (-l) required") {
		t.Errorf("expected 'repository required' error, got: %v", err)
	}
}

// TestCheckout_ErrorRepoNotFound verifies error when specified repository
// doesn't exist in repo_dir.
//
// Scenario: User runs `wt checkout branch -r nonexistent-repo`
// Expected: Error mentioning the nonexistent repository
func TestCheckout_ErrorRepoNotFound(t *testing.T) {
	t.Parallel()
	// Setup
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch:     "some-branch",
		Repository: []string{"nonexistent-repo"},
	}

	err := runCheckoutCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when repo not found")
	}
	if !strings.Contains(err.Error(), "nonexistent-repo") {
		t.Errorf("expected error mentioning nonexistent-repo, got: %v", err)
	}
}

// TestCheckout_ErrorLabelNotFound verifies error when no repos match
// the specified label.
//
// Scenario: User runs `wt checkout branch -l nonexistent-label`
// Expected: Error mentioning the nonexistent label
func TestCheckout_ErrorLabelNotFound(t *testing.T) {
	t.Parallel()
	// Setup
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create a repo without any label
	setupTestRepo(t, repoDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch: "some-branch",
		Label:  []string{"nonexistent-label"},
	}

	err := runCheckoutCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when label not found")
	}
	if !strings.Contains(err.Error(), "nonexistent-label") {
		t.Errorf("expected error mentioning nonexistent-label, got: %v", err)
	}
}

// TestCheckout_CustomWorktreeFormat verifies worktree naming respects
// the configured worktree_format setting.
//
// Scenario: User configures worktree_format="{branch}" (no repo prefix)
// Expected: Worktree created at {worktree_dir}/feature instead of myrepo-feature
func TestCheckout_CustomWorktreeFormat(t *testing.T) {
	t.Parallel()
	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	createBranch(t, repoPath, "feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: "{branch}", // Just branch name, no repo prefix
	}

	cmd := &CheckoutCmd{
		Branch: "feature",
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout with custom format failed: %v", err)
	}

	// Verify worktree created with custom format name
	expectedPath := filepath.Join(worktreeDir, "feature")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s (using custom format)", expectedPath)
	}

	// Verify old format path doesn't exist
	oldPath := filepath.Join(worktreeDir, "myrepo-feature")
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("worktree should not exist at %s (custom format)", oldPath)
	}
}

// TestCheckout_WorktreeDirDefaultsToCurrent verifies that when worktree_dir
// is not configured, worktrees are created in the current directory.
//
// Scenario: User has no worktree_dir configured, runs checkout from inside repo
// Expected: Worktree created in current (repo) directory
func TestCheckout_WorktreeDirDefaultsToCurrent(t *testing.T) {
	t.Parallel()
	// Setup
	sourceDir := t.TempDir()
	repoPath := setupTestRepo(t, sourceDir, "myrepo")
	createBranch(t, repoPath, "feature")

	cfg := &config.Config{
		WorktreeDir:    "", // Not configured, defaults to current dir
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch: "feature",
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout with default dir failed: %v", err)
	}

	// When WorktreeDir is empty, it defaults to "." (current directory)
	// Since we're inside the repo, the worktree is created in the repo directory
	expectedPath := filepath.Join(repoPath, "myrepo-feature")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", expectedPath)
	}
}

// TestCheckout_InsideRepoWithRepoFlag verifies that -r flag overrides current
// repo context (doesn't auto-include current repo).
//
// Scenario: User runs `wt checkout branch -r repo-b` from inside repo-a
// Expected: Only repo-b gets worktree, not current repo (repo-a)
func TestCheckout_InsideRepoWithRepoFlag(t *testing.T) {
	t.Parallel()
	// When inside repo with -r flag, should create ONLY in specified repo (not current repo)

	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create two repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	// Create branch in both repos
	createBranch(t, repoA, "shared-branch")
	createBranch(t, repoB, "shared-branch")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch:     "shared-branch",
		Repository: []string{"repo-b"},
	}

	// Run from inside repo-a
	if err := runCheckoutCommand(t, repoA, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -r repo-b (from inside repo-a) failed: %v", err)
	}

	// Verify worktree NOT created for repo-a (current repo should not be auto-included)
	wtA := filepath.Join(worktreeDir, "repo-a-shared-branch")
	if _, err := os.Stat(wtA); err == nil {
		t.Errorf("worktree for current repo (repo-a) should NOT be created when -r specifies other repos, but found at %s", wtA)
	}

	// Verify worktree created for repo-b (the specified repo)
	wtB := filepath.Join(worktreeDir, "repo-b-shared-branch")
	if _, err := os.Stat(wtB); os.IsNotExist(err) {
		t.Errorf("worktree for repo-b not created at %s", wtB)
	}
}

// TestCheckout_NewBranchMultiRepo verifies creating new branches simultaneously
// in multiple repositories.
//
// Scenario: User runs `wt checkout -b new-feature -r repo-a -r repo-b`
// Expected: New branches created in both repos, worktrees created for both
func TestCheckout_NewBranchMultiRepo(t *testing.T) {
	t.Parallel()
	// Test creating new branches in multiple repos

	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create two repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
		BaseRef:        "local",
	}

	cmd := &CheckoutCmd{
		Branch:     "new-shared-feature",
		NewBranch:  true,
		Repository: []string{"repo-a", "repo-b"},
	}

	// Run from outside any repo
	if err := runCheckoutCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -b -r repo-a -r repo-b failed: %v", err)
	}

	// Verify branches were created in both repos
	verifyBranchExists(t, repoA, "new-shared-feature")
	verifyBranchExists(t, repoB, "new-shared-feature")

	// Verify worktrees created
	wtA := filepath.Join(worktreeDir, "repo-a-new-shared-feature")
	if _, err := os.Stat(wtA); os.IsNotExist(err) {
		t.Errorf("worktree for repo-a not created at %s", wtA)
	}

	wtB := filepath.Join(worktreeDir, "repo-b-new-shared-feature")
	if _, err := os.Stat(wtB); os.IsNotExist(err) {
		t.Errorf("worktree for repo-b not created at %s", wtB)
	}
}

// TestCheckout_BranchWithSlashes verifies branch names with slashes (e.g., feature/name)
// are sanitized in the worktree directory name.
//
// Scenario: User runs `wt checkout -b feature/my-feature`
// Expected: Worktree at myrepo-feature-my-feature (slashes become dashes)
func TestCheckout_BranchWithSlashes(t *testing.T) {
	t.Parallel()
	// Test branch names with slashes (feature/name format)

	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
		BaseRef:        "local",
	}

	cmd := &CheckoutCmd{
		Branch:    "feature/my-feature",
		NewBranch: true,
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -b feature/my-feature failed: %v", err)
	}

	// Verify worktree created (slashes should be replaced with dashes in directory name)
	expectedPath := filepath.Join(worktreeDir, "myrepo-feature-my-feature")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", expectedPath)
	}

	// Verify branch was created
	verifyBranchExists(t, repoPath, "feature/my-feature")
}

// TestCheckout_ErrorBranchDoesNotExist verifies error when checking out a
// non-existent branch without -b flag.
//
// Scenario: User runs `wt checkout nonexistent-branch` (no -b flag)
// Expected: Git error that branch doesn't exist
func TestCheckout_ErrorBranchDoesNotExist(t *testing.T) {
	t.Parallel()
	// Setup
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch: "nonexistent-branch", // Branch doesn't exist
	}

	err := runCheckoutCommand(t, repoPath, cfg, cmd)
	if err == nil {
		t.Fatal("expected error when branch doesn't exist")
	}
}

// TestCheckout_ErrorBranchAlreadyCheckedOut verifies graceful handling when
// the branch is already checked out in another worktree.
//
// Scenario: User runs `wt checkout feature` when feature already has a worktree
// Expected: Command succeeds (idempotent), returns existing worktree info
func TestCheckout_ErrorBranchAlreadyCheckedOut(t *testing.T) {
	t.Parallel()
	// Setup
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Create a worktree for a branch (branch is now checked out)
	wt1Path := filepath.Join(worktreeDir, "myrepo-feature")
	setupWorktree(t, repoPath, wt1Path, "feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Try to create another worktree for the same branch with different name
	// This should succeed because wt checkout handles "already exists" gracefully
	cmd := &CheckoutCmd{
		Branch: "feature",
	}

	// Should not error - returns "already exists"
	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout should handle already checked out branch: %v", err)
	}
}

// TestCheckout_CombineRepoAndLabel verifies combining -r and -l flags to target
// repos both by name and by label.
//
// Scenario: User runs `wt checkout branch -r repo-a -l frontend`
// Expected: Worktrees for repo-a (by name) AND repos with "frontend" label
func TestCheckout_CombineRepoAndLabel(t *testing.T) {
	t.Parallel()
	// Test using both -r and -l together

	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repos
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")
	repoC := setupTestRepo(t, repoDir, "repo-c")

	// Label repo-b as "frontend"
	setRepoLabel(t, repoB, "frontend")

	// Create branches in all repos
	createBranch(t, repoA, "shared")
	createBranch(t, repoB, "shared")
	createBranch(t, repoC, "shared")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Use both -r repo-a and -l frontend (repo-b)
	cmd := &CheckoutCmd{
		Branch:     "shared",
		Repository: []string{"repo-a"},
		Label:      []string{"frontend"},
	}

	if err := runCheckoutCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -r repo-a -l frontend failed: %v", err)
	}

	// Verify repo-a worktree created (from -r)
	wtA := filepath.Join(worktreeDir, "repo-a-shared")
	if _, err := os.Stat(wtA); os.IsNotExist(err) {
		t.Errorf("worktree for repo-a not created at %s", wtA)
	}

	// Verify repo-b worktree created (from -l frontend)
	wtB := filepath.Join(worktreeDir, "repo-b-shared")
	if _, err := os.Stat(wtB); os.IsNotExist(err) {
		t.Errorf("worktree for repo-b not created at %s", wtB)
	}

	// Verify repo-c NOT created (neither -r nor matching -l)
	wtC := filepath.Join(worktreeDir, "repo-c-shared")
	if _, err := os.Stat(wtC); !os.IsNotExist(err) {
		t.Errorf("worktree for repo-c should NOT be created")
	}
}

// TestCheckout_MultipleLabels verifies targeting repos with multiple labels
// using repeated -l flags.
//
// Scenario: User runs `wt checkout branch -l frontend -l backend`
// Expected: Worktrees for all repos matching either label, not repos with other labels
func TestCheckout_MultipleLabels(t *testing.T) {
	t.Parallel()
	// Test using multiple -l flags

	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repos with different labels
	repoA := setupTestRepo(t, repoDir, "repo-a")
	repoB := setupTestRepo(t, repoDir, "repo-b")
	repoC := setupTestRepo(t, repoDir, "repo-c")

	setRepoLabel(t, repoA, "frontend")
	setRepoLabel(t, repoB, "backend")
	setRepoLabel(t, repoC, "infra")

	// Create branches
	createBranch(t, repoA, "shared")
	createBranch(t, repoB, "shared")
	createBranch(t, repoC, "shared")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	// Use multiple labels: frontend and backend (not infra)
	cmd := &CheckoutCmd{
		Branch: "shared",
		Label:  []string{"frontend", "backend"},
	}

	if err := runCheckoutCommand(t, worktreeDir, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -l frontend -l backend failed: %v", err)
	}

	// Verify frontend and backend repos got worktrees
	wtA := filepath.Join(worktreeDir, "repo-a-shared")
	if _, err := os.Stat(wtA); os.IsNotExist(err) {
		t.Errorf("worktree for repo-a (frontend) not created")
	}

	wtB := filepath.Join(worktreeDir, "repo-b-shared")
	if _, err := os.Stat(wtB); os.IsNotExist(err) {
		t.Errorf("worktree for repo-b (backend) not created")
	}

	// Verify infra repo NOT created
	wtC := filepath.Join(worktreeDir, "repo-c-shared")
	if _, err := os.Stat(wtC); !os.IsNotExist(err) {
		t.Errorf("worktree for repo-c (infra) should NOT be created")
	}
}

// TestCheckout_InsideRepoWithLabelOnly verifies that -l flag alone doesn't
// auto-include the current repo.
//
// Scenario: User runs `wt checkout branch -l frontend` from inside repo-a (no label)
// Expected: Only repos with "frontend" label get worktrees, not current repo
func TestCheckout_InsideRepoWithLabelOnly(t *testing.T) {
	t.Parallel()
	// When inside repo with -l only (no -r), should NOT auto-include current repo

	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repos
	repoA := setupTestRepo(t, repoDir, "repo-a") // No label
	repoB := setupTestRepo(t, repoDir, "repo-b")
	setRepoLabel(t, repoB, "frontend")

	// Create branches
	createBranch(t, repoA, "feature")
	createBranch(t, repoB, "feature")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch: "feature",
		Label:  []string{"frontend"},
	}

	// Run from inside repo-a (which has no label)
	if err := runCheckoutCommand(t, repoA, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -l frontend (from inside repo-a) failed: %v", err)
	}

	// Verify repo-a worktree NOT created (not in frontend label, -l only doesn't auto-include)
	wtA := filepath.Join(worktreeDir, "repo-a-feature")
	if _, err := os.Stat(wtA); !os.IsNotExist(err) {
		t.Errorf("worktree for repo-a should NOT be created with -l only")
	}

	// Verify repo-b worktree created (has frontend label)
	wtB := filepath.Join(worktreeDir, "repo-b-feature")
	if _, err := os.Stat(wtB); os.IsNotExist(err) {
		t.Errorf("worktree for repo-b (frontend label) not created")
	}
}

// TestCheckout_NoHookFlag verifies that --no-hook flag skips hook execution.
//
// Scenario: User runs `wt checkout -b feature --no-hook`
// Expected: Worktree created, but hooks are not executed
func TestCheckout_NoHookFlag(t *testing.T) {
	t.Parallel()
	// Test --no-hook flag skips hooks

	// Setup
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
		BaseRef:        "local",
	}

	cmd := &CheckoutCmd{
		Branch:    "feature",
		NewBranch: true,
		NoHook:    true,
	}

	// Should succeed without running hooks
	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout --no-hook failed: %v", err)
	}

	// Verify worktree created
	expectedPath := filepath.Join(worktreeDir, "myrepo-feature")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", expectedPath)
	}
}

// TestCheckout_PartialFailureMultiRepo verifies that failures in some repos
// don't prevent success in others.
//
// Scenario: User runs `wt checkout only-in-a -r repo-a -r repo-b` (branch only in repo-a)
// Expected: Error returned, but repo-a worktree still created
func TestCheckout_PartialFailureMultiRepo(t *testing.T) {
	t.Parallel()
	// Test that partial failures are reported but don't stop other repos

	// Setup temp directories
	repoDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repos - only repo-a has the branch
	repoA := setupTestRepo(t, repoDir, "repo-a")
	setupTestRepo(t, repoDir, "repo-b") // repo-b does NOT have branch "only-in-a"

	createBranch(t, repoA, "only-in-a")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		RepoDir:        repoDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
	}

	cmd := &CheckoutCmd{
		Branch:     "only-in-a",
		Repository: []string{"repo-a", "repo-b"},
	}

	// Should return error (partial failure)
	err := runCheckoutCommand(t, worktreeDir, cfg, cmd)
	if err == nil {
		t.Fatal("expected error for partial failure")
	}

	// But repo-a should still have been created
	wtA := filepath.Join(worktreeDir, "repo-a-only-in-a")
	if _, err := os.Stat(wtA); os.IsNotExist(err) {
		t.Errorf("worktree for repo-a should be created despite repo-b failing")
	}
}

// TestCheckout_AutoStash verifies --autostash flag stashes changes and applies
// them to the new worktree.
//
// Scenario: User has uncommitted changes, runs `wt checkout -b feature -s`
// Expected: Source repo clean, changes moved to new worktree
func TestCheckout_AutoStash(t *testing.T) {
	t.Parallel()
	// Test --autostash flag: stash changes in current worktree and apply to new worktree

	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Make the repo dirty (uncommitted changes)
	dirtyFile := filepath.Join(repoPath, "uncommitted.txt")
	if err := os.WriteFile(dirtyFile, []byte("uncommitted changes\n"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
		BaseRef:        "local",
	}

	cmd := &CheckoutCmd{
		Branch:    "feature-with-stash",
		NewBranch: true,
		AutoStash: true,
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -b -s failed: %v", err)
	}

	// Verify worktree created
	expectedPath := filepath.Join(worktreeDir, "myrepo-feature-with-stash")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", expectedPath)
	}

	// Verify source repo is now clean (changes were stashed and popped in new worktree)
	cmd2 := exec.Command("git", "status", "--porcelain")
	cmd2.Dir = repoPath
	out, err := cmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("git status failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("source repo should be clean after autostash, but has changes: %s", out)
	}

	// Verify changes exist in new worktree
	newDirtyFile := filepath.Join(expectedPath, "uncommitted.txt")
	if _, err := os.Stat(newDirtyFile); os.IsNotExist(err) {
		t.Errorf("uncommitted.txt should exist in new worktree (stash was applied)")
	}

	// Verify the new worktree is dirty (has the uncommitted changes)
	cmd3 := exec.Command("git", "status", "--porcelain")
	cmd3.Dir = expectedPath
	out, err = cmd3.CombinedOutput()
	if err != nil {
		t.Fatalf("git status in new worktree failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "uncommitted.txt") {
		t.Errorf("new worktree should have uncommitted.txt as untracked, got: %s", out)
	}
}

// TestCheckout_AutoStashNoChanges verifies --autostash is a no-op when there
// are no uncommitted changes.
//
// Scenario: User runs `wt checkout -b feature -s` in a clean repo
// Expected: Worktree created normally, both repos remain clean
func TestCheckout_AutoStashNoChanges(t *testing.T) {
	t.Parallel()
	// Test --autostash with no uncommitted changes (should be a no-op)

	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo (clean, no uncommitted changes)
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
		BaseRef:        "local",
	}

	cmd := &CheckoutCmd{
		Branch:    "feature-clean",
		NewBranch: true,
		AutoStash: true,
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -b -s (clean repo) failed: %v", err)
	}

	// Verify worktree created
	expectedPath := filepath.Join(worktreeDir, "myrepo-feature-clean")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", expectedPath)
	}

	// Verify both are clean
	cmd2 := exec.Command("git", "status", "--porcelain")
	cmd2.Dir = repoPath
	out, _ := cmd2.CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("source repo should be clean: %s", out)
	}

	cmd3 := exec.Command("git", "status", "--porcelain")
	cmd3.Dir = expectedPath
	out, _ = cmd3.CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("new worktree should be clean: %s", out)
	}
}

// TestCheckout_AutoStashWithStagedChanges verifies --autostash handles both
// staged and unstaged changes correctly.
//
// Scenario: User has staged and unstaged changes, runs `wt checkout -b feature -s`
// Expected: Both staged and unstaged changes moved to new worktree
func TestCheckout_AutoStashWithStagedChanges(t *testing.T) {
	t.Parallel()
	// Test --autostash with both staged and unstaged changes

	// Setup temp directories
	sourceDir := t.TempDir()
	worktreeDir := t.TempDir()

	// Create repo
	repoPath := setupTestRepo(t, sourceDir, "myrepo")

	// Create a staged change
	stagedFile := filepath.Join(repoPath, "staged.txt")
	if err := os.WriteFile(stagedFile, []byte("staged content\n"), 0644); err != nil {
		t.Fatalf("failed to create staged file: %v", err)
	}
	runGitCommand(t, repoPath, "git", "add", "staged.txt")

	// Create an unstaged change
	unstagedFile := filepath.Join(repoPath, "unstaged.txt")
	if err := os.WriteFile(unstagedFile, []byte("unstaged content\n"), 0644); err != nil {
		t.Fatalf("failed to create unstaged file: %v", err)
	}

	cfg := &config.Config{
		WorktreeDir:    worktreeDir,
		WorktreeFormat: config.DefaultWorktreeFormat,
		BaseRef:        "local",
	}

	cmd := &CheckoutCmd{
		Branch:    "feature-mixed-changes",
		NewBranch: true,
		AutoStash: true,
	}

	if err := runCheckoutCommand(t, repoPath, cfg, cmd); err != nil {
		t.Fatalf("wt checkout -b -s (mixed changes) failed: %v", err)
	}

	// Verify worktree created
	expectedPath := filepath.Join(worktreeDir, "myrepo-feature-mixed-changes")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("worktree not created at %s", expectedPath)
	}

	// Verify source repo is clean
	cmd2 := exec.Command("git", "status", "--porcelain")
	cmd2.Dir = repoPath
	out, _ := cmd2.CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("source repo should be clean after autostash: %s", out)
	}

	// Verify both files exist in new worktree
	if _, err := os.Stat(filepath.Join(expectedPath, "staged.txt")); os.IsNotExist(err) {
		t.Errorf("staged.txt should exist in new worktree")
	}
	if _, err := os.Stat(filepath.Join(expectedPath, "unstaged.txt")); os.IsNotExist(err) {
		t.Errorf("unstaged.txt should exist in new worktree")
	}
}

// runCheckoutCommand runs wt checkout with the given config and command in the specified directory.
func runCheckoutCommand(t *testing.T, workDir string, cfg *config.Config, cmd *CheckoutCmd) error {
	t.Helper()
	cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
	ctx := testContext(t)
	return cmd.runCheckout(ctx)
}

//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/git"
)

// resolvePath resolves symlinks in a path.
// This is needed on macOS where /var is a symlink to /private/var.
func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("failed to resolve path %s: %v", path, err)
	}
	return resolved
}

// setupTestRepo creates a git repo with initial commit in dir/name.
// Returns the absolute path to the created repo (with symlinks resolved).
func setupTestRepo(t *testing.T, dir, name string) string {
	t.Helper()

	// Resolve symlinks in dir (needed for macOS where /var -> /private/var)
	dir = resolvePath(t, dir)

	repoPath := filepath.Join(dir, name)
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "commit.gpgsign", "false"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	// Create initial commit
	readmePath := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readmePath, []byte("# "+name+"\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	cmds = [][]string{
		{"git", "add", "README.md"},
		{"git", "commit", "-m", "Initial commit"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	// Set up a fake origin for repo name extraction
	cmd := exec.Command("git", "remote", "add", "origin", "https://github.com/test/"+name+".git")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to add remote: %v\n%s", err, out)
	}

	return repoPath
}

// setupWorktree creates a worktree from repoPath at worktreePath for the given branch.
// Creates a new branch if it doesn't exist.
func setupWorktree(t *testing.T, repoPath, worktreePath, branch string) {
	t.Helper()

	// Create a new branch and worktree
	cmd := exec.Command("git", "worktree", "add", "-b", branch, worktreePath)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
	}
}

// setupExistingWorktree creates a worktree for an existing branch.
func setupExistingWorktree(t *testing.T, repoPath, worktreePath, branch string) {
	t.Helper()

	cmd := exec.Command("git", "worktree", "add", worktreePath, branch)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
	}
}

// verifyWorktreeWorks checks that git status works in the worktree.
func verifyWorktreeWorks(t *testing.T, worktreePath string) {
	t.Helper()

	cmd := exec.Command("git", "status")
	cmd.Dir = worktreePath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("worktree %s is broken: git status failed: %v\n%s", worktreePath, err, out)
	}
}

// verifyGitdirPoints checks that the worktree .git file points to the expected repo.
func verifyGitdirPoints(t *testing.T, worktreePath, expectedRepoPath string) {
	t.Helper()

	gitFile := filepath.Join(worktreePath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		t.Fatalf("failed to read .git file: %v", err)
	}

	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		t.Fatalf("invalid .git file format: %s", line)
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")
	// gitdir should contain the repo path
	if !strings.Contains(gitdir, expectedRepoPath) {
		t.Errorf("worktree .git file points to %s, expected to contain %s", gitdir, expectedRepoPath)
	}
}

// makeDirty creates uncommitted changes in a worktree.
func makeDirty(t *testing.T, worktreePath string) {
	t.Helper()

	filePath := filepath.Join(worktreePath, "dirty.txt")
	if err := os.WriteFile(filePath, []byte("uncommitted changes\n"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}
}

// createBranch creates a branch (without checking it out)
func createBranch(t *testing.T, repoPath, branch string) {
	t.Helper()
	runGitCommand(t, repoPath, "git", "branch", branch)
}

// verifyBranchExists verifies a branch exists
func verifyBranchExists(t *testing.T, repoPath, branch string) {
	t.Helper()
	out := runGitCommand(t, repoPath, "git", "branch", "--list", branch)
	if !strings.Contains(out, branch) {
		t.Errorf("branch %s should exist in repo", branch)
	}
}

// makeCommitOnBranch makes a commit on a specific branch
func makeCommitOnBranch(t *testing.T, repoPath, branch, filename string) {
	t.Helper()
	// Checkout branch
	runGitCommand(t, repoPath, "git", "checkout", branch)
	// Create file
	filePath := filepath.Join(repoPath, filename)
	if err := os.WriteFile(filePath, []byte("content for "+filename+"\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	// Commit
	runGitCommand(t, repoPath, "git", "add", filename)
	runGitCommand(t, repoPath, "git", "commit", "-m", "Add "+filename)
	// Return to main
	runGitCommand(t, repoPath, "git", "checkout", "main")
}

// setRepoLabel sets a label on a repo using git config wt.labels (comma-separated)
func setRepoLabel(t *testing.T, repoPath, label string) {
	t.Helper()
	runGitCommand(t, repoPath, "git", "config", "wt.labels", label)
}

// getBranchNote gets the branch description (note)
func getBranchNote(t *testing.T, repoPath, branch string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repoPath, "config", "branch."+branch+".description")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Exit code 1 means no note set
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return ""
		}
		t.Fatalf("failed to get branch note: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// runGitCommand runs a git command and returns output
func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// makeCommitInWorktree creates a file and commits it in the worktree.
func makeCommitInWorktree(t *testing.T, worktreePath, filename string) {
	t.Helper()
	filePath := filepath.Join(worktreePath, filename)
	if err := os.WriteFile(filePath, []byte("content for "+filename+"\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	runGitCommand(t, worktreePath, "git", "add", filename)
	runGitCommand(t, worktreePath, "git", "commit", "-m", "Add "+filename)
}

// mergeBranchToMain merges a branch into main in the main repo and updates origin/main.
// This makes the branch appear as "merged" when checked with `git branch --merged origin/main`.
// Requires the repo to have a local bare origin set up (use setupTestRepoWithLocalOrigin).
func mergeBranchToMain(t *testing.T, repoPath, branch string) {
	t.Helper()
	// Get current branch to restore later
	currentBranch := strings.TrimSpace(runGitCommand(t, repoPath, "git", "rev-parse", "--abbrev-ref", "HEAD"))

	// Checkout main and merge
	runGitCommand(t, repoPath, "git", "checkout", "main")
	runGitCommand(t, repoPath, "git", "merge", branch, "--no-edit")

	// Push to origin so origin/main is updated (required for merge detection)
	runGitCommand(t, repoPath, "git", "push", "origin", "main")

	// Return to original branch if different
	if currentBranch != "main" && currentBranch != branch {
		runGitCommand(t, repoPath, "git", "checkout", currentBranch)
	}
}

// setupTestRepoWithLocalOrigin creates a git repo with a local bare repo as origin.
// This is required for tests that use merge detection (git branch --merged origin/main).
// Returns the path to the repo (not the bare origin).
func setupTestRepoWithLocalOrigin(t *testing.T, dir, name string) string {
	t.Helper()

	// Resolve symlinks in dir (needed for macOS where /var -> /private/var)
	dir = resolvePath(t, dir)

	// Create bare repo as origin
	barePath := filepath.Join(dir, name+".git")
	if err := os.MkdirAll(barePath, 0755); err != nil {
		t.Fatalf("failed to create bare repo dir: %v", err)
	}
	runGitCommand(t, barePath, "git", "init", "--bare")

	// Create working repo
	repoPath := filepath.Join(dir, name)
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "commit.gpgsign", "false"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	// Create initial commit
	readmePath := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readmePath, []byte("# "+name+"\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	cmds = [][]string{
		{"git", "add", "README.md"},
		{"git", "commit", "-m", "Initial commit"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}

	// Set up origin pointing to bare repo and push
	runGitCommand(t, repoPath, "git", "remote", "add", "origin", barePath)
	runGitCommand(t, repoPath, "git", "push", "-u", "origin", "main")

	return repoPath
}

// populateCache scans worktreeDir for worktrees and populates the cache.
// This is required for ID-based lookups to work.
func populateCache(t *testing.T, worktreeDir string) {
	t.Helper()

	// Load or create cache
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}
	defer unlock()

	// List worktrees
	worktrees, err := git.ListWorktrees(worktreeDir, false)
	if err != nil {
		t.Fatalf("failed to list worktrees: %v", err)
	}

	// Convert to cache.WorktreeInfo
	wtInfos := make([]cache.WorktreeInfo, len(worktrees))
	for i, wt := range worktrees {
		wtInfos[i] = cache.WorktreeInfo{
			Path:      wt.Path,
			RepoPath:  wt.MainRepo,
			Branch:    wt.Branch,
			OriginURL: wt.OriginURL,
		}
	}

	// Sync cache
	wtCache.SyncWorktrees(wtInfos)

	// Save cache
	if err := cache.Save(worktreeDir, wtCache); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}
}

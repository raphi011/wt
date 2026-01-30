//go:build integration

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

// testContext creates a context with log and output set to discard.
// Use testContextWithOutput to capture output.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx := context.Background()
	ctx = log.WithLogger(ctx, log.New(io.Discard, false, false))
	ctx = output.WithPrinter(ctx, io.Discard)
	return ctx
}

// testContextWithOutput creates a context and returns the output writer for assertions.
func testContextWithOutput(t *testing.T) (context.Context, *strings.Builder) {
	t.Helper()
	var out strings.Builder
	ctx := context.Background()
	ctx = log.WithLogger(ctx, log.New(io.Discard, false, false))
	ctx = output.WithPrinter(ctx, &out)
	return ctx, &out
}

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

	// Initialize git repo with main branch
	cmds := [][]string{
		{"git", "init", "-b", "main"},
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
		t.Fatalf("failed to add origin: %v\n%s", err, out)
	}

	return repoPath
}

// setupTestRepoWithBranches creates a test repo with additional branches
func setupTestRepoWithBranches(t *testing.T, dir, name string, branches []string) string {
	t.Helper()
	repoPath := setupTestRepo(t, dir, name)

	for _, branch := range branches {
		cmd := exec.Command("git", "branch", branch)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to create branch %s: %v\n%s", branch, err, out)
		}
	}

	return repoPath
}

// testConfig returns a test config
func testConfig() *config.Config {
	return &config.Config{
		WorktreeFormat: "{repo}-{branch}",
		Forge: config.ForgeConfig{
			Default: "github",
		},
	}
}

// testRegistry creates an empty test registry
func testRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	return &registry.Registry{Repos: []registry.Repo{}}
}

// executeCommand executes a cobra command with arguments and returns output/error
func executeCommand(ctx context.Context, cmd *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	cmd.SetContext(ctx)

	err := cmd.Execute()
	return buf.String(), err
}

// createTestWorktree creates a worktree in a repo
func createTestWorktree(t *testing.T, repoPath, branch string) string {
	t.Helper()

	// Create the branch first
	cmd := exec.Command("git", "branch", branch)
	cmd.Dir = repoPath
	cmd.CombinedOutput() // Ignore error if branch exists

	// Create worktree
	wtPath := filepath.Join(filepath.Dir(repoPath), repoPath+"-"+branch)
	cmd = exec.Command("git", "worktree", "add", wtPath, branch)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
	}

	return wtPath
}

// addCommit adds a file and commits it
func addCommit(t *testing.T, path, filename, message string) {
	t.Helper()

	// Create file
	filePath := filepath.Join(path, filename)
	if err := os.WriteFile(filePath, []byte("content\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Add and commit
	cmds := [][]string{
		{"git", "add", filename},
		{"git", "commit", "-m", message},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = path
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run %v: %v\n%s", args, err, out)
		}
	}
}

// getGitBranch returns the current git branch
func getGitBranch(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get branch: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// setupBareRepo creates a bare git repository
func setupBareRepo(t *testing.T, dir, name string) string {
	t.Helper()

	dir = resolvePath(t, dir)
	repoPath := filepath.Join(dir, name+".git")

	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create bare repo dir: %v", err)
	}

	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to init bare repo: %v\n%s", err, out)
	}

	return repoPath
}

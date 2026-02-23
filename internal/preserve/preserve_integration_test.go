//go:build integration

package preserve

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
)

// resolvePath resolves symlinks (needed on macOS where /var -> /private/var).
func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("failed to resolve path %s: %v", path, err)
	}
	return resolved
}

// runGit runs a git command in the given directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// setupTestRepo creates a bare-in-.git repo with an initial commit and .gitignore.
// Returns the repo path (parent of .git).
func setupTestRepo(t *testing.T, dir string) string {
	t.Helper()

	repoDir := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	runGit(t, repoDir, "init", "--initial-branch=main")
	runGit(t, repoDir, "config", "user.email", "test@test.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "commit.gpgsign", "false")

	// Create .gitignore
	if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte(".env\n.env.*\n*.local\nbuild/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	// Initial commit
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")

	return resolvePath(t, repoDir)
}

// testContext returns a context with a logger and output writer (both discarded).
func testContext() context.Context {
	ctx := context.Background()
	ctx = log.WithLogger(ctx, log.New(io.Discard, false, true))
	ctx = output.WithPrinter(ctx, os.Stderr)
	return ctx
}

func TestFindSourceWorktree(t *testing.T) {
	t.Parallel()

	t.Run("returns default branch worktree", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		// Create a worktree on a feature branch
		featureDir := filepath.Join(tmpDir, "repo-feature")
		runGit(t, repoDir, "worktree", "add", "-b", "feature", featureDir)

		// FindSourceWorktree from the feature worktree should return the main worktree
		source, err := FindSourceWorktree(ctx, repoDir, featureDir)
		if err != nil {
			t.Fatalf("FindSourceWorktree() error: %v", err)
		}
		if source != repoDir {
			t.Errorf("FindSourceWorktree() = %q, want %q (default branch worktree)", source, repoDir)
		}
	})

	t.Run("falls back to first non-target worktree", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		// Create two feature worktrees (no default branch worktree will match
		// if we search from one of them and the repo has no origin/HEAD)
		feat1Dir := filepath.Join(tmpDir, "repo-feat1")
		feat2Dir := filepath.Join(tmpDir, "repo-feat2")
		runGit(t, repoDir, "worktree", "add", "-b", "feat1", feat1Dir)
		runGit(t, repoDir, "worktree", "add", "-b", "feat2", feat2Dir)

		// From feat2, should find feat1 or main (whichever comes first that isn't feat2)
		source, err := FindSourceWorktree(ctx, repoDir, feat2Dir)
		if err != nil {
			t.Fatalf("FindSourceWorktree() error: %v", err)
		}
		if source == feat2Dir {
			t.Error("FindSourceWorktree() returned the target worktree itself")
		}
	})

	t.Run("returns error when only one worktree exists", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		// Only the main worktree exists — no source available
		_, err := FindSourceWorktree(ctx, repoDir, repoDir)
		if !errors.Is(err, ErrNoSourceWorktree) {
			t.Errorf("FindSourceWorktree() error = %v, want ErrNoSourceWorktree", err)
		}
	})
}

func TestFindIgnoredFiles(t *testing.T) {
	t.Parallel()

	t.Run("returns ignored files", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		// Create some ignored files
		if err := os.WriteFile(filepath.Join(repoDir, ".env"), []byte("SECRET=x\n"), 0o644); err != nil {
			t.Fatalf("write .env: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, ".env.local"), []byte("LOCAL=y\n"), 0o644); err != nil {
			t.Fatalf("write .env.local: %v", err)
		}

		files, err := FindIgnoredFiles(ctx, repoDir)
		if err != nil {
			t.Fatalf("FindIgnoredFiles() error: %v", err)
		}

		if !slices.Contains(files, ".env") {
			t.Errorf("expected .env in ignored files, got %v", files)
		}
		if !slices.Contains(files, ".env.local") {
			t.Errorf("expected .env.local in ignored files, got %v", files)
		}
	})

	t.Run("returns nil for no ignored files", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		// No ignored files created — only tracked files
		files, err := FindIgnoredFiles(ctx, repoDir)
		if err != nil {
			t.Fatalf("FindIgnoredFiles() error: %v", err)
		}
		if files != nil {
			t.Errorf("expected nil, got %v", files)
		}
	})

	t.Run("returns nested ignored files", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		// Create nested ignored directory
		buildDir := filepath.Join(repoDir, "build")
		if err := os.MkdirAll(buildDir, 0o755); err != nil {
			t.Fatalf("mkdir build: %v", err)
		}
		if err := os.WriteFile(filepath.Join(buildDir, "output.bin"), []byte("binary"), 0o644); err != nil {
			t.Fatalf("write build/output.bin: %v", err)
		}

		files, err := FindIgnoredFiles(ctx, repoDir)
		if err != nil {
			t.Fatalf("FindIgnoredFiles() error: %v", err)
		}

		if !slices.Contains(files, "build/output.bin") {
			t.Errorf("expected build/output.bin in ignored files, got %v", files)
		}
	})
}

func TestPreserveFiles(t *testing.T) {
	t.Parallel()

	t.Run("copies matching ignored files to target", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		// Create ignored files in source
		if err := os.WriteFile(filepath.Join(repoDir, ".env"), []byte("DB_HOST=localhost\n"), 0o644); err != nil {
			t.Fatalf("write .env: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repoDir, ".env.local"), []byte("LOCAL=1\n"), 0o644); err != nil {
			t.Fatalf("write .env.local: %v", err)
		}

		// Create target directory
		targetDir := filepath.Join(tmpDir, "target")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}

		cfg := config.PreserveConfig{
			Patterns: []string{".env", ".env.*"},
		}

		copied, err := PreserveFiles(ctx, cfg, repoDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error: %v", err)
		}

		if len(copied) != 2 {
			t.Fatalf("expected 2 copied files, got %d: %v", len(copied), copied)
		}

		// Verify files were actually copied
		data, err := os.ReadFile(filepath.Join(targetDir, ".env"))
		if err != nil {
			t.Fatalf("read .env in target: %v", err)
		}
		if string(data) != "DB_HOST=localhost\n" {
			t.Errorf(".env content = %q, want %q", data, "DB_HOST=localhost\n")
		}
	})

	t.Run("respects exclude patterns", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		// Add node_modules to .gitignore
		gitignore := filepath.Join(repoDir, ".gitignore")
		if err := os.WriteFile(gitignore, []byte(".env\nnode_modules/\n"), 0o644); err != nil {
			t.Fatalf("write .gitignore: %v", err)
		}
		runGit(t, repoDir, "add", ".gitignore")
		runGit(t, repoDir, "commit", "-m", "update gitignore")

		// Create ignored files — one in excluded dir, one not
		if err := os.WriteFile(filepath.Join(repoDir, ".env"), []byte("ROOT=1\n"), 0o644); err != nil {
			t.Fatalf("write .env: %v", err)
		}
		nmDir := filepath.Join(repoDir, "node_modules")
		if err := os.MkdirAll(nmDir, 0o755); err != nil {
			t.Fatalf("mkdir node_modules: %v", err)
		}
		if err := os.WriteFile(filepath.Join(nmDir, ".env"), []byte("NM=1\n"), 0o644); err != nil {
			t.Fatalf("write node_modules/.env: %v", err)
		}

		targetDir := filepath.Join(tmpDir, "target")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}

		cfg := config.PreserveConfig{
			Patterns: []string{".env"},
			Exclude:  []string{"node_modules"},
		}

		copied, err := PreserveFiles(ctx, cfg, repoDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error: %v", err)
		}

		// Should only copy root .env, not node_modules/.env
		if len(copied) != 1 {
			t.Fatalf("expected 1 copied file, got %d: %v", len(copied), copied)
		}
		if copied[0] != ".env" {
			t.Errorf("copied[0] = %q, want .env", copied[0])
		}
	})

	t.Run("returns empty for no matching files", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		targetDir := filepath.Join(tmpDir, "target")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}

		cfg := config.PreserveConfig{
			Patterns: []string{".env"},
		}

		copied, err := PreserveFiles(ctx, cfg, repoDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error: %v", err)
		}
		if len(copied) != 0 {
			t.Errorf("expected 0 copied files, got %d: %v", len(copied), copied)
		}
	})

	t.Run("skips files that already exist in target", func(t *testing.T) {
		t.Parallel()

		tmpDir := resolvePath(t, t.TempDir())
		repoDir := setupTestRepo(t, tmpDir)
		ctx := testContext()

		// Create ignored file in source
		if err := os.WriteFile(filepath.Join(repoDir, ".env"), []byte("SOURCE=1\n"), 0o644); err != nil {
			t.Fatalf("write .env: %v", err)
		}

		// Create same file in target with different content
		targetDir := filepath.Join(tmpDir, "target")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			t.Fatalf("mkdir target: %v", err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, ".env"), []byte("TARGET=1\n"), 0o644); err != nil {
			t.Fatalf("write target .env: %v", err)
		}

		cfg := config.PreserveConfig{
			Patterns: []string{".env"},
		}

		copied, err := PreserveFiles(ctx, cfg, repoDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error: %v", err)
		}

		// Should not have copied (file exists)
		if len(copied) != 0 {
			t.Errorf("expected 0 copied files (existing), got %d: %v", len(copied), copied)
		}

		// Target content should be unchanged
		data, err := os.ReadFile(filepath.Join(targetDir, ".env"))
		if err != nil {
			t.Fatalf("read target .env: %v", err)
		}
		if string(data) != "TARGET=1\n" {
			t.Errorf("target .env was overwritten: got %q", data)
		}
	})
}

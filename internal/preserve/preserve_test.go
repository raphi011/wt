//go:build !integration

package preserve

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
)

func TestMatchesPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		relPath  string
		patterns []string
		exclude  []string
		want     bool
	}{
		{
			name:     "exact basename match",
			relPath:  ".env",
			patterns: []string{".env"},
			want:     true,
		},
		{
			name:     "glob pattern match",
			relPath:  ".env.local",
			patterns: []string{".env.*"},
			want:     true,
		},
		{
			name:     "nested file matches basename",
			relPath:  "config/.env",
			patterns: []string{".env"},
			want:     true,
		},
		{
			name:     "no match",
			relPath:  "main.go",
			patterns: []string{".env", ".envrc"},
			want:     false,
		},
		{
			name:     "excluded path segment",
			relPath:  "node_modules/.env",
			patterns: []string{".env"},
			exclude:  []string{"node_modules"},
			want:     false,
		},
		{
			name:     "deeply nested excluded segment",
			relPath:  "packages/app/node_modules/.cache/.env",
			patterns: []string{".env"},
			exclude:  []string{"node_modules"},
			want:     false,
		},
		{
			name:     "exclude does not match basename",
			relPath:  ".env",
			patterns: []string{".env"},
			exclude:  []string{"vendor"},
			want:     true,
		},
		{
			name:     "multiple patterns first matches",
			relPath:  ".envrc",
			patterns: []string{".env", ".envrc", "docker-compose.override.yml"},
			want:     true,
		},
		{
			name:     "docker-compose override",
			relPath:  "docker-compose.override.yml",
			patterns: []string{"docker-compose.override.yml"},
			want:     true,
		},
		{
			name:     "empty patterns",
			relPath:  ".env",
			patterns: nil,
			want:     false,
		},
		{
			name:     "invalid glob pattern does not match",
			relPath:  ".env",
			patterns: []string{"[invalid"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := matchesPattern(tt.relPath, tt.patterns, tt.exclude)
			if got != tt.want {
				t.Errorf("matchesPattern(%q, %v, %v) = %v, want %v",
					tt.relPath, tt.patterns, tt.exclude, got, tt.want)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	t.Run("copies file with contents", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src", ".env")
		dst := filepath.Join(tmpDir, "dst", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("SECRET=abc123\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		copied, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}
		if !copied {
			t.Error("CopyFile() should return true for new file")
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read dst: %v", err)
		}
		if string(got) != "SECRET=abc123\n" {
			t.Errorf("got %q, want %q", got, "SECRET=abc123\n")
		}
	})

	t.Run("skips existing file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src", ".env")
		dst := filepath.Join(tmpDir, "dst", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("NEW_CONTENT\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}
		if err := os.WriteFile(dst, []byte("EXISTING_CONTENT\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		copied, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile() should not error on existing: %v", err)
		}
		if copied {
			t.Error("CopyFile() should return false for existing file")
		}

		// Existing content should be preserved
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read dst: %v", err)
		}
		if string(got) != "EXISTING_CONTENT\n" {
			t.Errorf("existing file was overwritten: got %q, want %q", got, "EXISTING_CONTENT\n")
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src", "deep", "nested", ".env")
		dst := filepath.Join(tmpDir, "dst", "deep", "nested", ".env")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("NESTED=true\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		copied, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}
		if !copied {
			t.Error("CopyFile() should return true for new file")
		}

		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read dst: %v", err)
		}
		if string(got) != "NESTED=true\n" {
			t.Errorf("got %q, want %q", got, "NESTED=true\n")
		}
	})

	t.Run("preserves file permissions", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src", "script.sh")
		dst := filepath.Join(tmpDir, "dst", "script.sh")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		if _, err := CopyFile(src, dst); err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}

		info, err := os.Stat(dst)
		if err != nil {
			t.Fatalf("failed to stat dst: %v", err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("permissions = %o, want %o", info.Mode().Perm(), 0755)
		}
	})

	t.Run("returns error for non-existent source", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "does-not-exist")
		dst := filepath.Join(tmpDir, "dst", "file")

		_, err := CopyFile(src, dst)
		if err == nil {
			t.Fatal("CopyFile() should return error for non-existent source")
		}
	})

	t.Run("cleans up dst when src cannot be opened", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src", "secret")
		dst := filepath.Join(tmpDir, "dst", "secret")

		if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(src, []byte("content\n"), 0000); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		_, err := CopyFile(src, dst)
		if err == nil {
			t.Fatal("CopyFile() should return error when src cannot be opened")
		}

		// dst should have been cleaned up
		if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
			t.Error("dst should not exist after failed copy")
		}
	})

	t.Run("skips symlinks", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		// Create a real file and a symlink to it
		realFile := filepath.Join(tmpDir, "real.txt")
		if err := os.WriteFile(realFile, []byte("content\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}

		src := filepath.Join(tmpDir, "link.txt")
		if err := os.Symlink(realFile, src); err != nil {
			t.Fatalf("setup: symlink failed: %v", err)
		}

		dst := filepath.Join(tmpDir, "dst", "link.txt")

		copied, err := CopyFile(src, dst)
		if err != nil {
			t.Fatalf("CopyFile() error = %v", err)
		}
		if copied {
			t.Error("CopyFile() should return false for symlink")
		}

		// dst should not exist
		if _, err := os.Stat(dst); !os.IsNotExist(err) {
			t.Error("dst should not exist when source is a symlink")
		}
	})

	t.Run("returns error when dst parent path is a file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		src := filepath.Join(tmpDir, "src.txt")
		if err := os.WriteFile(src, []byte("hello\n"), 0644); err != nil {
			t.Fatalf("setup: write src failed: %v", err)
		}

		// Create a regular file where the directory should be — MkdirAll will fail.
		blocker := filepath.Join(tmpDir, "notadir")
		if err := os.WriteFile(blocker, []byte("block\n"), 0644); err != nil {
			t.Fatalf("setup: write blocker failed: %v", err)
		}

		dst := filepath.Join(blocker, "dst.txt")

		_, err := CopyFile(src, dst)
		if err == nil {
			t.Fatal("CopyFile() should return error when dst parent is a file")
		}
	})
}

func resolveTempDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		t.Fatalf("failed to resolve symlinks for %s: %v", tmpDir, err)
	}
	return resolved
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	if dir != "" {
		c.Dir = dir
	}
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func testContext() context.Context {
	l := log.New(io.Discard, false, true)
	return log.WithLogger(context.Background(), l)
}

// initBareRepoWithWorktree creates a bare-like repo setup with a main worktree
// and returns (gitDir, mainWorktreePath).
func initBareRepoWithWorktree(t *testing.T, baseDir string) (string, string) {
	t.Helper()

	repoDir := filepath.Join(baseDir, "repo.git")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("setup: mkdir failed: %v", err)
	}
	runGit(t, repoDir, "init", "--bare", "-b", "main")

	// Create an initial commit via a temporary clone
	cloneDir := filepath.Join(baseDir, "tmp-clone")
	runGit(t, baseDir, "clone", repoDir, cloneDir)
	runGit(t, cloneDir, "config", "user.email", "test@test.com")
	runGit(t, cloneDir, "config", "user.name", "Test")

	readmePath := filepath.Join(cloneDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("init\n"), 0644); err != nil {
		t.Fatalf("setup: write file failed: %v", err)
	}
	runGit(t, cloneDir, "add", ".")
	runGit(t, cloneDir, "commit", "-m", "initial")
	runGit(t, cloneDir, "push", "-u", "origin", "HEAD")

	// Add a main worktree from the bare repo
	mainWT := filepath.Join(baseDir, "main-wt")
	runGit(t, repoDir, "worktree", "add", mainWT, "main")

	return repoDir, mainWT
}

// TestFindSourceWorktree_InvalidGitDir verifies that FindSourceWorktree returns
// an error when git.ListWorktreesFromRepo fails (non-existent git directory).
func TestFindSourceWorktree_InvalidGitDir(t *testing.T) {
	t.Parallel()

	ctx := testContext()
	tmpDir := resolveTempDir(t)
	fakeGitDir := filepath.Join(tmpDir, "does-not-exist.git")

	_, err := FindSourceWorktree(ctx, fakeGitDir, filepath.Join(tmpDir, "some-worktree"))
	if err == nil {
		t.Error("FindSourceWorktree() expected error for invalid git dir, got nil")
	}
}

// TestFindIgnoredFiles_NonGitDir verifies that FindIgnoredFiles returns an
// error when the directory is not a git repository.
func TestFindIgnoredFiles_NonGitDir(t *testing.T) {
	t.Parallel()

	ctx := testContext()
	tmpDir := resolveTempDir(t)
	// tmpDir is not a git repo, so git ls-files should fail.
	_, err := FindIgnoredFiles(ctx, tmpDir)
	if err == nil {
		t.Error("FindIgnoredFiles() expected error for non-git directory, got nil")
	}
}

func TestFindSourceWorktree(t *testing.T) {
	t.Parallel()

	t.Run("returns default branch worktree", func(t *testing.T) {
		t.Parallel()
		baseDir := resolveTempDir(t)
		ctx := testContext()

		repoDir, mainWT := initBareRepoWithWorktree(t, baseDir)

		// Create a feature branch and worktree
		featureWT := filepath.Join(baseDir, "feature-wt")
		runGit(t, mainWT, "checkout", "-b", "feature")
		runGit(t, mainWT, "checkout", "main")
		runGit(t, repoDir, "worktree", "add", featureWT, "feature")

		got, err := FindSourceWorktree(ctx, repoDir, featureWT)
		if err != nil {
			t.Fatalf("FindSourceWorktree() error = %v", err)
		}
		if got != mainWT {
			t.Errorf("FindSourceWorktree() = %q, want %q", got, mainWT)
		}
	})

	t.Run("falls back to first non-target worktree", func(t *testing.T) {
		t.Parallel()
		baseDir := resolveTempDir(t)
		ctx := testContext()

		repoDir, mainWT := initBareRepoWithWorktree(t, baseDir)

		// Create two feature worktrees
		runGit(t, mainWT, "checkout", "-b", "feat-a")
		runGit(t, mainWT, "checkout", "main")
		runGit(t, mainWT, "checkout", "-b", "feat-b")
		runGit(t, mainWT, "checkout", "main")

		featAWT := filepath.Join(baseDir, "feat-a-wt")
		runGit(t, repoDir, "worktree", "add", featAWT, "feat-a")

		featBWT := filepath.Join(baseDir, "feat-b-wt")
		runGit(t, repoDir, "worktree", "add", featBWT, "feat-b")

		// Target is mainWT — default branch worktree is excluded, so it should
		// fall back. But mainWT IS the default branch. Let's target feat-b
		// and check we get something other than feat-b.
		// Actually, let's test the fallback by targeting mainWT so default branch
		// match is excluded, then it falls back to first non-target.
		got, err := FindSourceWorktree(ctx, repoDir, mainWT)
		if err != nil {
			t.Fatalf("FindSourceWorktree() error = %v", err)
		}
		// Should return one of the feature worktrees (first non-target after
		// default branch worktree is excluded because it IS the target)
		if got == mainWT {
			t.Errorf("FindSourceWorktree() returned target path %q", mainWT)
		}
		if got != featAWT && got != featBWT {
			t.Errorf("FindSourceWorktree() = %q, want one of %q or %q", got, featAWT, featBWT)
		}
	})

	t.Run("returns error when only target exists", func(t *testing.T) {
		t.Parallel()
		baseDir := resolveTempDir(t)
		ctx := testContext()

		repoDir, mainWT := initBareRepoWithWorktree(t, baseDir)

		_, err := FindSourceWorktree(ctx, repoDir, mainWT)
		if !errors.Is(err, ErrNoSourceWorktree) {
			t.Errorf("FindSourceWorktree() error = %v, want %v", err, ErrNoSourceWorktree)
		}
	})
}

func TestFindIgnoredFiles(t *testing.T) {
	t.Parallel()

	t.Run("returns ignored files", func(t *testing.T) {
		t.Parallel()
		baseDir := resolveTempDir(t)
		ctx := testContext()

		// Create a git repo
		runGit(t, baseDir, "init")
		runGit(t, baseDir, "config", "user.email", "test@test.com")
		runGit(t, baseDir, "config", "user.name", "Test")

		// Create .gitignore
		if err := os.WriteFile(filepath.Join(baseDir, ".gitignore"), []byte(".env\n"), 0644); err != nil {
			t.Fatalf("setup: write .gitignore failed: %v", err)
		}
		runGit(t, baseDir, "add", ".gitignore")
		runGit(t, baseDir, "commit", "-m", "add gitignore")

		// Create an ignored file
		if err := os.WriteFile(filepath.Join(baseDir, ".env"), []byte("SECRET=123\n"), 0644); err != nil {
			t.Fatalf("setup: write .env failed: %v", err)
		}

		got, err := FindIgnoredFiles(ctx, baseDir)
		if err != nil {
			t.Fatalf("FindIgnoredFiles() error = %v", err)
		}
		if len(got) != 1 || got[0] != ".env" {
			t.Errorf("FindIgnoredFiles() = %v, want [.env]", got)
		}
	})

	t.Run("returns nil for no ignored files", func(t *testing.T) {
		t.Parallel()
		baseDir := resolveTempDir(t)
		ctx := testContext()

		runGit(t, baseDir, "init")
		runGit(t, baseDir, "config", "user.email", "test@test.com")
		runGit(t, baseDir, "config", "user.name", "Test")

		// Create a tracked file so we have at least one commit
		if err := os.WriteFile(filepath.Join(baseDir, "README.md"), []byte("hi\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}
		runGit(t, baseDir, "add", ".")
		runGit(t, baseDir, "commit", "-m", "init")

		got, err := FindIgnoredFiles(ctx, baseDir)
		if err != nil {
			t.Fatalf("FindIgnoredFiles() error = %v", err)
		}
		if got != nil {
			t.Errorf("FindIgnoredFiles() = %v, want nil", got)
		}
	})
}

func TestPreserveFiles(t *testing.T) {
	t.Parallel()

	t.Run("copies matching files", func(t *testing.T) {
		t.Parallel()
		baseDir := resolveTempDir(t)
		ctx := testContext()

		sourceDir := filepath.Join(baseDir, "source")
		targetDir := filepath.Join(baseDir, "target")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}

		// Init git repo in source
		runGit(t, sourceDir, "init")
		runGit(t, sourceDir, "config", "user.email", "test@test.com")
		runGit(t, sourceDir, "config", "user.name", "Test")

		// Add .gitignore
		if err := os.WriteFile(filepath.Join(sourceDir, ".gitignore"), []byte(".env\n"), 0644); err != nil {
			t.Fatalf("setup: write .gitignore failed: %v", err)
		}
		runGit(t, sourceDir, "add", ".gitignore")
		runGit(t, sourceDir, "commit", "-m", "init")

		// Create ignored file
		if err := os.WriteFile(filepath.Join(sourceDir, ".env"), []byte("SECRET=abc\n"), 0644); err != nil {
			t.Fatalf("setup: write .env failed: %v", err)
		}

		cfg := config.PreserveConfig{
			Patterns: []string{".env"},
		}

		copied, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error = %v", err)
		}
		if len(copied) != 1 || copied[0] != ".env" {
			t.Errorf("PreserveFiles() copied = %v, want [.env]", copied)
		}

		// Verify file was actually copied
		got, err := os.ReadFile(filepath.Join(targetDir, ".env"))
		if err != nil {
			t.Fatalf("failed to read copied file: %v", err)
		}
		if string(got) != "SECRET=abc\n" {
			t.Errorf("copied file content = %q, want %q", got, "SECRET=abc\n")
		}
	})

	t.Run("skips non-matching files", func(t *testing.T) {
		t.Parallel()
		baseDir := resolveTempDir(t)
		ctx := testContext()

		sourceDir := filepath.Join(baseDir, "source")
		targetDir := filepath.Join(baseDir, "target")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}

		runGit(t, sourceDir, "init")
		runGit(t, sourceDir, "config", "user.email", "test@test.com")
		runGit(t, sourceDir, "config", "user.name", "Test")

		if err := os.WriteFile(filepath.Join(sourceDir, ".gitignore"), []byte("*.log\n"), 0644); err != nil {
			t.Fatalf("setup: write .gitignore failed: %v", err)
		}
		runGit(t, sourceDir, "add", ".gitignore")
		runGit(t, sourceDir, "commit", "-m", "init")

		// Create ignored file that does NOT match preserve patterns
		if err := os.WriteFile(filepath.Join(sourceDir, "random.log"), []byte("log data\n"), 0644); err != nil {
			t.Fatalf("setup: write random.log failed: %v", err)
		}

		cfg := config.PreserveConfig{
			Patterns: []string{".env"},
		}

		copied, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error = %v", err)
		}
		if len(copied) != 0 {
			t.Errorf("PreserveFiles() copied = %v, want empty", copied)
		}

		// Verify file was NOT copied
		if _, err := os.Stat(filepath.Join(targetDir, "random.log")); !os.IsNotExist(err) {
			t.Error("non-matching file should not be copied to target")
		}
	})

	t.Run("skips excluded directories", func(t *testing.T) {
		t.Parallel()
		baseDir := resolveTempDir(t)
		ctx := testContext()

		sourceDir := filepath.Join(baseDir, "source")
		targetDir := filepath.Join(baseDir, "target")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}

		runGit(t, sourceDir, "init")
		runGit(t, sourceDir, "config", "user.email", "test@test.com")
		runGit(t, sourceDir, "config", "user.name", "Test")

		if err := os.WriteFile(filepath.Join(sourceDir, ".gitignore"), []byte("node_modules/\n.env\n"), 0644); err != nil {
			t.Fatalf("setup: write .gitignore failed: %v", err)
		}
		runGit(t, sourceDir, "add", ".gitignore")
		runGit(t, sourceDir, "commit", "-m", "init")

		// Create ignored file inside excluded directory
		nmDir := filepath.Join(sourceDir, "node_modules")
		if err := os.MkdirAll(nmDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(nmDir, ".env"), []byte("NM_SECRET=x\n"), 0644); err != nil {
			t.Fatalf("setup: write .env failed: %v", err)
		}

		cfg := config.PreserveConfig{
			Patterns: []string{".env"},
			Exclude:  []string{"node_modules"},
		}

		copied, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error = %v", err)
		}
		if len(copied) != 0 {
			t.Errorf("PreserveFiles() copied = %v, want empty (excluded dir)", copied)
		}

		// Verify file was NOT copied
		if _, err := os.Stat(filepath.Join(targetDir, "node_modules", ".env")); !os.IsNotExist(err) {
			t.Error("excluded directory file should not be copied to target")
		}
	})

	t.Run("handles empty ignored files", func(t *testing.T) {
		t.Parallel()
		baseDir := resolveTempDir(t)
		ctx := testContext()

		sourceDir := filepath.Join(baseDir, "source")
		targetDir := filepath.Join(baseDir, "target")
		if err := os.MkdirAll(sourceDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			t.Fatalf("setup: mkdir failed: %v", err)
		}

		runGit(t, sourceDir, "init")
		runGit(t, sourceDir, "config", "user.email", "test@test.com")
		runGit(t, sourceDir, "config", "user.name", "Test")

		if err := os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("hi\n"), 0644); err != nil {
			t.Fatalf("setup: write file failed: %v", err)
		}
		runGit(t, sourceDir, "add", ".")
		runGit(t, sourceDir, "commit", "-m", "init")

		cfg := config.PreserveConfig{
			Patterns: []string{".env"},
		}

		copied, err := PreserveFiles(ctx, cfg, sourceDir, targetDir)
		if err != nil {
			t.Fatalf("PreserveFiles() error = %v", err)
		}
		if len(copied) != 0 {
			t.Errorf("PreserveFiles() copied = %v, want empty", copied)
		}
	})
}

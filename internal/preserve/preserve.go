package preserve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/raphi011/wt/internal/cmd"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
)

// FindSourceWorktree finds an existing worktree to copy preserved files from.
// It prefers the worktree on the default branch, falling back to the first
// worktree that isn't the target.
func FindSourceWorktree(ctx context.Context, gitDir, targetPath string) (string, error) {
	worktrees, err := git.ListWorktreesFromRepo(ctx, gitDir)
	if err != nil {
		return "", fmt.Errorf("list worktrees: %w", err)
	}

	defaultBranch := git.GetDefaultBranch(ctx, gitDir)

	// Prefer the worktree on the default branch
	for _, wt := range worktrees {
		if wt.Branch == defaultBranch && wt.Path != targetPath {
			return wt.Path, nil
		}
	}

	// Fall back to first worktree that isn't the target
	for _, wt := range worktrees {
		if wt.Path != targetPath {
			return wt.Path, nil
		}
	}

	return "", errors.New("no source worktree found")
}

// FindIgnoredFiles returns paths (relative to worktreeDir) of all git-ignored
// files present in the worktree.
func FindIgnoredFiles(ctx context.Context, worktreeDir string) ([]string, error) {
	output, err := cmd.OutputContext(ctx, worktreeDir, "git",
		"ls-files", "--others", "--ignored", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return nil, nil
	}

	return strings.Split(raw, "\n"), nil
}

// matchesPattern returns true if the file at relPath should be preserved
// based on the given patterns and exclusions.
// Patterns are matched against the file's basename.
// If any path segment matches an exclude entry, the file is skipped.
func matchesPattern(relPath string, patterns, exclude []string) bool {
	// Check exclusions first — if any path segment matches, skip
	for seg := range strings.SplitSeq(filepath.ToSlash(relPath), "/") {
		if slices.Contains(exclude, seg) {
			return false
		}
	}

	base := filepath.Base(relPath)
	for _, pat := range patterns {
		if matched, _ := filepath.Match(pat, base); matched {
			return true
		}
	}

	return false
}

// CopyFile copies src to dst, creating parent directories as needed.
// Uses O_CREATE|O_EXCL to skip files that already exist (never overwrite).
// Preserves the source file's permission bits.
// Returns true if the file was copied, false if it was skipped (already exists).
func CopyFile(src, dst string) (bool, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, err
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, err
	}

	// O_EXCL: fail if file exists (never overwrite)
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, srcInfo.Mode())
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil // skip existing files
		}
		return false, err
	}
	defer dstFile.Close()

	srcFile, err := os.Open(src)
	if err != nil {
		os.Remove(dst) // clean up empty dst
		return false, err
	}
	defer srcFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		os.Remove(dst) // clean up partial dst
		return false, err
	}

	return true, nil
}

// PreserveFiles copies git-ignored files matching the configured patterns
// from sourceDir into targetDir. Returns the list of relative paths that
// were copied.
func PreserveFiles(ctx context.Context, cfg config.PreserveConfig, sourceDir, targetDir string) ([]string, error) {
	l := log.FromContext(ctx)

	ignored, err := FindIgnoredFiles(ctx, sourceDir)
	if err != nil {
		return nil, err
	}

	var copied []string

	for _, relPath := range ignored {
		if !matchesPattern(relPath, cfg.Patterns, cfg.Exclude) {
			continue
		}

		src := filepath.Join(sourceDir, relPath)
		dst := filepath.Join(targetDir, relPath)

		ok, err := CopyFile(src, dst)
		if err != nil {
			l.Debug("preserve: failed to copy file", "file", relPath, "error", err)
			continue
		}

		if ok {
			copied = append(copied, relPath)
		}
	}

	return copied, nil
}

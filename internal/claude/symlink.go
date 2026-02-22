// Package claude handles integration with Claude Code's local data directories.
// It provides helpers for symlinking Claude Code session and auto-memory
// directories so that git worktrees share sessions with the main repository.
package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EncodeProjectPath converts an absolute filesystem path to the directory name
// format used by Claude Code under ~/.claude/projects/.
// The encoding replaces path separators with hyphens.
// Example: "/home/user/myproject" → "-home-user-myproject"
func EncodeProjectPath(absPath string) string {
	absPath = filepath.Clean(absPath)
	absPath = strings.TrimRight(absPath, string(filepath.Separator))
	return strings.ReplaceAll(absPath, string(filepath.Separator), "-")
}

// GetConfigDir returns the Claude Code configuration directory path.
// Checks CLAUDE_CONFIG_DIR env var first, then falls back to ~/.claude.
func GetConfigDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// SymlinkProjectDir creates a symlink in Claude Code's projects directory
// so that the worktree shares sessions and auto-memory with the main repo.
// Both paths are resolved with filepath.EvalSymlinks for macOS compatibility
// (/tmp → /private/tmp, /var → /private/var).
//
// Creates: <configDir>/projects/<worktree-encoded> → <configDir>/projects/<main-repo-encoded>
//
// If the worktree already has a real directory (not a symlink), it is left untouched
// to avoid destroying existing session data.
func SymlinkProjectDir(mainRepoPath, worktreePath string) error {
	// Resolve symlinks for macOS compatibility
	mainResolved, err := filepath.EvalSymlinks(mainRepoPath)
	if err != nil {
		return fmt.Errorf("resolve main repo path: %w", err)
	}
	wtResolved, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		return fmt.Errorf("resolve worktree path: %w", err)
	}

	if mainResolved == wtResolved {
		return nil
	}

	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	projectsDir := filepath.Join(configDir, "projects")
	mainEncoded := EncodeProjectPath(mainResolved)
	wtEncoded := EncodeProjectPath(wtResolved)

	mainDir := filepath.Join(projectsDir, mainEncoded)
	wtDir := filepath.Join(projectsDir, wtEncoded)

	// Create main repo project dir if it doesn't exist
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		return fmt.Errorf("create main project dir: %w", err)
	}

	// Check if worktree dir already exists
	info, err := os.Lstat(wtDir)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// Already a symlink — check if it points to the right place
			target, readErr := os.Readlink(wtDir)
			if readErr == nil && target == mainDir {
				return nil // Already correct
			}
			// Wrong target — remove and recreate
			if removeErr := os.Remove(wtDir); removeErr != nil {
				return fmt.Errorf("remove stale symlink: %w", removeErr)
			}
		} else {
			// Regular directory — don't overwrite existing session data
			return nil
		}
	}

	return os.Symlink(mainDir, wtDir)
}

// RemoveProjectDirSymlink removes a Claude project directory symlink for
// a worktree, if it exists and is a symlink. Regular directories are
// left untouched.
func RemoveProjectDirSymlink(worktreePath string) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// Try to resolve symlinks first; fall back to raw path if worktree
	// directory has already been deleted.
	resolved, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		resolved = filepath.Clean(worktreePath)
	}

	wtDir := filepath.Join(configDir, "projects", EncodeProjectPath(resolved))

	info, err := os.Lstat(wtDir)
	if err != nil {
		return nil // Doesn't exist, nothing to do
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(wtDir)
	}

	return nil // Not a symlink, don't touch
}

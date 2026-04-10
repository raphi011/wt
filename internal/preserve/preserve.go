package preserve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
)

// ErrDestExists is returned when the destination file already exists.
var ErrDestExists = errors.New("destination already exists")

// LinkFile creates a relative symlink from src to dst, creating parent
// directories as needed. Returns true if the link was created.
// Returns (false, nil) if src doesn't exist (caller should skip silently).
// Returns (false, ErrDestExists) if dst already exists.
func LinkFile(src, dst string) (bool, error) {
	// Check source exists
	if _, err := os.Lstat(src); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	// Check destination doesn't exist
	if _, err := os.Lstat(dst); err == nil {
		return false, ErrDestExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, err
	}

	// Compute relative path from dst's directory to src
	relPath, err := filepath.Rel(filepath.Dir(dst), src)
	if err != nil {
		return false, err
	}

	if err := os.Symlink(relPath, dst); err != nil {
		return false, err
	}

	return true, nil
}

// PreserveFiles symlinks files listed in cfg.Paths from sourceDir into
// targetDir. Returns the list of relative paths that were linked.
func PreserveFiles(ctx context.Context, cfg config.PreserveConfig, sourceDir, targetDir string) ([]string, error) {
	l := log.FromContext(ctx)

	var linked []string
	var lastErr error
	failCount := 0

	for _, path := range cfg.Paths {
		src := filepath.Join(sourceDir, path)
		dst := filepath.Join(targetDir, path)

		ok, err := LinkFile(src, dst)
		if err != nil {
			if errors.Is(err, ErrDestExists) {
				l.Printf("Warning: preserve: skipped %s (already exists in worktree)\n", path)
				continue
			}
			l.Printf("Warning: preserve: failed to link %s: %v\n", path, err)
			lastErr = err
			failCount++
			continue
		}

		if ok {
			linked = append(linked, path)
		} else {
			l.Debug("preserve: source not found, skipping", "path", path)
		}
	}

	if failCount > 0 && len(linked) == 0 {
		return nil, fmt.Errorf("all %d preserve path(s) failed (last error: %w)", failCount, lastErr)
	}

	return linked, nil
}

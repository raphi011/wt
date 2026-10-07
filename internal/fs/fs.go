// Package fs provides filesystem utilities: atomic JSON storage, path
// resolution (symlink canonicalization), and the ~/.wt/ data directory.
package fs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/statepath"
)

// WtDir returns the path to ~/.wt/, creating it if needed
func WtDir() (string, error) {
	dir, err := statepath.Dir("")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return dir, nil
}

// SaveJSON atomically writes data as JSON to the specified path.
// It ensures the parent directory exists, writes to a temp file,
// then renames to the final path for atomic operation.
// Use UpdateJSON for read-modify-write cycles.
func SaveJSON(path string, data any) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Unique temp name so concurrent writers never share a temp file
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tempPath := tmp.Name()

	if _, err := tmp.Write(jsonData); err != nil {
		return errors.Join(err, tmp.Close(), os.Remove(tempPath))
	}
	// Flush to disk so a crash after the rename can't leave an empty file
	if err := tmp.Sync(); err != nil {
		return errors.Join(err, tmp.Close(), os.Remove(tempPath))
	}
	if err := tmp.Close(); err != nil {
		return errors.Join(err, os.Remove(tempPath))
	}
	if err := os.Rename(tempPath, path); err != nil {
		return errors.Join(err, os.Remove(tempPath))
	}

	return nil
}

// LoadJSON reads JSON from the specified path into dest.
// Returns os.ErrNotExist if file doesn't exist (caller should handle).
func LoadJSON(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// ResolvePath returns the canonical form of path by resolving symlinks
// and normalizing case on case-insensitive filesystems (macOS).
//
// On macOS, /var is a symlink to /private/var, so paths through /var
// won't match paths through /private/var unless resolved. Additionally,
// APFS is case-insensitive by default, so /Users/foo/Git and
// /Users/foo/git refer to the same directory but won't match as strings.
//
// Returns the original path unchanged if resolution fails
// (e.g., broken symlink, permission denied).
func ResolvePath(path string) string {
	return canonicalizeCase(path)
}

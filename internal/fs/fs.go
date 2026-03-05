// Package fs provides filesystem utilities: atomic JSON storage, path
// resolution (symlink canonicalization), and the ~/.wt/ data directory.
package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// WtDir returns the path to ~/.wt/, creating it if needed
func WtDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".wt")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return dir, nil
}

// SaveJSON atomically writes data as JSON to the specified path.
// It ensures the parent directory exists, writes to a temp file,
// then renames to the final path for atomic operation.
func SaveJSON(path string, data any) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tempPath := path + ".tmp"

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tempPath, jsonData, 0o600); err != nil {
		return err
	}

	return os.Rename(tempPath, path)
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

// ResolvePath returns the canonical form of path by resolving symlinks.
// On macOS, /var is a symlink to /private/var, so paths through /var
// won't match paths through /private/var unless resolved.
// Returns the original path unchanged if symlink resolution fails
// (e.g., broken symlink, permission denied).
func ResolvePath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

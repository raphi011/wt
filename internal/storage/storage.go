// Package storage provides atomic file operations for JSON data in ~/.wt/
package storage

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

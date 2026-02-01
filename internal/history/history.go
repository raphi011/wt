// Package history tracks the most recently accessed worktree path.
// This enables `wt cd` with no arguments to return to the last visited worktree.
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// History stores the most recently accessed worktree path
type History struct {
	MostRecent string `json:"most_recent"`
}

// Path returns the path to the history file
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wt", "history.json")
}

// Load reads the history from disk
func Load() (*History, error) {
	historyPath := Path()

	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &History{}, nil
		}
		return nil, err
	}

	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		// Corrupted - start fresh
		return &History{}, nil
	}

	return &h, nil
}

// Save writes the history to disk atomically
func (h *History) Save() error {
	historyPath := Path()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		return err
	}

	tempPath := historyPath + ".tmp"

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}

	return os.Rename(tempPath, historyPath)
}

// RecordAccess saves the given path as the most recently accessed worktree
func RecordAccess(path string) error {
	h := &History{MostRecent: path}
	return h.Save()
}

// GetMostRecent returns the most recently accessed worktree path
// Returns empty string if no history exists
func GetMostRecent() (string, error) {
	h, err := Load()
	if err != nil {
		return "", err
	}
	return h.MostRecent, nil
}

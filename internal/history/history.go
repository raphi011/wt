// Package history tracks the most recently accessed worktree path.
// This enables `wt cd` with no arguments to return to the last visited worktree.
package history

import (
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/storage"
)

// History stores the most recently accessed worktree path
type History struct {
	MostRecent string `json:"most_recent"`
}

// DefaultPath returns the default path to the history file
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wt", "history.json")
}

// Load reads the history from disk at the given path
func Load(path string) (*History, error) {
	var h History
	if err := storage.LoadJSON(path, &h); err != nil {
		if os.IsNotExist(err) {
			return &History{}, nil
		}
		return nil, err
	}
	return &h, nil
}

// Save writes the history to disk atomically at the given path
func (h *History) Save(path string) error {
	return storage.SaveJSON(path, h)
}

// RecordAccess saves the given worktree path as the most recently accessed
func RecordAccess(path, historyPath string) error {
	h := &History{MostRecent: path}
	return h.Save(historyPath)
}

// GetMostRecent returns the most recently accessed worktree path
// Returns empty string if no history exists
func GetMostRecent(historyPath string) (string, error) {
	h, err := Load(historyPath)
	if err != nil {
		return "", err
	}
	return h.MostRecent, nil
}

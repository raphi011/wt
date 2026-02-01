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

// Path returns the path to the history file
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wt", "history.json")
}

// Load reads the history from disk
func Load() (*History, error) {
	var h History
	if err := storage.LoadJSON(Path(), &h); err != nil {
		if os.IsNotExist(err) {
			return &History{}, nil
		}
		return nil, err
	}
	return &h, nil
}

// Save writes the history to disk atomically
func (h *History) Save() error {
	return storage.SaveJSON(Path(), h)
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

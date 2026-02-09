// Package history tracks worktree access for ranking and quick navigation.
// Entries record path, repo name, branch, access count, and last access time.
// This enables `wt cd` to return to recent worktrees and `wt cd -i` to
// sort worktrees by recency.
package history

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/raphi011/wt/internal/storage"
)

// maxEntries is the maximum number of history entries kept.
// When exceeded, the oldest entries are evicted.
const maxEntries = 500

// Entry tracks a single worktree access.
type Entry struct {
	Path        string    `json:"path"`
	RepoName    string    `json:"repo_name"`
	Branch      string    `json:"branch"`
	AccessCount int       `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
}

// History stores worktree access entries.
type History struct {
	Entries []Entry `json:"entries"`
}

// DefaultPath returns the default path to the history file.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wt", "history.json")
}

// Load reads the history from disk at the given path.
// Returns empty history if the file doesn't exist. Unrecognized formats
// (e.g. the old {"most_recent": "..."} schema) decode into an empty
// History struct because unknown JSON keys are silently dropped.
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

// Save writes the history to disk atomically at the given path.
func (h *History) Save(path string) error {
	return storage.SaveJSON(path, h)
}

// FindByPath returns the entry matching the given path, or nil if not found.
func (h *History) FindByPath(path string) *Entry {
	for i := range h.Entries {
		if h.Entries[i].Path == path {
			return &h.Entries[i]
		}
	}
	return nil
}

// RemoveByPath removes the entry with the given path.
// Returns true if an entry was removed.
func (h *History) RemoveByPath(path string) bool {
	for i, e := range h.Entries {
		if e.Path == path {
			h.Entries = append(h.Entries[:i], h.Entries[i+1:]...)
			return true
		}
	}
	return false
}

// RemoveStale removes entries whose paths no longer exist on disk.
// Only entries with os.IsNotExist errors are removed; other stat errors
// (permissions, NFS timeouts) are kept to avoid purging temporarily
// inaccessible paths.
// Returns the number of entries removed.
func (h *History) RemoveStale() int {
	kept := h.Entries[:0]
	removed := 0
	for _, e := range h.Entries {
		if _, err := os.Stat(e.Path); err != nil && os.IsNotExist(err) {
			removed++
		} else {
			kept = append(kept, e)
		}
	}
	h.Entries = kept
	return removed
}

// SortByRecency sorts entries by LastAccess descending (most recent first).
func (h *History) SortByRecency() {
	sort.Slice(h.Entries, func(i, j int) bool {
		return h.Entries[i].LastAccess.After(h.Entries[j].LastAccess)
	})
}

// RecordAccess finds or creates an entry for the given path, increments its
// AccessCount, and updates LastAccess. Caps entries at maxEntries by evicting
// the oldest.
func RecordAccess(path, repoName, branch, historyPath string) error {
	if path == "" {
		return fmt.Errorf("path must not be empty")
	}

	h, err := Load(historyPath)
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}

	now := time.Now()

	if entry := h.FindByPath(path); entry != nil {
		entry.AccessCount++
		entry.LastAccess = now
		// Update repo/branch in case they changed
		entry.RepoName = repoName
		entry.Branch = branch
	} else {
		h.Entries = append(h.Entries, Entry{
			Path:        path,
			RepoName:    repoName,
			Branch:      branch,
			AccessCount: 1,
			LastAccess:  now,
		})
	}

	// Evict oldest entries if over cap
	if len(h.Entries) > maxEntries {
		h.SortByRecency()
		h.Entries = h.Entries[:maxEntries]
	}

	return h.Save(historyPath)
}

// GetMostRecent returns the path of the most recently accessed worktree.
// Returns empty string if no history exists.
func GetMostRecent(historyPath string) (string, error) {
	h, err := Load(historyPath)
	if err != nil {
		return "", err
	}
	if len(h.Entries) == 0 {
		return "", nil
	}

	h.SortByRecency()
	return h.Entries[0].Path, nil
}

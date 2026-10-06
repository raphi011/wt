// Package history tracks worktree access for ranking and quick navigation.
// Entries record path, repo name, branch, access count, and last access time.
// This enables `wt cd` to return to recent worktrees and `wt cd -i` to
// sort worktrees by recency.
package history

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/raphi011/wt/internal/fs"
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

// Load reads the history from disk at the given path.
// Returns empty history if the file doesn't exist. Unrecognized formats
// (e.g. the old {"most_recent": "..."} schema) decode into an empty
// History struct because unknown JSON keys are silently dropped.
func Load(path string) (*History, error) {
	var h History
	if err := fs.LoadJSON(path, &h); err != nil {
		if os.IsNotExist(err) {
			return &History{}, nil
		}
		return nil, err
	}
	return &h, nil
}

// Update applies fn to the history stored at the given path and saves the
// result. The read-modify-write cycle holds a file lock, so concurrent wt
// processes don't lose each other's changes. fn must only modify h.
func Update(path string, fn func(h *History) error) error {
	return fs.UpdateJSON(path, fn)
}

// FindByPath returns the entry matching the given path, or nil if not found.
// Resolves symlinks so lookups match regardless of path form.
func (h *History) FindByPath(path string) *Entry {
	resolved := fs.ResolvePath(path)
	for i := range h.Entries {
		if fs.ResolvePath(h.Entries[i].Path) == resolved {
			return &h.Entries[i]
		}
	}
	return nil
}

// RemoveByPath removes the entry with the given path.
// Returns true if an entry was removed.
// Resolves symlinks so removal works regardless of path form.
func (h *History) RemoveByPath(path string) bool {
	resolved := fs.ResolvePath(path)
	for i, e := range h.Entries {
		if fs.ResolvePath(e.Path) == resolved {
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

	// Store canonical path so future lookups match regardless of symlink form.
	path = fs.ResolvePath(path)

	return Update(historyPath, func(h *History) error {
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

		return nil
	})
}

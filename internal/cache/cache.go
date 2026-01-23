package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CacheMaxAge is the maximum age of cached PR info before it's considered stale
const CacheMaxAge = 24 * time.Hour

// PRInfo represents pull request information
type PRInfo struct {
	Number       int       `json:"number"`
	State        string    `json:"state"`         // Normalized: OPEN, MERGED, CLOSED
	IsDraft      bool      `json:"is_draft"`      // true if PR is a draft
	URL          string    `json:"url"`
	Author       string    `json:"author"`        // username/login
	CommentCount int       `json:"comment_count"` // number of comments
	HasReviews   bool      `json:"has_reviews"`   // any reviews submitted
	IsApproved   bool      `json:"is_approved"`   // approved status
	CachedAt     time.Time `json:"cached_at"`
	Fetched      bool      `json:"fetched"` // true = API was queried (distinguishes "not fetched" from "no PR")
}

// IsStale returns true if the cache entry is older than CacheMaxAge
func (m *PRInfo) IsStale() bool {
	if m.CachedAt.IsZero() {
		return true
	}
	return time.Since(m.CachedAt) > CacheMaxAge
}

// PRCache maps origin URL -> branch -> PR info (legacy format)
type PRCache map[string]map[string]*PRInfo

// WorktreeIDEntry stores the ID for a worktree with rich metadata for repair
type WorktreeIDEntry struct {
	ID        int        `json:"id"`
	Path      string     `json:"path"`
	RepoPath  string     `json:"repo_path,omitempty"`   // main repo path for repair
	Branch    string     `json:"branch,omitempty"`      // branch name for repair
	OriginURL string     `json:"origin_url,omitempty"`  // origin URL (may be empty for local-only repos)
	RemovedAt *time.Time `json:"removed_at,omitempty"`  // nil if still exists
	PR        *PRInfo    `json:"pr,omitempty"`          // embedded PR info
}

// Cache is the unified cache structure stored in .wt-cache.json
type Cache struct {
	PRs       PRCache                     `json:"prs,omitempty"`
	Worktrees map[string]*WorktreeIDEntry `json:"worktrees,omitempty"` // key: folder name (e.g., "repo-feature-branch")
	NextID    int                         `json:"next_id,omitempty"`
}

// MakeWorktreeKey creates a cache key from worktree path (folder name).
// The folder name is unique within a worktree_dir and human-readable.
func MakeWorktreeKey(path string) string {
	return filepath.Base(path)
}

// CachePath returns the path to the cache file for a directory
func CachePath(dir string) string {
	return filepath.Join(dir, ".wt-cache.json")
}

// LockPath returns the path to the lock file for a directory
func LockPath(dir string) string {
	return filepath.Join(dir, ".wt-cache.lock")
}

// Load loads the unified cache from disk
func Load(scanDir string) (*Cache, error) {
	cachePath := CachePath(scanDir)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Cache{
				PRs:       make(PRCache),
				Worktrees: make(map[string]*WorktreeIDEntry),
				NextID:    1,
			}, nil
		}
		return nil, err
	}

	// Try to unmarshal as new unified format
	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		// Try old format (PRCache only)
		var oldCache PRCache
		if err := json.Unmarshal(data, &oldCache); err != nil {
			// Corrupted - start fresh
			return &Cache{
				PRs:       make(PRCache),
				Worktrees: make(map[string]*WorktreeIDEntry),
				NextID:    1,
			}, nil
		}
		// Migrate from old format
		return &Cache{
			PRs:       oldCache,
			Worktrees: make(map[string]*WorktreeIDEntry),
			NextID:    1,
		}, nil
	}

	// Initialize nil maps
	if cache.PRs == nil {
		cache.PRs = make(PRCache)
	}
	if cache.Worktrees == nil {
		cache.Worktrees = make(map[string]*WorktreeIDEntry)
	}
	if cache.NextID < 1 {
		cache.NextID = 1
	}

	// Clear old PRs map (omitted on save via omitempty)
	// Old PR data in the PRs map is no longer used; PR info is now in WorktreeIDEntry
	cache.PRs = nil

	return &cache, nil
}

// Save saves the unified cache to disk atomically
func Save(scanDir string, cache *Cache) error {
	cachePath := CachePath(scanDir)
	tempPath := cachePath + ".tmp"

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return err
	}

	return os.Rename(tempPath, cachePath)
}

// LoadWithLock acquires a lock and loads the cache.
// Returns cache, unlock function, and error.
// Caller must defer unlock() if err == nil.
func LoadWithLock(dir string) (*Cache, func(), error) {
	lock := NewFileLock(LockPath(dir))
	if err := lock.Lock(); err != nil {
		return nil, nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	cache, err := Load(dir)
	if err != nil {
		lock.Unlock()
		return nil, nil, fmt.Errorf("failed to load cache: %w", err)
	}

	// Wrap unlock to discard the error (it's safe to ignore on unlock)
	unlock := func() { _ = lock.Unlock() }

	return cache, unlock, nil
}

// GetOrAssignID returns the existing ID for a worktree or assigns a new one.
// Uses folder name as key, stores rich metadata for repair/recovery.
func (c *Cache) GetOrAssignID(info WorktreeInfo) int {
	key := MakeWorktreeKey(info.Path)

	if entry, ok := c.Worktrees[key]; ok {
		// Update metadata if changed
		entry.Path = info.Path
		entry.RepoPath = info.RepoPath
		entry.Branch = info.Branch
		entry.OriginURL = info.OriginURL
		// Clear RemovedAt if worktree reappeared
		entry.RemovedAt = nil
		return entry.ID
	}

	// Assign new ID
	id := c.NextID
	c.NextID++
	c.Worktrees[key] = &WorktreeIDEntry{
		ID:        id,
		Path:      info.Path,
		RepoPath:  info.RepoPath,
		Branch:    info.Branch,
		OriginURL: info.OriginURL,
	}

	return id
}

// GetByID looks up a worktree by its ID
// Returns path, found, and whether it was marked as removed
func (c *Cache) GetByID(id int) (path string, found bool, removed bool) {
	for _, entry := range c.Worktrees {
		if entry.ID == id {
			return entry.Path, true, entry.RemovedAt != nil
		}
	}
	return "", false, false
}

// GetBranchByID looks up a worktree by its ID and returns the branch name
// Returns branch, path, found, and whether it was marked as removed
func (c *Cache) GetBranchByID(id int) (branch string, path string, found bool, removed bool) {
	for _, entry := range c.Worktrees {
		if entry.ID == id {
			return entry.Branch, entry.Path, true, entry.RemovedAt != nil
		}
	}
	return "", "", false, false
}

// GetBranchByPRNumber looks up PR info by PR number for a given origin URL
// Returns the branch name if found and cache is not stale, empty string otherwise
func (c *Cache) GetBranchByPRNumber(originURL string, prNumber int) string {
	// Search in embedded PRs
	for _, entry := range c.Worktrees {
		if entry.PR != nil && entry.PR.Number == prNumber && !entry.PR.IsStale() {
			// Compare origin URL from stored metadata
			if entry.OriginURL == originURL {
				return entry.Branch
			}
		}
	}
	return ""
}

// GetPRForBranch returns the cached PR for a worktree by folder name.
// Returns nil if not found or not fetched.
func (c *Cache) GetPRForBranch(folderName string) *PRInfo {
	if entry, ok := c.Worktrees[folderName]; ok {
		return entry.PR
	}
	return nil
}

// GetPRByOriginAndBranch returns the cached PR for a given origin URL and branch.
// Searches through all entries to find a match. Returns nil if not found.
// This is used for backwards compatibility in cases where folder name isn't available.
func (c *Cache) GetPRByOriginAndBranch(originURL, branch string) *PRInfo {
	for _, entry := range c.Worktrees {
		if entry.OriginURL == originURL && entry.Branch == branch {
			return entry.PR
		}
	}
	return nil
}

// SetPRForBranch stores PR info for a worktree by folder name.
// Updates the entry if it exists; does nothing if the entry doesn't exist.
func (c *Cache) SetPRForBranch(folderName string, pr *PRInfo) {
	if entry, ok := c.Worktrees[folderName]; ok {
		entry.PR = pr
	}
}

// SetPRByOriginAndBranch stores PR info by origin URL and branch.
// Searches through all entries to find a match.
// This is used for backwards compatibility in cases where folder name isn't available.
func (c *Cache) SetPRByOriginAndBranch(originURL, branch string, pr *PRInfo) {
	for _, entry := range c.Worktrees {
		if entry.OriginURL == originURL && entry.Branch == branch {
			entry.PR = pr
			return
		}
	}
}

// MarkRemoved sets the RemovedAt timestamp for a worktree entry by folder name.
// Call this immediately after successfully removing a worktree.
func (c *Cache) MarkRemoved(folderName string) {
	if entry, ok := c.Worktrees[folderName]; ok {
		now := time.Now()
		entry.RemovedAt = &now
	}
}

// MarkRemovedByKey is a package-level function for marking an entry as removed.
// Used by doctor command for fixing cache issues.
func MarkRemovedByKey(c *Cache, key string) {
	if entry, ok := c.Worktrees[key]; ok {
		now := time.Now()
		entry.RemovedAt = &now
	}
}

// WorktreeInfo contains the info needed to sync the cache.
// Path is required; other fields are stored as metadata for repair/recovery.
type WorktreeInfo struct {
	Path      string // worktree path (required)
	RepoPath  string // main repo path
	Branch    string // branch name
	OriginURL string // origin URL (may be empty for local-only repos)
}

// SyncWorktrees updates the cache with current worktrees
// Returns a map of path -> ID for display
// Note: entries are never deleted, only marked as removed for history
func (c *Cache) SyncWorktrees(worktrees []WorktreeInfo) map[string]int {
	// Track which keys still exist on disk
	currentKeys := make(map[string]bool)
	pathToID := make(map[string]int)

	for _, wt := range worktrees {
		key := MakeWorktreeKey(wt.Path)
		currentKeys[key] = true

		id := c.GetOrAssignID(wt)
		pathToID[wt.Path] = id
	}

	// Mark missing worktrees as removed (instead of deleting)
	now := time.Now()
	for key, entry := range c.Worktrees {
		if !currentKeys[key] && entry.RemovedAt == nil {
			entry.RemovedAt = &now
		}
	}

	return pathToID
}

// Reset clears all cached data and resets NextID to 1.
// Active worktrees will get new IDs on next sync.
func (c *Cache) Reset() {
	c.PRs = nil
	c.Worktrees = make(map[string]*WorktreeIDEntry)
	c.NextID = 1
}

// Package forge provides an abstraction over git hosting services (GitHub, GitLab).
package forge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/raphi011/wt/internal/git"
)

// CacheMaxAge is the maximum age of cached PR info before it's considered stale
const CacheMaxAge = 24 * time.Hour

// PRInfo represents pull request information
type PRInfo struct {
	Number       int       `json:"number"`
	State        string    `json:"state"`         // Normalized: OPEN, MERGED, CLOSED
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

// Forge represents a git hosting service (GitHub, GitLab, etc.)
type Forge interface {
	// Name returns the forge name ("github" or "gitlab")
	Name() string

	// Check verifies the CLI is installed and authenticated
	Check() error

	// GetPRForBranch fetches PR info for a branch
	GetPRForBranch(repoURL, branch string) (*PRInfo, error)

	// GetPRBranch gets the source branch name for a PR number
	GetPRBranch(repoURL string, number int) (string, error)

	// CloneRepo clones a repository to destPath, returns the full clone path
	CloneRepo(repoSpec, destPath string) (string, error)

	// MergePR merges a PR by number with the given strategy
	// strategy: "squash", "rebase", or "merge"
	// Returns error if repo doesn't allow the requested merge strategy
	MergePR(repoURL string, number int, strategy string) error

	// FormatIcon returns the nerd font icon for PR state
	FormatIcon(state string) string
}

// PRCache maps origin URL -> branch -> PR info
type PRCache map[string]map[string]*PRInfo

// WorktreeIDEntry stores the ID for a worktree
type WorktreeIDEntry struct {
	ID        int        `json:"id"`
	Path      string     `json:"path"`
	RemovedAt *time.Time `json:"removed_at,omitempty"` // nil if still exists
}

// Cache is the unified cache structure stored in .wt-cache.json
type Cache struct {
	PRs       PRCache                     `json:"prs,omitempty"`
	Worktrees map[string]*WorktreeIDEntry `json:"worktrees,omitempty"` // key: "origin::branch"
	NextID    int                         `json:"next_id,omitempty"`
}

// MakeWorktreeKey creates a cache key from origin URL and branch
func MakeWorktreeKey(originURL, branch string) string {
	return originURL + "::" + branch
}

// CachePath returns the path to the cache file for a directory
func CachePath(dir string) string {
	return filepath.Join(dir, ".wt-cache.json")
}

// LockPath returns the path to the lock file for a directory
func LockPath(dir string) string {
	return filepath.Join(dir, ".wt-cache.lock")
}

// LoadCache loads the unified cache from disk
func LoadCache(scanDir string) (*Cache, error) {
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

	return &cache, nil
}

// SaveCache saves the unified cache to disk atomically
func SaveCache(scanDir string, cache *Cache) error {
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

// GetOrAssignID returns the existing ID for a worktree or assigns a new one
func (c *Cache) GetOrAssignID(originURL, branch, path string) int {
	key := MakeWorktreeKey(originURL, branch)

	if entry, ok := c.Worktrees[key]; ok {
		// Update path if changed
		entry.Path = path
		// Clear RemovedAt if worktree reappeared
		entry.RemovedAt = nil
		return entry.ID
	}

	// Assign new ID
	id := c.NextID
	c.NextID++
	c.Worktrees[key] = &WorktreeIDEntry{
		ID:   id,
		Path: path,
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
	for key, entry := range c.Worktrees {
		if entry.ID == id {
			// Key format is "originURL::branch"
			parts := strings.SplitN(key, "::", 2)
			if len(parts) == 2 {
				return parts[1], entry.Path, true, entry.RemovedAt != nil
			}
			return "", entry.Path, true, entry.RemovedAt != nil
		}
	}
	return "", "", false, false
}

// WorktreeInfo contains the minimal info needed to sync the cache
type WorktreeInfo struct {
	Path      string
	Branch    string
	OriginURL string
}

// SyncWorktrees updates the cache with current worktrees
// Returns a map of path -> ID for display
// Note: entries are never deleted, only marked as removed for history
func (c *Cache) SyncWorktrees(worktrees []WorktreeInfo) map[string]int {
	// Track which keys still exist on disk
	currentKeys := make(map[string]bool)
	pathToID := make(map[string]int)

	for _, wt := range worktrees {
		key := MakeWorktreeKey(wt.OriginURL, wt.Branch)
		currentKeys[key] = true

		id := c.GetOrAssignID(wt.OriginURL, wt.Branch, wt.Path)
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

// LoadPRCache loads the PR cache from disk (legacy compatibility)
func LoadPRCache(scanDir string) (PRCache, error) {
	cache, err := LoadCache(scanDir)
	if err != nil {
		return nil, err
	}
	return cache.PRs, nil
}

// SavePRCache saves the PR cache to disk (legacy compatibility)
func SavePRCache(scanDir string, prCache PRCache) error {
	// Load existing cache to preserve worktree IDs
	cache, err := LoadCache(scanDir)
	if err != nil {
		cache = &Cache{
			Worktrees: make(map[string]*WorktreeIDEntry),
			NextID:    1,
		}
	}
	cache.PRs = prCache
	return SaveCache(scanDir, cache)
}

// CleanPRCache removes cache entries for origins/branches that no longer exist
func CleanPRCache(cache PRCache, worktrees []git.Worktree) PRCache {
	existing := make(map[string]map[string]bool)
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}
		if existing[wt.OriginURL] == nil {
			existing[wt.OriginURL] = make(map[string]bool)
		}
		existing[wt.OriginURL][wt.Branch] = true
	}

	cleaned := make(PRCache)
	for origin, branches := range cache {
		if existingBranches, ok := existing[origin]; ok {
			for branch, pr := range branches {
				if existingBranches[branch] && pr != nil {
					if cleaned[origin] == nil {
						cleaned[origin] = make(map[string]*PRInfo)
					}
					cleaned[origin][branch] = pr
				}
			}
		}
	}

	return cleaned
}

// NeedsFetch returns worktrees that need PR info fetched (not cached or stale)
func NeedsFetch(cache PRCache, worktrees []git.Worktree, forceRefresh bool) []git.Worktree {
	var toFetch []git.Worktree
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}

		// Skip branches without upstream (never pushed = no PR possible)
		if git.GetUpstreamBranch(wt.MainRepo, wt.Branch) == "" {
			continue
		}

		if forceRefresh {
			toFetch = append(toFetch, wt)
			continue
		}

		originCache, ok := cache[wt.OriginURL]
		if !ok {
			toFetch = append(toFetch, wt)
			continue
		}

		pr := originCache[wt.Branch]
		// Need to fetch if: no entry, not marked as fetched (old cache), or stale
		if pr == nil || !pr.Fetched || pr.IsStale() {
			toFetch = append(toFetch, wt)
		}
	}
	return toFetch
}

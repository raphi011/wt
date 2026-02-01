// Package prcache provides a simple PR cache stored in ~/.wt/prs.json.
// Unlike the previous cache system, this stores PRs independently of worktree entries,
// allowing PR info to be cached before worktrees are created.
package prcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CacheMaxAge is the maximum age of cached PR info before it's considered stale
const CacheMaxAge = 24 * time.Hour

// PRInfo represents pull request information
type PRInfo struct {
	Number       int       `json:"number"`
	State        string    `json:"state"`    // Normalized: OPEN, MERGED, CLOSED
	IsDraft      bool      `json:"is_draft"` // true if PR is a draft
	URL          string    `json:"url"`
	Author       string    `json:"author"`        // username/login
	CommentCount int       `json:"comment_count"` // number of comments
	HasReviews   bool      `json:"has_reviews"`   // any reviews submitted
	IsApproved   bool      `json:"is_approved"`   // approved status
	CachedAt     time.Time `json:"cached_at"`
	Fetched      bool      `json:"fetched"` // true = API was queried (distinguishes "not fetched" from "no PR")
}

// IsStale returns true if the cache entry is older than CacheMaxAge
func (p *PRInfo) IsStale() bool {
	if p.CachedAt.IsZero() {
		return true
	}
	return time.Since(p.CachedAt) > CacheMaxAge
}

// Cache stores PR info keyed by folder name
type Cache struct {
	PRs map[string]*PRInfo `json:"prs"`
}

// Path returns the path to the PR cache file
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wt", "prs.json")
}

// Load loads the PR cache from disk
func Load() (*Cache, error) {
	cachePath := Path()

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Cache{
				PRs: make(map[string]*PRInfo),
			}, nil
		}
		return nil, err
	}

	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		// Corrupted - start fresh
		return &Cache{
			PRs: make(map[string]*PRInfo),
		}, nil
	}

	// Initialize nil map
	if cache.PRs == nil {
		cache.PRs = make(map[string]*PRInfo)
	}

	return &cache, nil
}

// Save saves the PR cache to disk atomically
func (c *Cache) Save() error {
	cachePath := Path()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}

	tempPath := cachePath + ".tmp"

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}

	return os.Rename(tempPath, cachePath)
}

// Set stores PR info for a folder
func (c *Cache) Set(folder string, pr *PRInfo) {
	c.PRs[folder] = pr
}

// Get returns PR info for a folder, or nil if not found
func (c *Cache) Get(folder string) *PRInfo {
	return c.PRs[folder]
}

// Delete removes PR info for a folder
func (c *Cache) Delete(folder string) {
	delete(c.PRs, folder)
}

// Reset clears all cached data
func (c *Cache) Reset() {
	c.PRs = make(map[string]*PRInfo)
}

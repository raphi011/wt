// Package prcache provides PR status caching stored in ~/.wt/prs.json.
// PRs are stored independently of worktree entries, keyed by folder name,
// allowing PR info to be cached before worktrees are created.
package prcache

import (
	"os"
	"path/filepath"
	"time"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/storage"
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
	PRs   map[string]*PRInfo `json:"prs"`
	dirty bool
}

// New returns an empty, initialized cache.
func New() *Cache {
	return &Cache{PRs: make(map[string]*PRInfo)}
}

// Path returns the path to the PR cache file
func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wt", "prs.json")
}

// Load loads the PR cache from disk. Returns an empty cache if
// the file is missing or corrupted.
func Load() *Cache {
	var cache Cache
	if err := storage.LoadJSON(Path(), &cache); err != nil {
		return New()
	}

	// Initialize nil map
	if cache.PRs == nil {
		cache.PRs = make(map[string]*PRInfo)
	}

	return &cache
}

// Save saves the PR cache to disk atomically
func (c *Cache) Save() error {
	return storage.SaveJSON(Path(), c)
}

// Set stores PR info for a folder
func (c *Cache) Set(folder string, pr *PRInfo) {
	c.PRs[folder] = pr
	c.dirty = true
}

// Get returns PR info for a folder, or nil if not found
func (c *Cache) Get(folder string) *PRInfo {
	return c.PRs[folder]
}

// Delete removes PR info for a folder
func (c *Cache) Delete(folder string) {
	delete(c.PRs, folder)
	c.dirty = true
}

// Reset clears all cached data
func (c *Cache) Reset() {
	c.PRs = make(map[string]*PRInfo)
	c.dirty = true
}

// SaveIfDirty saves the cache to disk only if it has been modified.
// Resets the dirty flag after a successful save.
func (c *Cache) SaveIfDirty() error {
	if !c.dirty {
		return nil
	}
	if err := c.Save(); err != nil {
		return err
	}
	c.dirty = false
	return nil
}

// FromForge converts a forge.PRInfo to a prcache.PRInfo.
// Returns nil if pr is nil.
func FromForge(pr *forge.PRInfo) *PRInfo {
	if pr == nil {
		return nil
	}

	return &PRInfo{
		Number:       pr.Number,
		State:        pr.State,
		IsDraft:      pr.IsDraft,
		URL:          pr.URL,
		Author:       pr.Author,
		CommentCount: pr.CommentCount,
		HasReviews:   pr.HasReviews,
		IsApproved:   pr.IsApproved,
		CachedAt:     pr.CachedAt,
		Fetched:      pr.Fetched,
	}
}

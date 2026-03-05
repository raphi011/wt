// Package prcache provides PR status caching stored in ~/.wt/prs.json.
// PRs are stored independently of worktree entries, keyed by repoPath:branch,
// allowing PR info to be cached before worktrees are created.
package prcache

import (
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/fs"
)

// CacheKey returns the cache key for a worktree, namespaced by repo path.
// Uses repoPath:branch since the PR is tied to the branch, not the folder.
func CacheKey(repoPath, branch string) string {
	return repoPath + ":" + branch
}

// Cache stores PR info keyed by repoPath:branch
type Cache struct {
	PRs   map[string]*forge.PRInfo `json:"prs"`
	dirty bool
}

// New returns an empty, initialized cache.
func New() *Cache {
	return &Cache{PRs: make(map[string]*forge.PRInfo)}
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
	if err := fs.LoadJSON(Path(), &cache); err != nil {
		return New()
	}

	// Initialize nil map
	if cache.PRs == nil {
		cache.PRs = make(map[string]*forge.PRInfo)
	}

	return &cache
}

// Save saves the PR cache to disk atomically
func (c *Cache) Save() error {
	return fs.SaveJSON(Path(), c)
}

// Set stores PR info for a cache key
func (c *Cache) Set(key string, pr *forge.PRInfo) {
	c.PRs[key] = pr
	c.dirty = true
}

// Get returns PR info for a cache key, or nil if not found
func (c *Cache) Get(key string) *forge.PRInfo {
	return c.PRs[key]
}

// Delete removes PR info for a cache key
func (c *Cache) Delete(key string) {
	delete(c.PRs, key)
	c.dirty = true
}

// Reset clears all cached data
func (c *Cache) Reset() {
	c.PRs = make(map[string]*forge.PRInfo)
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

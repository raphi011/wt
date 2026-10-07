// Package prcache provides PR status caching stored in prs.json in the wt dir.
// PRs are stored independently of worktree entries, keyed by repoPath:branch,
// allowing PR info to be cached before worktrees are created.
package prcache

import (
	"errors"
	"fmt"
	"os"

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
	PRs  map[string]*forge.PRInfo `json:"prs"`
	path string
	// changes made since the cache was loaded or last saved, in order
	changes []func(*Cache)
	loadErr error
	reset   bool
}

// New returns an empty, initialized cache.
func New() *Cache {
	return &Cache{PRs: make(map[string]*forge.PRInfo)}
}

// LoadFrom loads the PR cache from the given path. A missing file is empty.
// Other read or decode errors produce an empty cache with a LoadError;
// persistence is disabled until an explicit Reset succeeds.
func LoadFrom(path string) *Cache {
	var cache Cache
	if err := fs.LoadJSON(path, &cache); err != nil {
		cache = Cache{}
		if !os.IsNotExist(err) {
			cache.loadErr = fmt.Errorf("load PR cache %s: %w", path, err)
		}
	}

	// Initialize nil map
	if cache.PRs == nil {
		cache.PRs = make(map[string]*forge.PRInfo)
	}
	cache.path = path

	return &cache
}

// LoadError reports a read or decode failure encountered by LoadFrom.
// A successful explicit reset clears the error.
func (c *Cache) LoadError() error {
	return c.loadErr
}

// Save applies the changes made since the cache was loaded or last saved to
// the file it was loaded from. The changes are replayed onto the current file
// contents under a file lock, so concurrent saves only overwrite each other
// per key. Corrupt files are preserved unless Reset was explicitly requested.
// Does nothing if there are no changes.
func (c *Cache) Save() error {
	if len(c.changes) == 0 {
		return nil
	}
	if c.path == "" {
		return errors.New("PR cache has no path: load it with LoadFrom")
	}
	if c.loadErr != nil && !c.reset {
		return c.loadErr
	}

	update := fs.UpdateJSON[Cache]
	if c.reset {
		update = fs.UpdateJSONLenient[Cache]
	}
	err := update(c.path, func(saved *Cache) error {
		if saved.PRs == nil {
			saved.PRs = make(map[string]*forge.PRInfo)
		}
		for _, change := range c.changes {
			change(saved)
		}
		return nil
	})
	if err != nil {
		return err
	}

	c.changes = nil
	c.loadErr = nil
	c.reset = false
	return nil
}

// apply makes a change to the cache and records it for Save
func (c *Cache) apply(change func(*Cache)) {
	change(c)
	c.changes = append(c.changes, change)
}

// Set stores PR info for a cache key
func (c *Cache) Set(key string, pr *forge.PRInfo) {
	c.apply(func(c *Cache) { c.PRs[key] = pr })
}

// Get returns PR info for a cache key, or nil if not found
func (c *Cache) Get(key string) *forge.PRInfo {
	return c.PRs[key]
}

// Delete removes PR info for a cache key
func (c *Cache) Delete(key string) {
	c.apply(func(c *Cache) { delete(c.PRs, key) })
}

// Reset clears all cached data and permits replacing corrupt JSON on the next
// successful Save. Read errors still prevent persistence.
func (c *Cache) Reset() {
	c.reset = true
	c.apply(func(c *Cache) { c.PRs = make(map[string]*forge.PRInfo) })
}

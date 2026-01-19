// Package forge provides an abstraction over git hosting services (GitHub, GitLab).
package forge

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	// FormatIcon returns the nerd font icon for PR state
	FormatIcon(state string) string
}

// PRCache maps origin URL -> branch -> PR info
type PRCache map[string]map[string]*PRInfo

// LoadPRCache loads the PR cache from disk
func LoadPRCache(scanDir string) (PRCache, error) {
	cachePath := filepath.Join(scanDir, ".wt-cache.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(PRCache), nil
		}
		return nil, err
	}

	var cache PRCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return cache, nil
}

// SavePRCache saves the PR cache to disk atomically
func SavePRCache(scanDir string, cache PRCache) error {
	cachePath := filepath.Join(scanDir, ".wt-cache.json")
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

// Package forge provides an abstraction over git hosting services (GitHub, GitLab).
package forge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/raphi011/wt/internal/git"
)

// CacheMaxAge is the maximum age of cached MR info before it's considered stale
const CacheMaxAge = 24 * time.Hour

// MRInfo represents merge/pull request information (works for both GitHub PRs and GitLab MRs)
type MRInfo struct {
	Number   int       `json:"number"`
	State    string    `json:"state"` // Normalized: OPEN, MERGED, CLOSED
	URL      string    `json:"url"`
	CachedAt time.Time `json:"cached_at"`
}

// IsStale returns true if the cache entry is older than CacheMaxAge
func (m *MRInfo) IsStale() bool {
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

	// GetMRForBranch fetches MR/PR info for a branch
	GetMRForBranch(repoURL, branch string) (*MRInfo, error)

	// GetMRBranch gets the source branch name for an MR/PR number
	GetMRBranch(repoURL string, number int) (string, error)

	// CloneRepo clones a repository to destPath, returns the full clone path
	CloneRepo(repoSpec, destPath string) (string, error)

	// FormatIcon returns the nerd font icon for MR state
	FormatIcon(state string) string
}

// MRCache maps origin URL -> branch -> MR info
type MRCache map[string]map[string]*MRInfo

// LoadMRCache loads the MR cache from disk
func LoadMRCache(scanDir string) (MRCache, error) {
	cachePath := filepath.Join(scanDir, ".wt-cache.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(MRCache), nil
		}
		return nil, err
	}

	var cache MRCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return cache, nil
}

// SaveMRCache saves the MR cache to disk atomically
func SaveMRCache(scanDir string, cache MRCache) error {
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

// CleanMRCache removes cache entries for origins/branches that no longer exist
func CleanMRCache(cache MRCache, worktrees []git.Worktree) MRCache {
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

	cleaned := make(MRCache)
	for origin, branches := range cache {
		if existingBranches, ok := existing[origin]; ok {
			for branch, mr := range branches {
				if existingBranches[branch] && mr != nil {
					if cleaned[origin] == nil {
						cleaned[origin] = make(map[string]*MRInfo)
					}
					cleaned[origin][branch] = mr
				}
			}
		}
	}

	return cleaned
}

// NeedsFetch returns worktrees that need MR info fetched (not cached or stale)
func NeedsFetch(cache MRCache, worktrees []git.Worktree, forceRefresh bool) []git.Worktree {
	var toFetch []git.Worktree
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}

		// Skip branches without upstream (never pushed = no MR possible)
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

		mr := originCache[wt.Branch]
		if mr == nil || mr.IsStale() {
			toFetch = append(toFetch, wt)
		}
	}
	return toFetch
}

package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/raphaelgruber/wt/internal/git"
)

// CacheMaxAge is the maximum age of cached PR info before it's considered stale
const CacheMaxAge = 24 * time.Hour

// PRInfo represents GitHub PR information
type PRInfo struct {
	Number   int       `json:"number"`
	State    string    `json:"state"` // OPEN, MERGED, CLOSED
	URL      string    `json:"url"`
	CachedAt time.Time `json:"cached_at"`
}

// IsStale returns true if the cache entry is older than CacheMaxAge
func (p *PRInfo) IsStale() bool {
	if p.CachedAt.IsZero() {
		return true
	}
	return time.Since(p.CachedAt) > CacheMaxAge
}

// GetPRForBranch fetches PR info for a branch using gh CLI
func GetPRForBranch(repoURL, branch string) (*PRInfo, error) {
	cmd := exec.Command("gh", "pr", "list",
		"-R", repoURL,
		"--head", branch,
		"--state", "all",
		"--json", "number,state,url",
		"--limit", "1")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("gh command failed: %s", errMsg)
		}
		return nil, fmt.Errorf("gh command failed: %w", err)
	}

	// Parse JSON array
	var prs []struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	if len(prs) == 0 {
		return nil, nil // No PR found
	}

	return &PRInfo{
		Number:   prs[0].Number,
		State:    prs[0].State,
		URL:      prs[0].URL,
		CachedAt: time.Now(),
	}, nil
}

// GetOriginURL gets the origin URL for a repository
func GetOriginURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("failed to get origin URL: %s", errMsg)
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// FormatPRIcon returns the nerd font icon for PR state
func FormatPRIcon(state string) string {
	switch state {
	case "MERGED":
		return "󰜘" // Merged icon
	case "OPEN":
		return "󰜛" // Open icon
	case "CLOSED":
		return "󰅖" // Closed icon
	default:
		return ""
	}
}

// PRCache maps origin URL -> branch -> PR info
type PRCache map[string]map[string]*PRInfo

// LoadPRCache loads the PR cache from disk
func LoadPRCache(scanDir string) (PRCache, error) {
	cachePath := filepath.Join(scanDir, ".wt-cache.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(PRCache), nil // Return empty cache if file doesn't exist
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

	// Write to temp file first (0600 for user-only read/write)
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return err
	}

	// Atomic rename
	return os.Rename(tempPath, cachePath)
}

// CleanPRCache removes cache entries for origins/branches that no longer exist
// Also removes stale entries older than CacheMaxAge
func CleanPRCache(cache PRCache, worktrees []git.Worktree) PRCache {
	// Build set of existing origin+branch combinations
	existing := make(map[string]map[string]bool)
	for _, wt := range worktrees {
		originURL, _ := GetOriginURL(wt.MainRepo)
		if originURL == "" {
			continue
		}
		if existing[originURL] == nil {
			existing[originURL] = make(map[string]bool)
		}
		existing[originURL][wt.Branch] = true
	}

	// Create new cache with only existing entries that aren't stale
	cleaned := make(PRCache)
	for origin, branches := range cache {
		if existingBranches, ok := existing[origin]; ok {
			for branch, pr := range branches {
				if existingBranches[branch] && pr != nil {
					// Keep if exists and not stale (stale entries will be refreshed)
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

// GetPRBranch fetches the head branch name for a PR number using gh CLI
// Returns an error if the PR is from a fork (cross-repository PR)
func GetPRBranch(repoURL string, number int) (string, error) {
	cmd := exec.Command("gh", "pr", "view",
		fmt.Sprintf("%d", number),
		"-R", repoURL,
		"--json", "headRefName,isCrossRepository")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("gh command failed: %s", errMsg)
		}
		return "", fmt.Errorf("gh command failed: %w", err)
	}

	var result struct {
		HeadRefName       string `json:"headRefName"`
		IsCrossRepository bool   `json:"isCrossRepository"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse gh output: %w", err)
	}

	if result.IsCrossRepository {
		return "", fmt.Errorf("PR #%d is from a fork - cross-repository PRs are not supported", number)
	}

	if result.HeadRefName == "" {
		return "", fmt.Errorf("PR #%d has no head branch", number)
	}

	return result.HeadRefName, nil
}

// NeedsFetch returns worktrees that need PR info fetched (not cached or stale)
// Skips worktrees without an upstream branch configured (never pushed = no PR)
func NeedsFetch(cache PRCache, worktrees []git.Worktree, forceRefresh bool) []git.Worktree {
	var toFetch []git.Worktree
	for _, wt := range worktrees {
		originURL, _ := GetOriginURL(wt.MainRepo)
		if originURL == "" {
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

		originCache, ok := cache[originURL]
		if !ok {
			toFetch = append(toFetch, wt)
			continue
		}

		pr := originCache[wt.Branch]
		if pr == nil || pr.IsStale() {
			toFetch = append(toFetch, wt)
		}
	}
	return toFetch
}

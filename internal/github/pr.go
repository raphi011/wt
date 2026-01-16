package github

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/raphaelgruber/wt/internal/git"
)

// PRInfo represents GitHub PR information
type PRInfo struct {
	Number int    `json:"number"`
	State  string `json:"state"` // OPEN, MERGED, CLOSED
	URL    string `json:"url"`
}

// GetPRForBranch fetches PR info for a branch using gh CLI
func GetPRForBranch(repoURL, branch string) (*PRInfo, error) {
	cmd := exec.Command("gh", "pr", "list",
		"-R", repoURL,
		"--head", branch,
		"--state", "all",
		"--json", "number,state,url",
		"--limit", "1")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh command failed: %w", err)
	}

	// Parse JSON array
	var prs []PRInfo
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	if len(prs) == 0 {
		return nil, nil // No PR found
	}

	return &prs[0], nil
}

// PRInfoWithHead extends PRInfo with head branch name
type PRInfoWithHead struct {
	PRInfo
	HeadRefName string `json:"headRefName"`
}

// GetAllPRsForRepo fetches PRs for a repository, optionally filtered by creation date
func GetAllPRsForRepo(repoURL string, oldestDate string) (map[string]*PRInfo, error) {
	args := []string{"pr", "list",
		"-R", repoURL,
		"--state", "all",
		"--json", "number,state,url,headRefName",
		"--limit", "200"}

	// Add date filter if provided
	if oldestDate != "" {
		args = append(args, "--search", fmt.Sprintf("created:>=%s", oldestDate))
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh command failed: %w", err)
	}

	// Parse JSON array
	var prs []PRInfoWithHead
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	// Build map of branch -> PR info
	result := make(map[string]*PRInfo)
	for _, pr := range prs {
		result[pr.HeadRefName] = &PRInfo{
			Number: pr.Number,
			State:  pr.State,
			URL:    pr.URL,
		}
	}

	return result, nil
}

// GetOriginURL gets the origin URL for a repository
func GetOriginURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
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

// FormatPRDisplay formats PR info for display
func FormatPRDisplay(pr *PRInfo) string {
	if pr == nil {
		return ""
	}
	icon := FormatPRIcon(pr.State)
	return fmt.Sprintf("%s #%d", icon, pr.Number)
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

// SavePRCache saves the PR cache to disk
func SavePRCache(scanDir string, cache PRCache) error {
	cachePath := filepath.Join(scanDir, ".wt-cache.json")

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// CleanPRCache removes cache entries for origins/branches that no longer exist
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

	// Create new cache with only existing entries
	cleaned := make(PRCache)
	for origin, branches := range cache {
		if existingBranches, ok := existing[origin]; ok {
			cleaned[origin] = make(map[string]*PRInfo)
			for branch, pr := range branches {
				if existingBranches[branch] {
					cleaned[origin][branch] = pr
				}
			}
		}
	}

	return cleaned
}

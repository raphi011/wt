package forge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitLab implements Forge for GitLab repositories using the glab CLI.
type GitLab struct{}

// Name returns "gitlab"
func (g *GitLab) Name() string {
	return "gitlab"
}

// Check verifies that glab CLI is available and authenticated
func (g *GitLab) Check() error {
	_, err := exec.LookPath("glab")
	if err != nil {
		return fmt.Errorf("glab not found: please install GitLab CLI (https://gitlab.com/gitlab-org/cli)")
	}

	cmd := exec.Command("glab", "auth", "status")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if strings.Contains(errMsg, "not logged") || strings.Contains(errMsg, "no token") {
			return fmt.Errorf("glab not authenticated: please run 'glab auth login'")
		}
		if errMsg != "" {
			return fmt.Errorf("glab auth check failed: %s", errMsg)
		}
		return fmt.Errorf("glab not authenticated: please run 'glab auth login'")
	}

	return nil
}

// GetMRForBranch fetches MR info for a branch using glab CLI
func (g *GitLab) GetMRForBranch(repoURL, branch string) (*MRInfo, error) {
	// glab uses -R for repo like gh, but needs project path format
	projectPath := extractGitLabProject(repoURL)

	cmd := exec.Command("glab", "mr", "list",
		"-R", projectPath,
		"--source-branch", branch,
		"--state", "all",
		"-F", "json",
		"-P", "1") // limit to 1

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("glab command failed: %s", errMsg)
		}
		return nil, fmt.Errorf("glab command failed: %w", err)
	}

	// glab returns an array of MRs
	var mrs []struct {
		IID      int    `json:"iid"`
		State    string `json:"state"` // opened, merged, closed
		WebURL   string `json:"web_url"`
	}
	if err := json.Unmarshal(output, &mrs); err != nil {
		return nil, fmt.Errorf("failed to parse glab output: %w", err)
	}

	if len(mrs) == 0 {
		return nil, nil
	}

	return &MRInfo{
		Number:   mrs[0].IID,
		State:    normalizeGitLabState(mrs[0].State),
		URL:      mrs[0].WebURL,
		CachedAt: time.Now(),
	}, nil
}

// GetMRBranch fetches the source branch name for an MR number using glab CLI
func (g *GitLab) GetMRBranch(repoURL string, number int) (string, error) {
	projectPath := extractGitLabProject(repoURL)

	cmd := exec.Command("glab", "mr", "view",
		fmt.Sprintf("%d", number),
		"-R", projectPath,
		"-F", "json")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("glab command failed: %s", errMsg)
		}
		return "", fmt.Errorf("glab command failed: %w", err)
	}

	var result struct {
		SourceBranch    string `json:"source_branch"`
		SourceProjectID int    `json:"source_project_id"`
		TargetProjectID int    `json:"target_project_id"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse glab output: %w", err)
	}

	// Check for cross-project MR (fork)
	if result.SourceProjectID != 0 && result.TargetProjectID != 0 &&
		result.SourceProjectID != result.TargetProjectID {
		return "", fmt.Errorf("MR !%d is from a fork - cross-project MRs are not supported", number)
	}

	if result.SourceBranch == "" {
		return "", fmt.Errorf("MR !%d has no source branch", number)
	}

	return result.SourceBranch, nil
}

// CloneRepo clones a GitLab repo using glab CLI
func (g *GitLab) CloneRepo(repoSpec, destPath string) (string, error) {
	parts := strings.Split(repoSpec, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid repo spec %q: expected group/repo format", repoSpec)
	}
	repoName := parts[len(parts)-1]
	clonePath := filepath.Join(destPath, repoName)

	cmd := exec.Command("glab", "repo", "clone", repoSpec, clonePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("glab repo clone failed: %s", errMsg)
		}
		return "", fmt.Errorf("glab repo clone failed: %w", err)
	}

	return clonePath, nil
}

// FormatIcon returns the nerd font icon for MR state
func (g *GitLab) FormatIcon(state string) string {
	switch state {
	case "MERGED":
		return "󰜘"
	case "OPEN":
		return "󰜛"
	case "CLOSED":
		return "󰅖"
	default:
		return ""
	}
}

// normalizeGitLabState converts GitLab state to normalized format
func normalizeGitLabState(state string) string {
	switch strings.ToLower(state) {
	case "opened":
		return "OPEN"
	case "merged":
		return "MERGED"
	case "closed":
		return "CLOSED"
	default:
		return strings.ToUpper(state)
	}
}

// extractGitLabProject extracts the project path from a GitLab URL
// e.g., "git@gitlab.com:group/project.git" -> "group/project"
// e.g., "https://gitlab.com/group/subgroup/project.git" -> "group/subgroup/project"
func extractGitLabProject(url string) string {
	url = strings.TrimSuffix(url, ".git")

	// SSH format: git@gitlab.com:group/project
	if strings.HasPrefix(url, "git@") {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}

	// HTTPS format: https://gitlab.com/group/project
	if strings.Contains(url, "://") {
		parts := strings.SplitN(url, "://", 2)
		if len(parts) == 2 {
			// Remove host, keep path
			pathParts := strings.SplitN(parts[1], "/", 2)
			if len(pathParts) == 2 {
				return pathParts[1]
			}
		}
	}

	return url
}

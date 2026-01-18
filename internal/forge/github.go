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

// GitHub implements Forge for GitHub repositories using the gh CLI.
type GitHub struct{}

// Name returns "github"
func (g *GitHub) Name() string {
	return "github"
}

// Check verifies that gh CLI is available and authenticated
func (g *GitHub) Check() error {
	_, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("gh not found: please install GitHub CLI (https://cli.github.com)")
	}

	cmd := exec.Command("gh", "auth", "status")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if strings.Contains(errMsg, "not logged") || strings.Contains(errMsg, "no accounts") {
			return fmt.Errorf("gh not authenticated: please run 'gh auth login'")
		}
		if errMsg != "" {
			return fmt.Errorf("gh auth check failed: %s", errMsg)
		}
		return fmt.Errorf("gh not authenticated: please run 'gh auth login'")
	}

	return nil
}

// GetMRForBranch fetches PR info for a branch using gh CLI
func (g *GitHub) GetMRForBranch(repoURL, branch string) (*MRInfo, error) {
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

	var prs []struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	if len(prs) == 0 {
		return nil, nil
	}

	return &MRInfo{
		Number:   prs[0].Number,
		State:    prs[0].State, // GitHub already uses OPEN, MERGED, CLOSED
		URL:      prs[0].URL,
		CachedAt: time.Now(),
	}, nil
}

// GetMRBranch fetches the head branch name for a PR number using gh CLI
func (g *GitHub) GetMRBranch(repoURL string, number int) (string, error) {
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

// CloneRepo clones a GitHub repo using gh CLI
func (g *GitHub) CloneRepo(repoSpec, destPath string) (string, error) {
	parts := strings.Split(repoSpec, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo spec %q: expected org/repo format", repoSpec)
	}
	repoName := parts[1]
	clonePath := filepath.Join(destPath, repoName)

	cmd := exec.Command("gh", "repo", "clone", repoSpec, clonePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("gh repo clone failed: %s", errMsg)
		}
		return "", fmt.Errorf("gh repo clone failed: %w", err)
	}

	return clonePath, nil
}

// FormatIcon returns the nerd font icon for PR state
func (g *GitHub) FormatIcon(state string) string {
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

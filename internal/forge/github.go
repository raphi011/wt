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

// GetPRForBranch fetches PR info for a branch using gh CLI
func (g *GitHub) GetPRForBranch(repoURL, branch string) (*PRInfo, error) {
	cmd := exec.Command("gh", "pr", "list",
		"-R", repoURL,
		"--head", branch,
		"--state", "all",
		"--json", "number,state,isDraft,url,author,comments,reviewDecision",
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
		Number  int    `json:"number"`
		State   string `json:"state"`
		IsDraft bool   `json:"isDraft"`
		URL     string `json:"url"`
		Author  struct {
			Login string `json:"login"`
		} `json:"author"`
		Comments       []any  `json:"comments"` // just need the count
		ReviewDecision string `json:"reviewDecision"`
	}
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	if len(prs) == 0 {
		// No PR found - return marker indicating we checked
		return &PRInfo{
			Fetched:  true,
			CachedAt: time.Now(),
		}, nil
	}

	pr := prs[0]
	return &PRInfo{
		Number:       pr.Number,
		State:        pr.State, // GitHub already uses OPEN, MERGED, CLOSED
		IsDraft:      pr.IsDraft,
		URL:          pr.URL,
		Author:       pr.Author.Login,
		CommentCount: len(pr.Comments),
		HasReviews:   pr.ReviewDecision != "",
		IsApproved:   pr.ReviewDecision == "APPROVED",
		CachedAt:     time.Now(),
		Fetched:      true,
	}, nil
}

// GetPRBranch fetches the head branch name for a PR number using gh CLI
func (g *GitHub) GetPRBranch(repoURL string, number int) (string, error) {
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
	org, repoName := parts[0], parts[1]
	if org == "" || repoName == "" {
		return "", fmt.Errorf("invalid repo spec %q: org and repo must not be empty", repoSpec)
	}
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

// CreatePR creates a new PR using gh CLI
func (g *GitHub) CreatePR(repoURL string, params CreatePRParams) (*CreatePRResult, error) {
	args := []string{"pr", "create",
		"-R", repoURL,
		"--title", params.Title,
		"--body", params.Body,
	}

	if params.Base != "" {
		args = append(args, "--base", params.Base)
	}
	if params.Head != "" {
		args = append(args, "--head", params.Head)
	}
	if params.Draft {
		args = append(args, "--draft")
	}

	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("gh pr create failed: %s", errMsg)
		}
		return nil, fmt.Errorf("gh pr create failed: %w", err)
	}

	// Parse PR URL from stdout (gh pr create outputs the URL)
	prURL := strings.TrimSpace(stdout.String())
	if prURL == "" {
		return nil, fmt.Errorf("gh pr create returned empty output")
	}

	// Extract PR number from URL (e.g., https://github.com/org/repo/pull/123)
	parts := strings.Split(prURL, "/")
	var prNumber int
	if len(parts) > 0 {
		fmt.Sscanf(parts[len(parts)-1], "%d", &prNumber)
	}

	return &CreatePRResult{
		Number: prNumber,
		URL:    prURL,
	}, nil
}

// MergePR merges a PR by number with the given strategy
func (g *GitHub) MergePR(repoURL string, number int, strategy string) error {
	// Map strategy to gh flag
	strategyFlag := "--squash" // default
	switch strategy {
	case "rebase":
		strategyFlag = "--rebase"
	case "merge":
		strategyFlag = "--merge"
	}

	cmd := exec.Command("gh", "pr", "merge", fmt.Sprintf("%d", number),
		"-R", repoURL,
		strategyFlag,
		"--delete-branch")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return fmt.Errorf("merge failed: %s", errMsg)
		}
		return fmt.Errorf("merge failed: %w", err)
	}
	return nil
}

// FormatState returns a human-readable PR state
func (g *GitHub) FormatState(state string) string {
	switch state {
	case "MERGED":
		return "merged"
	case "OPEN":
		return "open"
	case "DRAFT":
		return "draft"
	case "CLOSED":
		return "closed"
	default:
		return ""
	}
}

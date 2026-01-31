package forge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/raphi011/wt/internal/config"
)

// GitHub implements Forge for GitHub repositories using the gh CLI.
type GitHub struct {
	ForgeConfig *config.ForgeConfig
}

// Name returns "github"
func (g *GitHub) Name() string {
	return "github"
}

// Check verifies that gh CLI is available and authenticated
func (g *GitHub) Check(ctx context.Context) error {
	_, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("gh not found: please install GitHub CLI (https://cli.github.com)")
	}

	c := exec.CommandContext(ctx, "gh", "auth", "status")
	if out, err := c.CombinedOutput(); err != nil {
		errMsg := string(out)
		if strings.Contains(errMsg, "not logged") || strings.Contains(errMsg, "no accounts") {
			return fmt.Errorf("gh not authenticated: please run 'gh auth login'")
		}
		return fmt.Errorf("gh auth check failed: %s", errMsg)
	}

	return nil
}

// GetPRForBranch fetches PR info for a branch using gh CLI
func (g *GitHub) GetPRForBranch(ctx context.Context, repoURL, branch string) (*PRInfo, error) {
	repoPath := extractRepoPath(repoURL)
	output, err := g.outputWithUser(ctx, repoPath, "pr", "list",
		"-R", repoPath,
		"--head", branch,
		"--state", "all",
		"--json", "number,state,isDraft,url,author,comments,reviewDecision",
		"--limit", "1")
	if err != nil {
		return nil, fmt.Errorf("gh command failed: %v", err)
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
func (g *GitHub) GetPRBranch(ctx context.Context, repoURL string, number int) (string, error) {
	repoPath := extractRepoPath(repoURL)
	output, err := g.outputWithUser(ctx, repoPath, "pr", "view",
		fmt.Sprintf("%d", number),
		"-R", repoPath,
		"--json", "headRefName,isCrossRepository")
	if err != nil {
		return "", fmt.Errorf("gh command failed: %v", err)
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
func (g *GitHub) CloneRepo(ctx context.Context, repoSpec, destPath string) (string, error) {
	parts := strings.Split(repoSpec, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo spec %q: expected org/repo format", repoSpec)
	}
	org, repoName := parts[0], parts[1]
	if org == "" || repoName == "" {
		return "", fmt.Errorf("invalid repo spec %q: org and repo must not be empty", repoSpec)
	}
	clonePath := filepath.Join(destPath, repoName)

	if err := g.runWithUser(ctx, repoSpec, "repo", "clone", repoSpec, clonePath); err != nil {
		return "", fmt.Errorf("gh repo clone failed: %v", err)
	}

	return clonePath, nil
}

// CloneBareRepo clones a GitHub repo as a bare repo inside .git directory
func (g *GitHub) CloneBareRepo(ctx context.Context, repoSpec, destPath string) (string, error) {
	parts := strings.Split(repoSpec, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo spec %q: expected org/repo format", repoSpec)
	}
	org, repoName := parts[0], parts[1]
	if org == "" || repoName == "" {
		return "", fmt.Errorf("invalid repo spec %q: org and repo must not be empty", repoSpec)
	}

	// Final repo directory
	repoDir := filepath.Join(destPath, repoName)

	// Create the repo directory
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Clone as bare directly into .git subdirectory
	gitDir := filepath.Join(repoDir, ".git")
	if err := g.runWithUser(ctx, repoSpec, "repo", "clone", repoSpec, gitDir, "--", "--bare"); err != nil {
		os.RemoveAll(repoDir)
		return "", fmt.Errorf("gh repo clone failed: %v", err)
	}

	// Configure the repo for worktree support
	if err := g.configureBareRepo(ctx, gitDir); err != nil {
		os.RemoveAll(repoDir)
		return "", err
	}

	return repoDir, nil
}

// configureBareRepo configures a bare repo for worktree support
func (g *GitHub) configureBareRepo(ctx context.Context, gitDir string) error {
	// Set core.bare=false so worktree commands work properly
	c := exec.CommandContext(ctx, "git", "-C", gitDir, "config", "core.bare", "false")
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to configure core.bare: %w", err)
	}

	// Set fetch refspec to get all branches
	c = exec.CommandContext(ctx, "git", "-C", gitDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to configure fetch refspec: %w", err)
	}

	return nil
}

// CreatePR creates a new PR using gh CLI
func (g *GitHub) CreatePR(ctx context.Context, repoURL string, params CreatePRParams) (*CreatePRResult, error) {
	repoPath := extractRepoPath(repoURL)
	args := []string{"pr", "create",
		"-R", repoPath,
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

	output, err := g.outputWithUser(ctx, repoPath, args...)
	if err != nil {
		return nil, fmt.Errorf("gh pr create failed: %v", err)
	}

	// Parse PR URL from stdout (gh pr create outputs the URL)
	prURL := strings.TrimSpace(string(output))
	if prURL == "" {
		return nil, fmt.Errorf("gh pr create returned empty output")
	}

	// Extract PR number from URL (e.g., https://github.com/org/repo/pull/123)
	urlParts := strings.Split(prURL, "/")
	var prNumber int
	if len(urlParts) > 0 {
		fmt.Sscanf(urlParts[len(urlParts)-1], "%d", &prNumber)
	}

	return &CreatePRResult{
		Number: prNumber,
		URL:    prURL,
	}, nil
}

// MergePR merges a PR by number with the given strategy
func (g *GitHub) MergePR(ctx context.Context, repoURL string, number int, strategy string) error {
	repoPath := extractRepoPath(repoURL)

	// Map strategy to gh flag
	strategyFlag := "--squash" // default
	switch strategy {
	case "rebase":
		strategyFlag = "--rebase"
	case "merge":
		strategyFlag = "--merge"
	}

	if err := g.runWithUser(ctx, repoPath, "pr", "merge", fmt.Sprintf("%d", number),
		"-R", repoPath,
		strategyFlag,
		"--delete-branch"); err != nil {
		return fmt.Errorf("merge failed: %v", err)
	}
	return nil
}

// ViewPR shows PR details or opens in browser
func (g *GitHub) ViewPR(ctx context.Context, repoURL string, number int, web bool) error {
	repoPath := extractRepoPath(repoURL)
	args := []string{"pr", "view", fmt.Sprintf("%d", number), "-R", repoPath}
	if web {
		args = append(args, "--web")
	}
	c := exec.CommandContext(ctx, "gh", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	user := g.getUserForRepo(repoPath)
	if user != "" {
		token, err := g.getToken(ctx, user)
		if err != nil {
			return err
		}
		c.Env = append(os.Environ(), "GH_TOKEN="+token)
	}

	return c.Run()
}

// ListOpenPRs lists all open PRs for a repository
func (g *GitHub) ListOpenPRs(ctx context.Context, repoURL string) ([]OpenPR, error) {
	repoPath := extractRepoPath(repoURL)
	output, err := g.outputWithUser(ctx, repoPath, "pr", "list",
		"-R", repoPath,
		"--state", "open",
		"--json", "number,title,headRefName,author,isDraft",
		"--limit", "100")
	if err != nil {
		return nil, fmt.Errorf("gh command failed: %v", err)
	}

	var prs []struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		HeadRefName string `json:"headRefName"`
		IsDraft     bool   `json:"isDraft"`
		Author      struct {
			Login string `json:"login"`
		} `json:"author"`
	}
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse gh output: %w", err)
	}

	result := make([]OpenPR, len(prs))
	for i, pr := range prs {
		result[i] = OpenPR{
			Number:  pr.Number,
			Title:   pr.Title,
			Author:  pr.Author.Login,
			Branch:  pr.HeadRefName,
			IsDraft: pr.IsDraft,
		}
	}

	return result, nil
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

// getUserForRepo returns the gh username for a repo path based on forge rules
func (g *GitHub) getUserForRepo(repoPath string) string {
	if g.ForgeConfig == nil {
		return ""
	}
	return g.ForgeConfig.GetUserForRepo(repoPath)
}

// getToken retrieves the auth token for a specific gh user
func (g *GitHub) getToken(ctx context.Context, user string) (string, error) {
	out, err := exec.CommandContext(ctx, "gh", "auth", "token", "--user", user).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get token for user %s: %w", user, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runWithUser runs a gh command with the appropriate user token
func (g *GitHub) runWithUser(ctx context.Context, repoPath string, args ...string) error {
	user := g.getUserForRepo(repoPath)
	c := exec.CommandContext(ctx, "gh", args...)
	c.Stderr = os.Stderr

	if user != "" {
		token, err := g.getToken(ctx, user)
		if err != nil {
			return err
		}
		c.Env = append(os.Environ(), "GH_TOKEN="+token)
	}

	return c.Run()
}

// outputWithUser runs a gh command with the appropriate user token and returns output
func (g *GitHub) outputWithUser(ctx context.Context, repoPath string, args ...string) ([]byte, error) {
	user := g.getUserForRepo(repoPath)
	c := exec.CommandContext(ctx, "gh", args...)

	if user != "" {
		token, err := g.getToken(ctx, user)
		if err != nil {
			return nil, err
		}
		c.Env = append(os.Environ(), "GH_TOKEN="+token)
	}

	out, err := c.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w: %s", err, string(exitErr.Stderr))
		}
		return nil, err
	}
	return out, nil
}

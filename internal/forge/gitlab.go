package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/raphi011/wt/internal/cmd"
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

	c := exec.Command("glab", "auth", "status")
	if err := cmd.Run(c); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not logged") || strings.Contains(errMsg, "no token") {
			return fmt.Errorf("glab not authenticated: please run 'glab auth login'")
		}
		return fmt.Errorf("glab auth check failed: %s", errMsg)
	}

	return nil
}

// GetPRForBranch fetches PR info for a branch using glab CLI
func (g *GitLab) GetPRForBranch(repoURL, branch string) (*PRInfo, error) {
	// glab uses -R for repo like gh, but needs project path format
	projectPath := extractGitLabProject(repoURL)

	c := exec.Command("glab", "mr", "list",
		"-R", projectPath,
		"--source-branch", branch,
		"--state", "all",
		"-F", "json",
		"-P", "1") // limit to 1

	output, err := cmd.Output(c)
	if err != nil {
		return nil, fmt.Errorf("glab command failed: %v", err)
	}

	// glab returns an array of MRs with various fields
	var prs []struct {
		IID    int    `json:"iid"`
		State  string `json:"state"` // opened, merged, closed
		Draft  bool   `json:"draft"`
		WebURL string `json:"web_url"`
		Author struct {
			Username string `json:"username"`
		} `json:"author"`
		UserNotesCount int   `json:"user_notes_count"`
		ApprovedBy     []any `json:"approved_by"` // just need to check if non-empty
		Approved       bool  `json:"approved"`
	}
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse glab output: %w", err)
	}

	if len(prs) == 0 {
		// No MR found - return marker indicating we checked
		return &PRInfo{
			Fetched:  true,
			CachedAt: time.Now(),
		}, nil
	}

	pr := prs[0]
	return &PRInfo{
		Number:       pr.IID,
		State:        normalizeGitLabState(pr.State),
		IsDraft:      pr.Draft,
		URL:          pr.WebURL,
		Author:       pr.Author.Username,
		CommentCount: pr.UserNotesCount,
		HasReviews:   len(pr.ApprovedBy) > 0,
		IsApproved:   pr.Approved,
		CachedAt:     time.Now(),
		Fetched:      true,
	}, nil
}

// GetPRForBranchContext fetches PR info for a branch using glab CLI with context support.
func (g *GitLab) GetPRForBranchContext(ctx context.Context, repoURL, branch string) (*PRInfo, error) {
	projectPath := extractGitLabProject(repoURL)

	output, err := cmd.OutputContext(ctx, "", "glab", "mr", "list",
		"-R", projectPath,
		"--source-branch", branch,
		"--state", "all",
		"-F", "json",
		"-P", "1") // limit to 1
	if err != nil {
		return nil, fmt.Errorf("glab command failed: %v", err)
	}

	var prs []struct {
		IID    int    `json:"iid"`
		State  string `json:"state"` // opened, merged, closed
		Draft  bool   `json:"draft"`
		WebURL string `json:"web_url"`
		Author struct {
			Username string `json:"username"`
		} `json:"author"`
		UserNotesCount int   `json:"user_notes_count"`
		ApprovedBy     []any `json:"approved_by"` // just need to check if non-empty
		Approved       bool  `json:"approved"`
	}
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse glab output: %w", err)
	}

	if len(prs) == 0 {
		// No MR found - return marker indicating we checked
		return &PRInfo{
			Fetched:  true,
			CachedAt: time.Now(),
		}, nil
	}

	pr := prs[0]
	return &PRInfo{
		Number:       pr.IID,
		State:        normalizeGitLabState(pr.State),
		IsDraft:      pr.Draft,
		URL:          pr.WebURL,
		Author:       pr.Author.Username,
		CommentCount: pr.UserNotesCount,
		HasReviews:   len(pr.ApprovedBy) > 0,
		IsApproved:   pr.Approved,
		CachedAt:     time.Now(),
		Fetched:      true,
	}, nil
}

// GetPRBranch fetches the source branch name for a PR number using glab CLI
func (g *GitLab) GetPRBranch(repoURL string, number int) (string, error) {
	projectPath := extractGitLabProject(repoURL)

	c := exec.Command("glab", "mr", "view",
		fmt.Sprintf("%d", number),
		"-R", projectPath,
		"-F", "json")

	output, err := cmd.Output(c)
	if err != nil {
		return "", fmt.Errorf("glab command failed: %v", err)
	}

	var result struct {
		SourceBranch    string `json:"source_branch"`
		SourceProjectID int    `json:"source_project_id"`
		TargetProjectID int    `json:"target_project_id"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse glab output: %w", err)
	}

	// Check for cross-project PR (fork)
	if result.SourceProjectID != 0 && result.TargetProjectID != 0 &&
		result.SourceProjectID != result.TargetProjectID {
		return "", fmt.Errorf("PR !%d is from a fork - cross-project PRs are not supported", number)
	}

	if result.SourceBranch == "" {
		return "", fmt.Errorf("PR !%d has no source branch", number)
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
	if repoName == "" {
		return "", fmt.Errorf("invalid repo spec %q: repo name must not be empty", repoSpec)
	}
	// Validate at least one non-empty group
	hasGroup := false
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] != "" {
			hasGroup = true
			break
		}
	}
	if !hasGroup {
		return "", fmt.Errorf("invalid repo spec %q: group must not be empty", repoSpec)
	}
	clonePath := filepath.Join(destPath, repoName)

	c := exec.Command("glab", "repo", "clone", repoSpec, clonePath)
	if err := cmd.Run(c); err != nil {
		return "", fmt.Errorf("glab repo clone failed: %v", err)
	}

	return clonePath, nil
}

// CloneRepoContext clones a GitLab repo using glab CLI with context support.
func (g *GitLab) CloneRepoContext(ctx context.Context, repoSpec, destPath string) (string, error) {
	parts := strings.Split(repoSpec, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid repo spec %q: expected group/repo format", repoSpec)
	}
	repoName := parts[len(parts)-1]
	if repoName == "" {
		return "", fmt.Errorf("invalid repo spec %q: repo name must not be empty", repoSpec)
	}
	// Validate at least one non-empty group
	hasGroup := false
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] != "" {
			hasGroup = true
			break
		}
	}
	if !hasGroup {
		return "", fmt.Errorf("invalid repo spec %q: group must not be empty", repoSpec)
	}
	clonePath := filepath.Join(destPath, repoName)

	if err := cmd.RunContext(ctx, "", "glab", "repo", "clone", repoSpec, clonePath); err != nil {
		return "", fmt.Errorf("glab repo clone failed: %v", err)
	}

	return clonePath, nil
}

// CreatePR creates a new MR using glab CLI
func (g *GitLab) CreatePR(repoURL string, params CreatePRParams) (*CreatePRResult, error) {
	projectPath := extractGitLabProject(repoURL)

	args := []string{"mr", "create",
		"-R", projectPath,
		"--title", params.Title,
		"--description", params.Body,
		"--yes", // non-interactive
	}

	if params.Base != "" {
		args = append(args, "--target-branch", params.Base)
	}
	if params.Head != "" {
		args = append(args, "--source-branch", params.Head)
	}
	if params.Draft {
		args = append(args, "--draft")
	}

	c := exec.Command("glab", args...)
	var stdout bytes.Buffer
	c.Stdout = &stdout

	if err := cmd.Run(c); err != nil {
		return nil, fmt.Errorf("glab mr create failed: %v", err)
	}

	// Parse MR URL from stdout (glab mr create outputs something like "!123 https://...")
	output := strings.TrimSpace(stdout.String())

	// Try to extract URL - glab outputs the URL on a line
	var mrURL string
	var mrNumber int
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") {
			mrURL = line
			// Extract MR number from URL (e.g., https://gitlab.com/org/repo/-/merge_requests/123)
			parts := strings.Split(mrURL, "/")
			if len(parts) > 0 {
				fmt.Sscanf(parts[len(parts)-1], "%d", &mrNumber)
			}
			break
		}
		// Also check for !123 format
		if strings.HasPrefix(line, "!") {
			fmt.Sscanf(line, "!%d", &mrNumber)
		}
	}

	if mrURL == "" && mrNumber == 0 {
		return nil, fmt.Errorf("glab mr create returned unexpected output: %s", output)
	}

	return &CreatePRResult{
		Number: mrNumber,
		URL:    mrURL,
	}, nil
}

// MergePR merges a MR by number with the given strategy
func (g *GitLab) MergePR(repoURL string, number int, strategy string) error {
	// GitLab doesn't support rebase strategy via CLI
	if strategy == "rebase" {
		return fmt.Errorf("rebase merge strategy is not supported on GitLab (use squash or merge)")
	}

	projectPath := extractGitLabProject(repoURL)

	args := []string{"mr", "merge", fmt.Sprintf("%d", number),
		"-R", projectPath,
		"--remove-source-branch"}

	if strategy == "squash" {
		args = append(args, "--squash")
	}
	// "merge" uses default behavior (no extra flag)

	c := exec.Command("glab", args...)
	if err := cmd.Run(c); err != nil {
		return fmt.Errorf("merge failed: %v", err)
	}
	return nil
}

// ViewPR shows MR details or opens in browser
func (g *GitLab) ViewPR(repoURL string, number int, web bool) error {
	projectPath := extractGitLabProject(repoURL)
	args := []string{"mr", "view", fmt.Sprintf("%d", number), "-R", projectPath}
	if web {
		args = append(args, "--web")
	}
	c := exec.Command("glab", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// FormatState returns a human-readable PR state
func (g *GitLab) FormatState(state string) string {
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

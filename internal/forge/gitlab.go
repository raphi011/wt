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

// GitLab implements Forge for GitLab repositories using the glab CLI.
// Note: glab doesn't support --user flag like gh, so user field in config is ignored
type GitLab struct {
	ForgeConfig *config.ForgeConfig
}

// Name returns "gitlab"
func (g *GitLab) Name() string {
	return "gitlab"
}

// Check verifies that glab CLI is available and authenticated
func (g *GitLab) Check(ctx context.Context) error {
	_, err := exec.LookPath("glab")
	if err != nil {
		return fmt.Errorf("glab not found: please install GitLab CLI (https://gitlab.com/gitlab-org/cli)")
	}

	c := exec.CommandContext(ctx, "glab", "auth", "status")
	if out, err := c.CombinedOutput(); err != nil {
		errMsg := string(out)
		if strings.Contains(errMsg, "not logged") || strings.Contains(errMsg, "no token") {
			return fmt.Errorf("glab not authenticated: please run 'glab auth login'")
		}
		return fmt.Errorf("glab auth check failed: %s", errMsg)
	}

	return nil
}

// runGlab runs a glab command and returns error
func (g *GitLab) runGlab(ctx context.Context, args ...string) error {
	c := exec.CommandContext(ctx, "glab", args...)
	c.Stderr = os.Stderr
	return c.Run()
}

// outputGlab runs a glab command and returns output
func (g *GitLab) outputGlab(ctx context.Context, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, "glab", args...)
	out, err := c.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%w: %s", err, string(exitErr.Stderr))
		}
		return nil, err
	}
	return out, nil
}

// GetPRForBranch fetches PR info for a branch using glab CLI
func (g *GitLab) GetPRForBranch(ctx context.Context, repoURL, branch string) (*PRInfo, error) {
	projectPath := extractRepoPath(repoURL)

	output, err := g.outputGlab(ctx, "mr", "list",
		"-R", projectPath,
		"--source-branch", branch,
		"--all",
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
func (g *GitLab) GetPRBranch(ctx context.Context, repoURL string, number int) (string, error) {
	projectPath := extractRepoPath(repoURL)

	output, err := g.outputGlab(ctx, "mr", "view",
		fmt.Sprintf("%d", number),
		"-R", projectPath,
		"-F", "json")
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
func (g *GitLab) CloneRepo(ctx context.Context, repoSpec, destPath string) (string, error) {
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

	if err := g.runGlab(ctx, "repo", "clone", repoSpec, clonePath); err != nil {
		return "", fmt.Errorf("glab repo clone failed: %v", err)
	}

	return clonePath, nil
}

// CreatePR creates a new MR using glab CLI
func (g *GitLab) CreatePR(ctx context.Context, repoURL string, params CreatePRParams) (*CreatePRResult, error) {
	projectPath := extractRepoPath(repoURL)

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

	output, err := g.outputGlab(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("glab mr create failed: %v", err)
	}

	// Parse MR URL from stdout (glab mr create outputs something like "!123 https://...")
	outputStr := strings.TrimSpace(string(output))

	// Try to extract URL - glab outputs the URL on a line
	var mrURL string
	var mrNumber int
	for _, line := range strings.Split(outputStr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") {
			mrURL = line
			// Extract MR number from URL (e.g., https://gitlab.com/org/repo/-/merge_requests/123)
			urlParts := strings.Split(mrURL, "/")
			if len(urlParts) > 0 {
				fmt.Sscanf(urlParts[len(urlParts)-1], "%d", &mrNumber)
			}
			break
		}
		// Also check for !123 format
		if strings.HasPrefix(line, "!") {
			fmt.Sscanf(line, "!%d", &mrNumber)
		}
	}

	if mrURL == "" && mrNumber == 0 {
		return nil, fmt.Errorf("glab mr create returned unexpected output: %s", outputStr)
	}

	return &CreatePRResult{
		Number: mrNumber,
		URL:    mrURL,
	}, nil
}

// MergePR merges a MR by number with the given strategy
func (g *GitLab) MergePR(ctx context.Context, repoURL string, number int, strategy string) error {
	// GitLab doesn't support rebase strategy via CLI
	if strategy == "rebase" {
		return fmt.Errorf("rebase merge strategy is not supported on GitLab (use squash or merge)")
	}

	projectPath := extractRepoPath(repoURL)

	args := []string{"mr", "merge", fmt.Sprintf("%d", number),
		"-R", projectPath,
		"--remove-source-branch"}

	if strategy == "squash" {
		args = append(args, "--squash")
	}
	// "merge" uses default behavior (no extra flag)

	if err := g.runGlab(ctx, args...); err != nil {
		return fmt.Errorf("merge failed: %v", err)
	}
	return nil
}

// ViewPR shows MR details or opens in browser
func (g *GitLab) ViewPR(ctx context.Context, repoURL string, number int, web bool) error {
	projectPath := extractRepoPath(repoURL)
	args := []string{"mr", "view", fmt.Sprintf("%d", number), "-R", projectPath}
	if web {
		args = append(args, "--web")
	}
	c := exec.CommandContext(ctx, "glab", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// ListOpenPRs lists all open MRs for a repository
func (g *GitLab) ListOpenPRs(ctx context.Context, repoURL string) ([]OpenPR, error) {
	projectPath := extractRepoPath(repoURL)
	output, err := g.outputGlab(ctx, "mr", "list",
		"-R", projectPath,
		"--state", "opened",
		"-F", "json",
		"-P", "100")
	if err != nil {
		return nil, fmt.Errorf("glab command failed: %v", err)
	}

	var prs []struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		SourceBranch string `json:"source_branch"`
		Draft        bool   `json:"draft"`
		Author       struct {
			Username string `json:"username"`
		} `json:"author"`
	}
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse glab output: %w", err)
	}

	result := make([]OpenPR, len(prs))
	for i, pr := range prs {
		result[i] = OpenPR{
			Number:  pr.IID,
			Title:   pr.Title,
			Author:  pr.Author.Username,
			Branch:  pr.SourceBranch,
			IsDraft: pr.Draft,
		}
	}

	return result, nil
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

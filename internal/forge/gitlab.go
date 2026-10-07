package forge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
)

// GitLab implements Forge for GitLab repositories using the glab CLI.
// Note: glab doesn't support --user flag like gh, so user field in config is ignored
type GitLab struct {
	ForgeConfig *config.ForgeConfig
	host        string
}

func (g *GitLab) sessionUser(string) string { return "" }

func (g *GitLab) prepareSession(_ context.Context, host, _ string) error {
	g.host = host
	return nil
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

	args := []string{"auth", "status"}
	if g.host != "" {
		args = append(args, "--hostname", g.host)
	}
	c := exec.CommandContext(ctx, "glab", args...)
	if g.host != "" {
		c.Env = append(os.Environ(), "GITLAB_HOST="+g.host)
	}
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
	if g.host != "" {
		c.Env = append(os.Environ(), "GITLAB_HOST="+g.host)
	}
	c.Stderr = os.Stderr
	return c.Run()
}

// outputGlab runs a glab command and returns output
func (g *GitLab) outputGlab(ctx context.Context, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, "glab", args...)
	if g.host != "" {
		c.Env = append(os.Environ(), "GITLAB_HOST="+g.host)
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

// GetPRForBranch fetches PR info for a branch using glab CLI
func (g *GitLab) GetPRForBranch(ctx context.Context, repoURL, branch string) (*PRInfo, error) {
	projectPath := ExtractRepoPath(repoURL)

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
		UserNotesCount int `json:"user_notes_count"`
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

	// The MR list carries no approval data; a failed lookup only loses the
	// review status
	hasReviews, isApproved, err := g.reviewStatus(ctx, projectPath, pr.IID)
	if err != nil {
		log.FromContext(ctx).Printf("Warning: failed to fetch review status of !%d: %v\n", pr.IID, err)
	}

	return &PRInfo{
		Number:       pr.IID,
		State:        normalizeGitLabState(pr.State),
		IsDraft:      pr.Draft,
		URL:          pr.WebURL,
		Author:       pr.Author.Username,
		CommentCount: pr.UserNotesCount,
		HasReviews:   hasReviews,
		IsApproved:   isApproved,
		CachedAt:     time.Now(),
		Fetched:      true,
	}, nil
}

// reviewStatus fetches the approval state of a MR from the approvals API.
func (g *GitLab) reviewStatus(ctx context.Context, projectPath string, iid int) (hasReviews, isApproved bool, err error) {
	output, err := g.outputGlab(ctx, "api",
		fmt.Sprintf("projects/%s/merge_requests/%d/approvals", url.PathEscape(projectPath), iid))
	if err != nil {
		return false, false, fmt.Errorf("glab command failed: %v", err)
	}

	var approvals struct {
		Approved   bool  `json:"approved"`
		ApprovedBy []any `json:"approved_by"` // just need to check if non-empty
	}
	if err := json.Unmarshal(output, &approvals); err != nil {
		return false, false, fmt.Errorf("failed to parse glab output: %w", err)
	}

	hasReviews = len(approvals.ApprovedBy) > 0
	// GitLab reports approved=true when no approval is required, even if
	// nobody approved
	return hasReviews, approvals.Approved && hasReviews, nil
}

// GetPRBranch fetches the source branch name for a PR number using glab CLI
func (g *GitLab) GetPRBranch(ctx context.Context, repoURL string, number int) (string, error) {
	projectPath := ExtractRepoPath(repoURL)

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

// CloneBareRepo clones a GitLab repo as a bare repo inside .git directory
func (g *GitLab) CloneBareRepo(ctx context.Context, repoSpec, destPath string) (string, error) {
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

	// Final repo directory
	repoDir := filepath.Join(destPath, repoName)

	// Create the repo directory
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Clone as bare directly into .git subdirectory
	gitDir := filepath.Join(repoDir, ".git")
	if err := g.runGlab(ctx, "repo", "clone", repoSpec, gitDir, "--", "--bare"); err != nil {
		return "", errors.Join(fmt.Errorf("glab repo clone failed: %v", err), os.RemoveAll(repoDir))
	}

	// Configure the repo for worktree support
	if err := configureBareRepo(ctx, gitDir); err != nil {
		return "", errors.Join(err, os.RemoveAll(repoDir))
	}

	return repoDir, nil
}

// CreatePR creates a new MR using glab CLI
func (g *GitLab) CreatePR(ctx context.Context, repoURL string, params CreatePRParams) (*CreatePRResult, error) {
	projectPath := ExtractRepoPath(repoURL)

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

	return parseMRCreateOutput(string(output))
}

// parseMRCreateOutput extracts the MR URL and number from glab mr create stdout.
func parseMRCreateOutput(output string) (*CreatePRResult, error) {
	// Parse MR URL from stdout (glab mr create outputs something like "!123 https://...")
	outputStr := strings.TrimSpace(output)

	// Try to extract URL - glab outputs the URL on a line
	var mrURL string
	var mrNumber int
	for line := range strings.SplitSeq(outputStr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") {
			mrURL = line
			// Extract MR number from URL (e.g., https://gitlab.com/org/repo/-/merge_requests/123)
			urlParts := strings.Split(mrURL, "/")
			var n int
			if _, err := fmt.Sscanf(urlParts[len(urlParts)-1], "%d", &n); err == nil {
				mrNumber = n
			}
			break
		}
		// Also check for !123 format
		if strings.HasPrefix(line, "!") {
			var n int
			if _, err := fmt.Sscanf(line, "!%d", &n); err == nil {
				mrNumber = n
			}
		}
	}

	if mrURL == "" && mrNumber == 0 {
		return nil, fmt.Errorf("glab mr create returned unexpected output: %s", outputStr)
	}
	if mrNumber == 0 {
		return nil, fmt.Errorf("MR created but could not parse its number from glab output: %s", outputStr)
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

	projectPath := ExtractRepoPath(repoURL)

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

// ListOpenPRs lists all open MRs for a repository
func (g *GitLab) ListOpenPRs(ctx context.Context, repoURL string) ([]OpenPR, error) {
	projectPath := ExtractRepoPath(repoURL)
	// glab mr list shows open MRs by default (no --state flag)
	output, err := g.outputGlab(ctx, "mr", "list",
		"-R", projectPath,
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

// normalizeGitLabState converts GitLab state to normalized format
func normalizeGitLabState(state string) string {
	switch strings.ToLower(state) {
	case "opened":
		return PRStateOpen
	case "merged":
		return PRStateMerged
	case "closed":
		return PRStateClosed
	default:
		return strings.ToUpper(state)
	}
}

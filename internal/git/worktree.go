package git

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// Worktree represents a git worktree with its metadata.
// Used as the unified struct across all commands (list, prune, cd, exec).
// Fields tagged json:"-" are internal and excluded from user-facing JSON output.
type Worktree struct {
	Path        string    `json:"path"`
	Branch      string    `json:"branch"`
	CommitHash  string    `json:"commit"`
	RepoName    string    `json:"repo"`
	RepoPath    string    `json:"-"`
	OriginURL   string    `json:"-"`
	Note        string    `json:"note,omitempty"`
	HasUpstream bool      `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	PRNumber    int       `json:"pr_number,omitempty"`
	PRState     string    `json:"pr_state,omitempty"`
	PRURL       string    `json:"pr_url,omitempty"`
	PRDraft     bool      `json:"pr_draft,omitempty"`
}

// CreateWorktreeResult contains the result of creating a worktree
type CreateWorktreeResult struct {
	Path          string
	AlreadyExists bool
}

// RemoveWorktree removes a git worktree
func RemoveWorktree(ctx context.Context, worktree Worktree, force bool) error {
	args := []string{"worktree", "remove", worktree.Path}
	if force {
		args = append(args, "--force")
	}

	return runGit(ctx, worktree.RepoPath, args...)
}

// PruneWorktrees prunes stale worktree references
func PruneWorktrees(ctx context.Context, repoPath string) error {
	return runGit(ctx, repoPath, "worktree", "prune")
}

// IsWorktree returns true if path is a git worktree (not main repo)
// Worktrees have .git as a file pointing to the main repo,
// while main repos have .git as a directory.
func IsWorktree(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	// Worktrees have .git as file, main repos have .git as directory
	return !info.IsDir()
}

// CreateWorktree creates a worktree for an existing branch.
// gitDir is the .git directory (for regular repos) or the bare repo path.
// wtPath is the target worktree path.
// branch is the existing branch to checkout.
func CreateWorktree(ctx context.Context, gitDir, wtPath, branch string) error {
	return runGit(ctx, gitDir, "worktree", "add", wtPath, branch)
}

// CreateWorktreeNewBranch creates a worktree with a new branch.
// gitDir is the .git directory (for regular repos) or the bare repo path.
// wtPath is the target worktree path.
// branch is the new branch name.
// baseRef is the starting point (e.g., "origin/main").
func CreateWorktreeNewBranch(ctx context.Context, gitDir, wtPath, branch, baseRef string) error {
	args := []string{"worktree", "add", wtPath, "-b", branch}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	return runGit(ctx, gitDir, args...)
}

// CreateWorktreeOrphan creates a worktree with a new orphan branch.
// Used for empty repos (no commits) where there's no valid ref to branch from.
func CreateWorktreeOrphan(ctx context.Context, gitDir, wtPath, branch string) error {
	return runGit(ctx, gitDir, "worktree", "add", "--orphan", "-b", branch, wtPath)
}

package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Worktree represents a git worktree with its status
type Worktree struct {
	Path        string
	Branch      string
	MainRepo    string
	RepoName    string
	IsMerged    bool
	CommitCount int
	Additions   int
	Deletions   int
	IsDirty     bool
	LastCommit  string
}

// ListWorktrees scans a directory for git worktrees
func ListWorktrees(scanDir string) ([]Worktree, error) {
	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return nil, err
	}

	var worktrees []Worktree
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(scanDir, entry.Name())
		gitFile := filepath.Join(path, ".git")

		// Check if it's a worktree (has .git file, not directory)
		info, err := os.Stat(gitFile)
		if err != nil || info.IsDir() {
			continue
		}

		// Get main repo path
		mainRepo, err := GetMainRepoPath(path)
		if err != nil {
			continue
		}

		// Get branch
		branch, err := GetCurrentBranch(path)
		if err != nil {
			continue
		}

		// Get repo name from main repo
		repoName := filepath.Base(mainRepo)

		// Get merge status
		isMerged, _ := IsBranchMerged(mainRepo, branch)

		// Get commit count if not merged
		var commitCount int
		if !isMerged {
			commitCount, _ = GetCommitCount(mainRepo, branch)
		}

		// Get diff stats
		additions, deletions, _ := GetDiffStats(path)
		isDirty := additions > 0 || deletions > 0

		// Get last commit time
		lastCommit, _ := GetLastCommitRelative(path)

		worktrees = append(worktrees, Worktree{
			Path:        path,
			Branch:      branch,
			MainRepo:    mainRepo,
			RepoName:    repoName,
			IsMerged:    isMerged,
			CommitCount: commitCount,
			Additions:   additions,
			Deletions:   deletions,
			IsDirty:     isDirty,
			LastCommit:  lastCommit,
		})
	}

	return worktrees, nil
}

// CreateWorktree creates a new git worktree at basePath/<repo>-<branch>
func CreateWorktree(basePath, branch string) (string, error) {
	// Get repo name
	repoName, err := GetRepoName()
	if err != nil {
		return "", err
	}

	// Resolve base path
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}

	// Check if base path exists
	if _, err := os.Stat(absBasePath); os.IsNotExist(err) {
		return "", fmt.Errorf("directory does not exist: %s", absBasePath)
	}

	// Create worktree path: <basePath>/<repo>-<branch>
	worktreePath := filepath.Join(absBasePath, fmt.Sprintf("%s-%s", repoName, branch))

	// Check if already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return worktreePath, nil // Already exists
	}

	// Try to create with new branch first
	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branch)
	if err := cmd.Run(); err != nil {
		// Try existing branch
		cmd = exec.Command("git", "worktree", "add", worktreePath, branch)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to create worktree: %w", err)
		}
	}

	return worktreePath, nil
}

// RemoveWorktree removes a git worktree
func RemoveWorktree(worktree Worktree, force bool) error {
	args := []string{"-C", worktree.MainRepo, "worktree", "remove", worktree.Path}
	if force {
		args = append(args, "--force")
	}

	cmd := exec.Command("git", args...)
	return cmd.Run()
}

// PruneWorktrees prunes stale worktree references
func PruneWorktrees(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "prune")
	return cmd.Run()
}

// IsValidWorktree checks if a directory is a valid git worktree
func IsValidWorktree(path string) bool {
	gitFile := filepath.Join(path, ".git")
	info, err := os.Stat(gitFile)
	if err != nil {
		return false
	}

	// Worktrees have .git as a file, not directory
	if info.IsDir() {
		return false
	}

	// Verify main repo exists
	mainRepo, err := GetMainRepoPath(path)
	if err != nil {
		return false
	}

	if _, err := os.Stat(mainRepo); err != nil {
		return false
	}

	return true
}

// GroupWorktreesByRepo groups worktrees by their main repository
func GroupWorktreesByRepo(worktrees []Worktree) map[string][]Worktree {
	groups := make(map[string][]Worktree)
	for _, wt := range worktrees {
		groups[wt.RepoName] = append(groups[wt.RepoName], wt)
	}
	return groups
}

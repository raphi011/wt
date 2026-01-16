package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetRepoName extracts the repository name from the origin URL
func GetRepoName() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository or no origin remote")
	}

	url := strings.TrimSpace(string(output))

	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Extract last part of path
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid git origin URL")
	}

	return parts[len(parts)-1], nil
}

// GetCurrentBranch returns the current branch name
func GetCurrentBranch(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// IsBranchMerged checks if a branch is merged into origin/master
func IsBranchMerged(repoPath, branch string) (bool, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--merged", "origin/master")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == branch || strings.TrimSpace(line) == "* "+branch {
			return true, nil
		}
	}
	return false, nil
}

// GetCommitCount returns number of commits ahead of origin/master
func GetCommitCount(repoPath, branch string) (int, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-list", "--count", fmt.Sprintf("origin/master..%s", branch))
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, err
}

// GetLastCommitRelative returns relative time of last commit
func GetLastCommitRelative(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "log", "-1", "--format=%cr")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetDiffStats returns additions and deletions for uncommitted changes
func GetDiffStats(path string) (additions, deletions int, err error) {
	// Check if there are any changes
	statusCmd := exec.Command("git", "-C", path, "status", "--porcelain")
	statusOutput, err := statusCmd.Output()
	if err != nil {
		return 0, 0, err
	}

	if len(strings.TrimSpace(string(statusOutput))) == 0 {
		return 0, 0, nil
	}

	// Get diff stats (both staged and unstaged)
	cmd := exec.Command("git", "-C", path, "diff", "HEAD", "--numstat")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, nil // No error if no diff
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format: "additions\tdeletions\tfilename"
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			var add, del int
			fmt.Sscanf(fields[0], "%d", &add)
			fmt.Sscanf(fields[1], "%d", &del)
			additions += add
			deletions += del
		}
	}

	return additions, deletions, nil
}

// FetchOriginMaster fetches origin/master branch
func FetchOriginMaster(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "fetch", "origin", "master", "--quiet")
	return cmd.Run()
}

// GetMainRepoPath extracts main repo path from .git file in worktree
func GetMainRepoPath(worktreePath string) (string, error) {
	gitFile := filepath.Join(worktreePath, ".git")
	content, err := exec.Command("cat", gitFile).Output()
	if err != nil {
		return "", err
	}

	// Parse: "gitdir: /path/to/repo/.git/worktrees/name"
	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", fmt.Errorf("invalid .git file format")
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")

	// Remove /.git/worktrees/xxx to get main repo path
	repoPath := strings.Split(gitdir, "/.git/worktrees/")[0]
	return repoPath, nil
}

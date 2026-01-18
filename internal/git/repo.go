package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetRepoName extracts the repository name from the origin URL
func GetRepoName() (string, error) {
	return GetRepoNameFrom(".")
}

// GetRepoNameFrom extracts the repository name from the origin URL of a repo at the given path
func GetRepoNameFrom(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("not in a git repository or no origin remote: %s", strings.TrimSpace(stderr.String()))
		}
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

// GetRepoFolderName returns the actual folder name of the git repo on disk
// Uses git rev-parse --show-toplevel to get the root directory
func GetRepoFolderName() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("not in a git repository: %s", strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("not in a git repository")
	}

	repoPath := strings.TrimSpace(string(output))
	return filepath.Base(repoPath), nil
}

// GetDefaultBranch returns the default branch name for the remote (e.g., "main" or "master")
func GetDefaultBranch(repoPath string) string {
	// Try to get default branch from remote HEAD
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := cmd.Output()
	if err == nil {
		// Output is like "refs/remotes/origin/main"
		ref := strings.TrimSpace(string(output))
		if parts := strings.Split(ref, "/"); len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Fallback: check if origin/main exists
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/main")
	if cmd.Run() == nil {
		return "main"
	}

	// Fallback: check if origin/master exists
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/master")
	if cmd.Run() == nil {
		return "master"
	}

	// Last resort default
	return "main"
}

// GetCurrentBranch returns the current branch name
// Returns "(detached)" for detached HEAD state
func GetCurrentBranch(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "branch", "--show-current")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("failed to get branch: %s", strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "(detached)", nil
	}
	return branch, nil
}

// IsBranchMerged checks if a branch is merged into the default branch (main/master)
func IsBranchMerged(repoPath, branch string) (bool, error) {
	defaultBranch := GetDefaultBranch(repoPath)
	cmd := exec.Command("git", "-C", repoPath, "branch", "--merged", "origin/"+defaultBranch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return false, fmt.Errorf("failed to check merge status: %s", strings.TrimSpace(stderr.String()))
		}
		return false, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Handle both "branch" and "* branch" formats
		trimmed = strings.TrimPrefix(trimmed, "* ")
		if trimmed == branch {
			return true, nil
		}
	}
	return false, nil
}

// GetCommitCount returns number of commits ahead of the default branch
func GetCommitCount(repoPath, branch string) (int, error) {
	defaultBranch := GetDefaultBranch(repoPath)
	cmd := exec.Command("git", "-C", repoPath, "rev-list", "--count", fmt.Sprintf("origin/%s..%s", defaultBranch, branch))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return 0, fmt.Errorf("failed to count commits: %s", strings.TrimSpace(stderr.String()))
		}
		return 0, err
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, err
}

// GetLastCommitRelative returns relative time of last commit
func GetLastCommitRelative(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "log", "-1", "--format=%cr")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("failed to get last commit: %s", strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// HasUntrackedFiles checks if the worktree has untracked files
func HasUntrackedFiles(path string) bool {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "??") {
			return true
		}
	}
	return false
}

// GetDiffStats returns additions and deletions for uncommitted changes
// Also returns true for hasUntracked if there are untracked files
func GetDiffStats(path string) (additions, deletions int, hasUntracked bool, err error) {
	// Check if there are any changes (including untracked)
	statusCmd := exec.Command("git", "-C", path, "status", "--porcelain")
	var stderr bytes.Buffer
	statusCmd.Stderr = &stderr
	statusOutput, err := statusCmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return 0, 0, false, fmt.Errorf("failed to get status: %s", strings.TrimSpace(stderr.String()))
		}
		return 0, 0, false, err
	}

	statusStr := strings.TrimSpace(string(statusOutput))
	if statusStr == "" {
		return 0, 0, false, nil
	}

	// Check for untracked files
	for _, line := range strings.Split(statusStr, "\n") {
		if strings.HasPrefix(line, "??") {
			hasUntracked = true
			break
		}
	}

	// Get diff stats (both staged and unstaged)
	cmd := exec.Command("git", "-C", path, "diff", "HEAD", "--numstat")
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		// diff HEAD fails on initial commits, still return untracked status
		return 0, 0, hasUntracked, nil
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

	return additions, deletions, hasUntracked, nil
}

// FetchDefaultBranch fetches the default branch (main/master) from origin
func FetchDefaultBranch(repoPath string) error {
	defaultBranch := GetDefaultBranch(repoPath)
	cmd := exec.Command("git", "-C", repoPath, "fetch", "origin", defaultBranch, "--quiet")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("failed to fetch origin/%s: %s", defaultBranch, strings.TrimSpace(stderr.String()))
		}
		return err
	}
	return nil
}

// GetMainRepoPath extracts main repo path from .git file in worktree
func GetMainRepoPath(worktreePath string) (string, error) {
	gitFile := filepath.Join(worktreePath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return "", fmt.Errorf("failed to read .git file: %w", err)
	}

	// Parse: "gitdir: /path/to/repo/.git/worktrees/name"
	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", fmt.Errorf("invalid .git file format")
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")

	// Walk up from gitdir to find the .git directory, then get its parent
	// gitdir is like: /path/to/repo/.git/worktrees/name
	// We want: /path/to/repo
	dir := gitdir
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .git
			return "", fmt.Errorf("could not find main repo path from gitdir: %s", gitdir)
		}
		if filepath.Base(dir) == ".git" {
			// Found .git directory, parent is the repo path
			return parent, nil
		}
		dir = parent
	}
}

// GetUpstreamBranch returns the remote branch name for a local branch.
// Returns empty string if no upstream is configured.
func GetUpstreamBranch(repoPath, branch string) string {
	cmd := exec.Command("git", "-C", repoPath, "config", fmt.Sprintf("branch.%s.merge", branch))
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	// Output is like "refs/heads/feature-branch"
	ref := strings.TrimSpace(string(output))
	return strings.TrimPrefix(ref, "refs/heads/")
}

// BranchExists checks if a local branch exists
func BranchExists(branch string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch)
	err := cmd.Run()
	if err != nil {
		// Exit code 128 means branch doesn't exist
		return false, nil
	}
	return true, nil
}

// GetShortCommitHash returns the short (7 char) commit hash for HEAD in a worktree
func GetShortCommitHash(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--short", "HEAD")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("failed to get commit hash: %s", strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentRepoMainPath returns the main repository path from cwd
// Works whether you're in the main repo or a worktree
// Returns empty string if not in a git repo
func GetCurrentRepoMainPath() string {
	// First check if we're in a git repo at all
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	toplevel := strings.TrimSpace(string(output))

	// Check if we're in a worktree by seeing if .git is a file (not dir)
	gitPath := filepath.Join(toplevel, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}

	if info.IsDir() {
		// Main repo - toplevel is the main repo path
		return toplevel
	}

	// Worktree - need to resolve main repo from .git file
	mainRepo, err := GetMainRepoPath(toplevel)
	if err != nil {
		return ""
	}
	return mainRepo
}

// GetOriginURL gets the origin URL for a repository
func GetOriginURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("failed to get origin URL: %s", strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetBranchWorktree returns the worktree path if branch is checked out, empty string if not
func GetBranchWorktree(branch string) (string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("failed to list worktrees: %s", strings.TrimSpace(stderr.String()))
		}
		return "", err
	}

	// Parse porcelain output: each worktree has "worktree <path>" and "branch refs/heads/<name>"
	lines := strings.Split(string(output), "\n")
	var currentPath string
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch refs/heads/") {
			wtBranch := strings.TrimPrefix(line, "branch refs/heads/")
			if wtBranch == branch {
				return currentPath, nil
			}
		} else if line == "" {
			currentPath = ""
		}
	}

	return "", nil
}

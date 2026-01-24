package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GetRepoName extracts the repository name from the origin URL
func GetRepoName() (string, error) {
	return GetRepoNameFrom(".")
}

// ExtractRepoNameFromURL extracts the repository name from a git URL
func ExtractRepoNameFromURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

// GetRepoNameFrom extracts the repository name from the origin URL of a repo at the given path
func GetRepoNameFrom(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	output, err := outputCmd(cmd)
	if err != nil {
		return "", fmt.Errorf("not in a git repository or no origin remote: %v", err)
	}

	url := strings.TrimSpace(string(output))

	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Extract last part of path (repo name)
	parts := strings.Split(url, "/")
	repoName := parts[len(parts)-1]
	if repoName == "" {
		return "", fmt.Errorf("invalid git origin URL: could not extract repo name from %q", url)
	}

	return repoName, nil
}

// GetRepoDisplayName returns the folder name of the repository.
func GetRepoDisplayName(repoPath string) string {
	return filepath.Base(repoPath)
}

// GetRepoFolderName returns the actual folder name of the git repo on disk
// Uses git rev-parse --show-toplevel to get the root directory
func GetRepoFolderName() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := outputCmd(cmd)
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %v", err)
	}

	repoPath := strings.TrimSpace(string(output))
	return filepath.Base(repoPath), nil
}

// GetDefaultBranch returns the default branch name for the remote (e.g., "main" or "master")
func GetDefaultBranch(repoPath string) string {
	// Try to get default branch from remote HEAD
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := outputCmd(cmd)
	if err == nil {
		// Output is like "refs/remotes/origin/main"
		ref := strings.TrimSpace(string(output))
		if parts := strings.Split(ref, "/"); len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Fallback: check if origin/main exists
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/main")
	if runCmd(cmd) == nil {
		return "main"
	}

	// Fallback: check if origin/master exists
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/master")
	if runCmd(cmd) == nil {
		return "master"
	}

	// Last resort default
	return "main"
}

// GetCurrentBranch returns the current branch name
// Returns "(detached)" for detached HEAD state
func GetCurrentBranch(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "branch", "--show-current")
	output, err := outputCmd(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get branch: %v", err)
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
	output, err := outputCmd(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check merge status: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Handle "branch", "* branch" (current), and "+ branch" (in worktree) formats
		trimmed = strings.TrimPrefix(trimmed, "* ")
		trimmed = strings.TrimPrefix(trimmed, "+ ")
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
	output, err := outputCmd(cmd)
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %v", err)
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, err
}

// GetLastCommitRelative returns relative time of last commit
func GetLastCommitRelative(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "log", "-1", "--format=%cr")
	output, err := outputCmd(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get last commit: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetLastCommitTime returns the unix timestamp of the last commit
func GetLastCommitTime(path string) (time.Time, error) {
	cmd := exec.Command("git", "-C", path, "log", "-1", "--format=%ct")
	output, err := outputCmd(cmd)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last commit time: %v", err)
	}

	var timestamp int64
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse commit timestamp: %w", err)
	}

	return time.Unix(timestamp, 0), nil
}

// IsDirty returns true if the worktree has uncommitted changes or untracked files
func IsDirty(path string) bool {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	output, err := outputCmd(cmd)
	if err != nil {
		return false // Treat error as clean (safe default)
	}
	return strings.TrimSpace(string(output)) != ""
}

// FetchDefaultBranch fetches the default branch (main/master) from origin
func FetchDefaultBranch(repoPath string) error {
	defaultBranch := GetDefaultBranch(repoPath)
	return FetchBranch(repoPath, defaultBranch)
}

// FetchBranch fetches a specific branch from origin
func FetchBranch(repoPath, branch string) error {
	cmd := exec.Command("git", "-C", repoPath, "fetch", "origin", branch, "--quiet")
	if err := runCmd(cmd); err != nil {
		return fmt.Errorf("failed to fetch origin/%s: %v", branch, err)
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
	// Only the first line matters; any additional lines are ignored
	line := strings.TrimSpace(string(content))
	if idx := strings.Index(line, "\n"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", fmt.Errorf("invalid .git file format: expected 'gitdir: <path>'")
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")
	if gitdir == "" {
		return "", fmt.Errorf("invalid .git file format: empty gitdir path")
	}

	// Handle relative paths (gitdir can be relative to the worktree)
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(worktreePath, gitdir)
	}

	// Clean the path to resolve any .. or . components
	gitdir = filepath.Clean(gitdir)

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
	output, err := outputCmd(cmd)
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
	err := runCmd(cmd)
	if err != nil {
		// Exit code 128 means branch doesn't exist
		return false, nil
	}
	return true, nil
}

// GetShortCommitHash returns the short (7 char) commit hash for HEAD in a worktree
func GetShortCommitHash(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--short", "HEAD")
	output, err := outputCmd(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentRepoMainPath returns the main repository path from cwd
// Works whether you're in the main repo or a worktree
// Returns empty string if not in a git repo
func GetCurrentRepoMainPath() string {
	// First check if we're in a git repo at all
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := outputCmd(cmd)
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

// GetCurrentRepoMainPathFrom returns the main repository path from the given path
// Works whether you're in the main repo or a worktree
// Returns empty string if not in a git repo
func GetCurrentRepoMainPathFrom(path string) string {
	// First check if we're in a git repo at all
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	output, err := outputCmd(cmd)
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
	output, err := outputCmd(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get origin URL: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetBranchWorktree returns the worktree path if branch is checked out, empty string if not
func GetBranchWorktree(branch string) (string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := outputCmd(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %v", err)
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
				if currentPath == "" {
					// Branch found but path not parsed - malformed output
					return "", fmt.Errorf("malformed git worktree output: found branch %q without worktree path", branch)
				}
				return currentPath, nil
			}
		} else if line == "" {
			currentPath = ""
		}
	}

	return "", nil
}

// WorktreeInfo contains basic worktree information from git worktree list.
type WorktreeInfo struct {
	Path       string
	Branch     string
	CommitHash string // Full hash from git, caller can truncate
}

// ListWorktreesFromRepo returns all worktrees for a repository using git worktree list --porcelain.
// This is much faster than querying each worktree individually.
func ListWorktreesFromRepo(repoPath string) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	output, err := outputCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %v", err)
	}

	var worktrees []WorktreeInfo
	var current WorktreeInfo

	for _, line := range strings.Split(string(output), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			// Start of new worktree entry
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.CommitHash = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "detached":
			current.Branch = "(detached)"
		}
	}

	// Don't forget the last entry
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// GetMergedBranches returns a set of branches that are merged into the default branch.
// Uses a single git call: `git branch --merged origin/<default>`
func GetMergedBranches(repoPath string) map[string]bool {
	merged := make(map[string]bool)

	defaultBranch := GetDefaultBranch(repoPath)
	cmd := exec.Command("git", "-C", repoPath, "branch", "--merged", "origin/"+defaultBranch)
	output, err := outputCmd(cmd)
	if err != nil {
		return merged
	}

	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)
		// Handle "branch", "* branch" (current), and "+ branch" (in worktree) formats
		trimmed = strings.TrimPrefix(trimmed, "* ")
		trimmed = strings.TrimPrefix(trimmed, "+ ")
		if trimmed != "" {
			merged[trimmed] = true
		}
	}
	return merged
}

// GetAllBranchConfig returns branch notes and upstreams for a repository in one call.
// Uses: `git config --get-regexp 'branch\.'`
// Returns: notes map (branch -> note), upstreams map (branch -> upstream ref)
func GetAllBranchConfig(repoPath string) (notes map[string]string, upstreams map[string]bool) {
	notes = make(map[string]string)
	upstreams = make(map[string]bool)

	cmd := exec.Command("git", "-C", repoPath, "config", "--get-regexp", `branch\.`)
	output, err := outputCmd(cmd)
	if err != nil {
		// No config is not an error
		return notes, upstreams
	}

	// Parse output lines like:
	// branch.feature-x.description Note text here
	// branch.feature-x.merge refs/heads/feature-x
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 1 {
			continue
		}

		key := parts[0]

		// Handle branch.<name>.description
		if strings.HasPrefix(key, "branch.") && strings.HasSuffix(key, ".description") {
			branch := key[7 : len(key)-12] // Remove "branch." prefix and ".description" suffix
			if branch != "" && len(parts) == 2 {
				notes[branch] = parts[1]
			}
		}

		// Handle branch.<name>.merge
		if strings.HasPrefix(key, "branch.") && strings.HasSuffix(key, ".merge") {
			branch := key[7 : len(key)-6] // Remove "branch." prefix and ".merge" suffix
			if branch != "" {
				upstreams[branch] = true
			}
		}
	}

	return notes, upstreams
}

// DeleteLocalBranch deletes a local branch
func DeleteLocalBranch(repoPath, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	cmd := exec.Command("git", "-C", repoPath, "branch", flag, branch)
	if err := runCmd(cmd); err != nil {
		return fmt.Errorf("failed to delete branch: %v", err)
	}
	return nil
}

// DiffStats contains diff statistics
type DiffStats struct {
	Additions int
	Deletions int
	Files     int
}

// GetDiffStats returns additions, deletions, and files changed vs default branch
func GetDiffStats(repoPath, branch string) (DiffStats, error) {
	defaultBranch := GetDefaultBranch(repoPath)
	cmd := exec.Command("git", "-C", repoPath, "diff", "--numstat", fmt.Sprintf("origin/%s...%s", defaultBranch, branch))
	output, err := outputCmd(cmd)
	if err != nil {
		return DiffStats{}, fmt.Errorf("failed to get diff stats: %v", err)
	}

	var stats DiffStats
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			var add, del int
			// Handle binary files which show "-" instead of numbers
			if parts[0] != "-" {
				fmt.Sscanf(parts[0], "%d", &add)
			}
			if parts[1] != "-" {
				fmt.Sscanf(parts[1], "%d", &del)
			}
			stats.Additions += add
			stats.Deletions += del
			stats.Files++
		}
	}
	return stats, nil
}

// GetCommitsBehind returns number of commits behind the default branch
func GetCommitsBehind(repoPath, branch string) (int, error) {
	defaultBranch := GetDefaultBranch(repoPath)
	cmd := exec.Command("git", "-C", repoPath, "rev-list", "--count", fmt.Sprintf("%s..origin/%s", branch, defaultBranch))
	output, err := outputCmd(cmd)
	if err != nil {
		return 0, fmt.Errorf("failed to count commits behind: %v", err)
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, err
}

// FindRepoInDirs searches for a repo with the given folder name across multiple directories.
// Returns the absolute path to the repo if found, empty string otherwise.
// Similar to FindRepoByName but checks multiple directories (stops at first match).
func FindRepoInDirs(repoName string, searchDirs ...string) string {
	for _, dir := range searchDirs {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, repoName)
		gitPath := filepath.Join(candidate, ".git")
		info, err := os.Stat(gitPath)
		if err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

// GetRepoNameFromWorktree extracts the expected repo name from a worktree's .git file.
// Parses: gitdir: /path/to/repo/.git/worktrees/name
// Extracts: repo name from the path (parent of .git directory)
func GetRepoNameFromWorktree(worktreePath string) string {
	gitFile := filepath.Join(worktreePath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return ""
	}

	// Parse: "gitdir: /path/to/repo/.git/worktrees/name"
	line := strings.TrimSpace(string(content))
	if idx := strings.Index(line, "\n"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}
	if !strings.HasPrefix(line, "gitdir: ") {
		return ""
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(worktreePath, gitdir)
	}
	gitdir = filepath.Clean(gitdir)

	// Walk up from gitdir to find the .git directory, then get repo name
	// gitdir is like: /path/to/repo/.git/worktrees/name
	// We want: "repo" (the folder name)
	dir := gitdir
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .git
			return ""
		}
		if filepath.Base(dir) == ".git" {
			// Found .git directory, parent is the repo path
			return filepath.Base(parent)
		}
		dir = parent
	}
}

// GetBranchCreatedTime returns when the branch was created (first commit on branch)
// Falls back to first commit time if reflog is unavailable
func GetBranchCreatedTime(repoPath, branch string) (time.Time, error) {
	// Try reflog first - most reliable for local branches
	cmd := exec.Command("git", "-C", repoPath, "reflog", "show", "--date=iso", "--format=%gd", branch)
	output, err := outputCmd(cmd)
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) > 0 {
			// Last line is oldest entry (branch creation)
			lastLine := lines[len(lines)-1]
			// Parse ISO date format
			t, err := time.Parse("2006-01-02 15:04:05 -0700", lastLine)
			if err == nil {
				return t, nil
			}
		}
	}

	// Fallback: get the merge-base commit date (when branch diverged from default)
	defaultBranch := GetDefaultBranch(repoPath)
	cmd = exec.Command("git", "-C", repoPath, "merge-base", fmt.Sprintf("origin/%s", defaultBranch), branch)
	output, err = outputCmd(cmd)
	if err == nil {
		mergeBase := strings.TrimSpace(string(output))
		cmd = exec.Command("git", "-C", repoPath, "log", "-1", "--format=%ci", mergeBase)
		output, err = outputCmd(cmd)
		if err == nil {
			t, err := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(string(output)))
			if err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("could not determine branch creation time")
}

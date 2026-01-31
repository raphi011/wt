package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GetRepoName extracts the repository name from the origin URL
func GetRepoName(ctx context.Context) (string, error) {
	return GetRepoNameFrom(ctx, ".")
}

// ExtractRepoNameFromURL extracts the repository name from a git URL
func ExtractRepoNameFromURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

// GetRepoNameFrom extracts the repository name from the origin URL of a repo at the given path
func GetRepoNameFrom(ctx context.Context, repoPath string) (string, error) {
	output, err := outputGit(ctx, repoPath, "remote", "get-url", "origin")
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

// resolveToMainRepo returns the main repo path if path is a worktree, otherwise returns path unchanged.
// This ensures we get the actual repo folder name, not the worktree folder name.
func resolveToMainRepo(path string) string {
	if main, err := GetMainRepoPath(path); err == nil {
		return main
	}
	return path
}

// GetRepoFolderName returns the actual folder name of the git repo on disk
// Uses git rev-parse --show-toplevel to get the root directory.
// If inside a worktree, resolves to the main repo folder name.
func GetRepoFolderName(ctx context.Context) (string, error) {
	output, err := outputGit(ctx, "", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %v", err)
	}

	repoPath := strings.TrimSpace(string(output))
	// If inside a worktree, resolve to main repo folder name
	repoPath = resolveToMainRepo(repoPath)
	return filepath.Base(repoPath), nil
}

// GetDefaultBranch returns the default branch name for the remote (e.g., "main" or "master")
func GetDefaultBranch(ctx context.Context, repoPath string) string {
	// Try to get default branch from remote HEAD
	output, err := outputGit(ctx, repoPath, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		// Output is like "refs/remotes/origin/main"
		ref := strings.TrimSpace(string(output))
		if parts := strings.Split(ref, "/"); len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Fallback: check if origin/main exists
	if runGit(ctx, repoPath, "rev-parse", "--verify", "origin/main") == nil {
		return "main"
	}

	// Fallback: check if origin/master exists
	if runGit(ctx, repoPath, "rev-parse", "--verify", "origin/master") == nil {
		return "master"
	}

	// Last resort default
	return "main"
}

// GetCurrentBranch returns the current branch name
// Returns "(detached)" for detached HEAD state
func GetCurrentBranch(ctx context.Context, path string) (string, error) {
	output, err := outputGit(ctx, path, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("failed to get branch: %v", err)
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "(detached)", nil
	}
	return branch, nil
}

// GetCommitCount returns number of commits ahead of the default branch
func GetCommitCount(ctx context.Context, repoPath, branch string) (int, error) {
	return GetCommitCountWithBase(ctx, repoPath, branch, GetDefaultBranch(ctx, repoPath))
}

// GetCommitCountWithBase returns number of commits ahead of the given base branch.
// Use this when you already have the default branch to avoid redundant git calls.
func GetCommitCountWithBase(ctx context.Context, repoPath, branch, baseBranch string) (int, error) {
	output, err := outputGit(ctx, repoPath, "rev-list", "--count", fmt.Sprintf("origin/%s..%s", baseBranch, branch))
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %v", err)
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	return count, err
}

// LastCommitInfo contains both relative and absolute time of the last commit
type LastCommitInfo struct {
	Relative string    // Human-readable relative time (e.g., "2 days ago")
	Time     time.Time // Absolute timestamp
}

// GetLastCommitInfo returns both relative time and absolute timestamp in a single git call.
// Use this instead of calling GetLastCommitRelative and GetLastCommitTime separately.
func GetLastCommitInfo(ctx context.Context, path string) (LastCommitInfo, error) {
	output, err := outputGit(ctx, path, "log", "-1", "--format=%cr|%ct")
	if err != nil {
		return LastCommitInfo{}, fmt.Errorf("failed to get last commit: %v", err)
	}

	parts := strings.SplitN(strings.TrimSpace(string(output)), "|", 2)
	if len(parts) != 2 {
		return LastCommitInfo{}, fmt.Errorf("unexpected git log output format")
	}

	var timestamp int64
	_, err = fmt.Sscanf(parts[1], "%d", &timestamp)
	if err != nil {
		return LastCommitInfo{}, fmt.Errorf("failed to parse commit timestamp: %w", err)
	}

	return LastCommitInfo{
		Relative: parts[0],
		Time:     time.Unix(timestamp, 0),
	}, nil
}

// GetLastCommitRelative returns relative time of last commit
func GetLastCommitRelative(ctx context.Context, path string) (string, error) {
	output, err := outputGit(ctx, path, "log", "-1", "--format=%cr")
	if err != nil {
		return "", fmt.Errorf("failed to get last commit: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetLastCommitTime returns the unix timestamp of the last commit
func GetLastCommitTime(ctx context.Context, path string) (time.Time, error) {
	output, err := outputGit(ctx, path, "log", "-1", "--format=%ct")
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
func IsDirty(ctx context.Context, path string) bool {
	output, err := outputGit(ctx, path, "status", "--porcelain")
	if err != nil {
		return false // Treat error as clean (safe default)
	}
	return strings.TrimSpace(string(output)) != ""
}

// FetchDefaultBranch fetches the default branch (main/master) from origin
func FetchDefaultBranch(ctx context.Context, repoPath string) error {
	defaultBranch := GetDefaultBranch(ctx, repoPath)
	return FetchBranch(ctx, repoPath, defaultBranch)
}

// FetchBranch fetches a specific branch from origin
func FetchBranch(ctx context.Context, repoPath, branch string) error {
	if err := runGit(ctx, repoPath, "fetch", "origin", branch, "--quiet"); err != nil {
		return fmt.Errorf("failed to fetch origin/%s: %v", branch, err)
	}
	return nil
}

// HasRemote checks if a remote with the given name exists
func HasRemote(ctx context.Context, repoPath, remoteName string) bool {
	return runGit(ctx, repoPath, "remote", "get-url", remoteName) == nil
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
func GetUpstreamBranch(ctx context.Context, repoPath, branch string) string {
	output, err := outputGit(ctx, repoPath, "config", fmt.Sprintf("branch.%s.merge", branch))
	if err != nil {
		return ""
	}
	// Output is like "refs/heads/feature-branch"
	ref := strings.TrimSpace(string(output))
	return strings.TrimPrefix(ref, "refs/heads/")
}

// BranchExists checks if a local branch exists
func BranchExists(ctx context.Context, branch string) (bool, error) {
	err := runGit(ctx, "", "rev-parse", "--verify", "refs/heads/"+branch)
	if err != nil {
		// Exit code 128 means branch doesn't exist
		return false, nil
	}
	return true, nil
}

// GetShortCommitHash returns the short (7 char) commit hash for HEAD in a worktree
func GetShortCommitHash(ctx context.Context, path string) (string, error) {
	output, err := outputGit(ctx, path, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentRepoMainPath returns the main repository path from cwd
// Works whether you're in the main repo or a worktree
// Returns empty string if not in a git repo
func GetCurrentRepoMainPath(ctx context.Context) string {
	return GetCurrentRepoMainPathFrom(ctx, "")
}

// GetCurrentRepoMainPathFrom returns the main repository path from the given path
// Works whether you're in the main repo or a worktree
// Returns empty string if not in a git repo
func GetCurrentRepoMainPathFrom(ctx context.Context, path string) string {
	// First check if we're in a git repo at all
	output, err := outputGit(ctx, path, "rev-parse", "--show-toplevel")
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
func GetOriginURL(ctx context.Context, repoPath string) (string, error) {
	output, err := outputGit(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to get origin URL: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetBranchWorktree returns the worktree path if branch is checked out, empty string if not
func GetBranchWorktree(ctx context.Context, branch string) (string, error) {
	output, err := outputGit(ctx, "", "worktree", "list", "--porcelain")
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
func ListWorktreesFromRepo(ctx context.Context, repoPath string) ([]WorktreeInfo, error) {
	output, err := outputGit(ctx, repoPath, "worktree", "list", "--porcelain")
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

// GetWorktreeBranches returns a set of branch names that are currently checked out in worktrees.
// Useful for filtering out branches that can't be checked out again.
func GetWorktreeBranches(ctx context.Context, repoPath string) map[string]bool {
	branches := make(map[string]bool)
	worktrees, err := ListWorktreesFromRepo(ctx, repoPath)
	if err != nil {
		return branches
	}
	for _, wt := range worktrees {
		if wt.Branch != "" && wt.Branch != "(detached)" {
			branches[wt.Branch] = true
		}
	}
	return branches
}

// GetAllBranchConfig returns branch notes and upstreams for a repository in one call.
// Uses: `git config --get-regexp 'branch\.'`
// Returns: notes map (branch -> note), upstreams map (branch -> upstream ref)
func GetAllBranchConfig(ctx context.Context, repoPath string) (notes map[string]string, upstreams map[string]bool) {
	notes = make(map[string]string)
	upstreams = make(map[string]bool)

	output, err := outputGit(ctx, repoPath, "config", "--get-regexp", `branch\.`)
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
func DeleteLocalBranch(ctx context.Context, repoPath, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	if err := runGit(ctx, repoPath, "branch", flag, branch); err != nil {
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
func GetDiffStats(ctx context.Context, repoPath, branch string) (DiffStats, error) {
	return GetDiffStatsWithBase(ctx, repoPath, branch, GetDefaultBranch(ctx, repoPath))
}

// GetDiffStatsWithBase returns additions, deletions, and files changed vs the given base branch.
// Use this when you already have the default branch to avoid redundant git calls.
func GetDiffStatsWithBase(ctx context.Context, repoPath, branch, baseBranch string) (DiffStats, error) {
	output, err := outputGit(ctx, repoPath, "diff", "--numstat", fmt.Sprintf("origin/%s...%s", baseBranch, branch))
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
func GetCommitsBehind(ctx context.Context, repoPath, branch string) (int, error) {
	return GetCommitsBehindWithBase(ctx, repoPath, branch, GetDefaultBranch(ctx, repoPath))
}

// GetCommitsBehindWithBase returns number of commits behind the given base branch.
// Use this when you already have the default branch to avoid redundant git calls.
func GetCommitsBehindWithBase(ctx context.Context, repoPath, branch, baseBranch string) (int, error) {
	output, err := outputGit(ctx, repoPath, "rev-list", "--count", fmt.Sprintf("%s..origin/%s", branch, baseBranch))
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

// ListLocalBranches returns all local branch names for a repository.
func ListLocalBranches(ctx context.Context, repoPath string) ([]string, error) {
	output, err := outputGit(ctx, repoPath, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %v", err)
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// ListRemoteBranches returns all remote branch names (without origin/ prefix) for a repository.
func ListRemoteBranches(ctx context.Context, repoPath string) ([]string, error) {
	output, err := outputGit(ctx, repoPath, "branch", "-r", "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches: %v", err)
	}

	var branches []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Remove origin/ prefix and skip HEAD
		if strings.HasPrefix(line, "origin/") {
			branch := strings.TrimPrefix(line, "origin/")
			if branch != "HEAD" && !seen[branch] {
				branches = append(branches, branch)
				seen[branch] = true
			}
		}
	}
	return branches, nil
}

// RepoType indicates whether a repo is bare or regular
type RepoType int

const (
	RepoTypeRegular RepoType = iota
	RepoTypeBare
)

// DetectRepoType determines if a path is a bare or regular git repository
func DetectRepoType(path string) (RepoType, error) {
	// Check for .git directory or symlink (regular repo)
	gitDir := filepath.Join(path, ".git")

	// Use Lstat to check if .git is a symlink (without following it)
	linfo, err := os.Lstat(gitDir)
	if err == nil {
		// Check if .git is a symlink (bare repo with .bare/.git pattern)
		if linfo.Mode()&os.ModeSymlink != 0 {
			// .git is a symlink - check where it points
			target, err := os.Readlink(gitDir)
			if err != nil {
				return 0, fmt.Errorf("failed to read .git symlink: %w", err)
			}
			// Resolve relative paths
			if !filepath.IsAbs(target) {
				target = filepath.Join(path, target)
			}
			// Check if target is a bare repo
			if isBareRepo(target) {
				return RepoTypeBare, nil
			}
			return 0, fmt.Errorf("invalid .git symlink target: %s", target)
		}

		// .git exists and is not a symlink - check if directory or file
		if linfo.IsDir() {
			return RepoTypeRegular, nil
		}

		// .git file - could be a worktree or a pointer to a bare repo (grove pattern)
		// Read the gitdir to determine which
		content, err := os.ReadFile(gitDir)
		if err != nil {
			return 0, fmt.Errorf("failed to read .git file: %w", err)
		}
		gitdirLine := strings.TrimSpace(string(content))
		if !strings.HasPrefix(gitdirLine, "gitdir: ") {
			return 0, fmt.Errorf("invalid .git file format: %s", path)
		}
		targetDir := strings.TrimPrefix(gitdirLine, "gitdir: ")
		// Resolve relative paths
		if !filepath.IsAbs(targetDir) {
			targetDir = filepath.Join(path, targetDir)
		}
		// Check if target is a bare repo
		if isBareRepo(targetDir) {
			return RepoTypeBare, nil
		}
		// Otherwise it's a worktree pointer
		return 0, fmt.Errorf("path is a worktree, not a repository: %s", path)
	}

	// Check for bare repo markers (HEAD file at root)
	if isBareRepo(path) {
		return RepoTypeBare, nil
	}

	return 0, fmt.Errorf("not a git repository: %s", path)
}

// isBareRepo checks if a path has bare repo markers (HEAD, objects/, refs/)
func isBareRepo(path string) bool {
	headFile := filepath.Join(path, "HEAD")
	if _, err := os.Stat(headFile); err != nil {
		return false
	}
	objectsDir := filepath.Join(path, "objects")
	if _, err := os.Stat(objectsDir); err != nil {
		return false
	}
	refsDir := filepath.Join(path, "refs")
	if _, err := os.Stat(refsDir); err != nil {
		return false
	}
	return true
}

// GetGitDir returns the git directory for a repo
func GetGitDir(repoPath string, repoType RepoType) string {
	if repoType == RepoTypeBare {
		// Check for .bare/.git symlink pattern
		bareDir := filepath.Join(repoPath, ".bare")
		if isBareRepo(bareDir) {
			return bareDir
		}
		// Traditional bare repo (the path itself is the git dir)
		return repoPath
	}
	return filepath.Join(repoPath, ".git")
}

// GetGitDirForWorktree returns the .git directory that a worktree points to
func GetGitDirForWorktree(worktreePath string) (string, error) {
	gitFile := filepath.Join(worktreePath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		// Maybe it's the main repo, check if .git is a directory
		info, statErr := os.Stat(gitFile)
		if statErr == nil && info.IsDir() {
			return gitFile, nil
		}
		return "", fmt.Errorf("not a git worktree: %s", worktreePath)
	}

	// Parse: "gitdir: /path/to/repo/.git/worktrees/name"
	line := strings.TrimSpace(string(content))
	if idx := strings.Index(line, "\n"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", fmt.Errorf("invalid .git file format")
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(worktreePath, gitdir)
	}

	// Walk up to find the main .git directory
	dir := filepath.Clean(gitdir)
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find .git directory")
		}
		if filepath.Base(dir) == ".git" {
			return dir, nil
		}
		dir = parent
	}
}

// CloneBareWithWorktreeSupport clones a repo as a bare repo inside a .bare directory
// and creates a .git symlink pointing to it. This allows worktrees to be created as
// siblings to .bare while still having git commands work via the symlink.
//
// The directory structure will be:
//
//	destPath/
//	├── .bare/     # actual bare git repo
//	└── .git       # symlink -> .bare
func CloneBareWithWorktreeSupport(ctx context.Context, url, destPath string) error {
	// Create the destination directory
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Clone as bare into .bare subdirectory
	bareDir := filepath.Join(destPath, ".bare")
	if err := runGit(ctx, "", "clone", "--bare", url, bareDir); err != nil {
		// Clean up on failure
		os.RemoveAll(destPath)
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Create .git symlink pointing to .bare
	gitSymlink := filepath.Join(destPath, ".git")
	if err := os.Symlink(".bare", gitSymlink); err != nil {
		// Clean up on failure
		os.RemoveAll(destPath)
		return fmt.Errorf("failed to create .git symlink: %w", err)
	}

	// Configure the bare repo to know about worktrees
	// Set core.bare=false so worktree commands work properly
	if err := runGit(ctx, bareDir, "config", "core.bare", "false"); err != nil {
		os.RemoveAll(destPath)
		return fmt.Errorf("failed to configure repo: %w", err)
	}

	// Set fetch refspec to get all branches
	if err := runGit(ctx, bareDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		os.RemoveAll(destPath)
		return fmt.Errorf("failed to configure fetch refspec: %w", err)
	}

	return nil
}

// SetupBareWorktreeSupport converts an existing bare repo into the .bare/.git symlink pattern.
// This is useful when a forge CLI clones a bare repo and we need to add the symlink structure.
//
// The directory structure after setup will be:
//
//	destPath/
//	├── .bare/     # actual bare git repo (moved from original location)
//	└── .git       # symlink -> .bare
func SetupBareWorktreeSupport(ctx context.Context, bareRepoPath, destPath string) error {
	// Create the destination directory
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Move the bare repo to .bare subdirectory
	bareDir := filepath.Join(destPath, ".bare")
	if err := os.Rename(bareRepoPath, bareDir); err != nil {
		return fmt.Errorf("failed to move bare repo: %w", err)
	}

	// Create .git symlink pointing to .bare
	gitSymlink := filepath.Join(destPath, ".git")
	if err := os.Symlink(".bare", gitSymlink); err != nil {
		// Try to restore on failure
		os.Rename(bareDir, bareRepoPath)
		return fmt.Errorf("failed to create .git symlink: %w", err)
	}

	// Configure the bare repo to know about worktrees
	if err := runGit(ctx, bareDir, "config", "core.bare", "false"); err != nil {
		return fmt.Errorf("failed to configure repo: %w", err)
	}

	// Set fetch refspec to get all branches
	if err := runGit(ctx, bareDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return fmt.Errorf("failed to configure fetch refspec: %w", err)
	}

	return nil
}

// GetBranchCreatedTime returns when the branch was created (first commit on branch)
// Falls back to first commit time if reflog is unavailable
func GetBranchCreatedTime(ctx context.Context, repoPath, branch string) (time.Time, error) {
	// Try reflog first - most reliable for local branches
	output, err := outputGit(ctx, repoPath, "reflog", "show", "--date=iso", "--format=%gd", branch)
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
	defaultBranch := GetDefaultBranch(ctx, repoPath)
	output, err = outputGit(ctx, repoPath, "merge-base", fmt.Sprintf("origin/%s", defaultBranch), branch)
	if err == nil {
		mergeBase := strings.TrimSpace(string(output))
		output, err = outputGit(ctx, repoPath, "log", "-1", "--format=%ci", mergeBase)
		if err == nil {
			t, err := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(string(output)))
			if err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("could not determine branch creation time")
}

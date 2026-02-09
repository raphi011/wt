package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExtractRepoNameFromURL extracts the repository name from a git URL
func ExtractRepoNameFromURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

// GetRepoDisplayName returns the folder name of the repository.
func GetRepoDisplayName(repoPath string) string {
	return filepath.Base(repoPath)
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

	// Fallback: check if origin/main exists (remote tracking ref)
	if runGit(ctx, repoPath, "rev-parse", "--verify", "origin/main") == nil {
		return "main"
	}

	// Fallback: check if origin/master exists (remote tracking ref)
	if runGit(ctx, repoPath, "rev-parse", "--verify", "origin/master") == nil {
		return "master"
	}

	// For bare clones, remote tracking refs may not exist yet.
	// Check local branches as fallback.
	if runGit(ctx, repoPath, "rev-parse", "--verify", "refs/heads/main") == nil {
		return "main"
	}

	if runGit(ctx, repoPath, "rev-parse", "--verify", "refs/heads/master") == nil {
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

// FetchBranch fetches a specific branch from origin
func FetchBranch(ctx context.Context, repoPath, branch string) error {
	return FetchBranchFromRemote(ctx, repoPath, "origin", branch)
}

// FetchBranchFromRemote fetches a specific branch from a named remote.
func FetchBranchFromRemote(ctx context.Context, repoPath, remote, branch string) error {
	if err := runGit(ctx, repoPath, "fetch", remote, branch, "--quiet"); err != nil {
		return fmt.Errorf("failed to fetch %s/%s: %v", remote, branch, err)
	}
	return nil
}

// ParseRemoteRef checks if ref has a valid remote prefix (e.g., "origin/main").
// Returns the remote name, branch name, and whether it's a remote ref.
func ParseRemoteRef(ctx context.Context, repoPath, ref string) (remote, branch string, isRemote bool) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", ref, false
	}
	if HasRemote(ctx, repoPath, parts[0]) {
		return parts[0], parts[1], true
	}
	return "", ref, false
}

// HasRemote checks if a remote with the given name exists
func HasRemote(ctx context.Context, repoPath, remoteName string) bool {
	return runGit(ctx, repoPath, "remote", "get-url", remoteName) == nil
}

// ListRemotes returns all remote names for a repository.
func ListRemotes(ctx context.Context, repoPath string) ([]string, error) {
	output, err := outputGit(ctx, repoPath, "remote")
	if err != nil {
		return nil, fmt.Errorf("failed to list remotes: %v", err)
	}

	var remotes []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			remotes = append(remotes, line)
		}
	}
	return remotes, nil
}

// GetMainRepoPath returns the main repository path from a worktree path.
// Uses git commands rather than reading .git files directly.
func GetMainRepoPath(worktreePath string) (string, error) {
	return GetMainRepoPathWithContext(context.Background(), worktreePath)
}

// GetMainRepoPathWithContext returns the main repository path from a worktree path.
// Uses git rev-parse --git-common-dir to find the shared git directory.
func GetMainRepoPathWithContext(ctx context.Context, worktreePath string) (string, error) {
	output, err := outputGit(ctx, worktreePath, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}

	gitCommonDir := strings.TrimSpace(string(output))

	// Handle relative paths
	if !filepath.IsAbs(gitCommonDir) {
		gitCommonDir = filepath.Join(worktreePath, gitCommonDir)
	}
	gitCommonDir = filepath.Clean(gitCommonDir)

	// The main repo is the parent of the git common directory
	return filepath.Dir(gitCommonDir), nil
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

// SetUpstreamBranch sets the upstream tracking branch for a local branch.
// upstream should be "origin/<branch>" or just "<branch>" (will prepend origin/).
func SetUpstreamBranch(ctx context.Context, repoPath, localBranch, upstream string) error {
	if !strings.HasPrefix(upstream, "origin/") {
		upstream = "origin/" + upstream
	}
	return runGit(ctx, repoPath, "branch", "--set-upstream-to="+upstream, localBranch)
}

// RefExists checks if a git ref resolves to a valid object.
// Returns false for unborn HEAD (empty repos with no commits).
// Works for any ref: HEAD, origin/main, refs/heads/branch, etc.
func RefExists(ctx context.Context, repoPath, ref string) bool {
	return runGit(ctx, repoPath, "rev-parse", "--verify", ref) == nil
}

// RemoteBranchExists checks if a remote tracking branch exists.
func RemoteBranchExists(ctx context.Context, repoPath, branch string) bool {
	ref := "refs/remotes/origin/" + branch
	return runGit(ctx, repoPath, "rev-parse", "--verify", ref) == nil
}

// LocalBranchExists checks if a local branch exists in the given repo.
func LocalBranchExists(ctx context.Context, repoPath, branch string) bool {
	ref := "refs/heads/" + branch
	return runGit(ctx, repoPath, "rev-parse", "--verify", ref) == nil
}

// PushBranch pushes a branch to origin.
func PushBranch(ctx context.Context, repoPath, branch string) error {
	return runGit(ctx, repoPath, "push", "-u", "origin", branch)
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
	// Get the shared git directory (works for both worktrees and main repos)
	output, err := outputGit(ctx, path, "rev-parse", "--git-common-dir")
	if err != nil {
		return ""
	}
	gitCommonDir := strings.TrimSpace(string(output))

	// Handle relative paths (git may return relative path like ".git")
	if !filepath.IsAbs(gitCommonDir) {
		if path == "" {
			// Use current working directory
			gitCommonDir = filepath.Join(".", gitCommonDir)
		} else {
			gitCommonDir = filepath.Join(path, gitCommonDir)
		}
	}
	gitCommonDir = filepath.Clean(gitCommonDir)

	// The main repo is the parent of the git common directory
	// For regular repos: /path/to/repo/.git -> /path/to/repo
	// For bare-in-.git: /path/to/repo/.git -> /path/to/repo
	// For bare-in-.bare: /path/to/repo/.bare -> /path/to/repo
	return filepath.Dir(gitCommonDir)
}

// GetOriginURL gets the origin URL for a repository
func GetOriginURL(ctx context.Context, repoPath string) (string, error) {
	output, err := outputGit(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to get origin URL: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// WorktreeInfo contains basic worktree information from git worktree list.
type WorktreeInfo struct {
	Path       string
	Branch     string
	CommitHash string // Full hash from git, caller can truncate
}

// ListWorktreesFromRepo returns all worktrees for a repository using git worktree list --porcelain -z.
// Uses NUL-separated output for robust parsing of paths with special characters.
// Skips bare worktrees (e.g., .bare directory in bare repo layouts).
func ListWorktreesFromRepo(ctx context.Context, repoPath string) ([]WorktreeInfo, error) {
	output, err := outputGit(ctx, repoPath, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %v", err)
	}

	var worktrees []WorktreeInfo

	// Split on double-NUL for record boundaries
	records := strings.Split(string(output), "\x00\x00")

	for _, record := range records {
		if record == "" {
			continue
		}

		var wt WorktreeInfo
		var isBare bool

		// Split on single-NUL for fields within record
		fields := strings.Split(record, "\x00")
		for _, field := range fields {
			switch {
			case strings.HasPrefix(field, "worktree "):
				wt.Path = strings.TrimPrefix(field, "worktree ")
			case strings.HasPrefix(field, "HEAD "):
				wt.CommitHash = strings.TrimPrefix(field, "HEAD ")
			case strings.HasPrefix(field, "branch refs/heads/"):
				wt.Branch = strings.TrimPrefix(field, "branch refs/heads/")
			case field == "detached":
				wt.Branch = "(detached)"
			case field == "bare":
				isBare = true
			}
		}

		if wt.Path != "" && !isBare {
			worktrees = append(worktrees, wt)
		}
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

// ListRemoteBranches returns all remote branch names (with remote prefix, e.g. "origin/main") for a repository.
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
		// Skip HEAD pointers (e.g., "origin/HEAD")
		if strings.HasSuffix(line, "/HEAD") {
			continue
		}
		if !seen[line] {
			branches = append(branches, line)
			seen[line] = true
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
	// Check for .git directory (regular repo or bare-in-.git pattern)
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	if err == nil {
		if info.IsDir() {
			// .git is a directory - check if it's a bare repo inside .git
			// (bare-in-.git pattern: bare repo contents in .git, no working tree)
			if isBareRepo(gitDir) {
				return RepoTypeBare, nil
			}
			return RepoTypeRegular, nil
		}
		// .git file - could be a worktree or a pointer to a bare repo
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

// isBareRepo checks if a path is a bare repository by checking core.bare config.
// This is used to detect bare-in-.git pattern where a bare repo is placed inside .git/
func isBareRepo(path string) bool {
	// First check basic structure (HEAD, objects, refs)
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

	// Check core.bare config - this distinguishes bare from regular repos
	configPath := filepath.Join(path, "config")
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	// Look for "bare = true" in the config
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "bare") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				return value == "true"
			}
		}
	}

	return false
}

// GetGitDir returns the git directory for a repo
func GetGitDir(repoPath string, repoType RepoType) string {
	// Check for bare-in-.git pattern first
	gitDir := filepath.Join(repoPath, ".git")
	if isBareRepo(gitDir) {
		return gitDir
	}
	if repoType == RepoTypeBare {
		// Traditional bare repo (the path itself is the git dir)
		return repoPath
	}
	return gitDir
}

// CloneBareWithWorktreeSupport clones a repo as a bare repo inside the .git directory.
// This allows worktrees to be created as siblings while git commands work normally.
//
// The directory structure will be:
//
//	destPath/
//	└── .git/     # bare git repo contents (HEAD, objects/, refs/, etc.)
func CloneBareWithWorktreeSupport(ctx context.Context, url, destPath string) error {
	// Create the destination directory
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Clone as bare directly into .git subdirectory
	gitDir := filepath.Join(destPath, ".git")
	if err := runGit(ctx, "", "clone", "--bare", url, gitDir); err != nil {
		// Clean up on failure
		os.RemoveAll(destPath)
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Set fetch refspec to get all branches (bare clones don't set this up by default)
	if err := runGit(ctx, gitDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		os.RemoveAll(destPath)
		return fmt.Errorf("failed to configure fetch refspec: %w", err)
	}

	return nil
}


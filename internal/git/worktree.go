package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/raphi011/wt/internal/format"
)

// Worktree represents a git worktree with its status
type Worktree struct {
	Path         string `json:"path"`
	Branch       string `json:"branch"`
	MainRepo     string `json:"main_repo"`
	RepoName     string `json:"repo_name"`
	OriginURL    string `json:"origin_url"`
	IsMerged     bool   `json:"is_merged"`
	CommitCount  int    `json:"commit_count"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	IsDirty      bool   `json:"is_dirty"`
	HasUntracked bool   `json:"has_untracked"`
	LastCommit   string `json:"last_commit"`
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

		// Get origin URL (errors treated as empty string)
		originURL, _ := GetOriginURL(mainRepo)

		// Get merge status (errors treated as "not merged" - safe default)
		isMerged, _ := IsBranchMerged(mainRepo, branch)

		// Get commit count if not merged (errors treated as 0 commits)
		var commitCount int
		if !isMerged {
			commitCount, _ = GetCommitCount(mainRepo, branch)
		}

		// Get diff stats - errors treated as clean (safe for display purposes)
		additions, deletions, hasUntracked, _ := GetDiffStats(path)
		isDirty := additions > 0 || deletions > 0 || hasUntracked

		// Get last commit time (errors treated as empty string)
		lastCommit, _ := GetLastCommitRelative(path)

		worktrees = append(worktrees, Worktree{
			Path:         path,
			Branch:       branch,
			MainRepo:     mainRepo,
			RepoName:     repoName,
			OriginURL:    originURL,
			IsMerged:     isMerged,
			CommitCount:  commitCount,
			Additions:    additions,
			Deletions:    deletions,
			IsDirty:      isDirty,
			HasUntracked: hasUntracked,
			LastCommit:   lastCommit,
		})
	}

	return worktrees, nil
}

// CreateWorktreeResult contains the result of creating a worktree
type CreateWorktreeResult struct {
	Path          string
	AlreadyExists bool
}

// CreateWorktree creates a new git worktree at basePath/<formatted-name>
// The worktreeFmt parameter uses placeholders like {git-origin}, {branch-name}, {folder-name}
// Returns the result including whether the worktree already existed
// Returns an error if the branch already exists (use OpenWorktree instead)
func CreateWorktree(basePath, branch, worktreeFmt string) (*CreateWorktreeResult, error) {
	// Check if branch already exists
	exists, err := BranchExists(branch)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("branch %q already exists (use 'wt open' to checkout existing branch)", branch)
	}

	// Get repo name from origin URL
	gitOrigin, err := GetRepoName()
	if err != nil {
		return nil, err
	}

	// Get folder name from disk
	folderName, err := GetRepoFolderName()
	if err != nil {
		return nil, err
	}

	// Resolve base path
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	// Check if base path exists
	if _, err := os.Stat(absBasePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", absBasePath)
	}

	// Format worktree name using the template
	worktreeName := format.FormatWorktreeName(worktreeFmt, format.FormatParams{
		GitOrigin:  gitOrigin,
		BranchName: branch,
		FolderName: folderName,
	})

	// Create worktree path: <basePath>/<formatted-name>
	worktreePath := filepath.Join(absBasePath, worktreeName)

	// Check if already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: true}, nil
	}

	// Create worktree with new branch
	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("failed to create worktree: %s", errMsg)
		}
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: false}, nil
}

// CreateWorktreeFrom creates a worktree from a specified repository path
// Used when working with a repo that isn't the current working directory
func CreateWorktreeFrom(repoPath, basePath, branch, worktreeFmt string) (*CreateWorktreeResult, error) {
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}

	// Check if branch already exists in the repo
	cmd := exec.Command("git", "-C", absRepoPath, "rev-parse", "--verify", "refs/heads/"+branch)
	if cmd.Run() == nil {
		// Branch exists, check if it's already checked out
		wtPath, err := getBranchWorktreeFrom(absRepoPath, branch)
		if err != nil {
			return nil, err
		}
		if wtPath != "" {
			return nil, fmt.Errorf("branch %q is already checked out at %s", branch, wtPath)
		}
		// Branch exists but not checked out - use OpenWorktreeFrom instead
		return openWorktreeFrom(absRepoPath, basePath, branch, worktreeFmt)
	}

	// Get repo name from origin URL
	gitOrigin, err := GetRepoNameFrom(absRepoPath)
	if err != nil {
		return nil, err
	}

	// Get folder name from disk
	folderName := filepath.Base(absRepoPath)

	// Resolve base path
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	// Check if base path exists
	if _, err := os.Stat(absBasePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", absBasePath)
	}

	// Format worktree name using the template
	worktreeName := format.FormatWorktreeName(worktreeFmt, format.FormatParams{
		GitOrigin:  gitOrigin,
		BranchName: branch,
		FolderName: folderName,
	})

	// Create worktree path: <basePath>/<formatted-name>
	worktreePath := filepath.Join(absBasePath, worktreeName)

	// Check if already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: true}, nil
	}

	// Create worktree with new branch from the specified repo
	cmd = exec.Command("git", "-C", absRepoPath, "worktree", "add", worktreePath, "-b", branch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("failed to create worktree: %s", errMsg)
		}
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: false}, nil
}

// openWorktreeFrom creates a worktree for an existing branch in a specified repo
func openWorktreeFrom(absRepoPath, basePath, branch, worktreeFmt string) (*CreateWorktreeResult, error) {
	// Get repo name from origin URL
	gitOrigin, err := GetRepoNameFrom(absRepoPath)
	if err != nil {
		return nil, err
	}

	// Get folder name from disk
	folderName := filepath.Base(absRepoPath)

	// Resolve base path
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	// Check if base path exists
	if _, err := os.Stat(absBasePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", absBasePath)
	}

	// Format worktree name using the template
	worktreeName := format.FormatWorktreeName(worktreeFmt, format.FormatParams{
		GitOrigin:  gitOrigin,
		BranchName: branch,
		FolderName: folderName,
	})

	// Create worktree path: <basePath>/<formatted-name>
	worktreePath := filepath.Join(absBasePath, worktreeName)

	// Check if already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: true}, nil
	}

	// Create worktree for existing branch (no -b flag)
	cmd := exec.Command("git", "-C", absRepoPath, "worktree", "add", worktreePath, branch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("failed to create worktree: %s", errMsg)
		}
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: false}, nil
}

// getBranchWorktreeFrom returns the worktree path if branch is checked out in the given repo
func getBranchWorktreeFrom(repoPath, branch string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("failed to list worktrees: %s", strings.TrimSpace(stderr.String()))
		}
		return "", err
	}

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

// OpenWorktree creates a worktree for an existing local branch
// Returns error if branch doesn't exist or is already checked out
func OpenWorktree(basePath, branch, worktreeFmt string) (*CreateWorktreeResult, error) {
	// Check if branch exists
	exists, err := BranchExists(branch)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("branch %q does not exist (use 'wt create' to create a new branch)", branch)
	}

	// Check if branch is already checked out in another worktree
	wtPath, err := GetBranchWorktree(branch)
	if err != nil {
		return nil, err
	}
	if wtPath != "" {
		return &CreateWorktreeResult{Path: wtPath, AlreadyExists: true}, nil
	}

	// Get repo name from origin URL
	gitOrigin, err := GetRepoName()
	if err != nil {
		return nil, err
	}

	// Get folder name from disk
	folderName, err := GetRepoFolderName()
	if err != nil {
		return nil, err
	}

	// Resolve base path
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	// Check if base path exists
	if _, err := os.Stat(absBasePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", absBasePath)
	}

	// Format worktree name using the template
	worktreeName := format.FormatWorktreeName(worktreeFmt, format.FormatParams{
		GitOrigin:  gitOrigin,
		BranchName: branch,
		FolderName: folderName,
	})

	// Create worktree path: <basePath>/<formatted-name>
	worktreePath := filepath.Join(absBasePath, worktreeName)

	// Check if already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: true}, nil
	}

	// Create worktree for existing branch (no -b flag)
	cmd := exec.Command("git", "worktree", "add", worktreePath, branch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return nil, fmt.Errorf("failed to create worktree: %s", errMsg)
		}
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: false}, nil
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

// MoveWorktree moves a git worktree to a new path
func MoveWorktree(worktree Worktree, newPath string, force bool) error {
	args := []string{"-C", worktree.MainRepo, "worktree", "move", worktree.Path, newPath}
	if force {
		args = append(args, "--force")
	}

	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return fmt.Errorf("%s", errMsg)
		}
		return err
	}
	return nil
}

// PruneWorktrees prunes stale worktree references
func PruneWorktrees(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "prune")
	return cmd.Run()
}

// GroupWorktreesByRepo groups worktrees by their main repository
func GroupWorktreesByRepo(worktrees []Worktree) map[string][]Worktree {
	groups := make(map[string][]Worktree)
	for _, wt := range worktrees {
		groups[wt.RepoName] = append(groups[wt.RepoName], wt)
	}
	return groups
}

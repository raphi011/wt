package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/raphaelgruber/wt/internal/format"
)

// Worktree represents a git worktree with its status
type Worktree struct {
	Path         string `json:"path"`
	Branch       string `json:"branch"`
	MainRepo     string `json:"main_repo"`
	RepoName     string `json:"repo_name"`
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

		// Get merge status
		isMerged, _ := IsBranchMerged(mainRepo, branch)

		// Get commit count if not merged
		var commitCount int
		if !isMerged {
			commitCount, _ = GetCommitCount(mainRepo, branch)
		}

		// Get diff stats (includes untracked file detection)
		additions, deletions, hasUntracked, _ := GetDiffStats(path)
		isDirty := additions > 0 || deletions > 0 || hasUntracked

		// Get last commit time
		lastCommit, _ := GetLastCommitRelative(path)

		worktrees = append(worktrees, Worktree{
			Path:         path,
			Branch:       branch,
			MainRepo:     mainRepo,
			RepoName:     repoName,
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
func CreateWorktree(basePath, branch, worktreeFmt string) (*CreateWorktreeResult, error) {
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

	// Try to create with new branch first
	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Try existing branch
		cmd = exec.Command("git", "worktree", "add", worktreePath, branch)
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			errMsg := strings.TrimSpace(stderr.String())
			if errMsg != "" {
				return nil, fmt.Errorf("failed to create worktree: %s", errMsg)
			}
			return nil, fmt.Errorf("failed to create worktree: %w", err)
		}
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

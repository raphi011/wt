package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/raphi011/wt/internal/format"
)

// Worktree represents a git worktree with its status
type Worktree struct {
	Path           string    `json:"path"`
	Branch         string    `json:"branch"`
	MainRepo       string    `json:"main_repo"`
	RepoName       string    `json:"repo_name"`
	OriginURL      string    `json:"origin_url"`
	IsMerged       bool      `json:"is_merged"`
	CommitCount    int       `json:"commit_count"`
	HasUpstream    bool      `json:"has_upstream"`
	LastCommit     string    `json:"last_commit"`
	LastCommitTime time.Time `json:"last_commit_time"` // for sorting by commit date
	Note           string    `json:"note,omitempty"`
}

// Status returns a human-readable status string for the worktree.
// Priority: merged/prunable > commits ahead > clean
func (w *Worktree) Status() string {
	if w.IsMerged {
		return "prunable"
	}
	if w.CommitCount > 0 {
		return fmt.Sprintf("%d ahead", w.CommitCount)
	}
	return "clean"
}

// GetWorktreeInfo returns info for a single worktree at the given path
func GetWorktreeInfo(ctx context.Context, path string) (*Worktree, error) {
	gitFile := filepath.Join(path, ".git")

	// Check if it's a worktree (has .git file, not directory)
	info, err := os.Stat(gitFile)
	if err != nil {
		return nil, fmt.Errorf("not a git worktree: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("not a worktree (main repo)")
	}

	// Get main repo path
	mainRepo, err := GetMainRepoPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get main repo: %w", err)
	}

	// Get branch
	branch, err := GetCurrentBranch(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch: %w", err)
	}

	// Get repo name from main repo
	repoName := filepath.Base(mainRepo)

	// Get origin URL (errors treated as empty string)
	originURL, _ := GetOriginURL(ctx, mainRepo)

	// Get commit count (errors treated as 0 commits)
	commitCount, _ := GetCommitCount(ctx, mainRepo, branch)

	// Get last commit time (errors treated as empty/zero values)
	var lastCommit string
	var lastCommitTime time.Time
	if commitInfo, err := GetLastCommitInfo(ctx, path); err == nil {
		lastCommit = commitInfo.Relative
		lastCommitTime = commitInfo.Time
	}

	// Get branch note (errors treated as empty string)
	note, _ := GetBranchNote(ctx, mainRepo, branch)

	// Check if branch has upstream
	hasUpstream := GetUpstreamBranch(ctx, mainRepo, branch) != ""

	return &Worktree{
		Path:           path,
		Branch:         branch,
		MainRepo:       mainRepo,
		RepoName:       repoName,
		OriginURL:      originURL,
		IsMerged:       false, // Set later based on PR status
		CommitCount:    commitCount,
		HasUpstream:    hasUpstream,
		LastCommit:     lastCommit,
		LastCommitTime: lastCommitTime,
		Note:           note,
	}, nil
}

// CreateWorktreeResult contains the result of creating a worktree
type CreateWorktreeResult struct {
	Path          string
	AlreadyExists bool
}

// worktreeParams holds resolved parameters for worktree creation
type worktreeParams struct {
	worktreePath string
	absRepoPath  string // empty string means use current directory
}

// resolveWorktreeParams resolves common parameters needed for worktree creation.
// If repoPath is empty, uses current directory.
func resolveWorktreeParams(ctx context.Context, repoPath, basePath, branch, worktreeFmt string) (*worktreeParams, error) {
	var absRepoPath string
	var repoName, origin string
	var err error

	if repoPath != "" {
		absRepoPath, err = filepath.Abs(repoPath)
		if err != nil {
			return nil, err
		}
		// Resolve to main repo if this is a worktree path
		repoName = filepath.Base(resolveToMainRepo(absRepoPath))
		origin, err = GetRepoNameFrom(ctx, absRepoPath)
		if err != nil {
			// Fallback to folder name when no origin
			origin = repoName
		}
	} else {
		repoName, err = GetRepoFolderName(ctx)
		if err != nil {
			return nil, err
		}
		origin, err = GetRepoName(ctx)
		if err != nil {
			// Fallback to folder name when no origin
			origin = repoName
		}
	}

	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(absBasePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", absBasePath)
	}

	worktreeName := format.FormatWorktreeName(worktreeFmt, format.FormatParams{
		RepoName:   repoName,
		BranchName: branch,
		Origin:     origin,
	})

	return &worktreeParams{
		worktreePath: filepath.Join(absBasePath, worktreeName),
		absRepoPath:  absRepoPath,
	}, nil
}

// checkExistsOrCreate checks if worktree already exists, otherwise runs git command
func checkExistsOrCreate(ctx context.Context, params *worktreeParams, args []string) (*CreateWorktreeResult, error) {
	if _, err := os.Stat(params.worktreePath); err == nil {
		return &CreateWorktreeResult{Path: params.worktreePath, AlreadyExists: true}, nil
	}

	if err := runGit(ctx, params.absRepoPath, args...); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %v", err)
	}

	return &CreateWorktreeResult{Path: params.worktreePath, AlreadyExists: false}, nil
}

// AddWorktree creates a git worktree at basePath/<formatted-name>
// If createNew is true, creates a new branch (-b flag); otherwise checks out existing branch
// baseRef is the starting point for new branches (e.g., "origin/main")
func AddWorktree(ctx context.Context, basePath, branch, worktreeFmt string, createNew bool, baseRef string) (*CreateWorktreeResult, error) {
	if createNew {
		return createWorktreeInternal(ctx, basePath, branch, worktreeFmt, baseRef)
	}
	return openWorktreeInternal(ctx, basePath, branch, worktreeFmt)
}

// createWorktreeInternal creates a new git worktree with a new branch
// baseRef is the starting point for the new branch (e.g., "origin/main", "main", or empty for HEAD)
func createWorktreeInternal(ctx context.Context, basePath, branch, worktreeFmt, baseRef string) (*CreateWorktreeResult, error) {
	exists, err := BranchExists(ctx, branch)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("branch %q already exists (use 'wt checkout' without -b to checkout existing branch)", branch)
	}

	params, err := resolveWorktreeParams(ctx, "", basePath, branch, worktreeFmt)
	if err != nil {
		return nil, err
	}

	args := []string{"worktree", "add", params.worktreePath, "-b", branch}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	return checkExistsOrCreate(ctx, params, args)
}

// CreateWorktreeFrom creates a worktree from a specified repository path
// Used when working with a repo that isn't the current working directory
// baseRef is the starting point for the new branch (e.g., "origin/main", or empty for HEAD)
func CreateWorktreeFrom(ctx context.Context, repoPath, basePath, branch, worktreeFmt, baseRef string) (*CreateWorktreeResult, error) {
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}

	// Check if branch already exists in the repo
	if runGit(ctx, absRepoPath, "rev-parse", "--verify", "refs/heads/"+branch) == nil {
		// Branch exists, check if it's already checked out
		wtPath, err := getBranchWorktreeFrom(ctx, absRepoPath, branch)
		if err != nil {
			return nil, err
		}
		if wtPath != "" {
			return &CreateWorktreeResult{Path: wtPath, AlreadyExists: true}, nil
		}
		// Branch exists but not checked out
		return OpenWorktreeFrom(ctx, absRepoPath, basePath, branch, worktreeFmt)
	}

	params, err := resolveWorktreeParams(ctx, absRepoPath, basePath, branch, worktreeFmt)
	if err != nil {
		return nil, err
	}

	args := []string{"worktree", "add", params.worktreePath, "-b", branch}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	return checkExistsOrCreate(ctx, params, args)
}

// OpenWorktreeFrom creates a worktree for an existing branch in a specified repo
func OpenWorktreeFrom(ctx context.Context, absRepoPath, basePath, branch, worktreeFmt string) (*CreateWorktreeResult, error) {
	params, err := resolveWorktreeParams(ctx, absRepoPath, basePath, branch, worktreeFmt)
	if err != nil {
		return nil, err
	}

	args := []string{"worktree", "add", params.worktreePath, branch}
	return checkExistsOrCreate(ctx, params, args)
}

// getBranchWorktreeFrom returns the worktree path if branch is checked out in the given repo
func getBranchWorktreeFrom(ctx context.Context, repoPath, branch string) (string, error) {
	output, err := outputGit(ctx, repoPath, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %v", err)
	}

	// Split on double-NUL for record boundaries
	records := strings.Split(string(output), "\x00\x00")

	for _, record := range records {
		if record == "" {
			continue
		}

		var wtPath, wtBranch string

		// Split on single-NUL for fields within record
		fields := strings.Split(record, "\x00")
		for _, field := range fields {
			switch {
			case strings.HasPrefix(field, "worktree "):
				wtPath = strings.TrimPrefix(field, "worktree ")
			case strings.HasPrefix(field, "branch refs/heads/"):
				wtBranch = strings.TrimPrefix(field, "branch refs/heads/")
			}
		}

		if wtBranch == branch && wtPath != "" {
			return wtPath, nil
		}
	}

	return "", nil
}

// openWorktreeInternal creates a worktree for an existing local branch
func openWorktreeInternal(ctx context.Context, basePath, branch, worktreeFmt string) (*CreateWorktreeResult, error) {
	exists, err := BranchExists(ctx, branch)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("branch %q does not exist (use 'wt checkout -b' to create a new branch)", branch)
	}

	wtPath, err := GetBranchWorktree(ctx, branch)
	if err != nil {
		return nil, err
	}
	if wtPath != "" {
		return &CreateWorktreeResult{Path: wtPath, AlreadyExists: true}, nil
	}

	params, err := resolveWorktreeParams(ctx, "", basePath, branch, worktreeFmt)
	if err != nil {
		return nil, err
	}

	args := []string{"worktree", "add", params.worktreePath, branch}
	return checkExistsOrCreate(ctx, params, args)
}

// RemoveWorktree removes a git worktree
func RemoveWorktree(ctx context.Context, worktree Worktree, force bool) error {
	args := []string{"worktree", "remove", worktree.Path}
	if force {
		args = append(args, "--force")
	}

	return runGit(ctx, worktree.MainRepo, args...)
}

// MoveWorktree moves a git worktree to a new path
func MoveWorktree(ctx context.Context, worktree Worktree, newPath string, force bool) error {
	args := []string{"worktree", "move", worktree.Path, newPath}
	if force {
		args = append(args, "--force")
	}

	return runGit(ctx, worktree.MainRepo, args...)
}

// PruneWorktrees prunes stale worktree references
func PruneWorktrees(ctx context.Context, repoPath string) error {
	return runGit(ctx, repoPath, "worktree", "prune")
}

// GroupWorktreesByRepo groups worktrees by their main repository
func GroupWorktreesByRepo(worktrees []Worktree) map[string][]Worktree {
	groups := make(map[string][]Worktree)
	for _, wt := range worktrees {
		groups[wt.RepoName] = append(groups[wt.RepoName], wt)
	}
	return groups
}

// FilterWorktreesByRepo returns worktrees that belong to the given main repo path.
func FilterWorktreesByRepo(worktrees []Worktree, mainRepoPath string) []Worktree {
	var filtered []Worktree
	for _, wt := range worktrees {
		if wt.MainRepo == mainRepoPath {
			filtered = append(filtered, wt)
		}
	}
	return filtered
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

// RepairWorktree attempts to repair broken links for a single worktree.
// Uses `git worktree repair <path>` from the main repo.
func RepairWorktree(ctx context.Context, repoPath, worktreePath string) error {
	return runGit(ctx, repoPath, "worktree", "repair", worktreePath)
}

// RepairWorktreesFromRepo repairs all worktrees for a repository.
// Uses `git worktree repair` without arguments to repair all.
func RepairWorktreesFromRepo(ctx context.Context, repoPath string) error {
	return runGit(ctx, repoPath, "worktree", "repair")
}

// ListPrunableWorktrees returns worktree paths that git considers stale.
// Uses `git worktree prune --dry-run` and parses the output.
func ListPrunableWorktrees(ctx context.Context, repoPath string) ([]string, error) {
	output, err := outputGit(ctx, repoPath, "worktree", "prune", "--dry-run", "-v")
	if err != nil {
		return nil, fmt.Errorf("failed to list prunable worktrees: %v", err)
	}

	var prunable []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Output format: "Removing worktrees/<name>: <reason>"
		// We extract the worktree name/path
		if strings.HasPrefix(line, "Removing ") {
			// Extract path between "Removing " and ":"
			rest := strings.TrimPrefix(line, "Removing ")
			if idx := strings.Index(rest, ":"); idx > 0 {
				prunable = append(prunable, rest[:idx])
			}
		}
	}
	return prunable, nil
}

// IsWorktreeLinkValid checks if a worktree's bidirectional link is valid.
// Returns true if both the .git file in worktree and gitdir in main repo exist and match.
func IsWorktreeLinkValid(worktreePath string) bool {
	// Check .git file exists and is readable
	gitFile := filepath.Join(worktreePath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return false
	}

	// Parse gitdir from .git file
	line := strings.TrimSpace(string(content))
	if idx := strings.Index(line, "\n"); idx != -1 {
		line = strings.TrimSpace(line[:idx])
	}
	if !strings.HasPrefix(line, "gitdir: ") {
		return false
	}

	gitdir := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(worktreePath, gitdir)
	}
	gitdir = filepath.Clean(gitdir)

	// Check gitdir exists
	if _, err := os.Stat(gitdir); err != nil {
		return false
	}

	// Check the back-link exists (gitdir should contain a "gitdir" file pointing back)
	// The gitdir (e.g., .git/worktrees/name) should contain a file named "gitdir"
	// that points back to the worktree's .git file
	backLink := filepath.Join(gitdir, "gitdir")
	backContent, err := os.ReadFile(backLink)
	if err != nil {
		return false
	}

	// The back-link should resolve to the worktree path
	linkedPath := strings.TrimSpace(string(backContent))
	if !filepath.IsAbs(linkedPath) {
		linkedPath = filepath.Join(gitdir, linkedPath)
	}
	linkedPath = filepath.Clean(linkedPath)

	// Remove .git suffix if present to get worktree path
	expectedPath := filepath.Clean(worktreePath)
	linkedWorktree := filepath.Dir(linkedPath) // linkedPath points to .git file
	if strings.HasSuffix(linkedPath, "/.git") || strings.HasSuffix(linkedPath, "\\.git") {
		linkedWorktree = strings.TrimSuffix(linkedPath, "/.git")
		linkedWorktree = strings.TrimSuffix(linkedWorktree, "\\.git")
	}

	return linkedWorktree == expectedPath || linkedPath == filepath.Join(expectedPath, ".git")
}

// CanRepairWorktree checks if a worktree can potentially be repaired.
// Returns true if .git file exists but links are broken.
func CanRepairWorktree(worktreePath string) bool {
	gitFile := filepath.Join(worktreePath, ".git")
	info, err := os.Stat(gitFile)
	if err != nil || info.IsDir() {
		return false // No .git file or it's a directory (main repo)
	}

	// .git file exists but link is invalid - potentially repairable
	return !IsWorktreeLinkValid(worktreePath)
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

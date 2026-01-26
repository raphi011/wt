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
	IsDirty        bool      `json:"is_dirty"` // only populated when includeDirty=true
	HasUpstream    bool      `json:"has_upstream"`
	LastCommit     string    `json:"last_commit"`
	LastCommitTime time.Time `json:"last_commit_time"` // for sorting by commit date
	Note           string    `json:"note,omitempty"`
}

// Status returns a human-readable status string for the worktree.
// Priority: dirty > merged/prunable > commits ahead > clean
func (w *Worktree) Status() string {
	if w.IsDirty {
		return "dirty"
	}
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

	// Get merge status (errors treated as "not merged" - safe default)
	isMerged, _ := IsBranchMerged(ctx, mainRepo, branch)

	// Get commit count if not merged (errors treated as 0 commits)
	var commitCount int
	if !isMerged {
		commitCount, _ = GetCommitCount(ctx, mainRepo, branch)
	}

	// Check dirty status via git status --porcelain
	isDirty := IsDirty(ctx, path)

	// Get last commit time (errors treated as empty/zero values)
	lastCommit, _ := GetLastCommitRelative(ctx, path)
	lastCommitTime, _ := GetLastCommitTime(ctx, path)

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
		IsMerged:       isMerged,
		CommitCount:    commitCount,
		IsDirty:        isDirty,
		HasUpstream:    hasUpstream,
		LastCommit:     lastCommit,
		LastCommitTime: lastCommitTime,
		Note:           note,
	}, nil
}

// ListWorktrees scans a directory for git worktrees with batched git calls per repo.
// If includeDirty is true, checks each worktree for dirty status (adds subprocess calls).
// For 10 worktrees across 2 repos: ~8 calls (list) or ~18 calls with dirty checks (tidy).
func ListWorktrees(ctx context.Context, worktreeDir string, includeDirty bool) ([]Worktree, error) {
	entries, err := os.ReadDir(worktreeDir)
	if err != nil {
		return nil, err
	}

	// Phase 1: Find all worktrees and group by main repo (file I/O only)
	type pendingWorktree struct {
		path     string
		mainRepo string
	}
	var pending []pendingWorktree
	mainRepos := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(worktreeDir, entry.Name())
		gitFile := filepath.Join(path, ".git")

		// Check if it's a worktree (has .git file, not directory)
		info, err := os.Stat(gitFile)
		if err != nil || info.IsDir() {
			continue
		}

		// Get main repo path (file read, no subprocess)
		mainRepo, err := GetMainRepoPath(path)
		if err != nil {
			continue
		}

		pending = append(pending, pendingWorktree{path: path, mainRepo: mainRepo})
		mainRepos[mainRepo] = true
	}

	if len(pending) == 0 {
		return nil, nil
	}

	// Phase 2: Batch fetch info per main repo (4 subprocess calls per repo)
	// Map: mainRepo -> (worktree path -> WorktreeInfo)
	repoWorktrees := make(map[string]map[string]WorktreeInfo)
	// Map: mainRepo -> (branch -> note)
	repoNotes := make(map[string]map[string]string)
	// Map: mainRepo -> (branch -> hasUpstream)
	repoUpstreams := make(map[string]map[string]bool)
	// Map: mainRepo -> originURL
	repoOrigins := make(map[string]string)
	// Map: mainRepo -> merged branches
	repoMerged := make(map[string]map[string]bool)

	for mainRepo := range mainRepos {
		// Get all worktrees from this repo in one call
		wtInfos, err := ListWorktreesFromRepo(ctx, mainRepo)
		if err != nil {
			continue
		}
		repoWorktrees[mainRepo] = make(map[string]WorktreeInfo)
		for _, wti := range wtInfos {
			repoWorktrees[mainRepo][wti.Path] = wti
		}

		// Get origin URL once
		originURL, _ := GetOriginURL(ctx, mainRepo)
		repoOrigins[mainRepo] = originURL

		// Get all branch notes and upstreams in one call
		notes, upstreams := GetAllBranchConfig(ctx, mainRepo)
		repoNotes[mainRepo] = notes
		repoUpstreams[mainRepo] = upstreams

		// Get merged branches in one call
		repoMerged[mainRepo] = GetMergedBranches(ctx, mainRepo)
	}

	// Phase 3: Build worktrees by merging batched data
	var worktrees []Worktree
	for _, p := range pending {
		wtMap, ok := repoWorktrees[p.mainRepo]
		if !ok {
			continue
		}
		wtInfo, ok := wtMap[p.path]
		if !ok {
			continue
		}

		branch := wtInfo.Branch
		isMerged := repoMerged[p.mainRepo][branch]

		// Get commit count if not merged
		var commitCount int
		if !isMerged && branch != "(detached)" {
			commitCount, _ = GetCommitCount(ctx, p.mainRepo, branch)
		}

		// Get note for this branch
		note := ""
		if notes, ok := repoNotes[p.mainRepo]; ok {
			note = notes[branch]
		}

		// Check upstream
		hasUpstream := false
		if upstreams, ok := repoUpstreams[p.mainRepo]; ok {
			hasUpstream = upstreams[branch]
		}

		// Get last commit time (relative and absolute)
		lastCommit, _ := GetLastCommitRelative(ctx, p.path)
		lastCommitTime, _ := GetLastCommitTime(ctx, p.path)

		// Phase 4: Only check dirty status if requested
		var isDirty bool
		if includeDirty {
			isDirty = IsDirty(ctx, p.path)
		}

		worktrees = append(worktrees, Worktree{
			Path:           p.path,
			Branch:         branch,
			MainRepo:       p.mainRepo,
			RepoName:       filepath.Base(p.mainRepo),
			OriginURL:      repoOrigins[p.mainRepo],
			IsMerged:       isMerged,
			CommitCount:    commitCount,
			IsDirty:        isDirty,
			HasUpstream:    hasUpstream,
			LastCommit:     lastCommit,
			LastCommitTime: lastCommitTime,
			Note:           note,
		})
	}

	return worktrees, nil
}

// CreateWorktreeResult contains the result of creating a worktree
type CreateWorktreeResult struct {
	Path          string
	AlreadyExists bool
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
	// Check if branch already exists
	exists, err := BranchExists(ctx, branch)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("branch %q already exists (use 'wt checkout' without -b to checkout existing branch)", branch)
	}

	// Get repo name from origin URL
	gitOrigin, err := GetRepoName(ctx)
	if err != nil {
		return nil, err
	}

	// Get folder name from disk
	folderName, err := GetRepoFolderName(ctx)
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
	// If baseRef is provided, use it as the starting point
	var args []string
	if baseRef != "" {
		args = []string{"worktree", "add", worktreePath, "-b", branch, baseRef}
	} else {
		args = []string{"worktree", "add", worktreePath, "-b", branch}
	}
	if err := runGit(ctx, "", args...); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %v", err)
	}

	return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: false}, nil
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
			// Branch already checked out - return existing path with AlreadyExists flag
			return &CreateWorktreeResult{Path: wtPath, AlreadyExists: true}, nil
		}
		// Branch exists but not checked out - use OpenWorktreeFrom instead
		return OpenWorktreeFrom(ctx, absRepoPath, basePath, branch, worktreeFmt)
	}

	// Get repo name from origin URL
	gitOrigin, err := GetRepoNameFrom(ctx, absRepoPath)
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
	// If baseRef is provided, use it as the starting point
	var args []string
	if baseRef != "" {
		args = []string{"worktree", "add", worktreePath, "-b", branch, baseRef}
	} else {
		args = []string{"worktree", "add", worktreePath, "-b", branch}
	}
	if err := runGit(ctx, absRepoPath, args...); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %v", err)
	}

	return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: false}, nil
}

// OpenWorktreeFrom creates a worktree for an existing branch in a specified repo
func OpenWorktreeFrom(ctx context.Context, absRepoPath, basePath, branch, worktreeFmt string) (*CreateWorktreeResult, error) {
	// Get repo name from origin URL
	gitOrigin, err := GetRepoNameFrom(ctx, absRepoPath)
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
	if err := runGit(ctx, absRepoPath, "worktree", "add", worktreePath, branch); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %v", err)
	}

	return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: false}, nil
}

// getBranchWorktreeFrom returns the worktree path if branch is checked out in the given repo
func getBranchWorktreeFrom(ctx context.Context, repoPath, branch string) (string, error) {
	output, err := outputGit(ctx, repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %v", err)
	}

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

// openWorktreeInternal creates a worktree for an existing local branch
func openWorktreeInternal(ctx context.Context, basePath, branch, worktreeFmt string) (*CreateWorktreeResult, error) {
	// Check if branch exists
	exists, err := BranchExists(ctx, branch)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("branch %q does not exist (use 'wt checkout -b' to create a new branch)", branch)
	}

	// Check if branch is already checked out in another worktree
	wtPath, err := GetBranchWorktree(ctx, branch)
	if err != nil {
		return nil, err
	}
	if wtPath != "" {
		return &CreateWorktreeResult{Path: wtPath, AlreadyExists: true}, nil
	}

	// Get repo name from origin URL
	gitOrigin, err := GetRepoName(ctx)
	if err != nil {
		return nil, err
	}

	// Get folder name from disk
	folderName, err := GetRepoFolderName(ctx)
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
	if err := runGit(ctx, "", "worktree", "add", worktreePath, branch); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %v", err)
	}

	return &CreateWorktreeResult{Path: worktreePath, AlreadyExists: false}, nil
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

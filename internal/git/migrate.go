package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/raphi011/wt/internal/worktree"
)

// getWorktreeMetadataName returns the worktree's metadata directory name
// by running `git rev-parse --git-dir` in the worktree.
func getWorktreeMetadataName(ctx context.Context, wtPath string) (string, error) {
	gitDir, err := outputGit(ctx, wtPath, "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	return filepath.Base(strings.TrimSpace(string(gitDir))), nil
}

// MigrationOptions configures how the migration computes worktree paths
type MigrationOptions struct {
	WorktreeFormat string // Format string for worktree paths (e.g., "{branch}", "../{repo}-{branch}")
	RepoName       string // Repository name for path resolution
}

// MigrationPlan describes what will be done during migration
type MigrationPlan struct {
	RepoPath           string // Original repo path
	GitDir             string // Current .git directory path
	CurrentBranch      string // Branch to use for main worktree
	MainBranchUpstream string // Upstream for the main branch (e.g., "main" for origin/main)
	MainWorktreePath   string // Computed path for the main worktree
	WorktreesToFix     []WorktreeMigration
	HasSubmodules      bool
}

// WorktreeMigration describes a worktree that needs to be updated
type WorktreeMigration struct {
	OldPath   string // Current worktree path
	NewPath   string // New path after migration (may be same)
	Branch    string
	Upstream  string // Upstream branch (e.g., "feature" for origin/feature)
	OldName   string // Name in .git/worktrees/
	NewName   string // Name after migration (may be same)
	NeedsMove bool   // Whether the worktree folder needs to be moved
	IsOutside bool   // Whether worktree is outside the repo directory
}

// MigrateToBareResult contains the result of a successful migration
type MigrateToBareResult struct {
	MainWorktreePath string // Path to the new main worktree (e.g., repo/main)
	GitDir           string // Path to the .git directory
}

// ValidateMigration checks if a repo can be migrated and returns the migration plan.
// The opts parameter configures how worktree paths are computed.
func ValidateMigration(ctx context.Context, repoPath string, opts MigrationOptions) (*MigrationPlan, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	// Resolve symlinks (on macOS, /tmp -> /private/tmp)
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, fmt.Errorf("resolve symlinks: %w", err)
	}

	// Check if it's a git repo
	gitDir := filepath.Join(absPath, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %s", absPath)
	}

	// Check if it's already a bare repo
	if info.IsDir() {
		if isBareRepo(gitDir) {
			return nil, fmt.Errorf("repository is already using bare-in-.git structure: %s", absPath)
		}
	} else {
		// .git is a file - this is a worktree, not a main repo
		return nil, fmt.Errorf("path is a worktree, not a repository: %s", absPath)
	}

	// Get current branch
	branch, err := GetCurrentBranch(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("get current branch: %w", err)
	}

	// Capture upstream for main branch
	mainUpstream := GetUpstreamBranch(ctx, absPath, branch)

	// Check for submodules
	submodulePath := filepath.Join(absPath, ".gitmodules")
	hasSubmodules := false
	if _, err := os.Stat(submodulePath); err == nil {
		hasSubmodules = true
	}

	if hasSubmodules {
		return nil, fmt.Errorf("repositories with submodules are not yet supported")
	}

	// List existing worktrees
	worktrees, err := ListWorktreesFromRepo(ctx, absPath)
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	// Compute main worktree path using format
	mainWorktreePath := worktree.ResolvePath(absPath, opts.RepoName, branch, opts.WorktreeFormat)

	plan := &MigrationPlan{
		RepoPath:           absPath,
		GitDir:             gitDir,
		CurrentBranch:      branch,
		MainBranchUpstream: mainUpstream,
		MainWorktreePath:   mainWorktreePath,
		HasSubmodules:      hasSubmodules,
	}

	for _, wt := range worktrees {
		// Skip the main worktree (the one at absPath)
		if wt.Path == absPath {
			continue
		}

		// Get actual metadata directory name (may differ from folder name)
		metadataName, err := getWorktreeMetadataName(ctx, wt.Path)
		if err != nil {
			return nil, fmt.Errorf("get worktree %s metadata: %w", filepath.Base(wt.Path), err)
		}

		// Compute new path based on worktree format
		newPath := worktree.ResolvePath(absPath, opts.RepoName, wt.Branch, opts.WorktreeFormat)

		// Worktree name for metadata is based on branch name (sanitized)
		newName := strings.ReplaceAll(wt.Branch, "/", "-")

		// Determine if move is needed
		needsMove := wt.Path != newPath

		// Check for conflicts at target path
		if needsMove {
			if _, err := os.Stat(newPath); err == nil {
				return nil, fmt.Errorf("target path conflict: worktree %q would be moved to %q which already exists", wt.Path, newPath)
			}
		}

		// Capture upstream for this worktree's branch
		upstream := GetUpstreamBranch(ctx, wt.Path, wt.Branch)

		plan.WorktreesToFix = append(plan.WorktreesToFix, WorktreeMigration{
			OldPath:   wt.Path,
			NewPath:   newPath,
			Branch:    wt.Branch,
			Upstream:  upstream,
			OldName:   metadataName,
			NewName:   newName,
			NeedsMove: needsMove,
			IsOutside: wt.Path != newPath, // Track if location changes
		})
	}

	return plan, nil
}

// MigrateToBare converts a regular repo to bare-in-.git format.
// This preserves all working tree files including uncommitted changes.
func MigrateToBare(ctx context.Context, plan *MigrationPlan) (*MigrateToBareResult, error) {
	repoPath := plan.RepoPath

	// Phase 1: Create temp directory for bare repo
	tempGitDir := filepath.Join(repoPath, ".git.migrating")
	if err := os.MkdirAll(tempGitDir, 0755); err != nil {
		return nil, fmt.Errorf("create temp git dir: %w", err)
	}

	// Cleanup on error
	cleanup := func() {
		os.RemoveAll(tempGitDir)
	}

	// Phase 2: Move .git contents to temp directory
	oldGitDir := filepath.Join(repoPath, ".git")
	entries, err := os.ReadDir(oldGitDir)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("read .git directory: %w", err)
	}

	for _, entry := range entries {
		oldPath := filepath.Join(oldGitDir, entry.Name())
		newPath := filepath.Join(tempGitDir, entry.Name())
		if err := os.Rename(oldPath, newPath); err != nil {
			cleanup()
			return nil, fmt.Errorf("move %s: %w", entry.Name(), err)
		}
	}

	// Remove empty old .git directory
	if err := os.Remove(oldGitDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("remove old .git directory: %w", err)
	}

	// Rename temp to .git
	if err := os.Rename(tempGitDir, oldGitDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("rename temp git dir: %w", err)
	}

	// Phase 3: Configure as bare
	if err := runGit(ctx, oldGitDir, "config", "core.bare", "true"); err != nil {
		return nil, fmt.Errorf("set core.bare: %w", err)
	}

	// Set fetch refspec (only if origin exists)
	if HasRemote(ctx, oldGitDir, "origin") {
		if err := runGit(ctx, oldGitDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set fetch refspec: %v\n", err)
		}
	}

	// Phase 4: Create main worktree directory (using computed path from plan)
	mainWorktreePath := plan.MainWorktreePath
	if err := os.MkdirAll(mainWorktreePath, 0755); err != nil {
		return nil, fmt.Errorf("create main worktree dir: %w", err)
	}

	// Phase 5: Move all files (except .git) to main worktree
	// Only needed if main worktree is nested inside repo
	if filepath.Dir(mainWorktreePath) == repoPath {
		// Main worktree is nested (e.g., repo/main)
		mainWorktreeName := filepath.Base(mainWorktreePath)

		repoEntries, err := os.ReadDir(repoPath)
		if err != nil {
			return nil, fmt.Errorf("read repo directory: %w", err)
		}

		for _, entry := range repoEntries {
			name := entry.Name()
			if name == ".git" || name == mainWorktreeName {
				continue
			}

			// Skip worktree directories that are nested inside repo
			isWorktree := false
			for _, wt := range plan.WorktreesToFix {
				if filepath.Base(wt.OldPath) == name && filepath.Dir(wt.OldPath) == repoPath {
					isWorktree = true
					break
				}
			}
			if isWorktree {
				continue
			}

			oldPath := filepath.Join(repoPath, name)
			newPath := filepath.Join(mainWorktreePath, name)
			if err := os.Rename(oldPath, newPath); err != nil {
				return nil, fmt.Errorf("move %s to worktree: %w", name, err)
			}
		}
	} else {
		// Main worktree is a sibling (e.g., ../repo-main) - copy files instead
		repoEntries, err := os.ReadDir(repoPath)
		if err != nil {
			return nil, fmt.Errorf("read repo directory: %w", err)
		}

		for _, entry := range repoEntries {
			name := entry.Name()
			if name == ".git" {
				continue
			}

			// Skip worktree directories that are siblings
			isWorktree := false
			for _, wt := range plan.WorktreesToFix {
				if filepath.Base(wt.OldPath) == name && filepath.Dir(wt.OldPath) == repoPath {
					isWorktree = true
					break
				}
			}
			if isWorktree {
				continue
			}

			oldPath := filepath.Join(repoPath, name)
			newPath := filepath.Join(mainWorktreePath, name)
			if err := os.Rename(oldPath, newPath); err != nil {
				return nil, fmt.Errorf("move %s to worktree: %w", name, err)
			}
		}
	}

	// Phase 6: Create worktree metadata for main worktree
	// Worktree metadata name is the sanitized branch name
	worktreeName := strings.ReplaceAll(plan.CurrentBranch, "/", "-")
	worktreeMetaDir := filepath.Join(oldGitDir, "worktrees", worktreeName)
	if err := os.MkdirAll(worktreeMetaDir, 0755); err != nil {
		return nil, fmt.Errorf("create worktree metadata dir: %w", err)
	}

	// Create HEAD file pointing to branch
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", plan.CurrentBranch)
	if err := os.WriteFile(filepath.Join(worktreeMetaDir, "HEAD"), []byte(headContent), 0644); err != nil {
		return nil, fmt.Errorf("write HEAD: %w", err)
	}

	// Create gitdir file pointing back to worktree (absolute path)
	gitdirPath := filepath.Join(mainWorktreePath, ".git")
	if err := os.WriteFile(filepath.Join(worktreeMetaDir, "gitdir"), []byte(gitdirPath+"\n"), 0644); err != nil {
		return nil, fmt.Errorf("write gitdir: %w", err)
	}

	// Create commondir file (points from .git/worktrees/<name>/ back to .git/)
	if err := os.WriteFile(filepath.Join(worktreeMetaDir, "commondir"), []byte("../..\n"), 0644); err != nil {
		return nil, fmt.Errorf("write commondir: %w", err)
	}

	// Move index file to worktree metadata (preserves staging area)
	indexSrc := filepath.Join(oldGitDir, "index")
	indexDst := filepath.Join(worktreeMetaDir, "index")
	if _, err := os.Stat(indexSrc); err == nil {
		if err := os.Rename(indexSrc, indexDst); err != nil {
			return nil, fmt.Errorf("move index: %w", err)
		}
	}

	// Create logs directory for reflog
	logsDir := filepath.Join(worktreeMetaDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}

	// Phase 7: Create .git file in main worktree pointing to bare repo
	// Compute relative path from worktree to .git/worktrees/<name>
	relPath, err := filepath.Rel(mainWorktreePath, worktreeMetaDir)
	if err != nil {
		// Fall back to absolute path if relative path computation fails
		relPath = worktreeMetaDir
	}
	gitFileContent := fmt.Sprintf("gitdir: %s\n", relPath)
	if err := os.WriteFile(gitdirPath, []byte(gitFileContent), 0644); err != nil {
		return nil, fmt.Errorf("write .git file: %w", err)
	}

	// Phase 8: Update existing worktrees
	for _, wt := range plan.WorktreesToFix {
		if err := updateWorktreeLinks(ctx, repoPath, wt); err != nil {
			return nil, fmt.Errorf("update worktree %s: %w", wt.OldName, err)
		}
	}

	// Phase 9: Repair worktrees
	if err := runGit(ctx, oldGitDir, "worktree", "repair"); err != nil {
		// Log warning but don't fail - worktrees may have issues but might still work
		fmt.Fprintf(os.Stderr, "Warning: worktree repair had issues (may need manual repair): %v\n", err)
	}

	// Phase 10: Restore upstream tracking
	// Only restore if origin remote exists and the remote branch exists
	if plan.MainBranchUpstream != "" && HasRemote(ctx, oldGitDir, "origin") {
		if RemoteBranchExists(ctx, mainWorktreePath, plan.MainBranchUpstream) {
			if err := SetUpstreamBranch(ctx, mainWorktreePath, plan.CurrentBranch, plan.MainBranchUpstream); err != nil {
				fmt.Fprintf(os.Stderr, "Note: could not restore upstream tracking for %s: %v\n", plan.CurrentBranch, err)
			}
		}
	}

	for _, wt := range plan.WorktreesToFix {
		if wt.Upstream != "" {
			if RemoteBranchExists(ctx, wt.NewPath, wt.Upstream) {
				if err := SetUpstreamBranch(ctx, wt.NewPath, wt.Branch, wt.Upstream); err != nil {
					fmt.Fprintf(os.Stderr, "Note: could not restore upstream tracking for %s: %v\n", wt.Branch, err)
				}
			}
		}
	}

	return &MigrateToBareResult{
		MainWorktreePath: mainWorktreePath,
		GitDir:           oldGitDir,
	}, nil
}

// updateWorktreeLinks updates a worktree's links to point to the new bare repo location
func updateWorktreeLinks(_ context.Context, repoPath string, wt WorktreeMigration) error {
	gitDir := filepath.Join(repoPath, ".git")

	// Move worktree folder if needed (prefix stripping)
	if wt.NeedsMove {
		if err := os.Rename(wt.OldPath, wt.NewPath); err != nil {
			return fmt.Errorf("move worktree: %w", err)
		}
	}

	// Update .git file in worktree to point to new location
	wtPath := wt.NewPath
	relPath, err := filepath.Rel(wtPath, filepath.Join(gitDir, "worktrees", wt.NewName))
	if err != nil {
		// Fall back to absolute path
		relPath = filepath.Join(gitDir, "worktrees", wt.NewName)
	}

	gitFileContent := fmt.Sprintf("gitdir: %s\n", relPath)
	if err := os.WriteFile(filepath.Join(wtPath, ".git"), []byte(gitFileContent), 0644); err != nil {
		return fmt.Errorf("update .git file: %w", err)
	}

	// Update worktree metadata if name changed
	if wt.OldName != wt.NewName {
		oldMetaDir := filepath.Join(gitDir, "worktrees", wt.OldName)
		newMetaDir := filepath.Join(gitDir, "worktrees", wt.NewName)

		if err := os.Rename(oldMetaDir, newMetaDir); err != nil {
			return fmt.Errorf("rename worktree metadata: %w", err)
		}

		// Update gitdir in metadata
		gitdirPath := filepath.Join(wtPath, ".git")
		if err := os.WriteFile(filepath.Join(newMetaDir, "gitdir"), []byte(gitdirPath+"\n"), 0644); err != nil {
			return fmt.Errorf("update gitdir in metadata: %w", err)
		}
	} else {
		// Just update gitdir in existing metadata
		metaDir := filepath.Join(gitDir, "worktrees", wt.OldName)
		gitdirPath := filepath.Join(wtPath, ".git")
		if err := os.WriteFile(filepath.Join(metaDir, "gitdir"), []byte(gitdirPath+"\n"), 0644); err != nil {
			return fmt.Errorf("update gitdir in metadata: %w", err)
		}
	}

	return nil
}

// StripRepoPrefix removes the "repo-" prefix from worktree names if present.
// For example, if repoName is "myapp" and wtName is "myapp-feature", returns "feature".
func StripRepoPrefix(repoName, wtName string) string {
	prefix := repoName + "-"
	if strings.HasPrefix(wtName, prefix) {
		return strings.TrimPrefix(wtName, prefix)
	}
	return wtName
}

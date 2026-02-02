package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// MigrationPlan describes what will be done during migration
type MigrationPlan struct {
	RepoPath       string // Original repo path
	GitDir         string // Current .git directory path
	CurrentBranch  string // Branch to use for main worktree
	WorktreesToFix []WorktreeMigration
	HasSubmodules  bool
}

// WorktreeMigration describes a worktree that needs to be updated
type WorktreeMigration struct {
	OldPath   string // Current worktree path
	NewPath   string // New path after migration (may be same)
	Branch    string
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

// ValidateMigration checks if a repo can be migrated and returns the migration plan
func ValidateMigration(ctx context.Context, repoPath string) (*MigrationPlan, error) {
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

	plan := &MigrationPlan{
		RepoPath:      absPath,
		GitDir:        gitDir,
		CurrentBranch: branch,
		HasSubmodules: hasSubmodules,
	}

	repoName := filepath.Base(absPath)

	for _, wt := range worktrees {
		// Skip the main worktree (the one at absPath)
		if wt.Path == absPath {
			continue
		}

		wtName := filepath.Base(wt.Path)
		newName := StripRepoPrefix(repoName, wtName)

		// Get actual metadata directory name (may differ from folder name)
		metadataName, err := getWorktreeMetadataName(ctx, wt.Path)
		if err != nil {
			return nil, fmt.Errorf("get worktree %s metadata: %w", wtName, err)
		}

		// Check if worktree is outside repo directory
		wtParent := filepath.Dir(wt.Path)
		repoParent := filepath.Dir(absPath)
		isOutside := wtParent != repoParent

		// Compute new path
		newPath := wt.Path
		needsMove := false
		if !isOutside && wtName != newName {
			// Worktree is a sibling and needs renaming
			newPath = filepath.Join(wtParent, newName)
			needsMove = true
		}

		// Check for name conflicts after stripping prefix
		if needsMove {
			if _, err := os.Stat(newPath); err == nil {
				return nil, fmt.Errorf("name conflict: worktree %q would be renamed to %q which already exists", wtName, newName)
			}
		}

		plan.WorktreesToFix = append(plan.WorktreesToFix, WorktreeMigration{
			OldPath:   wt.Path,
			NewPath:   newPath,
			Branch:    wt.Branch,
			OldName:   metadataName,
			NewName:   newName,
			NeedsMove: needsMove,
			IsOutside: isOutside,
		})
	}

	return plan, nil
}

// MigrateToBare converts a regular repo to bare-in-.git format.
// This preserves all working tree files including uncommitted changes.
func MigrateToBare(ctx context.Context, plan *MigrationPlan) (*MigrateToBareResult, error) {
	repoPath := plan.RepoPath
	repoName := filepath.Base(repoPath)

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

	// Set fetch refspec
	if err := runGit(ctx, oldGitDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		// Not fatal - repo may not have origin
	}

	// Phase 4: Create main worktree directory
	mainWorktreePath := filepath.Join(repoPath, plan.CurrentBranch)
	if err := os.MkdirAll(mainWorktreePath, 0755); err != nil {
		return nil, fmt.Errorf("create main worktree dir: %w", err)
	}

	// Phase 5: Move all files (except .git) to main worktree
	repoEntries, err := os.ReadDir(repoPath)
	if err != nil {
		return nil, fmt.Errorf("read repo directory: %w", err)
	}

	for _, entry := range repoEntries {
		name := entry.Name()
		if name == ".git" || name == plan.CurrentBranch {
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

	// Phase 6: Create worktree metadata for main worktree
	worktreeName := plan.CurrentBranch
	worktreeMetaDir := filepath.Join(oldGitDir, "worktrees", worktreeName)
	if err := os.MkdirAll(worktreeMetaDir, 0755); err != nil {
		return nil, fmt.Errorf("create worktree metadata dir: %w", err)
	}

	// Create HEAD file pointing to branch
	headContent := fmt.Sprintf("ref: refs/heads/%s\n", plan.CurrentBranch)
	if err := os.WriteFile(filepath.Join(worktreeMetaDir, "HEAD"), []byte(headContent), 0644); err != nil {
		return nil, fmt.Errorf("write HEAD: %w", err)
	}

	// Create gitdir file pointing back to worktree
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
	gitFileContent := fmt.Sprintf("gitdir: ../.git/worktrees/%s\n", worktreeName)
	if err := os.WriteFile(gitdirPath, []byte(gitFileContent), 0644); err != nil {
		return nil, fmt.Errorf("write .git file: %w", err)
	}

	// Phase 8: Update existing worktrees
	for _, wt := range plan.WorktreesToFix {
		if err := updateWorktreeLinks(ctx, repoPath, repoName, wt); err != nil {
			return nil, fmt.Errorf("update worktree %s: %w", wt.OldName, err)
		}
	}

	// Phase 9: Repair worktrees
	if err := runGit(ctx, oldGitDir, "worktree", "repair"); err != nil {
		// Log warning but don't fail
		// This can happen if worktrees have issues, but they might still work
	}

	return &MigrateToBareResult{
		MainWorktreePath: mainWorktreePath,
		GitDir:           oldGitDir,
	}, nil
}

// updateWorktreeLinks updates a worktree's links to point to the new bare repo location
func updateWorktreeLinks(_ context.Context, repoPath, _ string, wt WorktreeMigration) error {
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

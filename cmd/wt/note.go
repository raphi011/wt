package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/resolve"
)

// resolveNoteTarget resolves the branch and repo path for note operations.
// If target is empty and inside a worktree, uses the current branch.
// Otherwise, uses the shared resolver to look up by ID or branch name.
// Returns branch name, repo path, and any error.
func resolveNoteTarget(target string, dir string) (branch string, repoPath string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current directory: %w", err)
	}

	inWorktree := git.IsWorktree(cwd)

	// If no target provided and inside a worktree, use current branch
	if target == "" {
		if !inWorktree {
			return "", "", fmt.Errorf("target required when not inside a worktree (run 'wt list' to see IDs)")
		}

		branch, err = git.GetCurrentBranch(cwd)
		if err != nil {
			return "", "", fmt.Errorf("failed to get current branch: %w", err)
		}

		repoPath, err = git.GetMainRepoPath(cwd)
		if err != nil {
			return "", "", fmt.Errorf("failed to get main repo path: %w", err)
		}

		return branch, repoPath, nil
	}

	// Use unified resolver to handle both IDs and branch names
	scanPath := dir
	if scanPath == "" {
		scanPath = "."
	}
	scanPath, err = filepath.Abs(scanPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	resolved, err := resolve.ByIDOrBranch(target, scanPath)
	if err != nil {
		return "", "", err
	}

	return resolved.Branch, resolved.MainRepo, nil
}

func runNoteSet(cmd *NoteSetCmd) error {
	branch, repoPath, err := resolveNoteTarget(cmd.Target, cmd.Dir)
	if err != nil {
		return err
	}
	if err := git.SetBranchNote(repoPath, branch, cmd.Text); err != nil {
		return fmt.Errorf("failed to set note: %w", err)
	}
	repoName := filepath.Base(repoPath)
	fmt.Printf("Note set on %s/%s\n", repoName, branch)
	return nil
}

func runNoteGet(cmd *NoteGetCmd) error {
	branch, repoPath, err := resolveNoteTarget(cmd.Target, cmd.Dir)
	if err != nil {
		return err
	}
	note, err := git.GetBranchNote(repoPath, branch)
	if err != nil {
		return fmt.Errorf("failed to get note: %w", err)
	}
	if note != "" {
		fmt.Println(note)
	}
	return nil
}

func runNoteClear(cmd *NoteClearCmd) error {
	branch, repoPath, err := resolveNoteTarget(cmd.Target, cmd.Dir)
	if err != nil {
		return err
	}
	if err := git.ClearBranchNote(repoPath, branch); err != nil {
		return fmt.Errorf("failed to clear note: %w", err)
	}
	repoName := filepath.Base(repoPath)
	fmt.Printf("Note cleared from %s/%s\n", repoName, branch)
	return nil
}

package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/resolve"
)

func runNoteSet(cmd *NoteSetCmd, cfg *config.Config, workDir string, out io.Writer) error {
	target, err := resolveNoteTarget(cmd.ID, cmd.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	if err := git.SetBranchNote(target.MainRepo, target.Branch, cmd.Text); err != nil {
		return fmt.Errorf("failed to set note: %w", err)
	}
	repoName := filepath.Base(target.MainRepo)
	fmt.Fprintf(out, "Note set on %s/%s\n", repoName, target.Branch)
	return nil
}

func runNoteGet(cmd *NoteGetCmd, cfg *config.Config, workDir string, out io.Writer) error {
	target, err := resolveNoteTarget(cmd.ID, cmd.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	note, err := git.GetBranchNote(target.MainRepo, target.Branch)
	if err != nil {
		return fmt.Errorf("failed to get note: %w", err)
	}
	if note != "" {
		fmt.Fprintln(out, note)
	}
	return nil
}

func runNoteClear(cmd *NoteClearCmd, cfg *config.Config, workDir string, out io.Writer) error {
	target, err := resolveNoteTarget(cmd.ID, cmd.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	if err := git.ClearBranchNote(target.MainRepo, target.Branch); err != nil {
		return fmt.Errorf("failed to clear note: %w", err)
	}
	repoName := filepath.Base(target.MainRepo)
	fmt.Fprintf(out, "Note cleared from %s/%s\n", repoName, target.Branch)
	return nil
}

// resolveNoteTarget resolves target for note commands with 3 modes:
// 1. --id: by worktree ID
// 2. -r: by repository name
// 3. neither: use workDir (worktree or main repo)
func resolveNoteTarget(id int, repository string, cfg *config.Config, workDir string) (*resolve.Target, error) {
	if id != 0 {
		scanPath, err := cfg.GetAbsWorktreeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		return resolve.ByID(id, scanPath)
	}

	if repository != "" {
		repoScanDir := cfg.RepoScanDir()
		if repoScanDir == "" {
			var err error
			repoScanDir, err = cfg.GetAbsWorktreeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
			}
		}
		return resolve.ByRepoName(repository, repoScanDir)
	}

	return resolve.FromWorktreeOrRepoPath(workDir)
}

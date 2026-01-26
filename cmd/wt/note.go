package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/resolve"
)

func (c *NoteSetCmd) runNoteSet(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	out := output.FromContext(ctx).Writer()
	target, err := resolveNoteTarget(ctx, c.ID, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	if err := git.SetBranchNote(ctx, target.MainRepo, target.Branch, c.Text); err != nil {
		return fmt.Errorf("failed to set note: %w", err)
	}
	repoName := filepath.Base(target.MainRepo)
	fmt.Fprintf(out, "Note set on %s/%s\n", repoName, target.Branch)
	return nil
}

func (c *NoteGetCmd) runNoteGet(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	out := output.FromContext(ctx).Writer()
	target, err := resolveNoteTarget(ctx, c.ID, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	note, err := git.GetBranchNote(ctx, target.MainRepo, target.Branch)
	if err != nil {
		return fmt.Errorf("failed to get note: %w", err)
	}
	if note != "" {
		fmt.Fprintln(out, note)
	}
	return nil
}

func (c *NoteClearCmd) runNoteClear(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	out := output.FromContext(ctx).Writer()
	target, err := resolveNoteTarget(ctx, c.ID, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	if err := git.ClearBranchNote(ctx, target.MainRepo, target.Branch); err != nil {
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
func resolveNoteTarget(ctx context.Context, id int, repository string, cfg *config.Config, workDir string) (*resolve.Target, error) {
	if id != 0 {
		worktreeDir, err := cfg.GetAbsWorktreeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		return resolve.ByID(id, worktreeDir)
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
		return resolve.ByRepoName(ctx, repository, repoScanDir)
	}

	return resolve.FromWorktreeOrRepoPath(ctx, workDir)
}

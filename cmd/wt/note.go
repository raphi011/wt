package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/resolve"
)

func (c *NoteSetCmd) runNoteSet(ctx context.Context) error {
	cfg := c.Config
	worktreeDir, _ := cfg.GetAbsWorktreeDir()
	target, err := resolve.ByIDOrRepoOrPath(ctx, c.ID, c.Repository, worktreeDir, cfg.RepoScanDir(), c.WorkDir)
	if err != nil {
		return err
	}

	if err := git.SetBranchNote(ctx, target.MainRepo, target.Branch, c.Text); err != nil {
		return fmt.Errorf("failed to set note: %w", err)
	}
	repoName := filepath.Base(target.MainRepo)
	output.FromContext(ctx).Printf("Note set on %s/%s\n", repoName, target.Branch)
	return nil
}

func (c *NoteGetCmd) runNoteGet(ctx context.Context) error {
	cfg := c.Config
	worktreeDir, _ := cfg.GetAbsWorktreeDir()
	target, err := resolve.ByIDOrRepoOrPath(ctx, c.ID, c.Repository, worktreeDir, cfg.RepoScanDir(), c.WorkDir)
	if err != nil {
		return err
	}

	note, err := git.GetBranchNote(ctx, target.MainRepo, target.Branch)
	if err != nil {
		return fmt.Errorf("failed to get note: %w", err)
	}
	if note != "" {
		output.FromContext(ctx).Println(note)
	}
	return nil
}

func (c *NoteClearCmd) runNoteClear(ctx context.Context) error {
	cfg := c.Config
	worktreeDir, _ := cfg.GetAbsWorktreeDir()
	target, err := resolve.ByIDOrRepoOrPath(ctx, c.ID, c.Repository, worktreeDir, cfg.RepoScanDir(), c.WorkDir)
	if err != nil {
		return err
	}

	if err := git.ClearBranchNote(ctx, target.MainRepo, target.Branch); err != nil {
		return fmt.Errorf("failed to clear note: %w", err)
	}
	repoName := filepath.Base(target.MainRepo)
	output.FromContext(ctx).Printf("Note cleared from %s/%s\n", repoName, target.Branch)
	return nil
}

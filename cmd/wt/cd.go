package main

import (
	"context"
	"fmt"
	"os"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/resolve"
)

func (c *CdCmd) runCd(ctx context.Context) error {
	// Determine targeting mode
	hasID := c.ID != 0
	hasRepo := c.Repository != ""
	hasLabel := c.Label != ""

	if !hasID && !hasRepo && !hasLabel {
		return fmt.Errorf("specify target: -i <id>, -r <repo>, or -l <label>")
	}

	// Mode: by label (no hooks for label mode)
	if hasLabel {
		return c.runCdForLabel(ctx)
	}

	// Mode: by repo name (no hooks for repo mode)
	if hasRepo {
		return c.runCdForRepo(ctx)
	}

	// Mode: by ID (worktree)
	return c.runCdForID(ctx)
}

func (c *CdCmd) runCdForID(ctx context.Context) error {
	cfg := c.Config
	worktreeDir, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	target, err := resolve.ByID(c.ID, worktreeDir)
	if err != nil {
		return err
	}

	path := target.Path
	if c.Project {
		path = target.MainRepo
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path no longer exists: %s", path)
	}

	output.FromContext(ctx).Println(path)

	// Run hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, c.Hook, c.NoHook, hooks.CommandCd)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		env, err := hooks.ParseEnvWithStdin(c.Env)
		if err != nil {
			return err
		}

		hookCtx := hooks.ContextFromWorktree(target, hooks.CommandCd, env)
		return hooks.RunAll(hookMatches, hookCtx)
	}

	return nil
}

func (c *CdCmd) runCdForRepo(ctx context.Context) error {
	cfg := c.Config
	worktreeDir, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	repoDir, err := resolveRepoDir(worktreeDir, cfg.RepoScanDir())
	if err != nil {
		return err
	}

	repoPath, err := git.FindRepoByName(repoDir, c.Repository)
	if err != nil {
		return err
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("path no longer exists: %s", repoPath)
	}

	output.FromContext(ctx).Println(repoPath)
	return nil
}

func (c *CdCmd) runCdForLabel(ctx context.Context) error {
	cfg := c.Config
	worktreeDir, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	repoDir, err := resolveRepoDir(worktreeDir, cfg.RepoScanDir())
	if err != nil {
		return err
	}

	repos, err := git.FindReposByLabel(ctx, repoDir, c.Label)
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		return fmt.Errorf("no repos found with label %q", c.Label)
	}

	if len(repos) > 1 {
		return fmt.Errorf("multiple repos match label %q (use -r to specify)", c.Label)
	}

	repoPath := repos[0]
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("path no longer exists: %s", repoPath)
	}

	output.FromContext(ctx).Println(repoPath)
	return nil
}

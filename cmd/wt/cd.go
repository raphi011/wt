package main

import (
	"fmt"
	"io"
	"os"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/resolve"
)

func runCd(cmd *CdCmd, cfg *config.Config, _ string, out io.Writer) error {
	// Determine targeting mode
	hasID := cmd.ID != 0
	hasRepo := cmd.Repository != ""
	hasLabel := cmd.Label != ""

	if !hasID && !hasRepo && !hasLabel {
		return fmt.Errorf("specify target: -i <id>, -r <repo>, or -l <label>")
	}

	scanPath, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Mode: by label (no hooks for label mode)
	if hasLabel {
		return runCdForLabel(cmd.Label, scanPath, cfg, out)
	}

	// Mode: by repo name (no hooks for repo mode)
	if hasRepo {
		return runCdForRepo(cmd.Repository, scanPath, cfg, out)
	}

	// Mode: by ID (worktree)
	return runCdForID(cmd, cfg, scanPath, out)
}

func runCdForID(cmd *CdCmd, cfg *config.Config, scanPath string, out io.Writer) error {

	target, err := resolve.ByID(cmd.ID, scanPath)
	if err != nil {
		return err
	}

	path := target.Path
	if cmd.Project {
		path = target.MainRepo
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path no longer exists: %s", path)
	}

	fmt.Fprintln(out, path)

	// Run hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandCd)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		env, err := hooks.ParseEnvWithStdin(cmd.Env)
		if err != nil {
			return err
		}

		ctx := hooks.ContextFromWorktree(target, hooks.CommandCd, env)
		return hooks.RunAll(hookMatches, ctx)
	}

	return nil
}

func runCdForRepo(repoName string, dir string, cfg *config.Config, out io.Writer) error {
	scanDir, err := resolveRepoScanDir(dir, cfg)
	if err != nil {
		return err
	}

	repoPath, err := git.FindRepoByName(scanDir, repoName)
	if err != nil {
		return err
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("path no longer exists: %s", repoPath)
	}

	fmt.Fprintln(out, repoPath)
	return nil
}

func runCdForLabel(label string, dir string, cfg *config.Config, out io.Writer) error {
	scanDir, err := resolveRepoScanDir(dir, cfg)
	if err != nil {
		return err
	}

	repos, err := git.FindReposByLabel(scanDir, label)
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		return fmt.Errorf("no repos found with label %q", label)
	}

	if len(repos) > 1 {
		return fmt.Errorf("multiple repos match label %q (use -r to specify)", label)
	}

	repoPath := repos[0]
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("path no longer exists: %s", repoPath)
	}

	fmt.Fprintln(out, repoPath)
	return nil
}

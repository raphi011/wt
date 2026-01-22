package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/resolve"
)

func runCd(cmd *CdCmd, cfg *config.Config) error {
	// Determine targeting mode
	hasID := cmd.ID != 0
	hasRepo := cmd.Repository != ""

	if !hasID && !hasRepo {
		return fmt.Errorf("specify target: -i <id> or -r <repo>")
	}

	// Mode: by repo name (no hooks for repo mode)
	if hasRepo {
		return runCdForRepo(cmd.Repository, cmd.Dir, cfg)
	}

	// Mode: by ID (worktree)
	return runCdForID(cmd, cfg)
}

func runCdForID(cmd *CdCmd, cfg *config.Config) error {
	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	scanPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

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

	fmt.Println(path)

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

		ctx := hooks.Context{
			Path:     target.Path,
			Branch:   target.Branch,
			MainRepo: target.MainRepo,
			Folder:   filepath.Base(target.MainRepo),
			Trigger:  string(hooks.CommandCd),
			Env:      env,
		}
		ctx.Repo, _ = git.GetRepoNameFrom(target.MainRepo)

		return hooks.RunAll(hookMatches, ctx)
	}

	return nil
}

func runCdForRepo(repoName string, dir string, cfg *config.Config) error {
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

	fmt.Println(repoPath)
	return nil
}

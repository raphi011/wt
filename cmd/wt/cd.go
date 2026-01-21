package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/resolve"
)

func runCd(cmd *CdCmd, cfg *config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Determine targeting mode
	hasID := cmd.ID != 0
	hasRepo := cmd.Repository != ""

	if !hasID && !hasRepo {
		return fmt.Errorf("specify target: -i <id> or -r <repo>")
	}

	// Mode: by repo name
	if hasRepo {
		return runCdForRepo(cmd.Repository, cmd.Dir, cfg)
	}

	// Mode: by ID (worktree)
	return runCdForID(cmd.ID, cmd.Dir, cmd.Project)
}

func runCdForID(id int, dir string, project bool) error {
	if dir == "" {
		dir = "."
	}

	scanPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	target, err := resolve.ByID(id, scanPath)
	if err != nil {
		return err
	}

	path := target.Path
	if project {
		path = target.MainRepo
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path no longer exists: %s", path)
	}

	fmt.Println(path)
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

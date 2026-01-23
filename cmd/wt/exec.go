package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/resolve"
)

func runExec(cmd *ExecCmd, cfg *config.Config) error {
	// Strip leading "--" if present (kong passthrough includes it)
	command := cmd.Command
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}

	// Validate command is provided
	if len(command) == 0 {
		return fmt.Errorf("no command specified (use: wt exec -i <id> -- <command>)")
	}

	// Determine targeting mode
	hasID := len(cmd.ID) > 0
	hasRepo := len(cmd.Repository) > 0 || len(cmd.Label) > 0

	if !hasID && !hasRepo {
		return fmt.Errorf("specify target: -i <id>, -r <repo>, or -l <label>")
	}

	scanPath, err := cfg.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Mode: by ID (worktrees)
	if hasID {
		return runExecForIDs(cmd.ID, command, scanPath)
	}

	// Mode: by repo/label
	return runExecForRepos(cmd.Repository, cmd.Label, command, scanPath, cfg)
}

func runExecForIDs(ids []int, command []string, scanPath string) error {

	var errs []error
	for _, id := range ids {
		if err := runExecForID(id, command, scanPath); err != nil {
			errs = append(errs, fmt.Errorf("ID %d: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to execute in some worktrees:\n%w", errors.Join(errs...))
	}
	return nil
}

func runExecForRepos(repos []string, labels []string, command []string, dir string, cfg *config.Config) error {
	scanDir, err := resolveRepoScanDir(dir, cfg)
	if err != nil {
		return err
	}

	repoPaths, errs := collectRepoPaths(repos, labels, scanDir, cfg)

	// Execute command in each repo
	for repoPath := range repoPaths {
		repoName := git.GetRepoDisplayName(repoPath)
		if err := runCommandInDir(command, repoPath); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to execute in some repos:\n%w", errors.Join(errs...))
	}
	return nil
}

func runCommandInDir(command []string, dir string) error {
	c := exec.Command(command[0], command[1:]...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

func runExecForID(id int, command []string, scanPath string) error {
	// Resolve target by ID
	target, err := resolve.ByID(id, scanPath)
	if err != nil {
		return err
	}

	// Validate path still exists
	if _, err := os.Stat(target.Path); os.IsNotExist(err) {
		return fmt.Errorf("worktree path no longer exists: %s (run 'wt list' to refresh)", target.Path)
	}

	return runCommandInDir(command, target.Path)
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/resolve"
)

func runExec(cmd *ExecCmd) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	scanPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Validate command is provided
	if len(cmd.Command) == 0 {
		return fmt.Errorf("no command specified (use: wt exec <id|branch> -- <command>)")
	}

	// Resolve target by ID or branch name
	target, err := resolve.ByIDOrBranch(cmd.Target, scanPath)
	if err != nil {
		return err
	}

	// Validate path still exists
	if _, err := os.Stat(target.Path); os.IsNotExist(err) {
		return fmt.Errorf("worktree path no longer exists: %s (run 'wt list' to refresh)", target.Path)
	}

	// Execute command in worktree directory
	c := exec.Command(cmd.Command[0], cmd.Command[1:]...)
	c.Dir = target.Path
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin

	return c.Run()
}

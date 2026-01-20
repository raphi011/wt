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

	// Strip leading "--" if present (kong passthrough includes it)
	command := cmd.Command
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}

	// Validate command is provided
	if len(command) == 0 {
		return fmt.Errorf("no command specified (use: wt exec -i <id> -- <command>)")
	}

	// Execute for each ID
	var errs []error
	for _, id := range cmd.ID {
		if err := runExecForID(id, command, scanPath); err != nil {
			errs = append(errs, fmt.Errorf("ID %d: %w", id, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to execute in some worktrees:\n%w", joinErrors(errs))
	}
	return nil
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

	// Execute command in worktree directory
	c := exec.Command(command[0], command[1:]...)
	c.Dir = target.Path
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin

	return c.Run()
}

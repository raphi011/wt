package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/resolve"
)

func runCd(cmd *CdCmd) error {
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
	return nil
}

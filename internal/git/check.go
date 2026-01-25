package git

import (
	"context"
	"fmt"
	"os/exec"
)

// ErrGitNotFound indicates git is not installed or not in PATH
var ErrGitNotFound = fmt.Errorf("git not found: please install git (https://git-scm.com)")

// CheckGit verifies that git is available in PATH
func CheckGit() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return ErrGitNotFound
	}
	return nil
}

// IsInsideRepo returns true if the current working directory is inside a git repository
func IsInsideRepo(ctx context.Context) bool {
	err := runGit(ctx, "", "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// IsInsideRepoPath returns true if the given path is inside a git repository
func IsInsideRepoPath(ctx context.Context, path string) bool {
	err := runGit(ctx, path, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

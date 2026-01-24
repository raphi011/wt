package git

import (
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
func IsInsideRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	err := runCmd(cmd)
	return err == nil
}

// IsInsideRepoPath returns true if the given path is inside a git repository
func IsInsideRepoPath(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	err := runCmd(cmd)
	return err == nil
}

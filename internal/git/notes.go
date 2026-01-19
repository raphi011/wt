package git

import (
	"bytes"
	"os/exec"
	"strings"
)

// GetBranchNote returns the note (description) for a branch
// Returns empty string if no note is set
func GetBranchNote(repoPath, branch string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "config", "branch."+branch+".description")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Exit code 1 means the config key doesn't exist - not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// SetBranchNote sets a note (description) on a branch
func SetBranchNote(repoPath, branch, note string) error {
	cmd := exec.Command("git", "-C", repoPath, "config", "branch."+branch+".description", note)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return err
		}
		return err
	}

	return nil
}

// ClearBranchNote removes the note (description) from a branch
func ClearBranchNote(repoPath, branch string) error {
	cmd := exec.Command("git", "-C", repoPath, "config", "--unset", "branch."+branch+".description")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Exit code 5 means the key doesn't exist - not an error for clearing
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 5 {
			return nil
		}
		return err
	}

	return nil
}

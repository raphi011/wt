package git

import (
	"os/exec"
	"strings"
)

// GetBranchNote returns the note (description) for a branch
// Returns empty string if no note is set
func GetBranchNote(repoPath, branch string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "config", "branch."+branch+".description")
	output, err := outputCmd(cmd)
	if err != nil {
		// Exit code 1 means the config key doesn't exist - not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// SetBranchNote sets a note (description) on a branch
func SetBranchNote(repoPath, branch, note string) error {
	cmd := exec.Command("git", "-C", repoPath, "config", "branch."+branch+".description", note)
	return runCmd(cmd)
}

// ClearBranchNote removes the note (description) from a branch
func ClearBranchNote(repoPath, branch string) error {
	cmd := exec.Command("git", "-C", repoPath, "config", "--unset", "branch."+branch+".description")
	if err := runCmd(cmd); err != nil {
		// Exit code 5 means the key doesn't exist - not an error for clearing
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 5 {
			return nil
		}
		return err
	}
	return nil
}

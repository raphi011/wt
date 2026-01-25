package git

import (
	"context"
	"os/exec"
	"strings"
)

// GetBranchNote returns the note (description) for a branch
// Returns empty string if no note is set
func GetBranchNote(ctx context.Context, repoPath, branch string) (string, error) {
	output, err := outputGit(ctx, repoPath, "config", "branch."+branch+".description")
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
func SetBranchNote(ctx context.Context, repoPath, branch, note string) error {
	return runGit(ctx, repoPath, "config", "branch."+branch+".description", note)
}

// ClearBranchNote removes the note (description) from a branch
func ClearBranchNote(ctx context.Context, repoPath, branch string) error {
	if err := runGit(ctx, repoPath, "config", "--unset", "branch."+branch+".description"); err != nil {
		// Exit code 5 means the key doesn't exist - not an error for clearing
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 5 {
			return nil
		}
		return err
	}
	return nil
}

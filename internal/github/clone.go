package github

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneRepo clones a GitHub repo using gh CLI.
// repoSpec is "org/repo", destPath is where to clone.
// Returns the full path to the cloned repo.
func CloneRepo(repoSpec, destPath string) (string, error) {
	// Extract repo name for the destination folder
	parts := strings.Split(repoSpec, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo spec %q: expected org/repo format", repoSpec)
	}
	repoName := parts[1]
	clonePath := filepath.Join(destPath, repoName)

	cmd := exec.Command("gh", "repo", "clone", repoSpec, clonePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("gh repo clone failed: %s", errMsg)
		}
		return "", fmt.Errorf("gh repo clone failed: %w", err)
	}

	return clonePath, nil
}

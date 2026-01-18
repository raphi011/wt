package github

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ErrGHNotFound indicates gh CLI is not installed or not in PATH
var ErrGHNotFound = fmt.Errorf("gh not found: please install GitHub CLI (https://cli.github.com)")

// ErrGHNotAuthenticated indicates gh CLI is installed but not authenticated
var ErrGHNotAuthenticated = fmt.Errorf("gh not authenticated: please run 'gh auth login'")

// CheckGH verifies that gh CLI is available and authenticated
func CheckGH() error {
	// Check if gh is in PATH
	_, err := exec.LookPath("gh")
	if err != nil {
		return ErrGHNotFound
	}

	// Check authentication status
	cmd := exec.Command("gh", "auth", "status")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// gh auth status exits non-zero when not authenticated
		errMsg := strings.TrimSpace(stderr.String())
		if strings.Contains(errMsg, "not logged") || strings.Contains(errMsg, "no accounts") {
			return ErrGHNotAuthenticated
		}
		// Other auth issues
		if errMsg != "" {
			return fmt.Errorf("gh auth check failed: %s", errMsg)
		}
		return ErrGHNotAuthenticated
	}

	return nil
}

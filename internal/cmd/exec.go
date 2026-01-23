// Package cmd provides helpers for executing shell commands with proper error handling.
package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes a command and returns stderr in the error message if it fails
func Run(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errMsg := strings.TrimSpace(stderr.String()); errMsg != "" {
			return fmt.Errorf("%s", errMsg)
		}
		return err
	}
	return nil
}

// Output executes a command and returns stdout, with stderr in error if it fails
func Output(cmd *exec.Cmd) ([]byte, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if errMsg := strings.TrimSpace(stderr.String()); errMsg != "" {
			return nil, fmt.Errorf("%s", errMsg)
		}
		return nil, err
	}
	return output, nil
}

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/raphi011/wt/internal/log"
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

// RunContext executes a command with context support and verbose logging.
func RunContext(ctx context.Context, dir, name string, args ...string) error {
	log.FromContext(ctx).Command(name, args...)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errMsg := strings.TrimSpace(stderr.String()); errMsg != "" {
			return fmt.Errorf("%s", errMsg)
		}
		return err
	}
	return nil
}

// OutputContext executes a command with context support and verbose logging,
// returning stdout.
func OutputContext(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	log.FromContext(ctx).Command(name, args...)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errMsg := strings.TrimSpace(stderr.String()); errMsg != "" {
			return nil, fmt.Errorf("%s", errMsg)
		}
		return nil, err
	}
	return output, nil
}

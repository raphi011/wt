package git

import (
	"context"
	"os/exec"

	"github.com/raphi011/wt/internal/cmd"
)

// runCmd executes a command and returns stderr in the error message if it fails
func runCmd(c *exec.Cmd) error {
	return cmd.Run(c)
}

// outputCmd executes a command and returns stdout, with stderr in error if it fails
func outputCmd(c *exec.Cmd) ([]byte, error) {
	return cmd.Output(c)
}

// runGit executes a git command with context support and verbose logging.
func runGit(ctx context.Context, dir string, args ...string) error {
	return cmd.RunContext(ctx, dir, "git", args...)
}

// outputGit executes a git command with context support and verbose logging,
// returning stdout.
func outputGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return cmd.OutputContext(ctx, dir, "git", args...)
}

package git

import (
	"context"

	"github.com/raphi011/wt/internal/cmd"
)

// runGit executes a git command with context support and verbose logging.
func runGit(ctx context.Context, dir string, args ...string) error {
	return cmd.RunContext(ctx, dir, "git", args...)
}

// outputGit executes a git command with context support and verbose logging,
// returning stdout.
func outputGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return cmd.OutputContext(ctx, dir, "git", args...)
}

// RunGitCommand executes a git command with context support and verbose logging.
// This is the exported version of runGit for use by commands.
func RunGitCommand(ctx context.Context, dir string, args ...string) error {
	return runGit(ctx, dir, args...)
}

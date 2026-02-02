package git

import (
	"context"

	"github.com/raphi011/wt/internal/cmd"
)

// gitArgs prepends -C <dir> to args if dir is non-empty.
func gitArgs(dir string, args []string) []string {
	if dir == "" {
		return args
	}
	return append([]string{"-C", dir}, args...)
}

// runGit executes a git command with context support and verbose logging.
func runGit(ctx context.Context, dir string, args ...string) error {
	return cmd.RunContext(ctx, "", "git", gitArgs(dir, args)...)
}

// outputGit executes a git command with context support and verbose logging,
// returning stdout.
func outputGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return cmd.OutputContext(ctx, "", "git", gitArgs(dir, args)...)
}

// RunGitCommand executes a git command with context support and verbose logging.
// This is the exported version of runGit for use by commands.
func RunGitCommand(ctx context.Context, dir string, args ...string) error {
	return runGit(ctx, dir, args...)
}

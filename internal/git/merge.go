package git

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

const wtMergedSuffix = ".wt-merged"

// ParseWtMerged parses a wt-merged config value like "squash:main@2026-03-28T14:30:00Z"
// into its strategy, target branch, and timestamp components.
// Returns zero values for any component that cannot be parsed.
func ParseWtMerged(value string) (strategy, target string, ts time.Time) {
	if value == "" {
		return "", "", time.Time{}
	}

	// Split on ":" to get strategy and the rest
	before, after, ok := strings.Cut(value, ":")
	if !ok {
		return "", "", time.Time{}
	}

	strategy = before
	rest := after

	// Split rest on "@" to get target and timestamp
	before, after, ok = strings.Cut(rest, "@")
	if !ok {
		// No timestamp
		return strategy, rest, time.Time{}
	}

	target = before
	tsStr := after

	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		return strategy, target, time.Time{}
	}

	return strategy, target, ts
}

// FormatWtMerged formats a wt-merged config value from strategy, target, and timestamp.
// Format: "strategy:target@timestamp" where timestamp is RFC3339.
func FormatWtMerged(strategy, target string, ts time.Time) string {
	return strategy + ":" + target + "@" + ts.UTC().Format(time.RFC3339)
}

// DetectMergeStrategy detects the merge strategy from git merge passthrough args.
// Returns "squash" for --squash, "ff" for --ff-only, "merge" otherwise.
func DetectMergeStrategy(args []string) string {
	for _, arg := range args {
		switch arg {
		case "--squash":
			return "squash"
		case "--ff-only":
			return "ff"
		}
	}
	return "merge"
}

// HasNoCommitFlag checks whether --no-commit is present in the args.
func HasNoCommitFlag(args []string) bool {
	return slices.Contains(args, "--no-commit")
}

// GetWtMerged reads the branch.<name>.wt-merged config value.
// Returns empty string if the key does not exist.
func GetWtMerged(ctx context.Context, repoPath, branch string) (string, error) {
	output, err := outputGit(ctx, repoPath, "config", "branch."+branch+wtMergedSuffix)
	if err != nil {
		// Exit code 1 means the config key doesn't exist - not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// SetWtMerged writes the branch.<name>.wt-merged config value.
func SetWtMerged(ctx context.Context, repoPath, branch, value string) error {
	return runGit(ctx, repoPath, "config", "branch."+branch+wtMergedSuffix, value)
}

// ClearWtMerged removes the branch.<name>.wt-merged config value.
func ClearWtMerged(ctx context.Context, repoPath, branch string) error {
	if err := runGit(ctx, repoPath, "config", "--unset", "branch."+branch+wtMergedSuffix); err != nil {
		// Exit code 5 means the key doesn't exist - not an error for clearing
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 5 {
			return nil
		}
		return err
	}
	return nil
}

// IsWorktreeClean checks whether a worktree has no uncommitted changes or untracked files.
func IsWorktreeClean(ctx context.Context, worktreePath string) (bool, error) {
	output, err := outputGit(ctx, worktreePath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check worktree status: %w", err)
	}

	return strings.TrimSpace(string(output)) == "", nil
}

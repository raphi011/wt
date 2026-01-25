package git

import (
	"context"
	"os/exec"
	"slices"
	"strings"
)

const labelsConfigKey = "wt.labels"

// GetLabels returns the labels for a repository
// Returns empty slice if no labels are set
func GetLabels(ctx context.Context, repoPath string) ([]string, error) {
	output, err := outputGit(ctx, repoPath, "config", "--local", labelsConfigKey)
	if err != nil {
		// Exit code 1 means the config key doesn't exist - not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}

	value := strings.TrimSpace(string(output))
	if value == "" {
		return nil, nil
	}

	// Split by comma and trim whitespace
	parts := strings.Split(value, ",")
	var labels []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			labels = append(labels, p)
		}
	}
	return labels, nil
}

// SetLabels sets the labels for a repository (replaces existing)
func SetLabels(ctx context.Context, repoPath string, labels []string) error {
	if len(labels) == 0 {
		return ClearLabels(ctx, repoPath)
	}

	value := strings.Join(labels, ",")
	return runGit(ctx, repoPath, "config", "--local", labelsConfigKey, value)
}

// AddLabel adds a label to a repository (if not already present)
func AddLabel(ctx context.Context, repoPath, label string) error {
	labels, err := GetLabels(ctx, repoPath)
	if err != nil {
		return err
	}

	// Check if already present
	if slices.Contains(labels, label) {
		return nil // Already exists
	}

	labels = append(labels, label)
	return SetLabels(ctx, repoPath, labels)
}

// RemoveLabel removes a label from a repository
func RemoveLabel(ctx context.Context, repoPath, label string) error {
	labels, err := GetLabels(ctx, repoPath)
	if err != nil {
		return err
	}

	// Filter out the label
	newLabels := slices.DeleteFunc(labels, func(l string) bool { return l == label })

	return SetLabels(ctx, repoPath, newLabels)
}

// ClearLabels removes all labels from a repository
func ClearLabels(ctx context.Context, repoPath string) error {
	if err := runGit(ctx, repoPath, "config", "--local", "--unset", labelsConfigKey); err != nil {
		// Exit code 5 means the key doesn't exist - not an error for clearing
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 5 {
			return nil
		}
		return err
	}
	return nil
}

// HasLabel checks if a repository has a specific label
func HasLabel(ctx context.Context, repoPath, label string) (bool, error) {
	labels, err := GetLabels(ctx, repoPath)
	if err != nil {
		return false, err
	}

	return slices.Contains(labels, label), nil
}

// FindReposByLabel scans a directory for repos with the given label
// Returns paths to matching repositories
func FindReposByLabel(ctx context.Context, scanDir, label string) ([]string, error) {
	repos, err := FindAllRepos(scanDir)
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, repoPath := range repos {
		has, err := HasLabel(ctx, repoPath, label)
		if err != nil {
			continue // Skip repos with errors
		}
		if has {
			matches = append(matches, repoPath)
		}
	}

	return matches, nil
}

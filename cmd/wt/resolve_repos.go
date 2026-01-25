package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
)

// collectRepoPaths collects unique repository paths from -r and -l flags.
// Returns a map of repo paths (for deduplication) and any errors encountered.
func collectRepoPaths(ctx context.Context, repos []string, labels []string, scanDir string, cfg *config.Config) (map[string]bool, []error) {
	var errs []error
	repoPaths := make(map[string]bool)

	// Determine scan directory
	repoScanDir := cfg.RepoScanDir()
	if repoScanDir == "" {
		repoScanDir = scanDir
	}

	// Process -r flags (repository names)
	for _, repoName := range repos {
		repoPath, err := git.FindRepoByName(repoScanDir, repoName)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
			continue
		}
		repoPaths[repoPath] = true
	}

	// Process -l flags (labels)
	for _, label := range labels {
		paths, err := git.FindReposByLabel(ctx, repoScanDir, label)
		if err != nil {
			errs = append(errs, fmt.Errorf("label %q: %w", label, err))
			continue
		}
		if len(paths) == 0 {
			errs = append(errs, fmt.Errorf("label %q: no repos found with this label", label))
			continue
		}
		for _, p := range paths {
			repoPaths[p] = true
		}
	}

	return repoPaths, errs
}

// resolveRepoScanDir resolves the directory to scan for repositories.
// Uses cfg.RepoScanDir() if set, otherwise falls back to the provided dir.
// Returns an error if no directory can be determined.
func resolveRepoScanDir(dir string, cfg *config.Config) (string, error) {
	scanDir := cfg.RepoScanDir()
	if scanDir == "" {
		scanDir = dir
	}
	if scanDir == "" {
		return "", fmt.Errorf("directory required when using -r or -l (-d flag or WT_WORKTREE_DIR)")
	}

	absPath, err := filepath.Abs(scanDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return absPath, nil
}

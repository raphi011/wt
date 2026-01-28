package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseRepoArg splits a repo argument into org and name components.
// "org/repo" returns ("org", "repo")
// "repo" returns ("", "repo")
func ParseRepoArg(repo string) (org, name string) {
	if idx := strings.Index(repo, "/"); idx != -1 {
		return repo[:idx], repo[idx+1:]
	}
	return "", repo
}

// FindRepoByName searches direct children of basePath for a git repo with the given folder name.
// Matches by folder name only (case-insensitive), excludes worktrees.
// Returns the full path if found, or an error if not found.
func FindRepoByName(basePath, name string) (string, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", basePath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.EqualFold(entry.Name(), name) {
			continue
		}
		repoPath := filepath.Join(basePath, entry.Name())
		if IsMainRepo(repoPath) {
			return repoPath, nil
		}
	}

	return "", fmt.Errorf("repository %q not found in %s", name, basePath)
}

// FindSimilarRepos returns repo names in basePath that contain the search string.
// Useful for providing suggestions when a repo is not found.
func FindSimilarRepos(basePath, search string) []string {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil
	}

	search = strings.ToLower(search)
	var matches []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		repoPath := filepath.Join(basePath, entry.Name())
		if !IsMainRepo(repoPath) {
			continue
		}

		if strings.Contains(strings.ToLower(entry.Name()), search) {
			matches = append(matches, entry.Name())
		}
	}

	return matches
}

// IsMainRepo checks if path is a main git repository (not a worktree).
// Main repos have .git as a directory; worktrees have .git as a file.
func IsMainRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FindAllRepos returns paths to all main git repositories in basePath (direct children only).
// Excludes worktrees (only includes repos where .git is a directory).
func FindAllRepos(basePath string) ([]string, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", basePath, err)
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		repoPath := filepath.Join(basePath, entry.Name())
		if IsMainRepo(repoPath) {
			repos = append(repos, repoPath)
		}
	}

	return repos, nil
}

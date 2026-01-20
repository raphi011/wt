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

// FindRepoByName searches direct children of basePath for a git repo with the given name.
// First checks for exact folder name match with matching origin, then searches all repos by origin.
// Returns the full path if found, or an error if not found.
func FindRepoByName(basePath, name string) (string, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %w", basePath, err)
	}

	// First pass: exact folder name match with verified origin
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() != name {
			continue
		}

		repoPath := filepath.Join(basePath, entry.Name())
		if !isGitRepo(repoPath) {
			continue
		}

		// Verify origin matches
		repoName, err := GetRepoNameFrom(repoPath)
		if err == nil && strings.EqualFold(repoName, name) {
			return repoPath, nil
		}
	}

	// Second pass: search all repos by origin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		repoPath := filepath.Join(basePath, entry.Name())
		if !isGitRepo(repoPath) {
			continue
		}

		repoName, err := GetRepoNameFrom(repoPath)
		if err != nil {
			continue
		}

		if strings.EqualFold(repoName, name) {
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
		if !isGitRepo(repoPath) {
			continue
		}

		if strings.Contains(strings.ToLower(entry.Name()), search) {
			matches = append(matches, entry.Name())
		}
	}

	return matches
}

// isGitRepo checks if a path is a git repository (has .git dir or file)
func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	// .git can be a directory (regular repo) or file (worktree)
	return info.IsDir() || info.Mode().IsRegular()
}

// FindAllRepos returns paths to all git repositories in basePath (direct children only)
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
		if isGitRepo(repoPath) {
			repos = append(repos, repoPath)
		}
	}

	return repos, nil
}


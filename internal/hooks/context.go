package hooks

import (
	"path/filepath"

	"github.com/raphi011/wt/internal/git"
)

// ContextFromRepo builds a Context for a repository (not worktree-specific).
func ContextFromRepo(repoPath string, trigger CommandType, env map[string]string) Context {
	repo := filepath.Base(repoPath) // folder name
	origin := git.GetRepoDisplayName(repoPath)
	if origin == "" {
		origin = repo
	}
	return Context{
		WorktreeDir: repoPath,
		RepoDir:     repoPath,
		Branch:      "", // No specific branch when targeting repo
		Repo:        repo,
		Origin:      origin,
		Trigger:     string(trigger),
		Env:         env,
	}
}

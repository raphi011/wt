package hooks

import (
	"path/filepath"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/resolve"
)

// ContextFromWorktree builds a Context from a resolved worktree target.
func ContextFromWorktree(target *resolve.Target, trigger CommandType, env map[string]string) Context {
	repo := filepath.Base(target.MainRepo) // folder name
	origin := git.GetRepoDisplayName(target.MainRepo)
	if origin == "" {
		origin = repo
	}
	return Context{
		WorktreeDir: target.Path,
		RepoDir:     target.MainRepo,
		Branch:      target.Branch,
		Repo:        repo,
		Origin:      origin,
		Trigger:     string(trigger),
		Env:         env,
	}
}

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

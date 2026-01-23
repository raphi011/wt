package hooks

import (
	"path/filepath"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/resolve"
)

// ContextFromWorktree builds a Context from a resolved worktree target.
func ContextFromWorktree(target *resolve.Target, trigger CommandType, env map[string]string) Context {
	ctx := Context{
		Path:     target.Path,
		Branch:   target.Branch,
		MainRepo: target.MainRepo,
		Folder:   filepath.Base(target.MainRepo),
		Trigger:  string(trigger),
		Env:      env,
	}
	ctx.Repo = git.GetRepoDisplayName(target.MainRepo)
	return ctx
}

// ContextFromRepo builds a Context for a repository (not worktree-specific).
func ContextFromRepo(repoPath string, trigger CommandType, env map[string]string) Context {
	return Context{
		Path:     repoPath,
		Branch:   "", // No specific branch when targeting repo
		MainRepo: repoPath,
		Folder:   filepath.Base(repoPath),
		Repo:     git.GetRepoDisplayName(repoPath),
		Trigger:  string(trigger),
		Env:      env,
	}
}

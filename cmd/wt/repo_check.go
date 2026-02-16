package main

import (
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

// filterOrphanedRepos checks repos for missing paths, logs warnings, and returns
// only the repos that exist on disk.
func filterOrphanedRepos(l *log.Logger, repos []registry.Repo) []registry.Repo {
	var valid []registry.Repo
	for _, repo := range repos {
		exists, err := repo.PathExists()
		if err != nil {
			l.Debug("cannot access repo path", "repo", repo.Name, "path", repo.Path, "error", err)
			continue
		}
		if !exists {
			l.Debug("repo path no longer exists", "repo", repo.Name, "path", repo.Path)
			continue
		}
		valid = append(valid, repo)
	}
	return valid
}

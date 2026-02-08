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
			l.Printf("Warning: %s: cannot access path (%s): %v\n", repo.Name, repo.Path, err)
			continue
		}
		if !exists {
			l.Printf("Warning: %s: path no longer exists (%s)\n", repo.Name, repo.Path)
			l.Printf("  Repo is orphaned. Update with: wt repo add <new-path> --name %s\n", repo.Name)
			continue
		}
		valid = append(valid, repo)
	}
	return valid
}

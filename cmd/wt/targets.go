package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/registry"
)

// resolveTargetRepos finds repos based on -r and -l flags, or current directory.
// If no flags are provided, attempts to find/register the current repo.
// Deduplicates results by path.
func resolveTargetRepos(ctx context.Context, reg *registry.Registry, repoNames, labels []string) ([]*registry.Repo, error) {
	var repos []*registry.Repo

	// If specific repos requested
	if len(repoNames) > 0 {
		for _, name := range repoNames {
			repo, err := reg.FindByName(name)
			if err != nil {
				return nil, err
			}
			repos = append(repos, repo)
		}
	}

	// If labels requested
	if len(labels) > 0 {
		labeled := reg.FindByLabels(labels)
		repos = append(repos, labeled...)
	}

	// If no specific target, try current directory
	if len(repos) == 0 {
		repo, err := findOrRegisterCurrentRepo(ctx, reg)
		if err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []*registry.Repo
	for _, r := range repos {
		if !seen[r.Path] {
			seen[r.Path] = true
			unique = append(unique, r)
		}
	}

	return unique, nil
}

// resolveTargetReposNoFallback resolves repos by name only, without auto-registering.
// Use this when the command should fail if not in a registered repo.
func resolveTargetReposNoFallback(ctx context.Context, reg *registry.Registry, repoNames []string) ([]*registry.Repo, error) {
	if len(repoNames) > 0 {
		var repos []*registry.Repo
		for _, name := range repoNames {
			repo, err := reg.FindByName(name)
			if err != nil {
				return nil, err
			}
			repos = append(repos, repo)
		}
		return repos, nil
	}

	// Try current repo without auto-registering
	repoPath := git.GetCurrentRepoMainPath(ctx)
	if repoPath == "" {
		return nil, fmt.Errorf("not in a git repository (use -r to specify)")
	}

	repo, err := reg.FindByPath(repoPath)
	if err != nil {
		return nil, err
	}

	return []*registry.Repo{repo}, nil
}

// findOrRegisterCurrentRepo finds the repo for cwd, auto-registering if needed.
// Returns error if not in a git repository.
func findOrRegisterCurrentRepo(ctx context.Context, reg *registry.Registry) (*registry.Repo, error) {
	// Get main repo path from current directory
	repoPath := git.GetCurrentRepoMainPath(ctx)
	if repoPath == "" {
		return nil, fmt.Errorf("not in a git repository")
	}

	// Try to find in registry
	repo, err := reg.FindByPath(repoPath)
	if err == nil {
		return repo, nil
	}

	// Auto-register
	newRepo := registry.Repo{
		Path:   repoPath,
		Name:   git.GetRepoDisplayName(repoPath),
		Labels: cfg.DefaultLabels,
	}

	if err := reg.Add(newRepo); err != nil {
		return nil, err
	}

	if err := reg.Save(); err != nil {
		return nil, err
	}

	return reg.FindByPath(repoPath)
}

// parseBranchTarget parses "repo:branch" or "branch" format.
// Returns (repo, branch) where repo is empty if not specified.
// Uses colon separator to avoid ambiguity with branches containing "/".
func parseBranchTarget(target string) (repo, branch string) {
	if idx := strings.Index(target, ":"); idx > 0 {
		return target[:idx], target[idx+1:]
	}
	return "", target // no repo specified
}

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

// ScopedTargetResult holds the result of parsing a scoped target
type ScopedTargetResult struct {
	Repos   []*registry.Repo // Matched repos (1 for repo name, multiple for label)
	Branch  string           // The branch part
	IsLabel bool             // True if scope matched a label (not a repo name)
}

// parseScopedTarget parses "scope:branch" where scope can be repo name or label.
// Resolution order: try repo name first, then label.
// If no scope provided, returns empty Repos slice (caller decides behavior).
func parseScopedTarget(reg *registry.Registry, target string) (ScopedTargetResult, error) {
	scope, branch := parseBranchTarget(target)

	if scope == "" {
		// No scope - return just the branch
		return ScopedTargetResult{Branch: branch}, nil
	}

	// Try repo name first
	repo, err := reg.FindByName(scope)
	if err == nil {
		return ScopedTargetResult{
			Repos:   []*registry.Repo{repo},
			Branch:  branch,
			IsLabel: false,
		}, nil
	}

	// Try label
	labelRepos := reg.FindByLabel(scope)
	if len(labelRepos) > 0 {
		return ScopedTargetResult{
			Repos:   labelRepos,
			Branch:  branch,
			IsLabel: true,
		}, nil
	}

	return ScopedTargetResult{}, fmt.Errorf("no repo or label found: %s", scope)
}

// WorktreeTarget holds a resolved worktree target
type WorktreeTarget struct {
	RepoName string
	RepoPath string
	Branch   string
	Path     string
}

// resolveWorktreeTargets parses [scope:]branch args and returns worktree paths.
// scope can be a repo name or label. If no scope, searches all repos.
// Returns error if any target is not found.
func resolveWorktreeTargets(ctx context.Context, reg *registry.Registry, targets []string) ([]WorktreeTarget, error) {
	var results []WorktreeTarget

	for _, target := range targets {
		parsed, err := parseScopedTarget(reg, target)
		if err != nil {
			return nil, err
		}

		if len(parsed.Repos) > 0 {
			// Scoped target - find worktree in specified repo(s)
			found := false
			for _, repo := range parsed.Repos {
				wts, err := git.ListWorktreesFromRepo(ctx, repo.Path)
				if err != nil {
					continue
				}
				for _, wt := range wts {
					if wt.Branch == parsed.Branch {
						results = append(results, WorktreeTarget{
							RepoName: repo.Name,
							RepoPath: repo.Path,
							Branch:   parsed.Branch,
							Path:     wt.Path,
						})
						found = true
						break
					}
				}
			}
			if !found {
				if parsed.IsLabel {
					return nil, fmt.Errorf("worktree not found: %s (label matched %d repos)", target, len(parsed.Repos))
				}
				return nil, fmt.Errorf("worktree not found: %s", target)
			}
		} else {
			// No scope - search all repos
			var matches []WorktreeTarget
			for i := range reg.Repos {
				repo := &reg.Repos[i]
				wts, err := git.ListWorktreesFromRepo(ctx, repo.Path)
				if err != nil {
					continue
				}
				for _, wt := range wts {
					if wt.Branch == parsed.Branch {
						matches = append(matches, WorktreeTarget{
							RepoName: repo.Name,
							RepoPath: repo.Path,
							Branch:   parsed.Branch,
							Path:     wt.Path,
						})
					}
				}
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("worktree not found: %s", parsed.Branch)
			}
			results = append(results, matches...)
		}
	}

	// Deduplicate by path
	seen := make(map[string]bool)
	var unique []WorktreeTarget
	for _, wt := range results {
		if !seen[wt.Path] {
			seen[wt.Path] = true
			unique = append(unique, wt)
		}
	}

	return unique, nil
}

// resolveScopedRepos resolves scope (repo name or label) to repos.
// If scope is empty, returns error asking for explicit scope.
// Used when targeting repos (not worktrees) like for checkout -b.
func resolveScopedRepos(reg *registry.Registry, scope string) ([]*registry.Repo, error) {
	if scope == "" {
		return nil, fmt.Errorf("repo or label required")
	}

	// Try repo name first
	repo, err := reg.FindByName(scope)
	if err == nil {
		return []*registry.Repo{repo}, nil
	}

	// Try label
	labelRepos := reg.FindByLabel(scope)
	if len(labelRepos) > 0 {
		return labelRepos, nil
	}

	return nil, fmt.Errorf("no repo or label found: %s", scope)
}

// resolveScopeArgsOrCurrent resolves scope arguments, falling back to current repo.
// If scopes provided: resolves each as repo name → label.
// If no scopes: uses current repo (errors if not in a registered repo).
func resolveScopeArgsOrCurrent(ctx context.Context, reg *registry.Registry, scopes []string) ([]*registry.Repo, error) {
	if len(scopes) > 0 {
		return resolveScopeArgs(reg, scopes)
	}

	// Fall back to current repo
	repoPath := git.GetCurrentRepoMainPath(ctx)
	if repoPath == "" {
		return nil, fmt.Errorf("not in a git repository (specify repo or label)")
	}

	repo, err := reg.FindByPath(repoPath)
	if err != nil {
		return nil, fmt.Errorf("repo not registered: %s", repoPath)
	}

	return []*registry.Repo{repo}, nil
}

// resolveScopeArgs resolves multiple scope arguments to repos.
// Each scope is tried as repo name first, then label.
// Results are deduplicated by path.
func resolveScopeArgs(reg *registry.Registry, scopes []string) ([]*registry.Repo, error) {
	var repos []*registry.Repo

	for _, scope := range scopes {
		resolved, err := resolveScopedRepos(reg, scope)
		if err != nil {
			return nil, err
		}
		repos = append(repos, resolved...)
	}

	// Deduplicate by path
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

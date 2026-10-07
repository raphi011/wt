package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

// resolveEffectiveConfig returns the effective config for a repo path,
// falling back to global config if local config can't be loaded.
func resolveEffectiveConfig(ctx context.Context, repoPath string) *config.Config {
	return resolveConfig(ctx, repoPath, config.Overrides{})
}

// resolveConfig includes operation overrides, preserving them when a local
// config error requires falling back to global config.
func resolveConfig(ctx context.Context, repoPath string, overrides config.Overrides) *config.Config {
	l := log.FromContext(ctx)
	resolver := config.ResolverFromContext(ctx)
	effCfg, err := resolver.ResolveForRepo(repoPath, overrides)
	if err != nil {
		l.Printf("Warning: failed to load local config for %s: %v\n", repoPath, err)
		return resolver.ResolveGlobal(overrides)
	}
	return effCfg
}

// reposToRefs converts registry repos to git.RepoRef for the parallel loader.
func reposToRefs(repos []registry.Repo) []git.RepoRef {
	refs := make([]git.RepoRef, len(repos))
	for i, r := range repos {
		refs[i] = git.RepoRef{Name: r.Name, Path: r.Path}
	}
	return refs
}

// errNotInRepo is returned when the working directory is not inside a git repository.
var errNotInRepo = errors.New("not in a git repository")

// currentRepoPath returns the main repo path of the working directory,
// or "" outside a git repository.
func currentRepoPath(ctx context.Context) string {
	return git.GetCurrentRepoMainPathFrom(ctx, config.WorkDirFromContext(ctx))
}

// unregisteredPolicy says what currentRepo does when the repo of the working
// directory is not in the registry.
type unregisteredPolicy int

const (
	autoRegister      unregisteredPolicy = iota // add it to the registry
	requireRegistered                           // return an error
)

// currentRepo returns the repo of the working directory.
// Returns errNotInRepo outside a git repository.
func currentRepo(ctx context.Context, reg *registry.Registry, policy unregisteredPolicy) (registry.Repo, error) {
	repoPath := currentRepoPath(ctx)
	if repoPath == "" {
		return registry.Repo{}, errNotInRepo
	}

	// Try to find in registry
	repo, err := reg.FindByPath(repoPath)
	if err == nil {
		return repo, nil
	}
	if policy == requireRegistered {
		return registry.Repo{}, fmt.Errorf("repo not registered: %s", repoPath)
	}

	// Auto-register
	cfg := config.FromContext(ctx)
	newRepo := registry.Repo{
		Path:   repoPath,
		Name:   git.GetRepoDisplayName(repoPath),
		Labels: cfg.DefaultLabels,
	}

	// Another process may have registered the repo since reg was loaded
	updated, err := registry.Update(cfg.RegistryPath, func(r *registry.Registry) error {
		if _, err := r.FindByPath(repoPath); err == nil {
			return nil
		}
		return r.Add(newRepo)
	})
	if err != nil {
		return registry.Repo{}, err
	}
	*reg = *updated

	return reg.FindByPath(repoPath)
}

// unscopedRepos returns the repos a command acts on when no scope is given:
// the current repo, or all repos outside a repo or with global (-g).
// current reports whether the result is the current repo only.
func unscopedRepos(ctx context.Context, reg *registry.Registry, global bool) (repos []registry.Repo, current bool, err error) {
	if !global {
		repo, err := currentRepo(ctx, reg, autoRegister)
		if err == nil {
			return []registry.Repo{repo}, true, nil
		}
		if !errors.Is(err, errNotInRepo) {
			return nil, false, err
		}
	}
	return filterOrphanedRepos(log.FromContext(ctx), reg.Repos), false, nil
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
	Repos   []registry.Repo // Matched repos (1 for repo name, multiple for label)
	Branch  string          // The branch part
	IsLabel bool            // True if scope matched a label (not a repo name)
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

	// Scope provided but no branch name
	if branch == "" {
		return ScopedTargetResult{}, fmt.Errorf("branch name required after %q", scope+":")
	}

	// Try repo name first
	repo, err := reg.FindByName(scope)
	if err == nil {
		return ScopedTargetResult{
			Repos:   []registry.Repo{repo},
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

// targetOpts controls how [scope:]branch targets resolve.
type targetOpts struct {
	Global bool // -g: an unscoped branch is searched in all repos, also inside a repo
	Multi  bool // the command acts on every match; otherwise several matches are an error
}

// targetEnv is what worktree target resolution reads besides the registry.
type targetEnv struct {
	// unscopedRepos returns the repos an unscoped branch is searched in and
	// whether that is the current repo only.
	unscopedRepos func() (repos []registry.Repo, current bool, err error)
	// worktrees lists the worktrees of a repo.
	worktrees func(repoPath string) ([]git.WorktreeInfo, error)
}

// resolveWorktreeTargets parses [scope:]branch args and returns worktree paths.
// scope can be a repo name or label. An unscoped branch means the current
// repo; outside a repo, or with opts.Global, all repos are searched.
// A target that matches several worktrees is an error unless the command is
// multi-target and the fan-out was asked for with a label scope or opts.Global.
// Returns error if any target is not found, or if git fails in a repo targeted by name.
// When searching several repos (label or all repos), git failures and repos
// without a matching worktree are logged and skipped.
func resolveWorktreeTargets(ctx context.Context, reg *registry.Registry, targets []string, opts targetOpts) ([]WorktreeTarget, error) {
	return resolveWorktreeTargetsIn(log.FromContext(ctx), reg, targets, opts, targetEnv{
		unscopedRepos: func() ([]registry.Repo, bool, error) {
			return unscopedRepos(ctx, reg, opts.Global)
		},
		worktrees: func(repoPath string) ([]git.WorktreeInfo, error) {
			return git.ListWorktreesFromRepo(ctx, repoPath)
		},
	})
}

// resolveWorktreeTargetsIn is resolveWorktreeTargets with the git and
// working directory lookups passed in.
func resolveWorktreeTargetsIn(l *log.Logger, reg *registry.Registry, targets []string, opts targetOpts, env targetEnv) ([]WorktreeTarget, error) {
	var results []WorktreeTarget

	for _, target := range targets {
		parsed, err := parseScopedTarget(reg, target)
		if err != nil {
			return nil, err
		}

		if len(parsed.Repos) > 0 {
			// Scoped target - find worktree in specified repo(s)
			matches, missing, err := findBranchWorktrees(l, env, parsed.Repos, parsed.Branch, !parsed.IsLabel)
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				if parsed.IsLabel {
					return nil, fmt.Errorf("worktree not found: %s (label matched %d repos)", target, len(parsed.Repos))
				}
				return nil, fmt.Errorf("worktree not found: %s", target)
			}
			if len(matches) > 1 && !opts.Multi {
				return nil, ambiguousTargetError(parsed.Branch, matches, "use repo:branch to pick one")
			}
			if len(missing) > 0 {
				l.Printf("Skipped (no worktree for %s): %s\n", parsed.Branch, strings.Join(missing, ", "))
			}
			results = append(results, matches...)
		} else {
			matches, err := resolveUnscopedWorktrees(l, env, parsed.Branch, opts)
			if err != nil {
				return nil, err
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

// findBranchWorktrees returns the worktree of branch in each of repos and the
// names of the repos that have none. A git failure is returned if strict,
// otherwise it is logged and the repo skipped.
func findBranchWorktrees(l *log.Logger, env targetEnv, repos []registry.Repo, branch string, strict bool) (matches []WorktreeTarget, missing []string, err error) {
	for _, repo := range repos {
		wts, err := env.worktrees(repo.Path)
		if err != nil {
			if strict {
				return nil, nil, fmt.Errorf("%s: %w", repo.Name, err)
			}
			l.Printf("Warning: %s: %v\n", repo.Name, err)
			continue
		}
		i := slices.IndexFunc(wts, func(wt git.WorktreeInfo) bool { return wt.Branch == branch })
		if i < 0 {
			missing = append(missing, repo.Name)
			continue
		}
		matches = append(matches, WorktreeTarget{
			RepoName: repo.Name,
			RepoPath: repo.Path,
			Branch:   branch,
			Path:     wts[i].Path,
		})
	}
	return matches, missing, nil
}

// resolveUnscopedWorktrees finds the worktrees of a branch given without a scope.
// Inside a repo only that repo is searched, unless opts.Global is set.
func resolveUnscopedWorktrees(l *log.Logger, env targetEnv, branch string, opts targetOpts) ([]WorktreeTarget, error) {
	repos, current, err := env.unscopedRepos()
	if err != nil {
		return nil, err
	}
	matches, _, err := findBranchWorktrees(l, env, repos, branch, current)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		if current {
			return nil, fmt.Errorf("worktree not found in %s: %s (use -g to search all repos, or repo:branch)", repos[0].Name, branch)
		}
		return nil, fmt.Errorf("worktree not found: %s", branch)
	}
	if len(matches) > 1 {
		if !opts.Multi {
			return nil, ambiguousTargetError(branch, matches, "use repo:branch to pick one")
		}
		if !opts.Global {
			return nil, ambiguousTargetError(branch, matches, "use -g to target all of them, or repo:branch to pick one")
		}
	}
	return matches, nil
}

// ambiguousTargetError reports a branch that has worktrees in several repos.
func ambiguousTargetError(branch string, matches []WorktreeTarget, hint string) error {
	var names []string
	for _, m := range matches {
		names = append(names, m.RepoName+":"+branch)
	}
	return fmt.Errorf("branch %q exists in multiple repos: %s (%s)", branch, strings.Join(names, ", "), hint)
}

// resolveOneWorktreeTarget resolves a single [scope:]branch target and returns exactly
// one match. Returns error if no match is found or if the target is ambiguous
// (branch exists in several of the searched repos).
func resolveOneWorktreeTarget(ctx context.Context, reg *registry.Registry, target string, global bool) (WorktreeTarget, error) {
	matches, err := resolveWorktreeTargets(ctx, reg, []string{target}, targetOpts{Global: global})
	if err != nil {
		return WorktreeTarget{}, err
	}
	return matches[0], nil
}

// resolveScopedRepos resolves scope (repo name or label) to repos.
// If scope is empty, returns error asking for explicit scope.
// Used when targeting repos (not worktrees) like for checkout -b.
func resolveScopedRepos(reg *registry.Registry, scope string) ([]registry.Repo, error) {
	if scope == "" {
		return nil, fmt.Errorf("repo or label required")
	}

	// Try repo name first
	repo, err := reg.FindByName(scope)
	if err == nil {
		return []registry.Repo{repo}, nil
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
func resolveScopeArgsOrCurrent(ctx context.Context, reg *registry.Registry, scopes []string) ([]registry.Repo, error) {
	if len(scopes) > 0 {
		return resolveScopeArgs(reg, scopes)
	}

	// Fall back to current repo
	repo, err := currentRepo(ctx, reg, requireRegistered)
	if errors.Is(err, errNotInRepo) {
		return nil, fmt.Errorf("%w (specify repo or label)", err)
	}
	if err != nil {
		return nil, err
	}

	return []registry.Repo{repo}, nil
}

// resolveScopeArgs resolves multiple scope arguments to repos.
// Each scope is tried as repo name first, then label.
// Results are deduplicated by path.
func resolveScopeArgs(reg *registry.Registry, scopes []string) ([]registry.Repo, error) {
	var repos []registry.Repo

	for _, scope := range scopes {
		resolved, err := resolveScopedRepos(reg, scope)
		if err != nil {
			return nil, err
		}
		repos = append(repos, resolved...)
	}

	// Deduplicate by path
	seen := make(map[string]bool)
	var unique []registry.Repo
	for _, r := range repos {
		if !seen[r.Path] {
			seen[r.Path] = true
			unique = append(unique, r)
		}
	}

	return unique, nil
}

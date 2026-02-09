package git

import (
	"context"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
)

// RepoRef identifies a repo for the loader.
// Keeps the git package independent of the registry package.
type RepoRef struct {
	Name string
	Path string
}

// LoadWarning represents a non-fatal error encountered while loading worktrees for a repo.
type LoadWarning struct {
	RepoName string
	Err      error
}

// LoadWorktreesForRepos fetches worktrees from all repos in parallel.
// Per repo: ListWorktreesFromRepo + GetAllBranchConfig + GetOriginURL + os.Stat per worktree.
// PR fields are NOT populated — callers do that from prcache after loading.
// Results maintain stable ordering (by repo index, then worktree order within repo).
// Errors per repo are collected as warnings (non-fatal).
func LoadWorktreesForRepos(ctx context.Context, repos []RepoRef) ([]Worktree, []LoadWarning) {
	// Per-repo results stored by index for stable ordering
	type repoResult struct {
		worktrees []Worktree
		warning   *LoadWarning
	}

	results := make([]repoResult, len(repos))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(8) // Bound concurrent git operations

	for i, repo := range repos {
		g.Go(func() error {
			wts, warn := loadWorktreesForRepo(ctx, repo)
			results[i] = repoResult{worktrees: wts, warning: warn}
			return nil // Never fail — warnings are non-fatal
		})
	}

	_ = g.Wait() // Always nil — goroutines collect errors as warnings

	// Flatten results in stable order, collect warnings
	var all []Worktree
	var warnings []LoadWarning
	for _, r := range results {
		all = append(all, r.worktrees...)
		if r.warning != nil {
			warnings = append(warnings, *r.warning)
		}
	}

	return all, warnings
}

// loadWorktreesForRepo fetches worktrees for a single repo.
func loadWorktreesForRepo(ctx context.Context, repo RepoRef) ([]Worktree, *LoadWarning) {
	wtInfos, err := ListWorktreesFromRepo(ctx, repo.Path)
	if err != nil {
		return nil, &LoadWarning{RepoName: repo.Name, Err: err}
	}

	// Batch-fetch branch config (notes + upstreams) in one git call
	notes, upstreams := GetAllBranchConfig(ctx, repo.Path)

	// Get origin URL once per repo (repos without origin simply skip PR refresh)
	originURL, _ := GetOriginURL(ctx, repo.Path)

	worktrees := make([]Worktree, 0, len(wtInfos))
	for _, wti := range wtInfos {
		var createdAt time.Time
		if info, err := os.Stat(wti.Path); err == nil {
			createdAt = info.ModTime()
		}

		worktrees = append(worktrees, Worktree{
			Path:        wti.Path,
			Branch:      wti.Branch,
			CommitHash:  wti.CommitHash,
			RepoName:    repo.Name,
			RepoPath:    repo.Path,
			OriginURL:   originURL,
			Note:        notes[wti.Branch],
			HasUpstream: upstreams[wti.Branch],
			CreatedAt:   createdAt,
		})
	}

	return worktrees, nil
}

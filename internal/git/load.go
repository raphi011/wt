package git

import (
	"context"

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
// Per repo: ListWorktreesFromRepo + GetAllBranchConfig + GetOriginURL + GetCommitMeta.
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

	// Get origin URL once per repo. Non-fatal: repos without an "origin"
	// remote get originURL="" and simply skip PR refresh downstream.
	originURL, _ := GetOriginURL(ctx, repo.Path) //nolint:errcheck

	// Batch-fetch commit metadata (age + date) in one git call.
	// Filter out zero-hash (orphan/unborn worktrees) to avoid failing the entire batch.
	shas := make([]string, 0, len(wtInfos))
	for _, wti := range wtInfos {
		if wti.CommitHash != "" && wti.CommitHash != zeroHash {
			shas = append(shas, wti.CommitHash)
		}
	}
	commitMetas, err := GetCommitMeta(ctx, repo.Path, shas)
	if err != nil {
		// Non-fatal: worktrees will render with empty age columns.
		// We still have the (empty) map, so the loop below works.
		commitMetas = make(map[string]CommitMeta)
	}

	worktrees := make([]Worktree, 0, len(wtInfos))
	for _, wti := range wtInfos {
		meta := commitMetas[wti.CommitHash]

		worktrees = append(worktrees, Worktree{
			Path:        wti.Path,
			Branch:      wti.Branch,
			CommitHash:  wti.CommitHash,
			CommitAge:   meta.Age,
			CommitDate:  meta.Date,
			RepoName:    repo.Name,
			RepoPath:    repo.Path,
			OriginURL:   originURL,
			Note:        notes[wti.Branch],
			HasUpstream: upstreams[wti.Branch],
		})
	}

	return worktrees, nil
}

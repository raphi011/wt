// Package forgetest supplies an in-memory forge for command and status tests.
package forgetest

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/raphi011/wt/internal/forge"
)

type prKey struct{ repo, branch string }
type pullRequest struct {
	info  forge.PRInfo
	title string
}

// CreateCall records a successful creation.
type CreateCall struct {
	RepoURL string
	Params  forge.CreatePRParams
}

// MergeCall records a successful merge.
type MergeCall struct {
	RepoURL  string
	Number   int
	Strategy string
}

// Forge stores PRs by repository URL and branch. All access is synchronized,
// and lookups and call histories return copies, so refresh workers can share it.
type Forge struct {
	mu      sync.Mutex
	prs     map[prKey]pullRequest
	creates []CreateCall
	merges  []MergeCall
}

var _ forge.Forge = (*Forge)(nil)

func New() *Forge { return &Forge{prs: make(map[prKey]pullRequest)} }

func (f *Forge) Name() string { return "memory" }

func (f *Forge) Check(ctx context.Context) error { return ctx.Err() }

// SetPR seeds a branch without recording a creation.
func (f *Forge) SetPR(repoURL, branch string, info forge.PRInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	info.Fetched = true
	if info.CachedAt.IsZero() {
		info.CachedAt = time.Now()
	}
	f.prs[prKey{repoURL, branch}] = pullRequest{info: info, title: fmt.Sprintf("PR #%d", info.Number)}
}

func (f *Forge) GetPRForBranch(ctx context.Context, repoURL, branch string) (*forge.PRInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	info := f.prs[prKey{repoURL, branch}].info
	info.Fetched = true
	info.CachedAt = time.Now()
	return &info, nil
}

func (f *Forge) GetPRBranch(ctx context.Context, repoURL string, number int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for key, pr := range f.prs {
		if key.repo == repoURL && pr.info.Number == number {
			return key.branch, nil
		}
	}
	return "", fmt.Errorf("PR #%d not found in %s", number, repoURL)
}

func (f *Forge) CreatePR(ctx context.Context, repoURL string, params forge.CreatePRParams) (*forge.CreatePRResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	number := 1
	for key, pr := range f.prs {
		if key.repo == repoURL {
			number = max(number, pr.info.Number+1)
		}
	}
	url := fmt.Sprintf("https://example.test/%s/pull/%d", forge.ExtractRepoPath(repoURL), number)
	f.prs[prKey{repoURL, params.Head}] = pullRequest{
		info:  forge.PRInfo{Number: number, State: forge.PRStateOpen, IsDraft: params.Draft, URL: url, Fetched: true, CachedAt: time.Now()},
		title: params.Title,
	}
	f.creates = append(f.creates, CreateCall{repoURL, params})
	return &forge.CreatePRResult{Number: number, URL: url}, nil
}

func (f *Forge) MergePR(ctx context.Context, repoURL string, number int, strategy string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for key, pr := range f.prs {
		if key.repo == repoURL && pr.info.Number == number {
			pr.info.State = forge.PRStateMerged
			pr.info.CachedAt = time.Now()
			f.prs[key] = pr
			f.merges = append(f.merges, MergeCall{repoURL, number, strategy})
			return nil
		}
	}
	return fmt.Errorf("PR #%d not found in %s", number, repoURL)
}

func (f *Forge) ListOpenPRs(ctx context.Context, repoURL string) ([]forge.OpenPR, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var prs []forge.OpenPR
	for key, pr := range f.prs {
		if key.repo == repoURL && pr.info.State == forge.PRStateOpen {
			prs = append(prs, forge.OpenPR{Number: pr.info.Number, Title: pr.title, Author: pr.info.Author, Branch: key.branch, IsDraft: pr.info.IsDraft})
		}
	}
	slices.SortFunc(prs, func(a, b forge.OpenPR) int { return a.Number - b.Number })
	return prs, nil
}

func (f *Forge) Creates() []CreateCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.creates)
}

func (f *Forge) Merges() []MergeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.merges)
}

// Cloning requires an external side effect.
// Tests that need these operations can embed Forge and implement them locally.
func (f *Forge) CloneRepo(ctx context.Context, _, _ string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("in-memory forge does not clone repositories")
}

func (f *Forge) CloneBareRepo(ctx context.Context, _, _ string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("in-memory forge does not clone repositories")
}

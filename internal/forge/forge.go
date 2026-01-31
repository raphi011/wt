package forge

import (
	"context"

	"github.com/raphi011/wt/internal/cache"
)

// PRInfo is an alias to cache.PRInfo for backward compatibility
type PRInfo = cache.PRInfo

// CreatePRParams contains parameters for creating a PR/MR
type CreatePRParams struct {
	Title string
	Body  string
	Base  string // base branch (empty = repo default)
	Head  string // head/source branch
	Draft bool
}

// CreatePRResult contains the result of creating a PR/MR
type CreatePRResult struct {
	Number int
	URL    string
}

// OpenPR represents a PR in a list of open PRs
type OpenPR struct {
	Number  int
	Title   string
	Author  string
	Branch  string
	IsDraft bool
}

// Forge represents a git hosting service (GitHub, GitLab, etc.)
type Forge interface {
	// Name returns the forge name ("github" or "gitlab")
	Name() string

	// Check verifies the CLI is installed and authenticated
	Check(ctx context.Context) error

	// GetPRForBranch fetches PR info for a branch
	GetPRForBranch(ctx context.Context, repoURL, branch string) (*PRInfo, error)

	// GetPRBranch gets the source branch name for a PR number
	GetPRBranch(ctx context.Context, repoURL string, number int) (string, error)

	// CloneRepo clones a repository to destPath, returns the full clone path
	CloneRepo(ctx context.Context, repoSpec, destPath string) (string, error)

	// CloneBareRepo clones a repository as a bare repo with .bare/.git symlink pattern.
	// This creates:
	//   destPath/<repo>/
	//   ├── .bare/   # actual bare git repo
	//   └── .git     # symlink -> .bare
	// Returns the full path to the repo directory (parent of .bare).
	CloneBareRepo(ctx context.Context, repoSpec, destPath string) (string, error)

	// CreatePR creates a new PR/MR
	CreatePR(ctx context.Context, repoURL string, params CreatePRParams) (*CreatePRResult, error)

	// MergePR merges a PR by number with the given strategy
	// strategy: "squash", "rebase", or "merge"
	// Returns error if repo doesn't allow the requested merge strategy
	MergePR(ctx context.Context, repoURL string, number int, strategy string) error

	// ViewPR shows PR details or opens in browser
	// If web is true, opens in browser; otherwise shows details in terminal
	ViewPR(ctx context.Context, repoURL string, number int, web bool) error

	// ListOpenPRs lists all open PRs for a repository
	ListOpenPRs(ctx context.Context, repoURL string) ([]OpenPR, error)

	// FormatState returns a human-readable PR state
	FormatState(state string) string
}

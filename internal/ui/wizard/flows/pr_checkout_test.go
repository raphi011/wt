package flows

import (
	"errors"
	"testing"

	"github.com/raphi011/wt/internal/forge"
)

func TestPrCheckoutInteractive_SingleRepo_FetchError(t *testing.T) {
	// When single repo and FetchPRs returns error, should return error
	params := PrCheckoutWizardParams{
		AvailableRepos:  []string{"/path/to/repo"},
		RepoNames:       []string{"my-repo"},
		PreSelectedRepo: -1,
		FetchPRs: func(repoPath string) ([]forge.OpenPR, error) {
			return nil, errors.New("network error")
		},
	}

	_, err := PrCheckoutInteractive(params)

	if err == nil {
		t.Fatal("expected error for fetch failure")
	}
	if err.Error() != "failed to fetch PRs: network error" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPrCheckoutInteractive_SingleRepo_NoPRs(t *testing.T) {
	// When single repo and no open PRs, should return error
	params := PrCheckoutWizardParams{
		AvailableRepos:  []string{"/path/to/repo"},
		RepoNames:       []string{"my-repo"},
		PreSelectedRepo: -1,
		FetchPRs: func(repoPath string) ([]forge.OpenPR, error) {
			return []forge.OpenPR{}, nil // Empty slice
		},
	}

	_, err := PrCheckoutInteractive(params)

	if err == nil {
		t.Fatal("expected error for empty PRs")
	}
	if err.Error() != "no open PRs found" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPrCheckoutInteractive_NoRepos(t *testing.T) {
	// Edge case: no repos at all (should not happen in practice but test defense)
	params := PrCheckoutWizardParams{
		AvailableRepos: nil,
		RepoNames:      nil,
	}

	// With no repos and no FetchPRs, the PR step will have no options
	// The wizard will run but there's nothing to select
	// This tests that we don't panic on empty state
	// Note: This will start the wizard which blocks, so we can't easily test it
	// without refactoring. Just documenting the expected behavior.
	_ = params
}

func TestPrCheckoutOptions_Structure(t *testing.T) {
	opts := PrCheckoutOptions{
		Cancelled:     false,
		SelectedRepo:  "/path/to/repo",
		SelectedPR:    123,
		SelectedHooks: []string{"build", "lint"},
		NoHook:        false,
	}

	if opts.Cancelled {
		t.Error("Cancelled should be false")
	}
	if opts.SelectedRepo != "/path/to/repo" {
		t.Errorf("SelectedRepo = %q, want /path/to/repo", opts.SelectedRepo)
	}
	if opts.SelectedPR != 123 {
		t.Errorf("SelectedPR = %d, want 123", opts.SelectedPR)
	}
	if len(opts.SelectedHooks) != 2 {
		t.Errorf("SelectedHooks length = %d, want 2", len(opts.SelectedHooks))
	}
	if opts.NoHook {
		t.Error("NoHook should be false")
	}
}

func TestPrCheckoutWizardParams_Structure(t *testing.T) {
	// Test that PRFetcher signature matches expected type
	var fetcher PRFetcher = func(repoPath string) ([]forge.OpenPR, error) {
		return []forge.OpenPR{
			{Number: 1, Title: "PR 1", Author: "user1", Branch: "feature-1", IsDraft: false},
			{Number: 2, Title: "PR 2", Author: "user2", Branch: "feature-2", IsDraft: true},
		}, nil
	}

	params := PrCheckoutWizardParams{
		AvailableRepos:  []string{"/path/to/repo1", "/path/to/repo2"},
		RepoNames:       []string{"repo1", "repo2"},
		PreSelectedRepo: 0,
		FetchPRs:        fetcher,
		AvailableHooks: []HookInfo{
			{Name: "build", Description: "Run build", IsDefault: true},
		},
		HooksFromCLI: false,
	}

	if len(params.AvailableRepos) != 2 {
		t.Errorf("AvailableRepos length = %d, want 2", len(params.AvailableRepos))
	}
	if params.PreSelectedRepo != 0 {
		t.Errorf("PreSelectedRepo = %d, want 0", params.PreSelectedRepo)
	}

	// Test the fetcher works
	prs, err := params.FetchPRs("/any/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Errorf("expected 2 PRs, got %d", len(prs))
	}
	if prs[0].Title != "PR 1" {
		t.Errorf("first PR title = %q, want 'PR 1'", prs[0].Title)
	}
	if !prs[1].IsDraft {
		t.Error("second PR should be a draft")
	}
}

// Note: Full interactive testing of PrCheckoutInteractive would require:
// 1. Refactoring to separate wizard building from wizard.Run()
// 2. Or using teatest with golden files for full TUI testing
//
// The wizard has complex behavior:
// - Single repo skips repo selection step
// - Multi repo shows repo selection with callback to fetch PRs
// - PRs are displayed with title + description (@author (branch) [draft])
// - Hooks step pre-selects default hooks
//
// For multi-repo case, we can't easily test without running the wizard
// since the PR fetch happens in the OnComplete callback.

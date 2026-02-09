package flows

import (
	"testing"
)

func TestCdInteractive_EmptyWorktrees(t *testing.T) {
	// When no worktrees are available, should return Cancelled without running wizard
	params := CdWizardParams{
		Worktrees: nil,
	}

	opts, err := CdInteractive(params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Cancelled {
		t.Error("expected Cancelled=true for empty worktrees")
	}
	if opts.SelectedPath != "" {
		t.Errorf("expected empty SelectedPath, got %q", opts.SelectedPath)
	}
}

func TestCdInteractive_EmptyWorktreesSlice(t *testing.T) {
	// Empty slice should also return Cancelled
	params := CdWizardParams{
		Worktrees: []CdWorktreeInfo{},
	}

	opts, err := CdInteractive(params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Cancelled {
		t.Error("expected Cancelled=true for empty worktrees slice")
	}
}

// TestCdWorktreeInfo_Formatting verifies the worktree info structure
// is set up correctly for wizard display.
func TestCdWorktreeInfo_Structure(t *testing.T) {
	info := CdWorktreeInfo{
		RepoName: "my-repo",
		Branch:   "feature-branch",
		Path:     "/path/to/worktree",
	}

	if info.RepoName != "my-repo" {
		t.Errorf("RepoName = %q, want my-repo", info.RepoName)
	}
	if info.Branch != "feature-branch" {
		t.Errorf("Branch = %q, want feature-branch", info.Branch)
	}
	if info.Path != "/path/to/worktree" {
		t.Errorf("Path = %q, want /path/to/worktree", info.Path)
	}
}

// Note: Full interactive testing of CdInteractive requires calling
// cdListModel.Update() directly with synthetic tea.KeyPressMsg values,
// bypassing the BubbleTea runtime. The model only processes KeyPressMsg;
// other message types are safely ignored.

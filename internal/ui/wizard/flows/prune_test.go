package flows

import (
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

func TestPruneInteractive_EmptyWorktrees(t *testing.T) {
	// When no worktrees are available, should return Cancelled without running wizard
	params := PruneWizardParams{
		Worktrees: nil,
	}

	opts, err := PruneInteractive(params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Cancelled {
		t.Error("expected Cancelled=true for empty worktrees")
	}
	if len(opts.SelectedIDs) != 0 {
		t.Errorf("expected empty SelectedIDs, got %v", opts.SelectedIDs)
	}
}

func TestPruneInteractive_EmptyWorktreesSlice(t *testing.T) {
	// Empty slice should also return Cancelled
	params := PruneWizardParams{
		Worktrees: []PruneWorktreeInfo{},
	}

	opts, err := PruneInteractive(params)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Cancelled {
		t.Error("expected Cancelled=true for empty worktrees slice")
	}
}

func TestPruneDescriptionRenderer_Prunable(t *testing.T) {
	opt := framework.Option{
		Label:       "repo:branch",
		Description: "Merged PR",
		Value: pruneOptionValue{
			ID:         1,
			IsPrunable: true,
			Reason:     "Merged PR",
		},
	}

	// Prunable items should be rendered in success style (green)
	result := pruneDescriptionRenderer(opt, false)

	// The result should contain the reason
	if !strings.Contains(result, "Merged PR") {
		t.Errorf("expected result to contain 'Merged PR', got %q", result)
	}
}

func TestPruneDescriptionRenderer_NonPrunableSelected(t *testing.T) {
	opt := framework.Option{
		Label:       "repo:branch",
		Description: "Has uncommitted changes",
		Value: pruneOptionValue{
			ID:         2,
			IsPrunable: false,
			Reason:     "Has uncommitted changes",
		},
	}

	// Non-prunable but selected should show "Force" prefix
	result := pruneDescriptionRenderer(opt, true)

	// The result should contain "Force" and the reason
	if !strings.Contains(result, "Force") {
		t.Errorf("expected result to contain 'Force', got %q", result)
	}
	if !strings.Contains(result, "Has uncommitted changes") {
		t.Errorf("expected result to contain reason, got %q", result)
	}
}

func TestPruneDescriptionRenderer_NonPrunableUnselected(t *testing.T) {
	opt := framework.Option{
		Label:       "repo:branch",
		Description: "Not merged",
		Value: pruneOptionValue{
			ID:         3,
			IsPrunable: false,
			Reason:     "Not merged",
		},
	}

	// Non-prunable and not selected should show reason in muted style
	result := pruneDescriptionRenderer(opt, false)

	// The result should contain the reason
	if !strings.Contains(result, "Not merged") {
		t.Errorf("expected result to contain 'Not merged', got %q", result)
	}
	// Should NOT contain "Force"
	if strings.Contains(result, "Force") {
		t.Errorf("unexpected 'Force' in result: %q", result)
	}
}

func TestPruneDescriptionRenderer_InvalidValue(t *testing.T) {
	// When Value is not a pruneOptionValue, fall back to Description
	opt := framework.Option{
		Label:       "repo:branch",
		Description: "Fallback description",
		Value:       "not a pruneOptionValue",
	}

	result := pruneDescriptionRenderer(opt, false)

	if !strings.Contains(result, "Fallback description") {
		t.Errorf("expected fallback to Description, got %q", result)
	}
}

func TestPruneWorktreeInfo_Structure(t *testing.T) {
	info := PruneWorktreeInfo{
		ID:         42,
		RepoName:   "my-repo",
		Branch:     "feature-branch",
		Reason:     "Merged PR",
		IsPrunable: true,
	}

	if info.ID != 42 {
		t.Errorf("ID = %d, want 42", info.ID)
	}
	if info.RepoName != "my-repo" {
		t.Errorf("RepoName = %q, want my-repo", info.RepoName)
	}
	if info.Branch != "feature-branch" {
		t.Errorf("Branch = %q, want feature-branch", info.Branch)
	}
	if info.Reason != "Merged PR" {
		t.Errorf("Reason = %q, want 'Merged PR'", info.Reason)
	}
	if !info.IsPrunable {
		t.Error("IsPrunable should be true")
	}
}

// Note: Full interactive testing of PruneInteractive would require:
// 1. Refactoring to separate wizard building from wizard.Run()
// 2. Or using teatest with golden files for full TUI testing
//
// The wizard:
// - Pre-selects prunable worktrees automatically
// - Shows custom description with colored status
// - Has info line showing count and dirty warnings
// - Uses "Confirm removal" as summary title

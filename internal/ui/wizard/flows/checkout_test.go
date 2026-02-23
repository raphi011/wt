package flows

import (
	"testing"
)

func TestBuildBranchOptions_FiltersCheckedOutBranches(t *testing.T) {
	branches := []BranchInfo{
		{Name: "main", InWorktree: true},       // Should be filtered
		{Name: "feature-a", InWorktree: false}, // Should be included
		{Name: "feature-b", InWorktree: true},  // Should be filtered
		{Name: "develop", InWorktree: false},   // Should be included
	}

	opts := buildBranchOptions(branches)

	if len(opts) != 2 {
		t.Fatalf("expected 2 options, got %d", len(opts))
	}

	// Verify correct branches are included
	labels := make([]string, len(opts))
	for i, opt := range opts {
		labels[i] = opt.Label
	}

	if labels[0] != "feature-a" {
		t.Errorf("first option = %q, want feature-a", labels[0])
	}
	if labels[1] != "develop" {
		t.Errorf("second option = %q, want develop", labels[1])
	}
}

func TestBuildBranchOptions_AllCheckedOut(t *testing.T) {
	branches := []BranchInfo{
		{Name: "main", InWorktree: true},
		{Name: "develop", InWorktree: true},
	}

	opts := buildBranchOptions(branches)

	if len(opts) != 0 {
		t.Errorf("expected 0 options when all checked out, got %d", len(opts))
	}
}

func TestBuildBranchOptions_Empty(t *testing.T) {
	opts := buildBranchOptions(nil)

	if len(opts) != 0 {
		t.Errorf("expected nil or empty slice, got %v", opts)
	}
}

func TestBuildBranchOptions_ValueEqualsLabel(t *testing.T) {
	branches := []BranchInfo{
		{Name: "feature-branch", InWorktree: false},
	}

	opts := buildBranchOptions(branches)

	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}

	// Value should equal Label (both are branch name)
	if opts[0].Label != opts[0].Value {
		t.Errorf("Label = %q, Value = %v, should be equal", opts[0].Label, opts[0].Value)
	}
}

func TestCheckoutOptions_Structure(t *testing.T) {
	opts := CheckoutOptions{
		Branch:        "feature-x",
		NewBranch:     true,
		Cancelled:     false,
		SelectedRepos: []string{"/path/to/repo1", "/path/to/repo2"},
		SelectedHooks: []string{"build", "test"},
		NoHook:        false,
	}

	if opts.Branch != "feature-x" {
		t.Errorf("Branch = %q, want feature-x", opts.Branch)
	}
	if !opts.NewBranch {
		t.Error("NewBranch should be true")
	}
	if opts.Cancelled {
		t.Error("Cancelled should be false")
	}
	if len(opts.SelectedRepos) != 2 {
		t.Errorf("SelectedRepos length = %d, want 2", len(opts.SelectedRepos))
	}
	if len(opts.SelectedHooks) != 2 {
		t.Errorf("SelectedHooks length = %d, want 2", len(opts.SelectedHooks))
	}
	if opts.NoHook {
		t.Error("NoHook should be false")
	}
}

func TestBranchInfo_Structure(t *testing.T) {
	info := BranchInfo{
		Name:       "feature-branch",
		InWorktree: true,
	}

	if info.Name != "feature-branch" {
		t.Errorf("Name = %q, want feature-branch", info.Name)
	}
	if !info.InWorktree {
		t.Error("InWorktree should be true")
	}
}

func TestHookInfo_Structure(t *testing.T) {
	info := HookInfo{
		Name:        "build",
		Description: "Run build script",
		IsDefault:   true,
	}

	if info.Name != "build" {
		t.Errorf("Name = %q, want build", info.Name)
	}
	if info.Description != "Run build script" {
		t.Errorf("Description = %q, want 'Run build script'", info.Description)
	}
	if !info.IsDefault {
		t.Error("IsDefault should be true")
	}
}

// Note: Full interactive testing of CheckoutInteractive would require:
// 1. Refactoring to separate wizard building from wizard.Run()
// 2. Or using teatest with golden files for full TUI testing
//
// The wizard has complex behavior:
// - Repo step triggers branch fetch callback
// - Branch step supports create-from-filter
// - Fetch step is conditionally skipped for existing branches
// - Hooks step pre-selects default hooks
//
// To test these, we would need to:
// - Build the wizard without calling Run()
// - Inspect skip conditions and callbacks
// - Simulate key events to test the full flow

package flows

import (
	"testing"
)

func TestBuildBranchOptions_MarksCheckedOutBranches(t *testing.T) {
	branches := []BranchInfo{
		{Name: "main", InWorktree: true},       // Should be marked
		{Name: "feature-a", InWorktree: false}, // Normal
		{Name: "feature-b", InWorktree: true},  // Should be marked
		{Name: "develop", InWorktree: false},   // Normal
	}

	opts := buildBranchOptions(branches)

	if len(opts) != 4 {
		t.Fatalf("expected 4 options, got %d", len(opts))
	}

	// Verify labels
	want := []struct{ label, value string }{
		{"main (worktree)", "main"},
		{"feature-a", "feature-a"},
		{"feature-b (worktree)", "feature-b"},
		{"develop", "develop"},
	}
	for i, w := range want {
		if opts[i].Label != w.label {
			t.Errorf("opts[%d].Label = %q, want %q", i, opts[i].Label, w.label)
		}
		if opts[i].Value != w.value {
			t.Errorf("opts[%d].Value = %v, want %q", i, opts[i].Value, w.value)
		}
	}
}

func TestBuildBranchOptions_AllCheckedOut(t *testing.T) {
	branches := []BranchInfo{
		{Name: "main", InWorktree: true},
		{Name: "develop", InWorktree: true},
	}

	opts := buildBranchOptions(branches)

	if len(opts) != 2 {
		t.Errorf("expected 2 options when all checked out, got %d", len(opts))
	}
	if len(opts) > 0 && opts[0].Label != "main (worktree)" {
		t.Errorf("opts[0].Label = %q, want %q", opts[0].Label, "main (worktree)")
	}
}

func TestBuildBranchOptions_Empty(t *testing.T) {
	opts := buildBranchOptions(nil)

	if len(opts) != 0 {
		t.Errorf("expected nil or empty slice, got %v", opts)
	}
}

func TestBuildBranchOptions_ValueEqualsLabelForNonWorktree(t *testing.T) {
	branches := []BranchInfo{
		{Name: "feature-branch", InWorktree: false},
	}

	opts := buildBranchOptions(branches)

	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}

	// Value should equal Label for non-worktree branches
	if opts[0].Label != opts[0].Value {
		t.Errorf("Label = %q, Value = %v, should be equal for non-worktree branch", opts[0].Label, opts[0].Value)
	}
}

func TestBuildBranchOptions_WorktreeLabelDiffersFromValue(t *testing.T) {
	branches := []BranchInfo{
		{Name: "feature-branch", InWorktree: true},
	}

	opts := buildBranchOptions(branches)

	if len(opts) != 1 {
		t.Fatalf("expected 1 option, got %d", len(opts))
	}

	// Label should have " (worktree)" suffix, Value should be plain branch name
	if opts[0].Label != "feature-branch (worktree)" {
		t.Errorf("Label = %q, want %q", opts[0].Label, "feature-branch (worktree)")
	}
	if opts[0].Value != "feature-branch" {
		t.Errorf("Value = %v, want %q", opts[0].Value, "feature-branch")
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

func TestCheckoutOptions_IncludesBase(t *testing.T) {
	opts := CheckoutOptions{
		Branch:    "feature-x",
		NewBranch: true,
		Base:      "main",
	}

	if opts.Base != "main" {
		t.Errorf("Base = %q, want %q", opts.Base, "main")
	}
}

func TestCheckoutWizardParams_IncludesDefaultBranch(t *testing.T) {
	params := CheckoutWizardParams{
		DefaultBranch: "main",
		BaseFromCLI:   false,
	}

	if params.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", params.DefaultBranch, "main")
	}
	if params.BaseFromCLI {
		t.Error("BaseFromCLI should be false")
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

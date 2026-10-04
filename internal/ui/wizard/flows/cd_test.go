package flows

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
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

func TestCdListModel_Paste(t *testing.T) {
	worktrees := []CdWorktreeInfo{
		{RepoName: "repo1", Branch: "feature-alpha", Path: "/path/alpha"},
		{RepoName: "repo2", Branch: "feature-beta", Path: "/path/beta"},
		{RepoName: "repo1", Branch: "main", Path: "/path/main"},
	}

	t.Run("paste filters worktree list", func(t *testing.T) {
		options := make([]framework.Option, len(worktrees))
		for i, wt := range worktrees {
			options[i] = framework.Option{
				Label: wt.RepoName + ":" + wt.Branch,
				Value: i,
			}
		}

		selectStep := steps.NewFilterableList("worktree", "Worktree", "", options)
		model := &cdListModel{
			step:       selectStep,
			worktrees:  worktrees,
			selectedAt: -1,
		}
		model.Init()

		// Paste "beta" to filter the list
		m, _ := model.Update(tea.PasteMsg{Content: "beta"})
		model = m.(*cdListModel)

		if model.step.GetFilter() != "beta" {
			t.Errorf("Filter = %q, want %q", model.step.GetFilter(), "beta")
		}
		if model.step.FilteredCount() != 1 {
			t.Errorf("FilteredCount = %d, want 1", model.step.FilteredCount())
		}
	})
}

func TestCdListModelInputEditingAndBlink(t *testing.T) {
	step := steps.NewFilterableList("worktree", "Worktree", "", []framework.Option{{Label: "repo:main", Value: 0}})
	model := &cdListModel{step: step, selectedAt: -1}
	model.Init()
	model.Update(tea.PasteMsg{Content: "repo:man"})
	model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	model.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	if step.GetFilter() != "repo:main" || model.done || model.selectedAt != -1 {
		t.Fatalf("editing selected an item or lost caret: %q, done=%v", step.GetFilter(), model.done)
	}
	if _, cmd := model.Update(textinput.Blink()); cmd == nil {
		t.Fatal("cursor initialization message did not reach cd input")
	}
	model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !model.cancelled || !step.HasClearableInput() {
		t.Fatal("Ctrl+C must immediately cancel cd with a filter present")
	}
}

package flows

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

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

func TestCdListModelPageNavigation(t *testing.T) {
	t.Parallel()
	for _, filtered := range []bool{false, true} {
		t.Run(fmt.Sprintf("filtered=%v", filtered), func(t *testing.T) {
			options := make([]framework.Option, 30)
			for i := range options {
				options[i] = framework.Option{Label: fmt.Sprintf("repo:branch-%02d", i), Value: i}
			}
			model := &cdListModel{step: steps.NewFilterableList("cd", "Cd", "", options), selectedAt: -1}
			model.Init()
			model.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
			for _, tc := range []struct {
				code rune
				want int
			}{{tea.KeyPgDown, 29}, {tea.KeyPgUp, 0}} {
				if filtered {
					model.Update(tea.PasteMsg{Content: "repo:"})
				}
				model.Update(tea.KeyPressMsg{Code: tc.code})
				if got := model.step.GetCursor(); got != tc.want {
					t.Fatalf("key=%v cursor=%d want=%d", tc.code, got, tc.want)
				}
				if filtered && model.step.GetFilter() != "repo:" {
					t.Fatalf("paging changed filter: %q", model.step.GetFilter())
				}
				if model.done || model.cancelled {
					t.Fatal("paging ended picker")
				}
				label := fmt.Sprintf("repo:branch-%02d", tc.want)
				if !strings.Contains(ansi.Strip(model.View().Content), label) {
					t.Fatalf("focused row %q missing from view", label)
				}
				if filtered {
					model.step.ClearInput()
				}
			}
			model.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
			model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if !model.done || model.selectedAt != 29 {
				t.Fatalf("selected=%d done=%v", model.selectedAt, model.done)
			}
		})
	}
}

func TestCdListModelHelp(t *testing.T) {
	t.Parallel()
	model := &cdListModel{step: steps.NewFilterableList("cd", "Cd", "", []framework.Option{{Label: "repo:main", Value: 0}}), selectedAt: -1}
	model.Init()
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	for _, filtered := range []bool{false, true} {
		if filtered {
			model.Update(tea.PasteMsg{Content: "repo:"})
		}
		help := ansi.Strip(model.help(80, 24))
		for _, hidden := range []string{"ctrl+c", "back", "next", "alt+"} {
			if strings.Contains(help, hidden) {
				t.Fatalf("unexpected %q hint: %s", hidden, help)
			}
		}
		for _, visible := range []string{"enter confirm", "pgup/pgdn jump", "esc"} {
			if !strings.Contains(help, visible) {
				t.Fatalf("missing %q hint: %s", visible, help)
			}
		}
		if strings.Contains(help, "\n") {
			t.Fatalf("help should fit in one 80-cell line: %s", help)
		}
		if filtered && !strings.Contains(help, "esc clear filter") {
			t.Fatalf("missing clear-filter hint: %s", help)
		}
	}
	model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !model.cancelled {
		t.Fatal("Ctrl+C must still cancel despite omitted help hint")
	}
}

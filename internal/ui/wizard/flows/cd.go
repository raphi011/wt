package flows

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/colorprofile"
	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
)

// CdOptions holds the options gathered from interactive mode.
type CdOptions struct {
	SelectedPath string // Selected worktree path
	RepoName     string // Repository name of selected worktree
	Branch       string // Branch name of selected worktree
	Cancelled    bool
}

// CdWorktreeInfo contains worktree data for display in the list.
type CdWorktreeInfo struct {
	RepoName   string
	Branch     string
	Path       string
	LastAccess time.Time
}

// CdWizardParams contains parameters for the cd interactive list.
type CdWizardParams struct {
	Worktrees []CdWorktreeInfo // All worktrees available for selection
}

// cdListModel is a lightweight BubbleTea model wrapping FilterableListStep
// directly, bypassing the wizard framework chrome (borders, title, tabs).
type cdListModel struct {
	step       *steps.FilterableListStep
	worktrees  []CdWorktreeInfo
	done       bool
	cancelled  bool
	selectedAt int // index into worktrees; -1 means no selection
}

func (m *cdListModel) Init() tea.Cmd {
	return m.step.Init()
}

// Update handles incoming messages. Only tea.KeyPressMsg is processed;
// other message types (window size, mouse, etc.) are safely ignored
// because FilterableListStep only operates on key events.
func (m *cdListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "esc":
		if m.step.HasClearableInput() {
			cmd := m.step.ClearInput()
			return m, cmd
		}
		m.cancelled = true
		return m, tea.Quit
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit
	}

	_, cmd, result := m.step.Update(keyMsg)

	if result == framework.StepSubmitIfReady || result == framework.StepAdvance {
		val := m.step.GetSelectedValue()
		if val != nil {
			m.selectedAt = val.(int)
			m.done = true
			return m, tea.Quit
		}
	}

	return m, cmd
}

func (m *cdListModel) View() tea.View {
	if m.done || m.cancelled {
		return tea.NewView("")
	}
	return tea.NewView(m.step.View() + "\n" + framework.HelpStyle().Render(m.step.Help()) + "\n")
}

// CdInteractive runs the interactive cd list with fuzzy search.
// This bypasses the wizard framework for a lightweight, fast experience.
func CdInteractive(params CdWizardParams) (CdOptions, error) {
	if len(params.Worktrees) == 0 {
		return CdOptions{Cancelled: true}, nil
	}

	options := make([]framework.Option, len(params.Worktrees))

	for i, wt := range params.Worktrees {
		options[i] = framework.Option{
			Label: fmt.Sprintf("%s:%s", wt.RepoName, wt.Branch),
			Value: i,
		}
	}

	selectStep := steps.NewFilterableList("worktree", "Worktree", "", options)

	model := &cdListModel{
		step:       selectStep,
		worktrees:  params.Worktrees,
		selectedAt: -1,
	}

	profile := colorprofile.Detect(os.Stderr, os.Environ())
	p := tea.NewProgram(model,
		tea.WithOutput(os.Stderr),
		tea.WithColorProfile(profile),
	)

	finalModel, err := p.Run()
	if err != nil {
		return CdOptions{}, err
	}

	m := finalModel.(*cdListModel)
	if m.cancelled || !m.done || m.selectedAt < 0 {
		return CdOptions{Cancelled: true}, nil
	}

	wt := m.worktrees[m.selectedAt]
	return CdOptions{
		SelectedPath: wt.Path,
		RepoName:     wt.RepoName,
		Branch:       wt.Branch,
	}, nil
}


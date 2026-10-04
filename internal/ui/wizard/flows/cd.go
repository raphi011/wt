package flows

import (
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/raphi011/wt/internal/ui/styles"

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
	width, height int
	step          *steps.FilterableListStep
	worktrees     []CdWorktreeInfo
	done          bool
	cancelled     bool
	selectedAt    int // index into worktrees; -1 means no selection
}

func (m *cdListModel) Init() tea.Cmd {
	m.resizeStep()
	return tea.Batch(m.step.Init(), styles.BackgroundCommand())
}

// Update handles navigation and forwards component messages to the list input.
func (m *cdListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer m.resizeStep()
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		m.resizeStep()
		return m, nil
	case tea.BackgroundColorMsg:
		styles.ApplyBackground(msg.IsDark())
	case tea.PasteMsg:
		// Paste never triggers navigation; discard StepResult.
		_, cmd, _ := m.step.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, framework.Keys.Clear):
			if m.step.HasClearableInput() {
				cmd := m.step.ClearInput()
				return m, cmd
			}
			m.cancelled = true
			return m, tea.Quit
		case key.Matches(msg, framework.Keys.Cancel):
			m.cancelled = true
			return m, tea.Quit
		}

		_, cmd, result := m.step.Update(msg)

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

	if !m.done && !m.cancelled {
		_, cmd, _ := m.step.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *cdListModel) View() tea.View {
	if m.done || m.cancelled {
		return tea.NewView("")
	}
	width, height := m.dimensions()
	help := m.help(width, height)
	return tea.NewView(framework.Fit(m.step.View()+"\n"+help, width, height))
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

func (m *cdListModel) dimensions() (int, int) {
	if m.width == 0 {
		return 80, 24
	}
	return m.width, m.height
}
func (m *cdListModel) help(width, height int) string {
	return framework.HelpStyle().MarginTop(0).Render(framework.Wrap(m.step.Help(), width, min(3, max(1, height/4))))
}
func (m *cdListModel) resizeStep() {
	width, height := m.dimensions()
	m.step.SetSize(width, max(1, height-lipgloss.Height(m.help(width, height))))
}

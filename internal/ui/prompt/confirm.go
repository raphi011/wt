package prompt

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// ConfirmResult holds the result of a confirmation prompt.
type ConfirmResult struct {
	Confirmed bool
	Cancelled bool
}

type confirmModel struct {
	prompt    string
	confirmed bool
	done      bool
	cancelled bool
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.confirmed = true
			m.done = true
			return m, tea.Quit
		case "n", "N":
			m.confirmed = false
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			// Default to no
			m.confirmed = false
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s [y/N] ", m.prompt)
}

// Confirm shows a yes/no prompt and returns the user's choice.
// The default answer is "no" if the user presses enter without input.
func Confirm(prompt string) (ConfirmResult, error) {
	model := confirmModel{prompt: prompt}
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return ConfirmResult{}, err
	}
	m := finalModel.(confirmModel)
	return ConfirmResult{
		Confirmed: m.confirmed,
		Cancelled: m.cancelled,
	}, nil
}

package prompt

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// TextInputResult holds the result of a text input prompt.
type TextInputResult struct {
	Value     string
	Cancelled bool
}

type textInputModel struct {
	textInput textinput.Model
	prompt    string
	done      bool
	cancelled bool
}

func (m textInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m textInputModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s\n%s", m.prompt, m.textInput.View())
}

// TextInput shows a text input prompt and returns the user's input.
func TextInput(prompt, placeholder string) (TextInputResult, error) {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	model := textInputModel{
		textInput: ti,
		prompt:    prompt,
	}
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return TextInputResult{}, err
	}
	m := finalModel.(textInputModel)
	return TextInputResult{
		Value:     m.textInput.Value(),
		Cancelled: m.cancelled,
	}, nil
}

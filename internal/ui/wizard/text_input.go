package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// TextInputStep allows entering free-form text.
type TextInputStep struct {
	id          string
	title       string
	prompt      string
	input       textinput.Model
	validate    func(string) error
	submitted   bool
	submitValue string
}

// NewTextInput creates a new text input step.
func NewTextInput(id, title, prompt, placeholder string) *TextInputStep {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 156
	ti.Width = 40

	return &TextInputStep{
		id:     id,
		title:  title,
		prompt: prompt,
		input:  ti,
	}
}

func (s *TextInputStep) ID() string    { return s.id }
func (s *TextInputStep) Title() string { return s.title }

func (s *TextInputStep) Init() tea.Cmd {
	s.input.Focus()
	return textinput.Blink
}

func (s *TextInputStep) Update(msg tea.KeyMsg) (Step, tea.Cmd, StepResult) {
	switch msg.String() {
	case "enter", "right":
		value := strings.TrimSpace(s.input.Value())
		if value == "" {
			return s, nil, StepContinue
		}
		if s.validate != nil {
			if err := s.validate(value); err != nil {
				// Could show error, for now just don't advance
				return s, nil, StepContinue
			}
		}
		s.submitted = true
		s.submitValue = value
		return s, nil, StepAdvance
	case "left":
		return s, nil, StepBack
	}

	// Let textinput handle other keys
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd, StepContinue
}

func (s *TextInputStep) View() string {
	var b strings.Builder
	b.WriteString(s.prompt + "\n\n")
	b.WriteString(s.input.View())
	return b.String()
}

func (s *TextInputStep) Help() string {
	return "type text • ← back • enter confirm • esc cancel"
}

func (s *TextInputStep) Value() StepValue {
	return StepValue{
		Key:   s.id,
		Label: s.submitValue,
		Raw:   s.submitValue,
	}
}

func (s *TextInputStep) IsComplete() bool {
	return s.submitted
}

func (s *TextInputStep) Reset() {
	s.input.SetValue("")
	s.submitted = false
	s.submitValue = ""
}

// SetValidate sets a validation function for the input.
// If validation fails, the step won't advance.
func (s *TextInputStep) SetValidate(fn func(string) error) {
	s.validate = fn
}

// SetValue sets the current input value.
func (s *TextInputStep) SetValue(value string) {
	s.input.SetValue(value)
}

// GetValue returns the current input value (not yet submitted).
func (s *TextInputStep) GetValue() string {
	return s.input.Value()
}

// Focus focuses the text input.
func (s *TextInputStep) Focus() tea.Cmd {
	s.input.Focus()
	return textinput.Blink
}

// Blur unfocuses the text input.
func (s *TextInputStep) Blur() {
	s.input.Blur()
}

// IsFocused returns true if the input is focused.
func (s *TextInputStep) IsFocused() bool {
	return s.input.Focused()
}

// SetPlaceholder sets the placeholder text.
func (s *TextInputStep) SetPlaceholder(placeholder string) {
	s.input.Placeholder = placeholder
}

// SetWidth sets the input width.
func (s *TextInputStep) SetWidth(width int) {
	s.input.Width = width
}

// SetCharLimit sets the character limit.
func (s *TextInputStep) SetCharLimit(limit int) {
	s.input.CharLimit = limit
}

// String implements fmt.Stringer for debugging.
func (s *TextInputStep) String() string {
	return fmt.Sprintf("TextInputStep{id=%s, submitted=%v, value=%q}",
		s.id, s.submitted, s.submitValue)
}

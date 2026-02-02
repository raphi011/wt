package steps

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

// TextInputStep allows entering free-form text.
type TextInputStep struct {
	id              string
	title           string
	prompt          string
	input           textinput.Model
	validate        func(string) error
	submitted       bool
	submitValue     string
	validationError string // Error message to display
}

// NewTextInput creates a new text input step.
// By default, uses a blinking bar cursor for better visibility.
func NewTextInput(id, title, prompt, placeholder string) *TextInputStep {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 156
	ti.SetWidth(40)

	// Set default cursor style: bar with blink
	styles := ti.Styles()
	styles.Cursor.Shape = tea.CursorBar
	styles.Cursor.Blink = true
	ti.SetStyles(styles)

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

func (s *TextInputStep) Update(msg tea.KeyPressMsg) (framework.Step, tea.Cmd, framework.StepResult) {
	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(s.input.Value())
		if value == "" {
			s.validationError = "Value cannot be empty"
			return s, nil, framework.StepContinue
		}
		if s.validate != nil {
			if err := s.validate(value); err != nil {
				s.validationError = err.Error()
				return s, nil, framework.StepContinue
			}
		}
		s.validationError = ""
		s.submitted = true
		s.submitValue = value
		return s, nil, framework.StepSubmitIfReady
	case "right":
		value := strings.TrimSpace(s.input.Value())
		if value == "" {
			s.validationError = "Value cannot be empty"
			return s, nil, framework.StepContinue
		}
		if s.validate != nil {
			if err := s.validate(value); err != nil {
				s.validationError = err.Error()
				return s, nil, framework.StepContinue
			}
		}
		s.validationError = ""
		s.submitted = true
		s.submitValue = value
		return s, nil, framework.StepAdvance
	case "left":
		return s, nil, framework.StepBack
	}

	// Clear error when user types
	s.validationError = ""

	// Let textinput handle other keys
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd, framework.StepContinue
}

func (s *TextInputStep) View() string {
	var b strings.Builder
	b.WriteString(s.prompt + "\n\n")
	b.WriteString(s.input.View())
	if s.validationError != "" {
		b.WriteString("\n" + framework.ErrorStyle().Render(s.validationError))
	}
	return b.String()
}

func (s *TextInputStep) Help() string {
	return "type text • ←/→ navigate • enter confirm • esc cancel"
}

func (s *TextInputStep) Value() framework.StepValue {
	return framework.StepValue{
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

func (s *TextInputStep) HasClearableInput() bool {
	return s.input.Value() != ""
}

func (s *TextInputStep) ClearInput() tea.Cmd {
	s.input.SetValue("")
	s.validationError = ""
	return nil
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
	s.input.SetWidth(width)
}

// SetCharLimit sets the character limit.
func (s *TextInputStep) SetCharLimit(limit int) {
	s.input.CharLimit = limit
}

// WithCursor configures the cursor shape and blink behavior.
// Available shapes: tea.CursorBar, tea.CursorBlock, tea.CursorUnderline.
func (s *TextInputStep) WithCursor(shape tea.CursorShape, blink bool) *TextInputStep {
	styles := s.input.Styles()
	styles.Cursor.Shape = shape
	styles.Cursor.Blink = blink
	s.input.SetStyles(styles)
	return s
}

// String implements fmt.Stringer for debugging.
func (s *TextInputStep) String() string {
	return fmt.Sprintf("TextInputStep{id=%s, submitted=%v, value=%q}",
		s.id, s.submitted, s.submitValue)
}

package steps

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

// SingleSelectStep allows selecting one option from a list.
type SingleSelectStep struct {
	width, height int
	id            string
	title         string
	prompt        string
	options       []framework.Option
	cursor        int
	selected      int // -1 if nothing selected yet
}

// NewSingleSelect creates a new single-select step.
func NewSingleSelect(id, title, prompt string, options []framework.Option) *SingleSelectStep {
	// Find first non-disabled option for initial cursor
	cursor := 0
	for i, opt := range options {
		if !opt.Disabled {
			cursor = i
			break
		}
	}

	return &SingleSelectStep{
		id:       id,
		title:    title,
		prompt:   prompt,
		options:  options,
		cursor:   cursor,
		selected: -1,
	}
}

func (s *SingleSelectStep) ID() string    { return s.id }
func (s *SingleSelectStep) Title() string { return s.title }

func (s *SingleSelectStep) Init() tea.Cmd {
	return nil
}

func (s *SingleSelectStep) Update(msg tea.Msg) (framework.Step, tea.Cmd, framework.StepResult) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil, framework.StepContinue
	}
	switch result := framework.Navigation(keyMsg, false); result {
	case framework.StepBack:
		return s, nil, result
	case framework.StepAdvance, framework.StepSubmitIfReady:
		if len(s.options) > 0 && !s.options[s.cursor].Disabled {
			s.selected = s.cursor
			return s, nil, result
		}
		return s, nil, framework.StepContinue
	}
	switch keyMsg.String() {
	case "up", "k":
		s.moveCursorUp()
	case "down", "j":
		s.moveCursorDown()
	case "home", "pgup":
		s.cursor = s.findFirstEnabled()
	case "end", "pgdown":
		s.cursor = s.findLastEnabled()
	}
	return s, nil, framework.StepContinue
}

func (s *SingleSelectStep) SetSize(width, height int) {
	s.width, s.height = max(1, width), max(1, height)
}

func (s *SingleSelectStep) View() string {
	width, height := s.width, s.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 20
	}
	return s.renderList(width, height)
}

func (s *SingleSelectStep) renderList(width, height int) string {
	header := ""
	if s.prompt != "" && height >= 2 {
		header = framework.Fit(s.prompt, width, 1) + "\n"
		height--
	}
	if len(s.options) == 0 {
		return header + framework.Fit("  No options available", width, height)
	}
	return header + renderList(len(s.options), s.cursor, width, height, func(i int) string {
		opt := s.options[i]
		prefix := "  "
		style := framework.OptionNormalStyle()
		if opt.Disabled {
			style = framework.OptionDisabledStyle()
		} else if i == s.cursor {
			prefix = "> "
			style = framework.OptionSelectedStyle()
		}
		return optionRow(prefix, style.Render(opt.Label), framework.OptionDescriptionStyle().Render(opt.Description), width)
	})
}

func (s *SingleSelectStep) Help() string {
	return "↑/↓ select • " + framework.NavigationHelp(false) + " • " + framework.CancellationHelp(false, "")
}

func (s *SingleSelectStep) Value() framework.StepValue {
	if s.selected < 0 || s.selected >= len(s.options) {
		return framework.StepValue{Key: s.id}
	}
	opt := s.options[s.selected]
	return framework.StepValue{
		Key:   s.id,
		Label: opt.Label,
		Raw:   opt.Value,
	}
}

func (s *SingleSelectStep) IsComplete() bool {
	return s.selected >= 0
}

func (s *SingleSelectStep) Reset() {
	s.selected = -1
	s.cursor = s.findFirstEnabled()
}

func (s *SingleSelectStep) HasClearableInput() bool {
	return false // SingleSelect has no text input
}

func (s *SingleSelectStep) ClearInput() tea.Cmd {
	// No-op: SingleSelect has no text input to clear
	return nil
}

// SetOptions updates the options list (useful for dynamic content).
func (s *SingleSelectStep) SetOptions(options []framework.Option) {
	s.options = options
	// Reset cursor to first non-disabled option
	s.cursor = s.findFirstEnabled()
	// Clear selection if current selection is now out of bounds
	if s.selected >= len(options) {
		s.selected = -1
	}
}

// GetCursor returns the current cursor position.
func (s *SingleSelectStep) GetCursor() int {
	return s.cursor
}

// SetCursor sets the cursor position.
func (s *SingleSelectStep) SetCursor(pos int) {
	if pos >= 0 && pos < len(s.options) && !s.options[pos].Disabled {
		s.cursor = pos
	}
}

// GetSelectedIndex returns the selected index.
func (s *SingleSelectStep) GetSelectedIndex() int {
	return s.selected
}

func (s *SingleSelectStep) moveCursorUp() {
	for i := s.cursor - 1; i >= 0; i-- {
		if !s.options[i].Disabled {
			s.cursor = i
			return
		}
	}
}

func (s *SingleSelectStep) moveCursorDown() {
	for i := s.cursor + 1; i < len(s.options); i++ {
		if !s.options[i].Disabled {
			s.cursor = i
			return
		}
	}
}

func (s *SingleSelectStep) findFirstEnabled() int {
	for i, opt := range s.options {
		if !opt.Disabled {
			return i
		}
	}
	return 0
}

func (s *SingleSelectStep) findLastEnabled() int {
	for i, v := range slices.Backward(s.options) {
		if !v.Disabled {
			return i
		}
	}
	return max(0, len(s.options)-1)
}

// DisableOption disables an option by index and adds a description.
func (s *SingleSelectStep) DisableOption(index int, reason string) {
	if index >= 0 && index < len(s.options) {
		s.options[index].Disabled = true
		s.options[index].Description = reason
		// Move cursor if it was on this option
		if s.cursor == index {
			s.cursor = s.findFirstEnabled()
		}
	}
}

// EnableAllOptions enables all options.
func (s *SingleSelectStep) EnableAllOptions() {
	for i := range s.options {
		s.options[i].Disabled = false
		s.options[i].Description = ""
	}
}

// RenderWithScroll displays the step with optional scrolling for long lists.
func (s *SingleSelectStep) RenderWithScroll(maxVisible int) string {
	width := s.width
	if width == 0 {
		width = 80
	}
	return s.renderList(width, max(1, maxVisible))
}

// OptionsCount returns the number of options.
func (s *SingleSelectStep) OptionsCount() int {
	return len(s.options)
}

// GetOption returns the option at index.
func (s *SingleSelectStep) GetOption(index int) (framework.Option, bool) {
	if index < 0 || index >= len(s.options) {
		return framework.Option{}, false
	}
	return s.options[index], true
}

// FormatValue formats the value for display in summary.
func (s *SingleSelectStep) FormatValue(displayLabels map[any]string) string {
	if s.selected < 0 || s.selected >= len(s.options) {
		return ""
	}
	opt := s.options[s.selected]
	if label, ok := displayLabels[opt.Value]; ok {
		return label
	}
	return opt.Label
}

// String implements fmt.Stringer for debugging.
func (s *SingleSelectStep) String() string {
	return fmt.Sprintf("SingleSelectStep{id=%s, cursor=%d, selected=%d, options=%d}",
		s.id, s.cursor, s.selected, len(s.options))
}

func (s *SingleSelectStep) CompactHelp() string {
	return "↑/↓ select • " + framework.BindingHelp(framework.Keys.Confirm, framework.Keys.Back) + " • " + framework.CancellationHelp(false, "")
}

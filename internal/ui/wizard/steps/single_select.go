package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

// SingleSelectStep allows selecting one option from a list.
type SingleSelectStep struct {
	id       string
	title    string
	prompt   string
	options  []framework.Option
	cursor   int
	selected int // -1 if nothing selected yet
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

func (s *SingleSelectStep) Update(msg tea.KeyMsg) (framework.Step, tea.Cmd, framework.StepResult) {
	switch msg.String() {
	case "up", "k":
		s.moveCursorUp()
	case "down", "j":
		s.moveCursorDown()
	case "home", "pgup":
		s.cursor = s.findFirstEnabled()
	case "end", "pgdown":
		s.cursor = s.findLastEnabled()
	case "enter":
		if len(s.options) > 0 && !s.options[s.cursor].Disabled {
			s.selected = s.cursor
			return s, nil, framework.StepSubmitIfReady
		}
	case "right":
		if len(s.options) > 0 && !s.options[s.cursor].Disabled {
			s.selected = s.cursor
			return s, nil, framework.StepAdvance
		}
	case "left":
		return s, nil, framework.StepBack
	}
	return s, nil, framework.StepContinue
}

func (s *SingleSelectStep) View() string {
	var b strings.Builder
	b.WriteString(s.prompt)
	b.WriteString("\n\n")

	for i, opt := range s.options {
		cursor := "  "
		style := framework.OptionNormalStyle()

		if opt.Disabled {
			style = framework.OptionDisabledStyle()
			label := opt.Label
			if opt.Description != "" {
				label += " (" + opt.Description + ")"
			}
			b.WriteString("  " + style.Render(label) + "\n")
			continue
		}

		if i == s.cursor {
			cursor = "> "
			style = framework.OptionSelectedStyle()
		}

		b.WriteString(cursor + style.Render(opt.Label) + "\n")
		if opt.Description != "" {
			b.WriteString("    " + framework.OptionDescriptionStyle().Render(opt.Description) + "\n")
		}
	}

	return b.String()
}

func (s *SingleSelectStep) Help() string {
	return "↑/↓ select • ←/→ navigate • enter confirm • esc cancel"
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
	for i := len(s.options) - 1; i >= 0; i-- {
		if !s.options[i].Disabled {
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
	var b strings.Builder
	b.WriteString(s.prompt)
	b.WriteString("\n\n")

	start := 0
	if s.cursor >= maxVisible {
		start = s.cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(s.options))

	if start > 0 {
		b.WriteString(framework.OptionNormalStyle().Render("  ↑ more above") + "\n")
	}

	for i := start; i < end; i++ {
		opt := s.options[i]
		cursor := "  "
		style := framework.OptionNormalStyle()

		if opt.Disabled {
			style = framework.OptionDisabledStyle()
			label := opt.Label
			if opt.Description != "" {
				label += " (" + opt.Description + ")"
			}
			b.WriteString("  " + style.Render(label) + "\n")
			continue
		}

		if i == s.cursor {
			cursor = "> "
			style = framework.OptionSelectedStyle()
		}

		b.WriteString(cursor + style.Render(opt.Label) + "\n")
		if opt.Description != "" {
			b.WriteString("    " + framework.OptionDescriptionStyle().Render(opt.Description) + "\n")
		}
	}

	if end < len(s.options) {
		b.WriteString(framework.OptionNormalStyle().Render("  ↓ more below") + "\n")
	}

	if len(s.options) == 0 {
		b.WriteString(framework.OptionNormalStyle().Render("  No options available") + "\n")
	}

	return b.String()
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
func (s *SingleSelectStep) FormatValue(displayLabels map[interface{}]string) string {
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

package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// FilterableListStep allows selecting one option from a filterable list
// with support for disabled items (cursor skips them).
type FilterableListStep struct {
	id       string
	title    string
	prompt   string
	options  []Option
	filtered []int // indices into options
	cursor   int   // position in filtered list
	selected int   // selected index in filtered list, -1 if none
	filter   string
}

// NewFilterableList creates a new filterable single-select step.
func NewFilterableList(id, title, prompt string, options []Option) *FilterableListStep {
	// Build initial filtered list (all indices)
	filtered := make([]int, len(options))
	for i := range options {
		filtered[i] = i
	}

	// Find first non-disabled option
	cursor := 0
	for i, opt := range options {
		if !opt.Disabled {
			cursor = i
			break
		}
	}

	return &FilterableListStep{
		id:       id,
		title:    title,
		prompt:   prompt,
		options:  options,
		filtered: filtered,
		cursor:   cursor,
		selected: -1,
	}
}

func (s *FilterableListStep) ID() string    { return s.id }
func (s *FilterableListStep) Title() string { return s.title }

func (s *FilterableListStep) Init() tea.Cmd {
	return nil
}

func (s *FilterableListStep) Update(msg tea.KeyMsg) (Step, tea.Cmd, StepResult) {
	key := msg.String()

	switch key {
	case "up", "k":
		s.moveCursorUp()
	case "down", "j":
		s.moveCursorDown()
	case "home", "pgup":
		s.cursor = s.findFirstEnabled()
	case "end", "pgdown":
		s.cursor = s.findLastEnabled()
	case "enter", "right":
		if len(s.filtered) > 0 && s.cursor >= 0 && s.cursor < len(s.filtered) {
			idx := s.filtered[s.cursor]
			if !s.options[idx].Disabled {
				s.selected = s.cursor
				return s, nil, StepAdvance
			}
		}
	case "left":
		return s, nil, StepBack
	case "backspace":
		if len(s.filter) > 0 {
			s.filter = s.filter[:len(s.filter)-1]
			s.applyFilter()
		}
	case "alt+backspace":
		if len(s.filter) > 0 {
			s.filter = DeleteLastWord(s.filter)
			s.applyFilter()
		}
	default:
		// Handle typing for filter
		if IsPrintable(key) {
			s.filter += key
			s.applyFilter()
		}
	}

	return s, nil, StepContinue
}

func (s *FilterableListStep) View() string {
	var b strings.Builder
	b.WriteString(s.prompt + ":\n")
	b.WriteString(FilterLabelStyle.Render("Filter: ") + FilterStyle.Render(s.filter) + "\n\n")

	// Show filtered list with scroll
	maxVisible := 10
	start := 0
	if s.cursor >= maxVisible {
		start = s.cursor - maxVisible + 1
	}
	end := min(start+maxVisible, len(s.filtered))

	if start > 0 {
		b.WriteString(OptionNormalStyle.Render("  ↑ more above") + "\n")
	}

	for i := start; i < end; i++ {
		idx := s.filtered[i]
		opt := s.options[idx]

		cursor := "  "
		style := OptionNormalStyle

		if opt.Disabled {
			style = OptionDisabledStyle
			label := opt.Label
			if opt.Description != "" {
				label += " (" + opt.Description + ")"
			}
			b.WriteString("  " + style.Render(label) + "\n")
			continue
		}

		if i == s.cursor {
			cursor = "> "
			style = OptionSelectedStyle
		}

		b.WriteString(cursor + style.Render(opt.Label) + "\n")
	}

	if end < len(s.filtered) {
		b.WriteString(OptionNormalStyle.Render("  ↓ more below") + "\n")
	}

	if len(s.filtered) == 0 {
		b.WriteString(OptionNormalStyle.Render("  No matching items") + "\n")
	}

	return b.String()
}

func (s *FilterableListStep) Help() string {
	return "↑/↓ select • pgup/pgdn jump • type to filter • ← back • enter confirm • esc cancel"
}

func (s *FilterableListStep) Value() StepValue {
	if s.selected < 0 || s.selected >= len(s.filtered) {
		return StepValue{Key: s.id}
	}
	idx := s.filtered[s.selected]
	opt := s.options[idx]
	return StepValue{
		Key:   s.id,
		Label: opt.Label,
		Raw:   opt.Value,
	}
}

func (s *FilterableListStep) IsComplete() bool {
	return s.selected >= 0
}

func (s *FilterableListStep) Reset() {
	s.selected = -1
	s.filter = ""
	s.applyFilter()
}

// SetOptions updates the options list.
func (s *FilterableListStep) SetOptions(options []Option) {
	s.options = options
	s.applyFilter()
}

// GetFilter returns the current filter string.
func (s *FilterableListStep) GetFilter() string {
	return s.filter
}

// GetCursor returns the current cursor position in the filtered list.
func (s *FilterableListStep) GetCursor() int {
	return s.cursor
}

// GetSelectedValue returns the selected option's value, or nil if none.
func (s *FilterableListStep) GetSelectedValue() interface{} {
	if s.selected < 0 || s.selected >= len(s.filtered) {
		return nil
	}
	idx := s.filtered[s.selected]
	return s.options[idx].Value
}

// GetSelectedLabel returns the selected option's label, or empty if none.
func (s *FilterableListStep) GetSelectedLabel() string {
	if s.selected < 0 || s.selected >= len(s.filtered) {
		return ""
	}
	idx := s.filtered[s.selected]
	return s.options[idx].Label
}

func (s *FilterableListStep) applyFilter() {
	if s.filter == "" {
		s.filtered = make([]int, len(s.options))
		for i := range s.options {
			s.filtered[i] = i
		}
	} else {
		filter := strings.ToLower(s.filter)
		s.filtered = nil
		for i, opt := range s.options {
			if strings.Contains(strings.ToLower(opt.Label), filter) {
				s.filtered = append(s.filtered, i)
			}
		}
	}
	// Reset cursor if out of bounds
	if s.cursor >= len(s.filtered) {
		s.cursor = max(0, len(s.filtered)-1)
	}
	// Ensure cursor is on a non-disabled item
	if len(s.filtered) > 0 {
		idx := s.filtered[s.cursor]
		if s.options[idx].Disabled {
			// Try to find next non-disabled
			if next := s.findNextEnabled(s.cursor); next >= 0 {
				s.cursor = next
			} else if prev := s.findPrevEnabled(s.cursor); prev >= 0 {
				s.cursor = prev
			}
		}
	}
}

func (s *FilterableListStep) moveCursorUp() {
	if prev := s.findPrevEnabled(s.cursor - 1); prev >= 0 {
		s.cursor = prev
	}
}

func (s *FilterableListStep) moveCursorDown() {
	if next := s.findNextEnabled(s.cursor + 1); next >= 0 {
		s.cursor = next
	}
}

func (s *FilterableListStep) findFirstEnabled() int {
	for i := 0; i < len(s.filtered); i++ {
		idx := s.filtered[i]
		if !s.options[idx].Disabled {
			return i
		}
	}
	return 0
}

func (s *FilterableListStep) findLastEnabled() int {
	for i := len(s.filtered) - 1; i >= 0; i-- {
		idx := s.filtered[i]
		if !s.options[idx].Disabled {
			return i
		}
	}
	return max(0, len(s.filtered)-1)
}

func (s *FilterableListStep) findNextEnabled(from int) int {
	for i := from; i < len(s.filtered); i++ {
		idx := s.filtered[i]
		if !s.options[idx].Disabled {
			return i
		}
	}
	return -1
}

func (s *FilterableListStep) findPrevEnabled(from int) int {
	for i := from; i >= 0; i-- {
		idx := s.filtered[i]
		if !s.options[idx].Disabled {
			return i
		}
	}
	return -1
}

// HasSelectableOptions returns true if there's at least one non-disabled option.
func (s *FilterableListStep) HasSelectableOptions() bool {
	for _, opt := range s.options {
		if !opt.Disabled {
			return true
		}
	}
	return false
}

// OptionsCount returns the total number of options.
func (s *FilterableListStep) OptionsCount() int {
	return len(s.options)
}

// FilteredCount returns the number of options matching the filter.
func (s *FilterableListStep) FilteredCount() int {
	return len(s.filtered)
}

// String implements fmt.Stringer for debugging.
func (s *FilterableListStep) String() string {
	return fmt.Sprintf("FilterableListStep{id=%s, cursor=%d, selected=%d, filter=%q}",
		s.id, s.cursor, s.selected, s.filter)
}

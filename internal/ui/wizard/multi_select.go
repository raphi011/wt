package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// MultiSelectStep allows selecting multiple options from a filterable list.
type MultiSelectStep struct {
	id        string
	title     string
	prompt    string
	options   []Option
	filtered  []int // indices into options
	cursor    int   // position in filtered list
	selected  map[int]bool
	filter    string
	minSelect int // minimum required selections (0 = no min)
	maxSelect int // maximum allowed selections (0 = no max)

	// Input filtering
	runeFilter RuneFilter // nil = allow all printable
}

// NewMultiSelect creates a new multi-select step.
func NewMultiSelect(id, title, prompt string, options []Option) *MultiSelectStep {
	// Build initial filtered list (all indices)
	filtered := make([]int, len(options))
	for i := range options {
		filtered[i] = i
	}

	return &MultiSelectStep{
		id:       id,
		title:    title,
		prompt:   prompt,
		options:  options,
		filtered: filtered,
		cursor:   0,
		selected: make(map[int]bool),
	}
}

func (s *MultiSelectStep) ID() string    { return s.id }
func (s *MultiSelectStep) Title() string { return s.title }

func (s *MultiSelectStep) Init() tea.Cmd {
	return nil
}

func (s *MultiSelectStep) Update(msg tea.KeyMsg) (Step, tea.Cmd, StepResult) {
	key := msg.String()

	switch key {
	case "up":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down":
		if s.cursor < len(s.filtered)-1 {
			s.cursor++
		}
	case "home", "pgup":
		s.cursor = 0
	case "end", "pgdown":
		if len(s.filtered) > 0 {
			s.cursor = len(s.filtered) - 1
		}
	case " ": // Space toggles selection
		if len(s.filtered) > 0 {
			idx := s.filtered[s.cursor]
			if s.selected[idx] {
				delete(s.selected, idx)
			} else {
				// Check max selection constraint
				if s.maxSelect == 0 || len(s.selected) < s.maxSelect {
					s.selected[idx] = true
				}
			}
			// Re-sort to keep selected items at top
			s.applyFilter()
		}
	case "enter", "right":
		if s.canAdvance() {
			return s, nil, StepAdvance
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
		// Handle typing/pasting for filter
		if msg.Type == tea.KeyRunes {
			if text := FilterRunes(msg.Runes, s.runeFilter); text != "" {
				s.filter += text
				s.applyFilter()
			}
		}
	}

	return s, nil, StepContinue
}

func (s *MultiSelectStep) View() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s (%d selected):\n", s.prompt, len(s.selected)))
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
		if i == s.cursor {
			cursor = "> "
			style = OptionSelectedStyle
		}

		checkbox := "[ ]"
		if s.selected[idx] {
			checkbox = "[✓]"
		}

		b.WriteString(cursor + checkbox + " " + style.Render(opt.Label) + "\n")
		if opt.Description != "" {
			b.WriteString("      " + OptionDescriptionStyle.Render(opt.Description) + "\n")
		}
	}

	if end < len(s.filtered) {
		b.WriteString(OptionNormalStyle.Render("  ↓ more below") + "\n")
	}

	if len(s.filtered) == 0 {
		b.WriteString(OptionNormalStyle.Render("  No matching items") + "\n")
	}

	return b.String()
}

func (s *MultiSelectStep) Help() string {
	return "↑/↓ move • pgup/pgdn jump • space toggle • type to filter • enter confirm • esc cancel"
}

func (s *MultiSelectStep) Value() StepValue {
	var labels []string
	var values []interface{}
	// Iterate in original option order for consistent display
	for idx := 0; idx < len(s.options); idx++ {
		if s.selected[idx] {
			labels = append(labels, s.options[idx].Label)
			values = append(values, s.options[idx].Value)
		}
	}
	return StepValue{
		Key:   s.id,
		Label: strings.Join(labels, ", "),
		Raw:   values,
	}
}

func (s *MultiSelectStep) IsComplete() bool {
	return s.canAdvance()
}

func (s *MultiSelectStep) Reset() {
	s.selected = make(map[int]bool)
	s.filter = ""
	s.applyFilter()
}

// SetMinMax sets the minimum and maximum selection constraints.
func (s *MultiSelectStep) SetMinMax(minSel, maxSel int) {
	s.minSelect = minSel
	s.maxSelect = maxSel
}

// WithRuneFilter sets a filter for allowed input characters.
func (s *MultiSelectStep) WithRuneFilter(f RuneFilter) *MultiSelectStep {
	s.runeFilter = f
	return s
}

// SetOptions updates the options list.
func (s *MultiSelectStep) SetOptions(options []Option) {
	s.options = options
	s.applyFilter()
	// Clean up selected items that no longer exist
	for idx := range s.selected {
		if idx >= len(options) {
			delete(s.selected, idx)
		}
	}
}

// SetSelected sets the selected indices.
func (s *MultiSelectStep) SetSelected(indices []int) {
	s.selected = make(map[int]bool)
	for _, idx := range indices {
		if idx >= 0 && idx < len(s.options) {
			s.selected[idx] = true
		}
	}
	// Re-sort to keep selected items at top
	s.applyFilter()
}

// GetSelectedIndices returns the selected option indices in original order.
func (s *MultiSelectStep) GetSelectedIndices() []int {
	var indices []int
	for idx := 0; idx < len(s.options); idx++ {
		if s.selected[idx] {
			indices = append(indices, idx)
		}
	}
	return indices
}

// SelectedCount returns the number of selected items.
func (s *MultiSelectStep) SelectedCount() int {
	return len(s.selected)
}

// GetFilter returns the current filter string.
func (s *MultiSelectStep) GetFilter() string {
	return s.filter
}

// GetCursor returns the current cursor position.
func (s *MultiSelectStep) GetCursor() int {
	return s.cursor
}

func (s *MultiSelectStep) applyFilter() {
	// First, collect all matching indices
	var matching []int
	if s.filter == "" {
		matching = make([]int, len(s.options))
		for i := range s.options {
			matching[i] = i
		}
	} else {
		filter := strings.ToLower(s.filter)
		for i, opt := range s.options {
			if strings.Contains(strings.ToLower(opt.Label), filter) {
				matching = append(matching, i)
			}
		}
	}

	// Sort: selected items first, then unselected (maintaining relative order within each group)
	var selectedItems []int
	var unselectedItems []int
	for _, idx := range matching {
		if s.selected[idx] {
			selectedItems = append(selectedItems, idx)
		} else {
			unselectedItems = append(unselectedItems, idx)
		}
	}
	s.filtered = append(selectedItems, unselectedItems...)

	// Reset cursor if out of bounds
	if s.cursor >= len(s.filtered) {
		s.cursor = max(0, len(s.filtered)-1)
	}
}

func (s *MultiSelectStep) canAdvance() bool {
	if s.minSelect > 0 && len(s.selected) < s.minSelect {
		return false
	}
	return len(s.selected) > 0 || s.minSelect == 0
}

// String implements fmt.Stringer for debugging.
func (s *MultiSelectStep) String() string {
	return fmt.Sprintf("MultiSelectStep{id=%s, cursor=%d, selected=%d, filter=%q}",
		s.id, s.cursor, len(s.selected), s.filter)
}

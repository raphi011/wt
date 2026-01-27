package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// FilterableListStep allows selecting one option from a filterable list
// with support for disabled items (cursor skips them).
// Optionally supports a "create from filter" option that appears when the filter
// doesn't match any existing option exactly.
type FilterableListStep struct {
	id       string
	title    string
	prompt   string
	options  []Option
	filtered []int // indices into options
	cursor   int   // position in filtered list (0 = create option if shown)
	selected int   // selected index in filtered list, -1 if none
	filter   string

	// Create-from-filter functionality
	allowCreate   bool                                              // Enable "Create {filter}" option
	createLabelFn func(filter string) string                        // Format create label
	valueLabelFn  func(value string, isNew bool, opt Option) string // Format summary label
	selectedIsNew bool                                              // True if "Create" was selected
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

// WithCreateFromFilter enables the "Create {filter}" option when the filter
// doesn't match any existing option exactly (case-insensitive).
// labelFn formats the create option label, e.g. func(f string) string { return fmt.Sprintf("+ Create %q", f) }
func (s *FilterableListStep) WithCreateFromFilter(labelFn func(filter string) string) *FilterableListStep {
	s.allowCreate = true
	s.createLabelFn = labelFn
	return s
}

// WithValueLabel sets a custom function for formatting the selected value in the summary.
// The function receives the value, whether it's a new/created item, and the original option (if not new).
func (s *FilterableListStep) WithValueLabel(fn func(value string, isNew bool, opt Option) string) *FilterableListStep {
	s.valueLabelFn = fn
	return s
}

// IsCreateSelected returns true if the user selected the "Create {filter}" option.
func (s *FilterableListStep) IsCreateSelected() bool {
	return s.selectedIsNew
}

// shouldShowCreate returns true if the create option should be shown.
// It's shown when: allowCreate is true, filter is non-empty, and no exact match exists.
func (s *FilterableListStep) shouldShowCreate() bool {
	if !s.allowCreate || s.filter == "" {
		return false
	}
	// Check for case-insensitive exact match
	filterLower := strings.ToLower(s.filter)
	for _, opt := range s.options {
		if strings.ToLower(opt.Label) == filterLower {
			return false
		}
	}
	return true
}

func (s *FilterableListStep) Init() tea.Cmd {
	return nil
}

func (s *FilterableListStep) Update(msg tea.KeyMsg) (Step, tea.Cmd, StepResult) {
	key := msg.String()
	showCreate := s.shouldShowCreate()

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
		// Check if selecting create option (cursor 0 when create is shown)
		if showCreate && s.cursor == 0 {
			s.selected = 0
			s.selectedIsNew = true
			return s, nil, StepAdvance
		}
		// Adjust cursor for option selection when create is shown
		optionCursor := s.cursor
		if showCreate {
			optionCursor = s.cursor - 1
		}
		if len(s.filtered) > 0 && optionCursor >= 0 && optionCursor < len(s.filtered) {
			idx := s.filtered[optionCursor]
			if !s.options[idx].Disabled {
				s.selected = s.cursor
				s.selectedIsNew = false
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

	showCreate := s.shouldShowCreate()

	// Calculate total items (create option + filtered options)
	totalItems := len(s.filtered)
	if showCreate {
		totalItems++
	}

	// Show filtered list with scroll
	maxVisible := 10
	start := 0
	if s.cursor >= maxVisible {
		start = s.cursor - maxVisible + 1
	}
	end := min(start+maxVisible, totalItems)

	if start > 0 {
		b.WriteString(OptionNormalStyle.Render("  ↑ more above") + "\n")
	}

	for i := start; i < end; i++ {
		// Handle create option at position 0 when shown
		if showCreate && i == 0 {
			cursor := "  "
			style := OptionNormalStyle
			if s.cursor == 0 {
				cursor = "> "
				style = OptionSelectedStyle
			}
			createLabel := s.createLabelFn(s.filter)
			b.WriteString(cursor + style.Render(createLabel) + "\n")
			continue
		}

		// Adjust index for filtered options when create is shown
		filteredIdx := i
		if showCreate {
			filteredIdx = i - 1
		}

		if filteredIdx < 0 || filteredIdx >= len(s.filtered) {
			continue
		}

		idx := s.filtered[filteredIdx]
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

		label := opt.Label
		if opt.Description != "" {
			label += " (" + opt.Description + ")"
		}
		b.WriteString(cursor + style.Render(label) + "\n")
	}

	if end < totalItems {
		b.WriteString(OptionNormalStyle.Render("  ↓ more below") + "\n")
	}

	if totalItems == 0 {
		b.WriteString(OptionNormalStyle.Render("  No matching items") + "\n")
	}

	return b.String()
}

func (s *FilterableListStep) Help() string {
	return "↑/↓ select • pgup/pgdn jump • type to filter • ← back • enter confirm • esc cancel"
}

func (s *FilterableListStep) Value() StepValue {
	if s.selected < 0 {
		return StepValue{Key: s.id}
	}

	// Handle create selection
	if s.selectedIsNew {
		value := s.filter
		label := value
		if s.valueLabelFn != nil {
			label = s.valueLabelFn(value, true, Option{})
		}
		return StepValue{
			Key:   s.id,
			Label: label,
			Raw:   value,
		}
	}

	// Handle existing option selection
	// Adjust for create option offset if it was shown when selected
	optionIdx := s.selected
	if s.shouldShowCreate() {
		optionIdx = s.selected - 1
	}

	if optionIdx < 0 || optionIdx >= len(s.filtered) {
		return StepValue{Key: s.id}
	}

	idx := s.filtered[optionIdx]
	opt := s.options[idx]

	label := opt.Label
	if s.valueLabelFn != nil {
		if strVal, ok := opt.Value.(string); ok {
			label = s.valueLabelFn(strVal, false, opt)
		} else {
			label = s.valueLabelFn(opt.Label, false, opt)
		}
	}

	return StepValue{
		Key:   s.id,
		Label: label,
		Raw:   opt.Value,
	}
}

func (s *FilterableListStep) IsComplete() bool {
	return s.selected >= 0
}

func (s *FilterableListStep) Reset() {
	s.selected = -1
	s.selectedIsNew = false
	// Note: filter is intentionally NOT reset to preserve user input when navigating back
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
// If "Create" was selected, returns the filter string.
func (s *FilterableListStep) GetSelectedValue() interface{} {
	if s.selected < 0 {
		return nil
	}
	if s.selectedIsNew {
		return s.filter
	}
	// Adjust for create option offset
	optionIdx := s.selected
	if s.shouldShowCreate() {
		optionIdx = s.selected - 1
	}
	if optionIdx < 0 || optionIdx >= len(s.filtered) {
		return nil
	}
	idx := s.filtered[optionIdx]
	return s.options[idx].Value
}

// GetSelectedLabel returns the selected option's label, or empty if none.
// If "Create" was selected, returns the filter string.
func (s *FilterableListStep) GetSelectedLabel() string {
	if s.selected < 0 {
		return ""
	}
	if s.selectedIsNew {
		return s.filter
	}
	// Adjust for create option offset
	optionIdx := s.selected
	if s.shouldShowCreate() {
		optionIdx = s.selected - 1
	}
	if optionIdx < 0 || optionIdx >= len(s.filtered) {
		return ""
	}
	idx := s.filtered[optionIdx]
	return s.options[idx].Label
}

// GetSelectedOption returns the selected option, or empty Option if none or if "Create" was selected.
func (s *FilterableListStep) GetSelectedOption() Option {
	if s.selected < 0 || s.selectedIsNew {
		return Option{}
	}
	// Adjust for create option offset
	optionIdx := s.selected
	if s.shouldShowCreate() {
		optionIdx = s.selected - 1
	}
	if optionIdx < 0 || optionIdx >= len(s.filtered) {
		return Option{}
	}
	idx := s.filtered[optionIdx]
	return s.options[idx]
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

	// Calculate total items (including create option if shown)
	showCreate := s.shouldShowCreate()
	totalItems := len(s.filtered)
	if showCreate {
		totalItems++
	}

	// Reset cursor if out of bounds
	if s.cursor >= totalItems {
		s.cursor = max(0, totalItems-1)
	}

	// Ensure cursor is on a non-disabled item (create option is always enabled)
	if showCreate && s.cursor == 0 {
		// Already on create option, which is enabled
		return
	}

	// Check if cursor is on a disabled option
	filteredIdx := s.cursor
	if showCreate {
		filteredIdx = s.cursor - 1
	}

	if filteredIdx >= 0 && filteredIdx < len(s.filtered) {
		idx := s.filtered[filteredIdx]
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
	// Create option at position 0 is always enabled
	if s.shouldShowCreate() {
		return 0
	}
	for i := 0; i < len(s.filtered); i++ {
		idx := s.filtered[i]
		if !s.options[idx].Disabled {
			return i
		}
	}
	return 0
}

func (s *FilterableListStep) findLastEnabled() int {
	showCreate := s.shouldShowCreate()
	offset := 0
	if showCreate {
		offset = 1
	}
	for i := len(s.filtered) - 1; i >= 0; i-- {
		idx := s.filtered[i]
		if !s.options[idx].Disabled {
			return i + offset
		}
	}
	if showCreate {
		return 0 // Create option
	}
	return max(0, len(s.filtered)-1)
}

func (s *FilterableListStep) findNextEnabled(from int) int {
	showCreate := s.shouldShowCreate()

	// Create option at position 0 is always enabled
	if showCreate && from == 0 {
		return 0
	}

	// Calculate total items
	totalItems := len(s.filtered)
	if showCreate {
		totalItems++
	}

	for i := from; i < totalItems; i++ {
		// Create option is always enabled
		if showCreate && i == 0 {
			return 0
		}

		// Adjust for filtered options
		filteredIdx := i
		if showCreate {
			filteredIdx = i - 1
		}

		if filteredIdx >= 0 && filteredIdx < len(s.filtered) {
			idx := s.filtered[filteredIdx]
			if !s.options[idx].Disabled {
				return i
			}
		}
	}
	return -1
}

func (s *FilterableListStep) findPrevEnabled(from int) int {
	showCreate := s.shouldShowCreate()

	for i := from; i >= 0; i-- {
		// Create option is always enabled
		if showCreate && i == 0 {
			return 0
		}

		// Adjust for filtered options
		filteredIdx := i
		if showCreate {
			filteredIdx = i - 1
		}

		if filteredIdx >= 0 && filteredIdx < len(s.filtered) {
			idx := s.filtered[filteredIdx]
			if !s.options[idx].Disabled {
				return i
			}
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

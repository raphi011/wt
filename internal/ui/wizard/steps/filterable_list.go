// Package steps provides reusable step components for wizards.
//
// This package contains implementations of the Step interface that
// can be composed to build interactive wizard flows.
package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

// optionSource implements fuzzy.Source for options.
type optionSource []framework.Option

func (s optionSource) String(i int) string { return s[i].Label }
func (s optionSource) Len() int            { return len(s) }

// FilterableListStep allows selecting one or more options from a filterable list
// with support for disabled items (cursor skips them).
// Uses fuzzy matching for filtering.
// Optionally supports a "create from filter" option that appears when the filter
// doesn't match any existing option exactly.
type FilterableListStep struct {
	id       string
	title    string
	prompt   string
	options  []framework.Option
	filtered []fuzzy.Match // fuzzy matches with indices and matched positions
	cursor   int           // position in filtered list (0 = create option if shown)
	selected int           // selected index in filtered list, -1 if none (single-select mode)
	filter   string

	// Create-from-filter functionality
	allowCreate   bool                                                        // Enable "Create {filter}" option
	createLabelFn func(filter string) string                                  // Format create label
	valueLabelFn  func(value string, isNew bool, opt framework.Option) string // Format summary label
	selectedIsNew bool                                                        // True if "Create" was selected

	// Multi-select mode
	multiSelect   bool         // Enable multi-select mode
	multiSelected map[int]bool // Selected indices in multi-select mode
	minSelect     int          // Minimum required selections (0 = no min)
	maxSelect     int          // Maximum allowed selections (0 = no max)

	// Input filtering
	runeFilter framework.RuneFilter // nil = allow all printable
}

// NewFilterableList creates a new filterable single-select step.
func NewFilterableList(id, title, prompt string, options []framework.Option) *FilterableListStep {
	// Build initial filtered list (all options)
	filtered := make([]fuzzy.Match, len(options))
	for i := range options {
		filtered[i] = fuzzy.Match{
			Str:   options[i].Label,
			Index: i,
		}
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
func (s *FilterableListStep) WithValueLabel(fn func(value string, isNew bool, opt framework.Option) string) *FilterableListStep {
	s.valueLabelFn = fn
	return s
}

// WithRuneFilter sets a filter for allowed input characters.
// Use RuneFilterNoSpaces for branch names or identifiers.
func (s *FilterableListStep) WithRuneFilter(f framework.RuneFilter) *FilterableListStep {
	s.runeFilter = f
	return s
}

// WithMultiSelect enables multi-select mode where users can select multiple options.
// In this mode, space toggles selection and enter confirms when constraints are met.
func (s *FilterableListStep) WithMultiSelect() *FilterableListStep {
	s.multiSelect = true
	s.multiSelected = make(map[int]bool)
	return s
}

// SetMinMax sets the minimum and maximum selection constraints for multi-select mode.
func (s *FilterableListStep) SetMinMax(minSel, maxSel int) *FilterableListStep {
	s.minSelect = minSel
	s.maxSelect = maxSel
	return s
}

// SetSelected sets the selected indices for multi-select mode.
func (s *FilterableListStep) SetSelected(indices []int) *FilterableListStep {
	if s.multiSelected == nil {
		s.multiSelected = make(map[int]bool)
	}
	for _, idx := range indices {
		if idx >= 0 && idx < len(s.options) {
			s.multiSelected[idx] = true
		}
	}
	// Re-sort to keep selected items at top
	s.applyFilter()
	return s
}

// GetSelectedIndices returns the selected option indices in original order (multi-select mode).
func (s *FilterableListStep) GetSelectedIndices() []int {
	var indices []int
	for idx := 0; idx < len(s.options); idx++ {
		if s.multiSelected[idx] {
			indices = append(indices, idx)
		}
	}
	return indices
}

// SelectedCount returns the number of selected items (multi-select mode).
func (s *FilterableListStep) SelectedCount() int {
	return len(s.multiSelected)
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

func (s *FilterableListStep) Update(msg tea.KeyMsg) (framework.Step, tea.Cmd, framework.StepResult) {
	key := msg.String()
	showCreate := s.shouldShowCreate()

	switch key {
	case "up":
		s.moveCursorUp()
	case "down":
		s.moveCursorDown()
	case "home", "pgup":
		s.cursor = s.findFirstEnabled()
	case "end", "pgdown":
		s.cursor = s.findLastEnabled()
	case " ": // Space toggles selection in multi-select mode
		if s.multiSelect && len(s.filtered) > 0 {
			idx := s.filtered[s.cursor].Index
			if s.multiSelected[idx] {
				delete(s.multiSelected, idx)
			} else {
				// Check max selection constraint
				if s.maxSelect == 0 || len(s.multiSelected) < s.maxSelect {
					s.multiSelected[idx] = true
				}
			}
		}
	case "enter", "right":
		// Multi-select mode: advance if constraints are met
		if s.multiSelect {
			if s.canAdvanceMulti() {
				return s, nil, framework.StepAdvance
			}
			return s, nil, framework.StepContinue
		}
		// Single-select mode
		// Check if selecting create option (cursor 0 when create is shown)
		if showCreate && s.cursor == 0 {
			s.selected = 0
			s.selectedIsNew = true
			return s, nil, framework.StepAdvance
		}
		// Adjust cursor for option selection when create is shown
		optionCursor := s.cursor
		if showCreate {
			optionCursor = s.cursor - 1
		}
		if len(s.filtered) > 0 && optionCursor >= 0 && optionCursor < len(s.filtered) {
			idx := s.filtered[optionCursor].Index
			if !s.options[idx].Disabled {
				s.selected = s.cursor
				s.selectedIsNew = false
				return s, nil, framework.StepAdvance
			}
		}
	case "left":
		return s, nil, framework.StepBack
	case "backspace":
		if len(s.filter) > 0 {
			s.filter = s.filter[:len(s.filter)-1]
			s.applyFilter()
		}
	case "alt+backspace":
		if len(s.filter) > 0 {
			s.filter = framework.DeleteLastWord(s.filter)
			s.applyFilter()
		}
	default:
		// Handle typing/pasting for filter
		if msg.Type == tea.KeyRunes {
			if text := framework.FilterRunes(msg.Runes, s.runeFilter); text != "" {
				s.filter += text
				s.applyFilter()
			}
		}
	}

	return s, nil, framework.StepContinue
}

// canAdvanceMulti returns true if multi-select constraints are met.
func (s *FilterableListStep) canAdvanceMulti() bool {
	if s.minSelect > 0 && len(s.multiSelected) < s.minSelect {
		return false
	}
	return len(s.multiSelected) > 0 || s.minSelect == 0
}

func (s *FilterableListStep) View() string {
	var b strings.Builder
	if s.multiSelect {
		b.WriteString(fmt.Sprintf("%s (%d selected):\n", s.prompt, len(s.multiSelected)))
	} else {
		b.WriteString(s.prompt + ":\n")
	}
	b.WriteString(framework.FilterLabelStyle().Render("Filter: ") + framework.FilterStyle().Render(s.filter) + "\n\n")

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
		b.WriteString(framework.OptionNormalStyle().Render("  ↑ more above") + "\n")
	}

	for i := start; i < end; i++ {
		// Handle create option at position 0 when shown
		if showCreate && i == 0 {
			cursor := "  "
			style := framework.OptionNormalStyle()
			if s.cursor == 0 {
				cursor = "> "
				style = framework.OptionSelectedStyle()
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

		match := s.filtered[filteredIdx]
		opt := s.options[match.Index]

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

		// Show checkbox in multi-select mode
		checkbox := ""
		if s.multiSelect {
			if s.multiSelected[match.Index] {
				checkbox = "[✓] "
			} else {
				checkbox = "[ ] "
			}
		}

		// Highlight matched characters if filtering
		label := opt.Label
		if s.filter != "" && len(match.MatchedIndexes) > 0 {
			label = s.highlightMatches(opt.Label, match.MatchedIndexes, i == s.cursor)
		} else {
			label = style.Render(opt.Label)
		}

		b.WriteString(cursor + checkbox + label + "\n")
		if opt.Description != "" {
			descIndent := "    "
			if s.multiSelect {
				descIndent = "      " // Extra indent for checkbox
			}
			b.WriteString(descIndent + framework.OptionDescriptionStyle().Render(opt.Description) + "\n")
		}
	}

	if end < totalItems {
		b.WriteString(framework.OptionNormalStyle().Render("  ↓ more below") + "\n")
	}

	if totalItems == 0 {
		b.WriteString(framework.OptionNormalStyle().Render("  No matching items") + "\n")
	}

	return b.String()
}

func (s *FilterableListStep) Help() string {
	if s.multiSelect {
		return "↑/↓ move • space toggle • pgup/pgdn jump • type to filter • enter confirm • esc cancel"
	}
	return "↑/↓ select • pgup/pgdn jump • type to filter • ← back • enter confirm • esc cancel"
}

func (s *FilterableListStep) Value() framework.StepValue {
	// Multi-select mode
	if s.multiSelect {
		var labels []string
		var values []interface{}
		// Iterate in original option order for consistent display
		for idx := 0; idx < len(s.options); idx++ {
			if s.multiSelected[idx] {
				labels = append(labels, s.options[idx].Label)
				values = append(values, s.options[idx].Value)
			}
		}
		return framework.StepValue{
			Key:   s.id,
			Label: strings.Join(labels, ", "),
			Raw:   values,
		}
	}

	// Single-select mode
	if s.selected < 0 {
		return framework.StepValue{Key: s.id}
	}

	// Handle create selection
	if s.selectedIsNew {
		value := s.filter
		label := value
		if s.valueLabelFn != nil {
			label = s.valueLabelFn(value, true, framework.Option{})
		}
		return framework.StepValue{
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
		return framework.StepValue{Key: s.id}
	}

	idx := s.filtered[optionIdx].Index
	opt := s.options[idx]

	label := opt.Label
	if s.valueLabelFn != nil {
		if strVal, ok := opt.Value.(string); ok {
			label = s.valueLabelFn(strVal, false, opt)
		} else {
			label = s.valueLabelFn(opt.Label, false, opt)
		}
	}

	return framework.StepValue{
		Key:   s.id,
		Label: label,
		Raw:   opt.Value,
	}
}

func (s *FilterableListStep) IsComplete() bool {
	if s.multiSelect {
		return s.canAdvanceMulti()
	}
	return s.selected >= 0
}

func (s *FilterableListStep) Reset() {
	s.selected = -1
	s.selectedIsNew = false
	if s.multiSelect {
		s.multiSelected = make(map[int]bool)
	}
	// Note: filter is intentionally NOT reset to preserve user input when navigating back
}

// SetOptions updates the options list.
func (s *FilterableListStep) SetOptions(options []framework.Option) {
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
	idx := s.filtered[optionIdx].Index
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
	idx := s.filtered[optionIdx].Index
	return s.options[idx].Label
}

// GetSelectedOption returns the selected option, or empty Option if none or if "Create" was selected.
func (s *FilterableListStep) GetSelectedOption() framework.Option {
	if s.selected < 0 || s.selectedIsNew {
		return framework.Option{}
	}
	// Adjust for create option offset
	optionIdx := s.selected
	if s.shouldShowCreate() {
		optionIdx = s.selected - 1
	}
	if optionIdx < 0 || optionIdx >= len(s.filtered) {
		return framework.Option{}
	}
	idx := s.filtered[optionIdx].Index
	return s.options[idx]
}

// highlightMatches renders the label with matched characters highlighted.
func (s *FilterableListStep) highlightMatches(label string, matchedIndexes []int, isSelected bool) string {
	// Create a set of matched indices for quick lookup
	matchSet := make(map[int]bool)
	for _, idx := range matchedIndexes {
		matchSet[idx] = true
	}

	var result strings.Builder
	runes := []rune(label)
	for i, r := range runes {
		char := string(r)
		if matchSet[i] {
			// Highlight matched character
			result.WriteString(framework.MatchHighlightStyle().Render(char))
		} else if isSelected {
			result.WriteString(framework.OptionSelectedStyle().Render(char))
		} else {
			result.WriteString(framework.OptionNormalStyle().Render(char))
		}
	}
	return result.String()
}

func (s *FilterableListStep) applyFilter() {
	var matching []fuzzy.Match

	if s.filter == "" {
		// No filter - show all options in original order
		matching = make([]fuzzy.Match, len(s.options))
		for i := range s.options {
			matching[i] = fuzzy.Match{
				Str:   s.options[i].Label,
				Index: i,
			}
		}
	} else {
		// Apply fuzzy search - results are sorted by score (best first)
		matching = fuzzy.FindFrom(s.filter, optionSource(s.options))
	}

	s.filtered = matching

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
		idx := s.filtered[filteredIdx].Index
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
		idx := s.filtered[i].Index
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
		idx := s.filtered[i].Index
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
			idx := s.filtered[filteredIdx].Index
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
			idx := s.filtered[filteredIdx].Index
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

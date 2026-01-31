package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

// FuzzyListStep allows selecting one option from a fuzzy-filtered list.
// Uses the sahilm/fuzzy library for ranking matches.
type FuzzyListStep struct {
	id       string
	title    string
	prompt   string
	options  []Option
	filtered []fuzzy.Match // matches with scores and indices
	cursor   int           // position in filtered list
	selected int           // selected index in filtered list, -1 if none
	filter   string

	// Input filtering
	runeFilter RuneFilter // nil = allow all printable
}

// optionSource implements fuzzy.Source for our options.
type optionSource []Option

func (s optionSource) String(i int) string { return s[i].Label }
func (s optionSource) Len() int            { return len(s) }

// NewFuzzyList creates a new fuzzy single-select step.
func NewFuzzyList(id, title, prompt string, options []Option) *FuzzyListStep {
	s := &FuzzyListStep{
		id:       id,
		title:    title,
		prompt:   prompt,
		options:  options,
		cursor:   0,
		selected: -1,
	}
	s.applyFilter()
	return s
}

func (s *FuzzyListStep) ID() string    { return s.id }
func (s *FuzzyListStep) Title() string { return s.title }

// WithRuneFilter sets a filter for allowed input characters.
func (s *FuzzyListStep) WithRuneFilter(f RuneFilter) *FuzzyListStep {
	s.runeFilter = f
	return s
}

func (s *FuzzyListStep) Init() tea.Cmd {
	return nil
}

func (s *FuzzyListStep) Update(msg tea.KeyMsg) (Step, tea.Cmd, StepResult) {
	key := msg.String()

	switch key {
	case "up":
		s.moveCursorUp()
	case "down":
		s.moveCursorDown()
	case "home", "pgup":
		s.cursor = s.findFirstEnabled()
	case "end", "pgdown":
		s.cursor = s.findLastEnabled()
	case "enter", "right":
		if len(s.filtered) > 0 && s.cursor >= 0 && s.cursor < len(s.filtered) {
			idx := s.filtered[s.cursor].Index
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

func (s *FuzzyListStep) View() string {
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
		match := s.filtered[i]
		opt := s.options[match.Index]

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

		// Highlight matched characters if filtering
		label := opt.Label
		if s.filter != "" && len(match.MatchedIndexes) > 0 {
			label = s.highlightMatches(opt.Label, match.MatchedIndexes, i == s.cursor)
		} else {
			label = style.Render(opt.Label)
		}

		b.WriteString(cursor + label + "\n")
		if opt.Description != "" {
			b.WriteString("    " + OptionDescriptionStyle.Render(opt.Description) + "\n")
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

// highlightMatches renders the label with matched characters highlighted.
func (s *FuzzyListStep) highlightMatches(label string, matchedIndexes []int, isSelected bool) string {
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
			result.WriteString(MatchHighlightStyle.Render(char))
		} else if isSelected {
			result.WriteString(OptionSelectedStyle.Render(char))
		} else {
			result.WriteString(OptionNormalStyle.Render(char))
		}
	}
	return result.String()
}

func (s *FuzzyListStep) Help() string {
	return "↑/↓ select • pgup/pgdn jump • type to filter • ← back • enter confirm • esc cancel"
}

func (s *FuzzyListStep) Value() StepValue {
	if s.selected < 0 || s.selected >= len(s.filtered) {
		return StepValue{Key: s.id}
	}

	idx := s.filtered[s.selected].Index
	opt := s.options[idx]

	return StepValue{
		Key:   s.id,
		Label: opt.Label,
		Raw:   opt.Value,
	}
}

func (s *FuzzyListStep) IsComplete() bool {
	return s.selected >= 0
}

func (s *FuzzyListStep) Reset() {
	s.selected = -1
	// Note: filter is intentionally NOT reset to preserve user input when navigating back
}

// GetSelectedValue returns the selected option's value, or nil if none.
func (s *FuzzyListStep) GetSelectedValue() interface{} {
	if s.selected < 0 || s.selected >= len(s.filtered) {
		return nil
	}
	idx := s.filtered[s.selected].Index
	return s.options[idx].Value
}

// GetSelectedLabel returns the selected option's label, or empty if none.
func (s *FuzzyListStep) GetSelectedLabel() string {
	if s.selected < 0 || s.selected >= len(s.filtered) {
		return ""
	}
	idx := s.filtered[s.selected].Index
	return s.options[idx].Label
}

// GetSelectedOption returns the selected option, or empty Option if none.
func (s *FuzzyListStep) GetSelectedOption() Option {
	if s.selected < 0 || s.selected >= len(s.filtered) {
		return Option{}
	}
	idx := s.filtered[s.selected].Index
	return s.options[idx]
}

func (s *FuzzyListStep) applyFilter() {
	if s.filter == "" {
		// No filter - show all options in original order
		s.filtered = make([]fuzzy.Match, len(s.options))
		for i := range s.options {
			s.filtered[i] = fuzzy.Match{
				Str:   s.options[i].Label,
				Index: i,
			}
		}
	} else {
		// Apply fuzzy search - results are sorted by score (best first)
		s.filtered = fuzzy.FindFrom(s.filter, optionSource(s.options))
	}

	// Reset cursor if out of bounds
	if s.cursor >= len(s.filtered) {
		s.cursor = max(0, len(s.filtered)-1)
	}

	// Ensure cursor is on a non-disabled item
	if len(s.filtered) > 0 {
		idx := s.filtered[s.cursor].Index
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

func (s *FuzzyListStep) moveCursorUp() {
	if prev := s.findPrevEnabled(s.cursor - 1); prev >= 0 {
		s.cursor = prev
	}
}

func (s *FuzzyListStep) moveCursorDown() {
	if next := s.findNextEnabled(s.cursor + 1); next >= 0 {
		s.cursor = next
	}
}

func (s *FuzzyListStep) findFirstEnabled() int {
	for i := 0; i < len(s.filtered); i++ {
		idx := s.filtered[i].Index
		if !s.options[idx].Disabled {
			return i
		}
	}
	return 0
}

func (s *FuzzyListStep) findLastEnabled() int {
	for i := len(s.filtered) - 1; i >= 0; i-- {
		idx := s.filtered[i].Index
		if !s.options[idx].Disabled {
			return i
		}
	}
	return max(0, len(s.filtered)-1)
}

func (s *FuzzyListStep) findNextEnabled(from int) int {
	for i := from; i < len(s.filtered); i++ {
		idx := s.filtered[i].Index
		if !s.options[idx].Disabled {
			return i
		}
	}
	return -1
}

func (s *FuzzyListStep) findPrevEnabled(from int) int {
	for i := from; i >= 0; i-- {
		idx := s.filtered[i].Index
		if !s.options[idx].Disabled {
			return i
		}
	}
	return -1
}

// String implements fmt.Stringer for debugging.
func (s *FuzzyListStep) String() string {
	return fmt.Sprintf("FuzzyListStep{id=%s, cursor=%d, selected=%d, filter=%q}",
		s.id, s.cursor, s.selected, s.filter)
}

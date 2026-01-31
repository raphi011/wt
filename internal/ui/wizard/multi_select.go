package wizard

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// MultiSelectStep is a wrapper around FilterableListStep with multi-select enabled.
// Deprecated: Use NewFilterableList(...).WithMultiSelect() instead.
type MultiSelectStep struct {
	*FilterableListStep
}

// NewMultiSelect creates a new multi-select step.
// This is equivalent to NewFilterableList(...).WithMultiSelect().
func NewMultiSelect(id, title, prompt string, options []Option) *MultiSelectStep {
	fl := NewFilterableList(id, title, prompt, options).WithMultiSelect()
	return &MultiSelectStep{FilterableListStep: fl}
}

// Update delegates to FilterableListStep.
func (s *MultiSelectStep) Update(msg tea.KeyMsg) (Step, tea.Cmd, StepResult) {
	step, cmd, result := s.FilterableListStep.Update(msg)
	s.FilterableListStep = step.(*FilterableListStep)
	return s, cmd, result
}

// SetMinMax sets the minimum and maximum selection constraints.
func (s *MultiSelectStep) SetMinMax(minSel, maxSel int) {
	s.FilterableListStep.SetMinMax(minSel, maxSel)
}

// WithRuneFilter sets a filter for allowed input characters.
func (s *MultiSelectStep) WithRuneFilter(f RuneFilter) *MultiSelectStep {
	s.FilterableListStep.WithRuneFilter(f)
	return s
}

// SetOptions updates the options list.
func (s *MultiSelectStep) SetOptions(options []Option) {
	s.FilterableListStep.SetOptions(options)
	// Clean up selected items that no longer exist
	for idx := range s.multiSelected {
		if idx >= len(options) {
			delete(s.multiSelected, idx)
		}
	}
}

// SetSelected sets the selected indices.
func (s *MultiSelectStep) SetSelected(indices []int) {
	s.FilterableListStep.SetSelected(indices)
}

// GetSelectedIndices returns the selected option indices in original order.
func (s *MultiSelectStep) GetSelectedIndices() []int {
	return s.FilterableListStep.GetSelectedIndices()
}

// SelectedCount returns the number of selected items.
func (s *MultiSelectStep) SelectedCount() int {
	return s.FilterableListStep.SelectedCount()
}

// GetFilter returns the current filter string.
func (s *MultiSelectStep) GetFilter() string {
	return s.FilterableListStep.GetFilter()
}

// GetCursor returns the current cursor position.
func (s *MultiSelectStep) GetCursor() int {
	return s.FilterableListStep.GetCursor()
}

// String implements fmt.Stringer for debugging.
func (s *MultiSelectStep) String() string {
	return fmt.Sprintf("MultiSelectStep{id=%s, cursor=%d, selected=%d, filter=%q}",
		s.ID(), s.GetCursor(), s.SelectedCount(), s.GetFilter())
}

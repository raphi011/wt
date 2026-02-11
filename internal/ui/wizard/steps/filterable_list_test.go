package steps

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

func TestFilterableListStep_BasicNavigation(t *testing.T) {
	options := []framework.Option{
		{Label: "apple", Value: "apple"},
		{Label: "banana", Value: "banana"},
		{Label: "cherry", Value: "cherry"},
	}

	t.Run("navigate down moves cursor", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		if step.GetCursor() != 0 {
			t.Errorf("Initial cursor = %d, want 0", step.GetCursor())
		}

		updateStep(t, step, keyMsg("down"))
		if step.GetCursor() != 1 {
			t.Errorf("Cursor after down = %d, want 1", step.GetCursor())
		}
	})

	t.Run("navigate up at top focuses filter", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// First down, then up should work normally
		updateStep(t, step, keyMsg("down"))
		updateStep(t, step, keyMsg("up"))
		if step.GetCursor() != 0 {
			t.Errorf("Cursor after up = %d, want 0", step.GetCursor())
		}
	})

	t.Run("home jumps to first option", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		updateStep(t, step, keyMsg("end"))

		updateStep(t, step, keyMsg("home"))
		if step.GetCursor() != 0 {
			t.Errorf("Cursor after home = %d, want 0", step.GetCursor())
		}
	})

	t.Run("end jumps to last option", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		updateStep(t, step, keyMsg("end"))
		if step.GetCursor() != 2 {
			t.Errorf("Cursor after end = %d, want 2", step.GetCursor())
		}
	})
}

func TestFilterableListStep_FilterInput(t *testing.T) {
	options := []framework.Option{
		{Label: "apple", Value: "apple"},
		{Label: "apricot", Value: "apricot"},
		{Label: "banana", Value: "banana"},
		{Label: "cherry", Value: "cherry"},
	}

	t.Run("typing updates filter", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Type 'a' - should focus filter and start typing
		updateStep(t, step, keyMsg("a"))
		if step.GetFilter() != "a" {
			t.Errorf("Filter = %q, want %q", step.GetFilter(), "a")
		}
	})

	t.Run("filter narrows visible options", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Type "ap" - should filter to apple and apricot
		updateStep(t, step, keyMsg("a"))
		updateStep(t, step, keyMsg("p"))

		if step.FilteredCount() != 2 {
			t.Errorf("FilteredCount = %d, want 2", step.FilteredCount())
		}
		if step.GetFilter() != "ap" {
			t.Errorf("Filter = %q, want %q", step.GetFilter(), "ap")
		}
	})

	t.Run("fuzzy matching works", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Type "ae" - should match "apple" (a...e)
		updateStep(t, step, keyMsg("a"))
		updateStep(t, step, keyMsg("e"))

		if step.FilteredCount() == 0 {
			t.Error("Fuzzy matching should find at least one result for 'ae'")
		}
	})

	t.Run("backspace clears filter character", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		updateStep(t, step, keyMsg("a"))
		updateStep(t, step, keyMsg("b"))
		if step.GetFilter() != "ab" {
			t.Errorf("Filter before backspace = %q, want %q", step.GetFilter(), "ab")
		}

		updateStep(t, step, keyMsg("backspace"))
		if step.GetFilter() != "a" {
			t.Errorf("Filter after backspace = %q, want %q", step.GetFilter(), "a")
		}
	})

	t.Run("ClearInput clears filter", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		updateStep(t, step, keyMsg("a"))
		updateStep(t, step, keyMsg("p"))
		if step.GetFilter() != "ap" {
			t.Errorf("Filter before clear = %q, want %q", step.GetFilter(), "ap")
		}

		step.ClearInput()
		if step.GetFilter() != "" {
			t.Errorf("Filter after clear = %q, want empty", step.GetFilter())
		}
	})

	t.Run("HasClearableInput returns true when filter is set", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		if step.HasClearableInput() {
			t.Error("HasClearableInput should be false initially")
		}

		updateStep(t, step, keyMsg("a"))
		if !step.HasClearableInput() {
			t.Error("HasClearableInput should be true after typing")
		}
	})
}

func TestFilterableListStep_Selection(t *testing.T) {
	options := []framework.Option{
		{Label: "apple", Value: "apple"},
		{Label: "banana", Value: "banana"},
		{Label: "cherry", Value: "cherry"},
	}

	t.Run("enter selects current option", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		updateStep(t, step, keyMsg("down"))

		_, result := updateStep(t, step, keyMsg("enter"))
		if result != framework.StepSubmitIfReady {
			t.Errorf("Result = %v, want StepSubmitIfReady", result)
		}
		if !step.IsComplete() {
			t.Error("Step should be complete")
		}
		if step.GetSelectedLabel() != "banana" {
			t.Errorf("Selected = %s, want banana", step.GetSelectedLabel())
		}
	})

	t.Run("right arrow advances", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		_, result := updateStep(t, step, keyMsg("right"))
		if result != framework.StepAdvance {
			t.Errorf("Result = %v, want StepAdvance", result)
		}
		if !step.IsComplete() {
			t.Error("Step should be complete")
		}
	})

	t.Run("left arrow goes back", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		_, result := updateStep(t, step, keyMsg("left"))
		if result != framework.StepBack {
			t.Errorf("Result = %v, want StepBack", result)
		}
	})

	t.Run("Value returns selected option", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		updateStep(t, step, keyMsg("enter"))

		value := step.Value()
		if value.Key != "test" {
			t.Errorf("Value.Key = %s, want test", value.Key)
		}
		if value.Label != "apple" {
			t.Errorf("Value.Label = %s, want apple", value.Label)
		}
		if value.Raw != "apple" {
			t.Errorf("Value.Raw = %v, want apple", value.Raw)
		}
	})
}

func TestFilterableListStep_CreateFromFilter(t *testing.T) {
	options := []framework.Option{
		{Label: "main", Value: "main"},
		{Label: "develop", Value: "develop"},
	}

	t.Run("create option appears when no exact match", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithCreateFromFilter(func(f string) string {
				return "+ Create " + f
			})

		// Type something that doesn't exist
		updateStep(t, step, keyMsg("f"))
		updateStep(t, step, keyMsg("e"))
		updateStep(t, step, keyMsg("a"))
		updateStep(t, step, keyMsg("t"))

		// The filter should be "feat"
		if step.GetFilter() != "feat" {
			t.Errorf("Filter = %q, want %q", step.GetFilter(), "feat")
		}

		// Select the create option (should be at cursor 0)
		step.SetOptions(options) // Ensure options are set
		updateStep(t, step, keyMsg("enter"))

		if !step.IsCreateSelected() {
			t.Error("Create option should be selected")
		}
		if step.GetSelectedValue() != "feat" {
			t.Errorf("Selected value = %v, want feat", step.GetSelectedValue())
		}
	})

	t.Run("create option does not appear with exact match", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithCreateFromFilter(func(f string) string {
				return "+ Create " + f
			})

		// Type "main" - exact match exists
		for _, r := range "main" {
			updateStep(t, step, tea.KeyPressMsg{Code: r, Text: string(r)})
		}

		// Select should be the "main" option, not create
		updateStep(t, step, keyMsg("enter"))

		if step.IsCreateSelected() {
			t.Error("Create option should not be selected for exact match")
		}
		if step.GetSelectedLabel() != "main" {
			t.Errorf("Selected = %s, want main", step.GetSelectedLabel())
		}
	})
}

func TestFilterableListStep_MultiSelect(t *testing.T) {
	options := []framework.Option{
		{Label: "Option A", Value: "a"},
		{Label: "Option B", Value: "b"},
		{Label: "Option C", Value: "c"},
	}

	t.Run("space toggles selection", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect()

		// Select first item
		updateStep(t, step, keyMsg("space"))
		if step.SelectedCount() != 1 {
			t.Errorf("SelectedCount = %d, want 1", step.SelectedCount())
		}

		// Deselect first item
		updateStep(t, step, keyMsg("space"))
		if step.SelectedCount() != 0 {
			t.Errorf("SelectedCount after toggle = %d, want 0", step.SelectedCount())
		}
	})

	t.Run("can select multiple items", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect()

		// Select first
		updateStep(t, step, keyMsg("space"))
		// Move to second and select
		updateStep(t, step, keyMsg("down"))
		updateStep(t, step, keyMsg("space"))
		// Move to third and select
		updateStep(t, step, keyMsg("down"))
		updateStep(t, step, keyMsg("space"))

		if step.SelectedCount() != 3 {
			t.Errorf("SelectedCount = %d, want 3", step.SelectedCount())
		}

		indices := step.GetSelectedIndices()
		if len(indices) != 3 {
			t.Errorf("GetSelectedIndices len = %d, want 3", len(indices))
		}
	})

	t.Run("SetSelected pre-selects items", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect().
			SetSelected([]int{0, 2})

		if step.SelectedCount() != 2 {
			t.Errorf("SelectedCount = %d, want 2", step.SelectedCount())
		}

		indices := step.GetSelectedIndices()
		if len(indices) != 2 || indices[0] != 0 || indices[1] != 2 {
			t.Errorf("GetSelectedIndices = %v, want [0, 2]", indices)
		}
	})

	t.Run("SetMinMax enforces constraints", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect().
			SetMinMax(2, 3)

		// Try to advance with 0 selections - should not advance
		_, result := updateStep(t, step, keyMsg("enter"))
		if result == framework.StepSubmitIfReady {
			t.Error("Should not submit with 0 selections when min is 2")
		}

		// Select one
		updateStep(t, step, keyMsg("space"))
		_, result = updateStep(t, step, keyMsg("enter"))
		if result == framework.StepSubmitIfReady {
			t.Error("Should not submit with 1 selection when min is 2")
		}

		// Select second
		updateStep(t, step, keyMsg("down"))
		updateStep(t, step, keyMsg("space"))
		_, result = updateStep(t, step, keyMsg("enter"))
		if result != framework.StepSubmitIfReady {
			t.Error("Should submit with 2 selections when min is 2")
		}
	})

	t.Run("Value returns all selected", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect()

		// Select first two
		updateStep(t, step, keyMsg("space"))
		updateStep(t, step, keyMsg("down"))
		updateStep(t, step, keyMsg("space"))

		value := step.Value()
		if value.Key != "test" {
			t.Errorf("Value.Key = %s, want test", value.Key)
		}
		if value.Label != "Option A, Option B" {
			t.Errorf("Value.Label = %s, want 'Option A, Option B'", value.Label)
		}

		raw, ok := value.Raw.([]any)
		if !ok {
			t.Errorf("Value.Raw type = %T, want []interface{}", value.Raw)
		} else if len(raw) != 2 {
			t.Errorf("Value.Raw len = %d, want 2", len(raw))
		}
	})
}

func TestFilterableListStep_DisabledOptions(t *testing.T) {
	options := []framework.Option{
		{Label: "Enabled 1", Value: "e1"},
		{Label: "Disabled", Value: "d1", Disabled: true},
		{Label: "Enabled 2", Value: "e2"},
	}

	t.Run("cursor skips disabled options", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Navigate down should skip disabled option
		updateStep(t, step, keyMsg("down"))
		if step.GetCursor() != 2 {
			t.Errorf("Cursor after down = %d, want 2 (skipped disabled)", step.GetCursor())
		}
	})

	t.Run("cannot select disabled option", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Navigate to last (enabled) option
		updateStep(t, step, keyMsg("down"))
		updateStep(t, step, keyMsg("enter"))

		if step.GetSelectedLabel() != "Enabled 2" {
			t.Errorf("Selected = %s, want Enabled 2", step.GetSelectedLabel())
		}
	})
}

func TestFilterableListStep_Reset(t *testing.T) {
	options := []framework.Option{
		{Label: "apple", Value: "apple"},
		{Label: "banana", Value: "banana"},
	}

	t.Run("reset clears single selection", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		updateStep(t, step, keyMsg("enter"))

		if !step.IsComplete() {
			t.Error("Should be complete after enter")
		}

		step.Reset()

		if step.IsComplete() {
			t.Error("Should not be complete after reset")
		}
	})

	t.Run("reset clears multi selection", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect()

		updateStep(t, step, keyMsg("space"))
		updateStep(t, step, keyMsg("down"))
		updateStep(t, step, keyMsg("space"))

		if step.SelectedCount() != 2 {
			t.Errorf("SelectedCount before reset = %d, want 2", step.SelectedCount())
		}

		step.Reset()

		if step.SelectedCount() != 0 {
			t.Errorf("SelectedCount after reset = %d, want 0", step.SelectedCount())
		}
	})

	t.Run("reset preserves filter", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		updateStep(t, step, keyMsg("a"))

		step.Reset()

		// Filter should be preserved (intentional design decision)
		if step.GetFilter() != "a" {
			t.Errorf("Filter after reset = %q, should be preserved", step.GetFilter())
		}
	})
}

func TestFilterableListStep_Interface(t *testing.T) {
	step := NewFilterableList("test", "Test Title", "Select", nil)

	if step.ID() != "test" {
		t.Errorf("ID() = %s, want test", step.ID())
	}
	if step.Title() != "Test Title" {
		t.Errorf("Title() = %s, want Test Title", step.Title())
	}
	if step.Init() != nil {
		t.Error("Init() should return nil")
	}
	if step.Help() == "" {
		t.Error("Help() should return help text")
	}
	if step.OptionsCount() != 0 {
		t.Errorf("OptionsCount() = %d, want 0", step.OptionsCount())
	}
}

func TestFilterableListStep_RuneFilter(t *testing.T) {
	options := []framework.Option{
		{Label: "branch-name", Value: "branch-name"},
	}

	t.Run("RuneFilterNoSpaces blocks spaces", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithRuneFilter(framework.RuneFilterNoSpaces)

		updateStep(t, step, keyMsg("a"))
		updateStep(t, step, keyMsg("space"))
		updateStep(t, step, keyMsg("b"))

		// Space should be filtered out
		if step.GetFilter() != "ab" {
			t.Errorf("Filter with RuneFilterNoSpaces = %q, want %q", step.GetFilter(), "ab")
		}
	})
}

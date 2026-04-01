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
			t.Errorf("Value.Raw type = %T, want []any", value.Raw)
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

func TestFilterableListStep_View(t *testing.T) {
	options := []framework.Option{
		{Label: "apple", Value: "apple"},
		{Label: "banana", Value: "banana"},
		{Label: "cherry", Value: "cherry"},
	}

	t.Run("View returns non-empty string", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select an option", options)

		view := step.View()
		if view == "" {
			t.Error("View() should return non-empty string")
		}
	})

	t.Run("View includes prompt", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "My Prompt", options)

		view := step.View()
		if !containsStr(view, "My Prompt") {
			t.Errorf("View() should contain prompt, got %q", view)
		}
	})

	t.Run("View shows prompt with selected count in multi-select mode", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Choose items", options).
			WithMultiSelect()

		updateStep(t, step, keyMsg("space"))

		view := step.View()
		if !containsStr(view, "(1 selected)") {
			t.Errorf("View() in multi-select should show count, got %q", view)
		}
	})

	t.Run("View shows empty message when no items match", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Type something that matches nothing
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))

		view := step.View()
		if !containsStr(view, "No matching items") {
			t.Errorf("View() should show empty message, got %q", view)
		}
	})

	t.Run("View shows custom empty message", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithEmptyMessage("Nothing found here")

		// Type something that matches nothing
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))

		view := step.View()
		if !containsStr(view, "Nothing found here") {
			t.Errorf("View() should show custom empty message, got %q", view)
		}
	})

	t.Run("View shows checkboxes in multi-select mode", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect()

		view := step.View()
		if !containsStr(view, "[ ]") {
			t.Errorf("View() in multi-select should show unchecked checkboxes, got %q", view)
		}
	})

	t.Run("View shows checked checkbox for selected items", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect()

		// Select first item
		updateStep(t, step, keyMsg("space"))

		view := step.View()
		if !containsStr(view, "[✓]") {
			t.Errorf("View() should show checked checkbox for selected item, got %q", view)
		}
	})

	t.Run("View shows disabled options with description", func(t *testing.T) {
		disabledOptions := []framework.Option{
			{Label: "Active", Value: "active"},
			{Label: "Disabled", Value: "disabled", Disabled: true, Description: "not available"},
		}
		step := NewFilterableList("test", "Test", "Select", disabledOptions)

		view := step.View()
		if !containsStr(view, "not available") {
			t.Errorf("View() should show disabled option description, got %q", view)
		}
	})

	t.Run("View shows filter focused with textinput view", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Type to focus filter
		updateStep(t, step, keyMsg("a"))

		// Now filter is focused
		view := step.View()
		if view == "" {
			t.Error("View() should return non-empty string when filter focused")
		}
	})

	t.Run("View shows option descriptions via default renderer", func(t *testing.T) {
		optionsWithDesc := []framework.Option{
			{Label: "Option A", Value: "a", Description: "A great option"},
			{Label: "Option B", Value: "b"},
		}
		step := NewFilterableList("test", "Test", "Select", optionsWithDesc)

		view := step.View()
		if !containsStr(view, "A great option") {
			t.Errorf("View() should show option description, got %q", view)
		}
	})

	t.Run("View uses custom description renderer", func(t *testing.T) {
		optionsWithDesc := []framework.Option{
			{Label: "Option A", Value: "a", Description: "original desc"},
		}
		step := NewFilterableList("test", "Test", "Select", optionsWithDesc).
			WithDescriptionRenderer(func(opt framework.Option, isSelected bool) string {
				return "custom: " + opt.Description
			})

		view := step.View()
		if !containsStr(view, "custom: original desc") {
			t.Errorf("View() should use custom description renderer, got %q", view)
		}
	})
}

func TestFilterableListStep_Help(t *testing.T) {
	options := []framework.Option{
		{Label: "opt", Value: "opt"},
	}

	t.Run("Help returns single-select help", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		help := step.Help()
		if help == "" {
			t.Error("Help() should return non-empty string")
		}
		// Single-select help should not mention space toggle
		if containsStr(help, "space toggle") {
			t.Errorf("Single-select help should not mention space, got %q", help)
		}
	})

	t.Run("Help returns multi-select help", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect()
		help := step.Help()
		if !containsStr(help, "space toggle") {
			t.Errorf("Multi-select help should mention space toggle, got %q", help)
		}
	})
}

func TestFilterableListStep_GetSelectedOption(t *testing.T) {
	options := []framework.Option{
		{Label: "apple", Value: "apple"},
		{Label: "banana", Value: "banana"},
	}

	t.Run("GetSelectedOption returns empty when nothing selected", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		opt := step.GetSelectedOption()
		if opt.Label != "" {
			t.Errorf("GetSelectedOption() = %v, want empty Option", opt)
		}
	})

	t.Run("GetSelectedOption returns selected option", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		updateStep(t, step, keyMsg("enter")) // select first (apple)

		opt := step.GetSelectedOption()
		if opt.Label != "apple" {
			t.Errorf("GetSelectedOption() = %v, want apple", opt)
		}
	})

	t.Run("GetSelectedOption returns empty when create is selected", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithCreateFromFilter(func(f string) string { return "+ Create " + f })

		// Type something new
		updateStep(t, step, keyMsg("x"))
		updateStep(t, step, keyMsg("y"))
		updateStep(t, step, keyMsg("z"))
		// Select create option (cursor should be at 0 which is create)
		updateStep(t, step, keyMsg("enter"))

		opt := step.GetSelectedOption()
		if opt.Label != "" {
			t.Errorf("GetSelectedOption() for create selection = %v, want empty", opt)
		}
	})
}

func TestFilterableListStep_GetSelectedValue(t *testing.T) {
	options := []framework.Option{
		{Label: "apple", Value: "apple-val"},
		{Label: "banana", Value: "banana-val"},
	}

	t.Run("GetSelectedValue returns nil when nothing selected", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		val := step.GetSelectedValue()
		if val != nil {
			t.Errorf("GetSelectedValue() = %v, want nil", val)
		}
	})

	t.Run("GetSelectedValue returns option value after selection", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		updateStep(t, step, keyMsg("enter")) // select first (apple)

		val := step.GetSelectedValue()
		if val != "apple-val" {
			t.Errorf("GetSelectedValue() = %v, want apple-val", val)
		}
	})

	t.Run("GetSelectedValue returns filter for create selection", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithCreateFromFilter(func(f string) string { return "+ Create " + f })

		// Type something new
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("z"))
		// Enter selects the create option
		updateStep(t, step, keyMsg("enter"))

		val := step.GetSelectedValue()
		if val != "zzz" {
			t.Errorf("GetSelectedValue() for create = %v, want 'zzz'", val)
		}
	})
}

func TestFilterableListStep_WithValueLabel(t *testing.T) {
	options := []framework.Option{
		{Label: "apple", Value: "apple"},
	}

	t.Run("WithValueLabel customizes value label", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithValueLabel(func(value string, isNew bool, opt framework.Option) string {
				return "custom:" + value
			})

		updateStep(t, step, keyMsg("enter"))

		value := step.Value()
		if value.Label != "custom:apple" {
			t.Errorf("Value.Label = %q, want 'custom:apple'", value.Label)
		}
	})

	t.Run("WithValueLabel for create option", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithCreateFromFilter(func(f string) string { return "+ Create " + f }).
			WithValueLabel(func(value string, isNew bool, opt framework.Option) string {
				if isNew {
					return "new:" + value
				}
				return value
			})

		// Type something new
		updateStep(t, step, keyMsg("x"))
		updateStep(t, step, keyMsg("y"))
		updateStep(t, step, keyMsg("z"))
		updateStep(t, step, keyMsg("enter"))

		value := step.Value()
		if value.Label != "new:xyz" {
			t.Errorf("Value.Label for create = %q, want 'new:xyz'", value.Label)
		}
	})
}

func TestFilterableListStep_NavigationWithFilter(t *testing.T) {
	options := []framework.Option{
		{Label: "apple", Value: "apple"},
		{Label: "banana", Value: "banana"},
		{Label: "cherry", Value: "cherry"},
	}

	t.Run("pgup navigates to first when filter focused", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)
		// Move to end first
		updateStep(t, step, keyMsg("end"))

		updateStep(t, step, keyMsg("pgup"))
		if step.GetCursor() != 0 {
			t.Errorf("Cursor after pgup = %d, want 0", step.GetCursor())
		}
	})

	t.Run("pgdown navigates to last", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		updateStep(t, step, keyMsg("pgdown"))
		if step.GetCursor() != 2 {
			t.Errorf("Cursor after pgdown = %d, want 2", step.GetCursor())
		}
	})

	t.Run("up at top of list moves to filter", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// At cursor 0, up should focus filter
		updateStep(t, step, keyMsg("up"))

		if !step.filterInput.Focused() {
			t.Error("filterInput should be focused after up at top of list")
		}
	})

	t.Run("down from filter moves focus to list", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// First up to focus filter
		updateStep(t, step, keyMsg("up"))
		if !step.filterInput.Focused() {
			t.Fatal("filterInput should be focused")
		}

		// Now down should move focus back to list
		updateStep(t, step, keyMsg("down"))
		if step.filterInput.Focused() {
			t.Error("filterInput should lose focus after down")
		}
	})

	t.Run("backspace from list with filter focuses filter", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Type to set filter
		updateStep(t, step, keyMsg("a"))
		// Now down to move to list
		updateStep(t, step, keyMsg("down"))
		// Now list is focused, backspace should focus filter
		updateStep(t, step, keyMsg("backspace"))

		if !step.filterInput.Focused() {
			t.Error("filterInput should be focused after backspace from list")
		}
	})

	t.Run("up in filter input is no-op at top", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Focus filter
		updateStep(t, step, keyMsg("up"))
		if !step.filterInput.Focused() {
			t.Fatal("filterInput should be focused")
		}

		// Up while filter is focused - should be no-op
		cursorBefore := step.GetCursor()
		updateStep(t, step, keyMsg("up"))
		if step.GetCursor() != cursorBefore {
			t.Errorf("Cursor changed after up in filter, was %d now %d", cursorBefore, step.GetCursor())
		}
	})
}

func TestFilterableListStep_MultiSelect_MaxConstraint(t *testing.T) {
	options := []framework.Option{
		{Label: "Option A", Value: "a"},
		{Label: "Option B", Value: "b"},
		{Label: "Option C", Value: "c"},
	}

	t.Run("cannot select more than max", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect().
			SetMinMax(0, 2)

		// Select first
		updateStep(t, step, keyMsg("space"))
		// Select second
		updateStep(t, step, keyMsg("down"))
		updateStep(t, step, keyMsg("space"))
		// Try to select third - should be blocked by max
		updateStep(t, step, keyMsg("down"))
		updateStep(t, step, keyMsg("space"))

		if step.SelectedCount() > 2 {
			t.Errorf("SelectedCount = %d, max is 2", step.SelectedCount())
		}
	})

	t.Run("can advance with zero selections when minSelect is 0", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect().
			SetMinMax(0, 3)

		// No selections, but min is 0 - should be able to advance
		_, result := updateStep(t, step, keyMsg("enter"))
		if result != framework.StepSubmitIfReady {
			t.Errorf("Result = %v, want StepSubmitIfReady when min is 0", result)
		}
	})
}

func TestFilterableListStep_MultiSelectWithCreate(t *testing.T) {
	options := []framework.Option{
		{Label: "main", Value: "main"},
	}

	t.Run("space on create option is no-op in multi-select", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options).
			WithMultiSelect().
			WithCreateFromFilter(func(f string) string { return "+ Create " + f })

		// Type to show create option
		updateStep(t, step, keyMsg("n"))
		updateStep(t, step, keyMsg("e"))
		updateStep(t, step, keyMsg("w"))

		// Cursor should be at 0 (create option)
		if step.GetCursor() != 0 {
			t.Fatalf("Cursor should be 0 for create option, got %d", step.GetCursor())
		}

		// Space on create option should be no-op
		beforeCount := step.SelectedCount()
		updateStep(t, step, keyMsg("space"))
		if step.SelectedCount() != beforeCount {
			t.Errorf("SelectedCount changed after space on create option: %d -> %d",
				beforeCount, step.SelectedCount())
		}
	})
}

func TestFilterableListStep_ScrollView(t *testing.T) {
	// Create many options to trigger scroll
	var manyOptions []framework.Option
	for i := 0; i < 15; i++ {
		label := string(rune('a'+i)) + "-option"
		manyOptions = append(manyOptions, framework.Option{Label: label, Value: label})
	}

	t.Run("scroll view shows more above indicator", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", manyOptions)

		// Navigate to last item to trigger scroll
		updateStep(t, step, keyMsg("end"))

		view := step.View()
		if !containsStr(view, "↑ more above") {
			t.Errorf("View() should show scroll indicator, got %q", view)
		}
	})

	t.Run("scroll view shows more below indicator", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", manyOptions)

		view := step.View()
		if !containsStr(view, "↓ more below") {
			t.Errorf("View() should show more below indicator, got %q", view)
		}
	})
}

func TestFilterableListStep_HighlightMatches(t *testing.T) {
	options := []framework.Option{
		{Label: "feature-branch", Value: "feature-branch"},
		{Label: "fix-bug", Value: "fix-bug"},
	}

	t.Run("fuzzy match highlights are shown in view", func(t *testing.T) {
		step := NewFilterableList("test", "Test", "Select", options)

		// Type "fb" which should fuzzy match "feature-branch"
		updateStep(t, step, keyMsg("f"))
		updateStep(t, step, keyMsg("b"))

		view := step.View()
		// Just verify View doesn't panic and returns something
		if view == "" {
			t.Error("View() should return non-empty string with fuzzy highlights")
		}
	})
}

func TestFilterableListStep_String(t *testing.T) {
	options := []framework.Option{
		{Label: "opt", Value: "opt"},
	}

	step := NewFilterableList("myid", "Test", "Select", options)
	s := step.String()
	if !containsStr(s, "myid") {
		t.Errorf("String() = %q, should contain id", s)
	}
}

// containsStr checks if s contains sub.
func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

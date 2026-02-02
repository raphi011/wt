package steps

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

// keyMsg creates a tea.KeyPressMsg from a string key.
// Supports: "enter", "up", "down", "left", "right", "home", "end", "pgup", "pgdown",
// "esc", "space", and single character keys like "a", "k", "j".
func keyMsg(key string) tea.KeyPressMsg {
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	default:
		// Single character key
		if len(key) == 1 {
			r := rune(key[0])
			return tea.KeyPressMsg{Code: r, Text: key}
		}
		return tea.KeyPressMsg{}
	}
}

// updateStep is a helper that performs Update and returns the concrete type.
func updateStep[T framework.Step](t *testing.T, s T, msg tea.KeyPressMsg) (T, framework.StepResult) {
	t.Helper()
	result, _, stepResult := s.Update(msg)
	concrete, ok := result.(T)
	if !ok {
		t.Fatalf("Update returned unexpected type: %T", result)
	}
	return concrete, stepResult
}

func TestSingleSelectStep_Navigation(t *testing.T) {
	options := []framework.Option{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
		{Label: "Option 3", Value: "opt3"},
	}

	t.Run("navigate down moves cursor", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)

		if step.GetCursor() != 0 {
			t.Errorf("Initial cursor = %d, want 0", step.GetCursor())
		}

		// Navigate down
		_, result := updateStep(t, step, keyMsg("down"))
		if result != framework.StepContinue {
			t.Errorf("Result = %v, want StepContinue", result)
		}
		if step.GetCursor() != 1 {
			t.Errorf("Cursor after down = %d, want 1", step.GetCursor())
		}
	})

	t.Run("navigate up moves cursor", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)
		step.SetCursor(2)

		_, result := updateStep(t, step, keyMsg("up"))
		if result != framework.StepContinue {
			t.Errorf("Result = %v, want StepContinue", result)
		}
		if step.GetCursor() != 1 {
			t.Errorf("Cursor after up = %d, want 1", step.GetCursor())
		}
	})

	t.Run("j/k navigation works", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)

		updateStep(t, step, keyMsg("j"))
		if step.GetCursor() != 1 {
			t.Errorf("Cursor after j = %d, want 1", step.GetCursor())
		}

		updateStep(t, step, keyMsg("k"))
		if step.GetCursor() != 0 {
			t.Errorf("Cursor after k = %d, want 0", step.GetCursor())
		}
	})

	t.Run("home jumps to first option", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)
		step.SetCursor(2)

		updateStep(t, step, keyMsg("home"))
		if step.GetCursor() != 0 {
			t.Errorf("Cursor after home = %d, want 0", step.GetCursor())
		}
	})

	t.Run("end jumps to last option", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)

		updateStep(t, step, keyMsg("end"))
		if step.GetCursor() != 2 {
			t.Errorf("Cursor after end = %d, want 2", step.GetCursor())
		}
	})

	t.Run("pgup jumps to first option", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)
		step.SetCursor(2)

		updateStep(t, step, keyMsg("pgup"))
		if step.GetCursor() != 0 {
			t.Errorf("Cursor after pgup = %d, want 0", step.GetCursor())
		}
	})

	t.Run("pgdown jumps to last option", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)

		updateStep(t, step, keyMsg("pgdown"))
		if step.GetCursor() != 2 {
			t.Errorf("Cursor after pgdown = %d, want 2", step.GetCursor())
		}
	})

	t.Run("cursor stops at top", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)

		updateStep(t, step, keyMsg("up"))
		if step.GetCursor() != 0 {
			t.Errorf("Cursor after up at top = %d, want 0", step.GetCursor())
		}
	})

	t.Run("cursor stops at bottom", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)
		step.SetCursor(2)

		updateStep(t, step, keyMsg("down"))
		if step.GetCursor() != 2 {
			t.Errorf("Cursor after down at bottom = %d, want 2", step.GetCursor())
		}
	})
}

func TestSingleSelectStep_Selection(t *testing.T) {
	options := []framework.Option{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
		{Label: "Option 3", Value: "opt3"},
	}

	t.Run("enter selects current option", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)
		step.SetCursor(1)

		_, result := updateStep(t, step, keyMsg("enter"))
		if result != framework.StepSubmitIfReady {
			t.Errorf("Result = %v, want StepSubmitIfReady", result)
		}
		if !step.IsComplete() {
			t.Error("Step should be complete after enter")
		}
		if step.GetSelectedIndex() != 1 {
			t.Errorf("Selected index = %d, want 1", step.GetSelectedIndex())
		}
	})

	t.Run("right arrow selects and advances", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)

		_, result := updateStep(t, step, keyMsg("right"))
		if result != framework.StepAdvance {
			t.Errorf("Result = %v, want StepAdvance", result)
		}
		if !step.IsComplete() {
			t.Error("Step should be complete after right")
		}
	})

	t.Run("left arrow returns StepBack", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)

		_, result := updateStep(t, step, keyMsg("left"))
		if result != framework.StepBack {
			t.Errorf("Result = %v, want StepBack", result)
		}
	})

	t.Run("Value returns selected option", func(t *testing.T) {
		step := NewSingleSelect("test", "Test", "Select:", options)
		step.SetCursor(1)
		updateStep(t, step, keyMsg("enter"))

		value := step.Value()
		if value.Key != "test" {
			t.Errorf("Value.Key = %s, want test", value.Key)
		}
		if value.Label != "Option 2" {
			t.Errorf("Value.Label = %s, want Option 2", value.Label)
		}
		if value.Raw != "opt2" {
			t.Errorf("Value.Raw = %v, want opt2", value.Raw)
		}
	})
}

func TestSingleSelectStep_DisabledOptions(t *testing.T) {
	t.Run("cursor skips disabled options on creation", func(t *testing.T) {
		options := []framework.Option{
			{Label: "Disabled 1", Value: "d1", Disabled: true},
			{Label: "Enabled", Value: "e1"},
			{Label: "Disabled 2", Value: "d2", Disabled: true},
		}

		step := NewSingleSelect("test", "Test", "Select:", options)

		// Cursor should start at first non-disabled option
		if step.GetCursor() != 1 {
			t.Errorf("Initial cursor = %d, want 1 (first enabled)", step.GetCursor())
		}
	})

	t.Run("navigation skips disabled options", func(t *testing.T) {
		options := []framework.Option{
			{Label: "Enabled 1", Value: "e1"},
			{Label: "Disabled", Value: "d1", Disabled: true},
			{Label: "Enabled 2", Value: "e2"},
		}

		step := NewSingleSelect("test", "Test", "Select:", options)

		// Navigate down should skip disabled option
		updateStep(t, step, keyMsg("down"))
		if step.GetCursor() != 2 {
			t.Errorf("Cursor after down = %d, want 2 (skipped disabled)", step.GetCursor())
		}

		// Navigate up should also skip disabled option
		updateStep(t, step, keyMsg("up"))
		if step.GetCursor() != 0 {
			t.Errorf("Cursor after up = %d, want 0 (skipped disabled)", step.GetCursor())
		}
	})

	t.Run("cannot select disabled option", func(t *testing.T) {
		options := []framework.Option{
			{Label: "Enabled 1", Value: "e1"},
			{Label: "Enabled 2", Value: "e2"},
			{Label: "Disabled", Value: "d1", Disabled: true},
		}

		step := NewSingleSelect("test", "Test", "Select:", options)
		// Disable option 0, cursor should move to option 1
		step.DisableOption(0, "reason")
		if step.GetCursor() != 1 {
			t.Errorf("Cursor after disabling = %d, want 1", step.GetCursor())
		}

		// Try to set cursor back to disabled option - should be rejected
		step.SetCursor(0)
		if step.GetCursor() != 1 {
			t.Errorf("SetCursor on disabled should not change cursor, got %d", step.GetCursor())
		}

		// Attempt to select on enabled option should succeed
		_, result := updateStep(t, step, keyMsg("enter"))
		if result != framework.StepSubmitIfReady {
			t.Error("Should be able to select enabled option")
		}
		if step.GetSelectedIndex() != 1 {
			t.Errorf("Selected index = %d, want 1", step.GetSelectedIndex())
		}
	})

	t.Run("DisableOption moves cursor if on disabled option", func(t *testing.T) {
		options := []framework.Option{
			{Label: "Option 1", Value: "opt1"},
			{Label: "Option 2", Value: "opt2"},
		}

		step := NewSingleSelect("test", "Test", "Select:", options)
		step.SetCursor(0)

		step.DisableOption(0, "No longer available")

		if step.GetCursor() != 1 {
			t.Errorf("Cursor after DisableOption = %d, want 1", step.GetCursor())
		}
	})
}

func TestSingleSelectStep_Reset(t *testing.T) {
	options := []framework.Option{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
	}

	step := NewSingleSelect("test", "Test", "Select:", options)
	step.SetCursor(1)
	updateStep(t, step, keyMsg("enter"))

	if !step.IsComplete() {
		t.Error("Step should be complete before reset")
	}

	step.Reset()

	if step.IsComplete() {
		t.Error("Step should not be complete after reset")
	}
	if step.GetSelectedIndex() != -1 {
		t.Errorf("Selected index after reset = %d, want -1", step.GetSelectedIndex())
	}
	if step.GetCursor() != 0 {
		t.Errorf("Cursor after reset = %d, want 0", step.GetCursor())
	}
}

func TestSingleSelectStep_EmptyOptions(t *testing.T) {
	step := NewSingleSelect("test", "Test", "Select:", nil)

	if step.OptionsCount() != 0 {
		t.Errorf("OptionsCount = %d, want 0", step.OptionsCount())
	}

	// Should not panic on enter with no options
	_, result := updateStep(t, step, keyMsg("enter"))
	if result == framework.StepSubmitIfReady {
		t.Error("Should not submit with no options")
	}
}

func TestSingleSelectStep_SetOptions(t *testing.T) {
	step := NewSingleSelect("test", "Test", "Select:", nil)

	newOptions := []framework.Option{
		{Label: "New 1", Value: "new1"},
		{Label: "New 2", Value: "new2"},
	}
	step.SetOptions(newOptions)

	if step.OptionsCount() != 2 {
		t.Errorf("OptionsCount after SetOptions = %d, want 2", step.OptionsCount())
	}
	if step.GetCursor() != 0 {
		t.Errorf("Cursor after SetOptions = %d, want 0", step.GetCursor())
	}

	opt, ok := step.GetOption(0)
	if !ok {
		t.Error("GetOption(0) should return true")
	}
	if opt.Label != "New 1" {
		t.Errorf("Option label = %s, want New 1", opt.Label)
	}
}

func TestSingleSelectStep_Interface(t *testing.T) {
	step := NewSingleSelect("test", "Test Title", "Select:", nil)

	// Verify interface methods
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
	if step.HasClearableInput() {
		t.Error("HasClearableInput() should return false for SingleSelect")
	}
	if step.ClearInput() != nil {
		t.Error("ClearInput() should return nil for SingleSelect")
	}
}

func TestSingleSelectStep_FormatValue(t *testing.T) {
	options := []framework.Option{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
	}

	step := NewSingleSelect("test", "Test", "Select:", options)
	step.SetCursor(1)
	step.Update(keyMsg("enter"))

	// Without custom labels
	formatted := step.FormatValue(nil)
	if formatted != "Option 2" {
		t.Errorf("FormatValue() = %s, want Option 2", formatted)
	}

	// With custom labels
	displayLabels := map[interface{}]string{
		"opt2": "Custom Label",
	}
	formatted = step.FormatValue(displayLabels)
	if formatted != "Custom Label" {
		t.Errorf("FormatValue() with custom labels = %s, want Custom Label", formatted)
	}
}

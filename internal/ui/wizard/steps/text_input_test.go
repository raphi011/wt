package steps

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

func TestTextInputStep_Paste(t *testing.T) {
	t.Run("paste inserts text", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.Init() // Focus the input

		result, _, stepResult := step.Update(tea.PasteMsg{Content: "pasted-text"})
		if stepResult != framework.StepContinue {
			t.Errorf("Result = %v, want StepContinue", stepResult)
		}
		step = result.(*TextInputStep)

		if step.GetValue() != "pasted-text" {
			t.Errorf("GetValue() = %q, want %q", step.GetValue(), "pasted-text")
		}
	})

	t.Run("paste clears validation error", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.Init()

		// Trigger validation error by submitting empty
		updateStep(t, step, keyMsg("enter"))
		if !strings.Contains(step.View(), "cannot be empty") {
			t.Fatal("Expected validation error after empty submit")
		}

		// Paste should clear the error
		result, _, _ := step.Update(tea.PasteMsg{Content: "hello"})
		step = result.(*TextInputStep)

		if strings.Contains(step.View(), "cannot be empty") {
			t.Error("Validation error should be cleared after paste")
		}
		if step.GetValue() != "hello" {
			t.Errorf("GetValue() = %q, want %q", step.GetValue(), "hello")
		}
	})

	t.Run("paste appends to existing text", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.Init()
		step.SetValue("hello-")

		result, _, _ := step.Update(tea.PasteMsg{Content: "world"})
		step = result.(*TextInputStep)

		if step.GetValue() != "hello-world" {
			t.Errorf("GetValue() = %q, want %q", step.GetValue(), "hello-world")
		}
	})
}

func TestTextInputStep_BasicInput(t *testing.T) {
	t.Run("typing updates input value", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "placeholder")
		step.Init() // Focus the input so it accepts key events

		updateStep(t, step, keyMsg("h"))
		updateStep(t, step, keyMsg("e"))
		updateStep(t, step, keyMsg("l"))
		updateStep(t, step, keyMsg("l"))
		updateStep(t, step, keyMsg("o"))

		if step.GetValue() != "hello" {
			t.Errorf("GetValue() = %q, want %q", step.GetValue(), "hello")
		}
	})

	t.Run("SetValue sets input value", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.SetValue("preset")

		if step.GetValue() != "preset" {
			t.Errorf("GetValue() = %q, want %q", step.GetValue(), "preset")
		}
	})

	t.Run("backspace deletes character", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.Init() // Focus the input
		step.SetValue("hello")

		updateStep(t, step, keyMsg("backspace"))

		if step.GetValue() != "hell" {
			t.Errorf("GetValue() after backspace = %q, want %q", step.GetValue(), "hell")
		}
	})
}

func TestTextInputStep_Submission(t *testing.T) {
	t.Run("enter submits with value", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.SetValue("hello")

		_, result := updateStep(t, step, keyMsg("enter"))

		if result != framework.StepSubmitIfReady {
			t.Errorf("Result = %v, want StepSubmitIfReady", result)
		}
		if !step.IsComplete() {
			t.Error("Step should be complete")
		}
	})

	t.Run("enter does not submit empty value", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")

		_, result := updateStep(t, step, keyMsg("enter"))

		if result == framework.StepSubmitIfReady {
			t.Error("Should not submit with empty value")
		}
		if step.IsComplete() {
			t.Error("Step should not be complete")
		}
	})

	t.Run("enter does not submit whitespace-only value", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.SetValue("   ")

		_, result := updateStep(t, step, keyMsg("enter"))

		if result == framework.StepSubmitIfReady {
			t.Error("Should not submit with whitespace-only value")
		}
	})

	t.Run("right arrow submits and advances", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.SetValue("hello")

		_, result := updateStep(t, step, keyMsg("right"))

		if result != framework.StepAdvance {
			t.Errorf("Result = %v, want StepAdvance", result)
		}
		if !step.IsComplete() {
			t.Error("Step should be complete")
		}
	})

	t.Run("left arrow goes back", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")

		_, result := updateStep(t, step, keyMsg("left"))

		if result != framework.StepBack {
			t.Errorf("Result = %v, want StepBack", result)
		}
	})
}

func TestTextInputStep_Validation(t *testing.T) {
	t.Run("validation prevents submission on error", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.SetValidate(func(s string) error {
			if len(s) < 3 {
				return errors.New("must be at least 3 characters")
			}
			return nil
		})
		step.SetValue("ab")

		_, result := updateStep(t, step, keyMsg("enter"))

		if result == framework.StepSubmitIfReady {
			t.Error("Should not submit with validation error")
		}
		if step.IsComplete() {
			t.Error("Step should not be complete")
		}
	})

	t.Run("validation allows submission when valid", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.SetValidate(func(s string) error {
			if len(s) < 3 {
				return errors.New("must be at least 3 characters")
			}
			return nil
		})
		step.SetValue("abc")

		_, result := updateStep(t, step, keyMsg("enter"))

		if result != framework.StepSubmitIfReady {
			t.Errorf("Result = %v, want StepSubmitIfReady", result)
		}
		if !step.IsComplete() {
			t.Error("Step should be complete")
		}
	})

	t.Run("right arrow respects validation", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.SetValidate(func(s string) error {
			if s != "valid" {
				return errors.New("must be 'valid'")
			}
			return nil
		})
		step.SetValue("invalid")

		_, result := updateStep(t, step, keyMsg("right"))

		if result == framework.StepAdvance {
			t.Error("Should not advance with validation error")
		}
	})
}

func TestTextInputStep_Value(t *testing.T) {
	t.Run("Value returns submitted value", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.SetValue("hello world")
		updateStep(t, step, keyMsg("enter"))

		value := step.Value()

		if value.Key != "test" {
			t.Errorf("Value.Key = %s, want test", value.Key)
		}
		if value.Label != "hello world" {
			t.Errorf("Value.Label = %s, want 'hello world'", value.Label)
		}
		if value.Raw != "hello world" {
			t.Errorf("Value.Raw = %v, want 'hello world'", value.Raw)
		}
	})

	t.Run("Value trims whitespace", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter name:", "")
		step.SetValue("  hello  ")
		updateStep(t, step, keyMsg("enter"))

		value := step.Value()

		if value.Label != "hello" {
			t.Errorf("Value.Label = %q, want 'hello' (trimmed)", value.Label)
		}
	})
}

func TestTextInputStep_Reset(t *testing.T) {
	step := NewTextInput("test", "Test", "Enter name:", "")
	step.SetValue("hello")
	updateStep(t, step, keyMsg("enter"))

	if !step.IsComplete() {
		t.Error("Should be complete before reset")
	}

	step.Reset()

	if step.IsComplete() {
		t.Error("Should not be complete after reset")
	}
	if step.GetValue() != "" {
		t.Errorf("GetValue() after reset = %q, want empty", step.GetValue())
	}
}

func TestTextInputStep_ClearInput(t *testing.T) {
	step := NewTextInput("test", "Test", "Enter name:", "")
	step.SetValue("hello")

	if !step.HasClearableInput() {
		t.Error("HasClearableInput should be true when value is set")
	}

	step.ClearInput()

	if step.GetValue() != "" {
		t.Errorf("GetValue() after ClearInput = %q, want empty", step.GetValue())
	}
	if step.HasClearableInput() {
		t.Error("HasClearableInput should be false after clear")
	}
}

func TestTextInputStep_Interface(t *testing.T) {
	step := NewTextInput("test", "Test Title", "Enter name:", "placeholder")

	if step.ID() != "test" {
		t.Errorf("ID() = %s, want test", step.ID())
	}
	if step.Title() != "Test Title" {
		t.Errorf("Title() = %s, want 'Test Title'", step.Title())
	}
	if step.Help() == "" {
		t.Error("Help() should return help text")
	}
}

func TestTextInputStep_Focus(t *testing.T) {
	step := NewTextInput("test", "Test", "Enter name:", "")

	// Init should focus the input
	step.Init()
	if !step.IsFocused() {
		t.Error("Should be focused after Init")
	}

	step.Blur()
	if step.IsFocused() {
		t.Error("Should not be focused after Blur")
	}

	step.Focus()
	if !step.IsFocused() {
		t.Error("Should be focused after Focus")
	}
}

func TestTextInputStep_Configuration(t *testing.T) {
	step := NewTextInput("test", "Test", "Enter:", "")

	step.SetPlaceholder("new placeholder")
	step.SetWidth(80)
	step.SetCharLimit(10)

	// These should not panic - we just verify they work
	if step.ID() != "test" {
		t.Error("Configuration methods should not break step")
	}
}

func TestTextInputStep_View(t *testing.T) {
	t.Run("View returns non-empty string", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter something:", "")
		view := step.View()
		if view == "" {
			t.Error("View() should return non-empty string")
		}
	})

	t.Run("View shows prompt", func(t *testing.T) {
		step := NewTextInput("test", "Test", "My Prompt:", "")
		view := step.View()
		if !containsStr(view, "My Prompt:") {
			t.Errorf("View() should contain prompt, got %q", view)
		}
	})

	t.Run("View shows validation error", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter:", "")
		// Trigger validation error by submitting empty value
		updateStep(t, step, keyMsg("enter"))

		view := step.View()
		if !containsStr(view, "cannot be empty") {
			t.Errorf("View() should contain validation error, got %q", view)
		}
	})

	t.Run("View clears error when typing after error", func(t *testing.T) {
		step := NewTextInput("test", "Test", "Enter:", "")
		step.Init()
		// Trigger validation error
		updateStep(t, step, keyMsg("enter"))

		// Now type something to clear the error
		updateStep(t, step, keyMsg("a"))

		view := step.View()
		if containsStr(view, "cannot be empty") {
			t.Errorf("View() should not show error after typing, got %q", view)
		}
	})
}

func TestTextInputStep_WithCursor(t *testing.T) {
	step := NewTextInput("test", "Test", "Enter:", "")

	// WithCursor should configure cursor without panicking
	step = step.WithCursor(tea.CursorBlock, false)

	if step.ID() != "test" {
		t.Error("WithCursor should not break step")
	}
}

func TestTextInputStep_String(t *testing.T) {
	step := NewTextInput("myid", "Test", "Enter:", "")
	s := step.String()
	if !containsStr(s, "myid") {
		t.Errorf("String() = %q, should contain id", s)
	}
}

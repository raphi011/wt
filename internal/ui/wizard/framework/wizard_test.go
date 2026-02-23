package framework

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// mockStep is a minimal Step implementation for testing wizard orchestration.
type mockStep struct {
	id        string
	title     string
	complete  bool
	value     StepValue
	hasClear  bool
	clearCb   func()
	advanceOn string // key that triggers advance (e.g., "enter")
}

func newMockStep(id, title string) *mockStep {
	return &mockStep{
		id:        id,
		title:     title,
		advanceOn: "enter",
	}
}

func (s *mockStep) ID() string    { return s.id }
func (s *mockStep) Title() string { return s.title }
func (s *mockStep) Init() tea.Cmd { return nil }

func (s *mockStep) Update(msg tea.KeyPressMsg) (Step, tea.Cmd, StepResult) {
	switch msg.String() {
	case "left":
		return s, nil, StepBack
	case "right":
		s.complete = true
		return s, nil, StepAdvance
	case s.advanceOn:
		s.complete = true
		return s, nil, StepSubmitIfReady
	}
	return s, nil, StepContinue
}

func (s *mockStep) View() string { return s.title }
func (s *mockStep) Help() string { return "mock help" }

func (s *mockStep) Value() StepValue {
	if s.value.Key != "" {
		return s.value
	}
	return StepValue{Key: s.id, Label: "mock", Raw: "mock"}
}

func (s *mockStep) IsComplete() bool { return s.complete }
func (s *mockStep) Reset()           { s.complete = false }

func (s *mockStep) HasClearableInput() bool { return s.hasClear }

func (s *mockStep) ClearInput() tea.Cmd {
	if s.clearCb != nil {
		s.clearCb()
	}
	s.hasClear = false
	return nil
}

func (s *mockStep) setComplete() *mockStep {
	s.complete = true
	return s
}

func (s *mockStep) setValue(v StepValue) *mockStep {
	s.value = v
	return s
}

// keyMsg creates a KeyPressMsg for testing.
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
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		if len(key) == 1 {
			r := rune(key[0])
			return tea.KeyPressMsg{Code: r, Text: key}
		}
		return tea.KeyPressMsg{}
	}
}

// updateWizard sends a key to the wizard and returns it (with type assertion).
func updateWizard(t *testing.T, w *Wizard, key string) *Wizard {
	t.Helper()
	m, _ := w.Update(keyMsg(key))
	return m.(*Wizard)
}

func TestWizard_StepNavigation(t *testing.T) {
	t.Run("Init sets first step as current", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		if w.CurrentStepID() != "step1" {
			t.Errorf("CurrentStepID = %s, want step1", w.CurrentStepID())
		}
	})

	t.Run("enter advances to next step", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		w = updateWizard(t, w, "enter")

		if w.CurrentStepID() != "step2" {
			t.Errorf("CurrentStepID = %s, want step2", w.CurrentStepID())
		}
	})

	t.Run("left goes back to previous step", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		// Advance to step2
		w = updateWizard(t, w, "enter")
		if w.CurrentStepID() != "step2" {
			t.Fatalf("Should be on step2, got %s", w.CurrentStepID())
		}

		// Go back
		w = updateWizard(t, w, "left")

		if w.CurrentStepID() != "step1" {
			t.Errorf("CurrentStepID = %s, want step1", w.CurrentStepID())
		}
	})

	t.Run("right advances to next step", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		w = updateWizard(t, w, "right")

		if w.CurrentStepID() != "step2" {
			t.Errorf("CurrentStepID = %s, want step2", w.CurrentStepID())
		}
	})

	t.Run("last step goes to summary", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		// Advance through both steps
		w = updateWizard(t, w, "enter")
		w = updateWizard(t, w, "enter")

		if w.CurrentStepID() != "summary" {
			t.Errorf("CurrentStepID = %s, want summary", w.CurrentStepID())
		}
	})
}

func TestWizard_SkipConditions(t *testing.T) {
	t.Run("SkipWhen skips step when condition is true", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")
		step3 := newMockStep("step3", "Step 3")

		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2).
			AddStep(step3).
			SkipWhen("step2", func(w *Wizard) bool { return true })

		w.Init()
		w = updateWizard(t, w, "enter")

		// Should skip step2 and go directly to step3
		if w.CurrentStepID() != "step3" {
			t.Errorf("CurrentStepID = %s, want step3 (skipped step2)", w.CurrentStepID())
		}
	})

	t.Run("SkipWhen does not skip when condition is false", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2).
			SkipWhen("step2", func(w *Wizard) bool { return false })

		w.Init()
		w = updateWizard(t, w, "enter")

		if w.CurrentStepID() != "step2" {
			t.Errorf("CurrentStepID = %s, want step2", w.CurrentStepID())
		}
	})

	t.Run("backward navigation also skips", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")
		step3 := newMockStep("step3", "Step 3")

		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2).
			AddStep(step3).
			SkipWhen("step2", func(w *Wizard) bool { return true })

		w.Init()
		w = updateWizard(t, w, "enter") // step1 -> step3 (skip step2)
		w = updateWizard(t, w, "left")  // step3 -> step1 (skip step2)

		if w.CurrentStepID() != "step1" {
			t.Errorf("CurrentStepID = %s, want step1 (skipped step2 backward)", w.CurrentStepID())
		}
	})
}

func TestWizard_Callbacks(t *testing.T) {
	t.Run("OnComplete fires when step completes", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		callbackFired := false
		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2).
			OnComplete("step1", func(w *Wizard) {
				callbackFired = true
			})

		w.Init()
		_ = updateWizard(t, w, "enter")

		if !callbackFired {
			t.Error("OnComplete callback should have fired")
		}
	})

	t.Run("OnComplete provides access to wizard state", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1").setValue(StepValue{
			Key:   "step1",
			Label: "Selected Value",
			Raw:   "selected", // GetString returns Raw when it's a string
		})
		step2 := newMockStep("step2", "Step 2")

		var capturedValue string
		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2).
			OnComplete("step1", func(w *Wizard) {
				capturedValue = w.GetString("step1")
			})

		w.Init()
		_ = updateWizard(t, w, "enter")

		// GetString returns Raw (the actual value) when it's a string
		if capturedValue != "selected" {
			t.Errorf("Captured value = %q, want 'selected'", capturedValue)
		}
	})
}

func TestWizard_Summary(t *testing.T) {
	t.Run("enter on summary completes wizard", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		// Go to summary
		w = updateWizard(t, w, "enter")
		if w.CurrentStepID() != "summary" {
			t.Fatalf("Should be on summary, got %s", w.CurrentStepID())
		}

		// Press enter on summary
		w = updateWizard(t, w, "enter")

		if !w.done {
			t.Error("Wizard should be done after enter on summary")
		}
		if w.IsCancelled() {
			t.Error("Wizard should not be cancelled")
		}
	})

	t.Run("left on summary goes back to last step", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		// Go to summary
		w = updateWizard(t, w, "enter")
		w = updateWizard(t, w, "enter")
		if w.CurrentStepID() != "summary" {
			t.Fatalf("Should be on summary, got %s", w.CurrentStepID())
		}

		// Go back
		w = updateWizard(t, w, "left")

		if w.CurrentStepID() != "step2" {
			t.Errorf("CurrentStepID = %s, want step2", w.CurrentStepID())
		}
	})

	t.Run("WithSummary sets summary title", func(t *testing.T) {
		w := NewWizard("Test").WithSummary("Review Changes")

		if w.summaryTitle != "Review Changes" {
			t.Errorf("summaryTitle = %q, want 'Review Changes'", w.summaryTitle)
		}
	})
}

func TestWizard_SkipSummary(t *testing.T) {
	t.Run("WithSkipSummary completes after last step", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").
			AddStep(step1).
			WithSkipSummary(true)

		w.Init()
		w = updateWizard(t, w, "enter")

		// Should be done immediately after last step (no summary)
		if !w.done {
			t.Error("Wizard should be done after last step with skipSummary")
		}
	})

	t.Run("without skipSummary goes to summary", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").
			AddStep(step1).
			WithSkipSummary(false)

		w.Init()
		w = updateWizard(t, w, "enter")

		if w.CurrentStepID() != "summary" {
			t.Errorf("CurrentStepID = %s, want summary", w.CurrentStepID())
		}
	})
}

func TestWizard_Cancel(t *testing.T) {
	t.Run("esc cancels wizard", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		w = updateWizard(t, w, "esc")

		if !w.IsCancelled() {
			t.Error("Wizard should be cancelled after esc")
		}
		if !w.done {
			t.Error("Wizard should be done after cancel")
		}
	})

	t.Run("ctrl+c cancels wizard", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		w = updateWizard(t, w, "ctrl+c")

		if !w.IsCancelled() {
			t.Error("Wizard should be cancelled after ctrl+c")
		}
	})

	t.Run("esc clears input before cancelling if step has clearable input", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")
		step1.hasClear = true

		clearCalled := false
		step1.clearCb = func() { clearCalled = true }

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		// First esc should clear input
		w = updateWizard(t, w, "esc")

		if !clearCalled {
			t.Error("ClearInput should have been called")
		}
		if w.IsCancelled() {
			t.Error("Wizard should not be cancelled after first esc (cleared input)")
		}

		// Second esc should cancel (input is now cleared)
		w = updateWizard(t, w, "esc")

		if !w.IsCancelled() {
			t.Error("Wizard should be cancelled after second esc")
		}
	})
}

func TestWizard_PrefilledSteps(t *testing.T) {
	t.Run("Init skips to first incomplete step", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1").setComplete()
		step2 := newMockStep("step2", "Step 2").setComplete()
		step3 := newMockStep("step3", "Step 3")

		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2).
			AddStep(step3)

		w.Init()

		if w.CurrentStepID() != "step3" {
			t.Errorf("CurrentStepID = %s, want step3 (first incomplete)", w.CurrentStepID())
		}
	})

	t.Run("Init goes to summary if all steps complete", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1").setComplete()
		step2 := newMockStep("step2", "Step 2").setComplete()

		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2)

		w.Init()

		if w.CurrentStepID() != "summary" {
			t.Errorf("CurrentStepID = %s, want summary (all complete)", w.CurrentStepID())
		}
	})
}

func TestWizard_GetValues(t *testing.T) {
	t.Run("GetString returns step value", func(t *testing.T) {
		step1 := newMockStep("branch", "Branch").setValue(StepValue{
			Key:   "branch",
			Label: "feature-x",
			Raw:   "feature-x",
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		if w.GetString("branch") != "feature-x" {
			t.Errorf("GetString = %q, want 'feature-x'", w.GetString("branch"))
		}
	})

	t.Run("GetBool returns step value", func(t *testing.T) {
		step1 := newMockStep("confirm", "Confirm").setValue(StepValue{
			Key:   "confirm",
			Label: "Yes",
			Raw:   true,
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		if !w.GetBool("confirm") {
			t.Error("GetBool should return true")
		}
	})

	t.Run("GetStrings returns step value", func(t *testing.T) {
		step1 := newMockStep("hooks", "Hooks").setValue(StepValue{
			Key:   "hooks",
			Label: "hook1, hook2",
			Raw:   []string{"hook1", "hook2"},
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		strs := w.GetStrings("hooks")
		if len(strs) != 2 || strs[0] != "hook1" || strs[1] != "hook2" {
			t.Errorf("GetStrings = %v, want [hook1, hook2]", strs)
		}
	})

	t.Run("GetStep returns step by ID", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)

		got := w.GetStep("step1")
		if got == nil {
			t.Fatal("GetStep returned nil")
		}
		if got.ID() != "step1" {
			t.Errorf("GetStep ID = %s, want step1", got.ID())
		}
	})

	t.Run("GetStep returns nil for unknown ID", func(t *testing.T) {
		w := NewWizard("Test")

		if w.GetStep("unknown") != nil {
			t.Error("GetStep should return nil for unknown ID")
		}
	})
}

func TestWizard_AllStepsComplete(t *testing.T) {
	t.Run("returns false when steps incomplete", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1").setComplete()
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)

		if w.AllStepsComplete() {
			t.Error("AllStepsComplete should be false")
		}
	})

	t.Run("returns true when all steps complete", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1").setComplete()
		step2 := newMockStep("step2", "Step 2").setComplete()

		w := NewWizard("Test").AddStep(step1).AddStep(step2)

		if !w.AllStepsComplete() {
			t.Error("AllStepsComplete should be true")
		}
	})

	t.Run("ignores skipped steps", func(t *testing.T) {
		step1 := newMockStep("step1", "Step 1").setComplete()
		step2 := newMockStep("step2", "Step 2") // incomplete but skipped

		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2).
			SkipWhen("step2", func(w *Wizard) bool { return true })

		if !w.AllStepsComplete() {
			t.Error("AllStepsComplete should be true (skipped step2)")
		}
	})
}

func TestWizard_StepCount(t *testing.T) {
	step1 := newMockStep("step1", "Step 1")
	step2 := newMockStep("step2", "Step 2")

	w := NewWizard("Test").AddStep(step1).AddStep(step2)

	if w.StepCount() != 2 {
		t.Errorf("StepCount = %d, want 2", w.StepCount())
	}
}

func TestWizard_EmptyWizard(t *testing.T) {
	w := NewWizard("Empty")
	w.Init()

	// Should handle empty wizard gracefully
	if w.StepCount() != 0 {
		t.Errorf("StepCount = %d, want 0", w.StepCount())
	}
}

func TestWizard_InfoLine(t *testing.T) {
	step1 := newMockStep("step1", "Step 1")

	w := NewWizard("Test").
		AddStep(step1).
		WithInfoLine(func(w *Wizard) string {
			return "Dynamic info"
		})

	if w.infoLine == nil {
		t.Error("infoLine should be set")
	}
}

func TestWizard_WindowSizeMsg(t *testing.T) {
	step1 := newMockStep("step1", "Step 1")

	w := NewWizard("Test").AddStep(step1)
	w.Init()

	// Send window size message
	m, _ := w.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	w = m.(*Wizard)

	if w.width != 100 || w.height != 50 {
		t.Errorf("Window size = %dx%d, want 100x50", w.width, w.height)
	}
}

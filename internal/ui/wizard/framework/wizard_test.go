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

func TestWizard_View(t *testing.T) {
	t.Parallel()

	t.Run("View renders title and step content", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("My Wizard").AddStep(step1)
		w.Init()

		view := w.View().Content
		if view == "" {
			t.Error("View() should return non-empty string")
		}
		// Should contain the wizard title
		if !contains(view, "My Wizard") {
			t.Errorf("View() = %q, should contain title 'My Wizard'", view)
		}
	})

	t.Run("View returns empty when done", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)
		w.Init()
		w.done = true

		view := w.View().Content
		if view != "" {
			t.Errorf("View() when done = %q, want empty string", view)
		}
	})

	t.Run("View renders step tabs with multiple steps", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		view := w.View().Content
		if view == "" {
			t.Error("View() should return non-empty string")
		}
		// Should show step tabs with step titles
		if !contains(view, "Step 1") {
			t.Errorf("View() should contain step title, got %q", view)
		}
	})

	t.Run("View renders summary when on summary step", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1").setValue(StepValue{
			Key:   "step1",
			Label: "my value",
			Raw:   "my value",
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()
		// Advance to summary
		w = updateWizard(t, w, "enter")

		view := w.View().Content
		if view == "" {
			t.Error("View() should return non-empty string")
		}
	})

	t.Run("View renders info line when set and non-empty", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").
			AddStep(step1).
			WithInfoLine(func(w *Wizard) string {
				return "Some info text"
			})
		w.Init()

		view := w.View().Content
		if !contains(view, "Some info text") {
			t.Errorf("View() should contain info line, got %q", view)
		}
	})

	t.Run("View skips step tabs for single step with skipSummary", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "My Step")

		w := NewWizard("Test").
			AddStep(step1).
			WithSkipSummary(true)
		w.Init()

		view := w.View().Content
		// With single step and skipSummary, no tabs rendered, but step content is shown
		if view == "" {
			t.Error("View() should return non-empty string")
		}
	})

	t.Run("View includes help text from step", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		view := w.View().Content
		if !contains(view, "mock help") {
			t.Errorf("View() should contain step help text, got %q", view)
		}
	})
}

func TestWizard_RenderStepTabs(t *testing.T) {
	t.Parallel()

	t.Run("tabs show checkmarks for confirmed steps", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		// Advance step1 (confirms it)
		w = updateWizard(t, w, "enter")

		tabs := w.renderStepTabs()
		if tabs == "" {
			t.Error("renderStepTabs() should return non-empty string")
		}
	})

	t.Run("tabs skip skipped steps", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")
		step3 := newMockStep("step3", "Step 3")

		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2).
			AddStep(step3).
			SkipWhen("step2", func(w *Wizard) bool { return true })
		w.Init()

		tabs := w.renderStepTabs()
		if tabs == "" {
			t.Error("renderStepTabs() should return non-empty string")
		}
		// step2 is skipped, so should not appear in tabs
		if contains(tabs, "Step 2") {
			t.Errorf("renderStepTabs() should not contain skipped step, got %q", tabs)
		}
	})

	t.Run("tabs include summary tab when not skipSummary", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1).WithSkipSummary(false)
		w.Init()

		tabs := w.renderStepTabs()
		if !contains(tabs, "Summary") {
			t.Errorf("renderStepTabs() should include Summary tab, got %q", tabs)
		}
	})

	t.Run("tabs do not include summary tab when skipSummary", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1).WithSkipSummary(true)
		w.Init()

		tabs := w.renderStepTabs()
		if contains(tabs, "Summary") {
			t.Errorf("renderStepTabs() should not include Summary tab with skipSummary, got %q", tabs)
		}
	})

	t.Run("active step on summary highlighted correctly", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)
		w.Init()
		// Advance to summary
		w = updateWizard(t, w, "enter")

		tabs := w.renderStepTabs()
		if !contains(tabs, "Summary") {
			t.Errorf("renderStepTabs() should include Summary, got %q", tabs)
		}
	})

	t.Run("confirmed and active step shows checkmark", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		// Advance to step2 (confirms step1), then go back to step1
		w = updateWizard(t, w, "enter") // now on step2
		w = updateWizard(t, w, "left")  // back to step1 (confirmed)

		tabs := w.renderStepTabs()
		if tabs == "" {
			t.Error("renderStepTabs() should return non-empty string")
		}
		// step1 should show checkmark since it's confirmed and active
		if !contains(tabs, "✓") {
			t.Errorf("renderStepTabs() should contain checkmark for confirmed+active step, got %q", tabs)
		}
	})
}

func TestWizard_RenderSummary(t *testing.T) {
	t.Parallel()

	t.Run("summary shows step values", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Branch").setValue(StepValue{
			Key:   "step1",
			Label: "feature-x",
			Raw:   "feature-x",
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		summary := w.renderSummary()
		if !contains(summary, "Branch") {
			t.Errorf("renderSummary() should contain step title, got %q", summary)
		}
		if !contains(summary, "feature-x") {
			t.Errorf("renderSummary() should contain step value, got %q", summary)
		}
	})

	t.Run("summary skips steps with empty label", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1").setValue(StepValue{
			Key:   "step1",
			Label: "", // empty label, should be skipped
			Raw:   nil,
		})
		step2 := newMockStep("step2", "Step 2").setValue(StepValue{
			Key:   "step2",
			Label: "val2",
			Raw:   "val2",
		})

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		summary := w.renderSummary()
		if contains(summary, "Step 1") {
			t.Errorf("renderSummary() should skip step with empty label, got %q", summary)
		}
		if !contains(summary, "val2") {
			t.Errorf("renderSummary() should contain step2 value, got %q", summary)
		}
	})

	t.Run("summary skips skipped steps", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1").setValue(StepValue{
			Key:   "step1",
			Label: "value1",
			Raw:   "value1",
		})
		step2 := newMockStep("step2", "Skipped Step").setValue(StepValue{
			Key:   "step2",
			Label: "skipped-value",
			Raw:   "skipped-value",
		})

		w := NewWizard("Test").
			AddStep(step1).
			AddStep(step2).
			SkipWhen("step2", func(w *Wizard) bool { return true })
		w.Init()

		summary := w.renderSummary()
		if contains(summary, "skipped-value") {
			t.Errorf("renderSummary() should skip skipped steps, got %q", summary)
		}
	})

	t.Run("summary uses WithSummary title", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").
			AddStep(step1).
			WithSummary("Confirm Action")
		w.Init()

		summary := w.renderSummary()
		if !contains(summary, "Confirm Action") {
			t.Errorf("renderSummary() should contain custom title, got %q", summary)
		}
	})
}

func TestWizard_GetStrings_AllBranches(t *testing.T) {
	t.Parallel()

	t.Run("GetStrings with []any slice", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("items", "Items").setValue(StepValue{
			Key:   "items",
			Label: "a, b",
			Raw:   []any{"a", "b", "c"},
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		strs := w.GetStrings("items")
		if len(strs) != 3 {
			t.Errorf("GetStrings with []any = %v, want 3 items", strs)
		}
		if strs[0] != "a" || strs[1] != "b" || strs[2] != "c" {
			t.Errorf("GetStrings = %v, want [a, b, c]", strs)
		}
	})

	t.Run("GetStrings with []any containing non-string skips them", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("items", "Items").setValue(StepValue{
			Key:   "items",
			Label: "mixed",
			Raw:   []any{"a", 42, "b"},
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		strs := w.GetStrings("items")
		if len(strs) != 2 {
			t.Errorf("GetStrings with mixed []any = %v, want 2 strings", strs)
		}
	})

	t.Run("GetStrings returns nil for non-slice raw", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("items", "Items").setValue(StepValue{
			Key:   "items",
			Label: "single",
			Raw:   "single-string",
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		strs := w.GetStrings("items")
		if strs != nil {
			t.Errorf("GetStrings with non-slice raw = %v, want nil", strs)
		}
	})

	t.Run("GetStrings returns nil for unknown step", func(t *testing.T) {
		t.Parallel()
		w := NewWizard("Test")

		strs := w.GetStrings("unknown")
		if strs != nil {
			t.Errorf("GetStrings unknown step = %v, want nil", strs)
		}
	})

	t.Run("GetBool returns false for non-bool raw", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("confirm", "Confirm").setValue(StepValue{
			Key:   "confirm",
			Label: "no",
			Raw:   "not-a-bool",
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		b := w.GetBool("confirm")
		if b {
			t.Error("GetBool with non-bool raw should return false")
		}
	})

	t.Run("GetString falls back to label when raw is not string", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("items", "Items").setValue(StepValue{
			Key:   "items",
			Label: "display label",
			Raw:   42, // non-string raw
		})

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		s := w.GetString("items")
		if s != "display label" {
			t.Errorf("GetString with non-string raw = %q, want 'display label'", s)
		}
	})
}

func TestWizard_SetCurrentStep(t *testing.T) {
	t.Parallel()

	t.Run("SetCurrentStep changes active step", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")
		step2 := newMockStep("step2", "Step 2")

		w := NewWizard("Test").AddStep(step1).AddStep(step2)
		w.Init()

		w.SetCurrentStep("step2")

		if w.CurrentStepID() != "step2" {
			t.Errorf("CurrentStepID = %s, want step2", w.CurrentStepID())
		}
	})

	t.Run("SetCurrentStep is no-op for unknown ID", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		w.SetCurrentStep("unknown")

		if w.CurrentStepID() != "step1" {
			t.Errorf("CurrentStepID after unknown SetCurrentStep = %s, want step1", w.CurrentStepID())
		}
	})
}

func TestWizard_AdvanceWithStepAdvanceResult(t *testing.T) {
	t.Parallel()

	t.Run("StepAdvance on last step goes to summary", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		// right arrow triggers StepAdvance
		w = updateWizard(t, w, "right")

		if w.CurrentStepID() != "summary" {
			t.Errorf("CurrentStepID = %s, want summary", w.CurrentStepID())
		}
	})

	t.Run("StepAdvance on last step with skipSummary completes wizard", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1).WithSkipSummary(true)
		w.Init()

		// right arrow triggers StepAdvance
		w = updateWizard(t, w, "right")

		if !w.done {
			t.Error("Wizard should be done after StepAdvance on last step with skipSummary")
		}
	})
}

func TestWizard_UnknownMsg(t *testing.T) {
	t.Parallel()

	t.Run("unknown message is ignored gracefully", func(t *testing.T) {
		t.Parallel()
		step1 := newMockStep("step1", "Step 1")

		w := NewWizard("Test").AddStep(step1)
		w.Init()

		// Send an unknown message type (not KeyPressMsg or WindowSizeMsg)
		m, _ := w.Update("unknown message")
		w = m.(*Wizard)

		// Should still be on step1
		if w.CurrentStepID() != "step1" {
			t.Errorf("CurrentStepID = %s, want step1", w.CurrentStepID())
		}
	})
}

// contains is a helper to check if a string contains a substring.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

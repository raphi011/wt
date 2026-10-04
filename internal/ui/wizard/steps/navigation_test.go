package steps

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

func TestFocusedInputsEditWithoutNavigating(t *testing.T) {
	step := NewFilterableList("input", "Input", "", nil).WithCreateFromFilter(func(f string) string { return f })
	w := framework.NewWizard("Test").AddStep(step).AddStep(NewSingleSelect("next", "Next", "", nil))
	w.Init()
	w.Update(tea.PasteMsg{Content: "hello world"})
	// Word navigation still works with Ctrl+arrows; Alt+arrows navigate steps.
	for _, msg := range []tea.KeyPressMsg{keyMsg("ctrl+left"), keyMsg("right"), keyMsg("left"), keyMsg("X")} {
		w.Update(msg)
		if w.CurrentStepID() != "input" || step.IsComplete() {
			t.Fatal("editing navigated or confirmed the input")
		}
	}
	if got := step.GetFilter(); got != "hello Xworld" {
		t.Fatalf("middle edit = %q, want hello Xworld", got)
	}
	w.Update(keyMsg("ctrl+right"))
	w.Update(keyMsg("!"))
	if got := step.GetFilter(); got != "hello Xworld!" {
		t.Fatalf("word forward edit = %q", got)
	}
	w.Update(keyMsg("alt+right"))
	if w.CurrentStepID() != "next" || !step.IsComplete() {
		t.Fatal("explicit next did not confirm and advance")
	}
}

func TestWizardCancellationWithInput(t *testing.T) {
	for _, cancel := range []string{"ctrl+c", "esc"} {
		t.Run(cancel, func(t *testing.T) {
			step := NewFilterableList("input", "Input", "", nil)
			w := framework.NewWizard("Test").AddStep(step)
			w.Init()
			w.Update(tea.PasteMsg{Content: "hello"})
			if !strings.Contains(step.Help(), "esc clear") || !strings.Contains(step.Help(), "ctrl+c cancel") {
				t.Fatalf("help does not explain cancellation: %s", step.Help())
			}
			msg := keyMsg(cancel)
			if cancel == "ctrl+c" {
				msg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
			}
			w.Update(msg)
			if cancel == "esc" {
				if w.IsCancelled() || step.HasClearableInput() {
					t.Fatal("Escape should clear input first")
				}
				if !strings.Contains(step.Help(), "esc cancel") {
					t.Fatalf("empty-input help = %s", step.Help())
				}
				w.Update(msg)
			} else if !step.HasClearableInput() {
				t.Fatal("Ctrl+C cleared the input instead of cancelling immediately")
			}
			if !w.IsCancelled() {
				t.Fatal("wizard did not cancel")
			}
		})
	}
}

func TestWizardForwardsBlinkAndInitializesOnBack(t *testing.T) {
	step := NewFilterableList("input", "Input", "", []framework.Option{{Label: "hello", Value: "hello"}})
	w := framework.NewWizard("Test").AddStep(step).AddStep(NewSingleSelect("next", "Next", "", []framework.Option{{Label: "done"}}))
	w.Init()
	w.Update(tea.PasteMsg{Content: "hello"})
	if _, cmd := w.Update(textinput.Blink()); cmd == nil {
		t.Fatal("cursor initialization message did not reach the input")
	}
	w.Update(keyMsg("alt+right"))
	// Back from another step must restart the previous input's blink loop.
	_, cmd := w.Update(keyMsg("alt+left"))
	if w.CurrentStepID() != "input" || cmd == nil {
		t.Fatal("back did not initialize the previous input")
	}
	if _, blinkCmd := w.Update(cmd()); blinkCmd == nil {
		t.Fatal("back initialization did not restart blinking")
	}
	// Returning from summary must do the same.
	w.Update(keyMsg("alt+right"))
	w.Update(keyMsg("enter"))
	w.Update(keyMsg("alt+left"))
	_, cmd = w.Update(keyMsg("alt+left"))
	if w.CurrentStepID() != "input" || cmd == nil {
		t.Fatal("back through summary did not initialize input")
	}
}

func TestWizardSummaryBackRestartsInput(t *testing.T) {
	step := NewFilterableList("input", "Input", "", []framework.Option{{Label: "hello", Value: "hello"}})
	w := framework.NewWizard("Test").AddStep(step)
	w.Init()
	w.Update(tea.PasteMsg{Content: "hello"})
	w.Update(keyMsg("enter"))
	if w.CurrentStepID() != "summary" {
		t.Fatal("expected summary")
	}
	_, cmd := w.Update(keyMsg("alt+left"))
	if w.CurrentStepID() != "input" || cmd == nil {
		t.Fatal("summary back did not initialize input")
	}
	if _, blinkCmd := w.Update(cmd()); blinkCmd == nil {
		t.Fatal("summary back did not restart blinking")
	}
}

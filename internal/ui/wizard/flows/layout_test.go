package flows

import (
	"context"
	"fmt"
	"github.com/raphi011/wt/internal/forge"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
)

func TestWizardAndCdTerminalLayouts(t *testing.T) {
	options := make([]framework.Option, 50)
	for i := range options {
		options[i] = framework.Option{Label: fmt.Sprintf("branch-%02d 界é👩‍💻 %s", i, strings.Repeat("long", 20)), Description: strings.Repeat("description ", 20)}
	}
	for _, size := range [][2]int{{80, 24}, {120, 40}, {40, 12}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			list := steps.NewFilterableList("branches", "Branches", "Choose a branch", options)
			w := framework.NewWizard("Checkout").AddStep(list).AddStep(steps.NewSingleSelect("next", "Long next step title", "Next", options)).WithInfoLine(func(*framework.Wizard) string { return strings.Repeat("repo 界 ", 40) })
			w.Init()
			w.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			w.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
			check := func(view string, focused string, cancelKey string) {
				t.Helper()
				if len(strings.Split(view, "\n")) > size[1] {
					t.Fatalf("height overflow:\n%s", view)
				}
				for line := range strings.SplitSeq(view, "\n") {
					if ansi.StringWidth(line) > size[0] {
						t.Fatalf("width overflow: %q", line)
					}
				}
				for _, text := range []string{focused, cancelKey, "enter"} {
					if !strings.Contains(view, text) {
						t.Fatalf("%q missing:\n%s", text, view)
					}
				}
			}
			check(w.View().Content, "branch-49", "ctrl+c")
			w.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			w.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
			check(w.View().Content, "branch-49", "ctrl+c")
			// Lightweight cd uses the same row budget without wizard chrome.
			cd := &cdListModel{step: steps.NewFilterableList("cd", "Cd", "", options)}
			cd.Init()
			cd.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			cd.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
			check(cd.View().Content, "branch-49", "esc")
			// Resize with the filter active, retaining input and contextual help.
			cd.Update(tea.PasteMsg{Content: "branch-49"})
			check(cd.View().Content, "branch-49", "esc")
			if !strings.Contains(strings.Join(strings.Fields(ansi.Strip(cd.View().Content)), " "), "esc clear filter") {
				t.Fatal("clear-filter help missing")
			}
		})
	}
}

func TestLongSummaryScrollsAndReturnsToStep(t *testing.T) {
	input := steps.NewTextInput("text", "Text", "Enter text", "")
	input.SetCharLimit(1000)
	input.SetValue(strings.Repeat("summary-value 界é👩‍💻 ", 7) + "final-value")
	w := framework.NewWizard("Summary test").AddStep(input)
	w.Init()
	w.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	w.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if w.CurrentStepID() != "summary" {
		t.Fatal("summary not reached")
	}
	before := w.View().Content
	for range 100 {
		w.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	after := w.View().Content
	if before == after || !strings.Contains(after, "final-value") {
		t.Fatalf("could not scroll to final summary value:\n%s", after)
	}
	// A single up at the bottom moves immediately despite repeated down presses.
	w.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if w.View().Content == after {
		t.Fatal("summary overscroll trapped the cursor")
	}
	w.Update(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModAlt})
	if w.CurrentStepID() != "text" || input.GetValue() == "" {
		t.Fatal("back navigation lost input")
	}
}

func TestLoadedPRLayoutFitsAndRetainsRetryHelp(t *testing.T) {
	prs := make([]forge.OpenPR, 60)
	for i := range prs {
		prs[i] = forge.OpenPR{Number: i + 1, Title: fmt.Sprintf("PR-%02d %s", i, strings.Repeat("Unicode 界é👩‍💻 ", 8)), Author: "author", Branch: strings.Repeat("branch-", 20), IsDraft: true}
	}
	w := buildPrCheckoutWizard(PrCheckoutWizardParams{AvailableRepos: []string{"fixture"}, FetchPRs: func(context.Context, string) ([]forge.OpenPR, error) { return prs, nil }})
	defer w.Close()
	w.Init()
	// Execute a loading command directly, independent of the terminal theme query.
	w.Update(w.GetStep("pr").Init()())
	w.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	for _, size := range [][2]int{{80, 24}, {120, 40}, {40, 12}} {
		w.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		view := w.View().Content
		if len(strings.Split(view, "\n")) > size[1] {
			t.Fatalf("height overflow: %s", view)
		}
		for line := range strings.SplitSeq(view, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("width overflow: %s", line)
			}
		}
		for _, text := range []string{"PR-59", "ctrl+c", "ctrl+r", "enter"} {
			if !strings.Contains(view, text) {
				t.Fatalf("%q missing at %dx%d:\n%s", text, size[0], size[1], view)
			}
		}
	}
	w.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if w.GetValue("pr").Raw != 60 {
		t.Fatal("resizing changed the selected PR")
	}
}

package steps

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

func assertBounds(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > height {
		t.Fatalf("height %d > %d:\n%s", len(lines), height, view)
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > width {
			t.Fatalf("width %d > %d: %q", ansi.StringWidth(line), width, line)
		}
	}
}

func TestSelectorLayoutsKeepFocusedRow(t *testing.T) {
	options := make([]framework.Option, 60)
	for i := range options {
		options[i] = framework.Option{Label: fmt.Sprintf("option-%02d 界é👩‍💻 %s", i, strings.Repeat("long-", 12)), Description: strings.Repeat("description 界é👩‍💻 ", 8)}
	}
	for _, size := range [][2]int{{75, 12}, {115, 28}, {38, 5}, {20, 2}} {
		for _, multi := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dx%d/multi=%v", size[0], size[1], multi), func(t *testing.T) {
				list := NewFilterableList("list", "List", "Choose", options)
				if multi {
					list.WithMultiSelect()
				}
				list.SetSize(size[0], size[1])
				list.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
				view := list.View()
				assertBounds(t, view, size[0], size[1])
				if !strings.Contains(view, "option-59") {
					t.Fatalf("focused row missing:\n%s", view)
				}
				if strings.Contains(view, "option-00") {
					t.Fatal("list rendered offscreen first option")
				}
				// Shrinking after moving the cursor must retain the focused label.
				list.SetSize(20, 3)
				view = list.View()
				assertBounds(t, view, 20, 3)
				if !strings.Contains(view, "option-59") {
					t.Fatalf("focused row lost on resize: %s", view)
				}
			})
		}
		single := NewSingleSelect("single", "Single", "Choose", options)
		single.SetSize(size[0], size[1])
		single.SetCursor(59)
		view := single.View()
		assertBounds(t, view, size[0], size[1])
		if !strings.Contains(view, "option-59") {
			t.Fatalf("single focused row missing: %s", view)
		}
	}
}

func TestSelectorEmptyAndDisabledLayouts(t *testing.T) {
	for _, options := range [][]framework.Option{nil, {{Label: "disabled 界👩‍💻", Disabled: true, Description: strings.Repeat("reason ", 20)}}} {
		for _, step := range []framework.Step{NewSingleSelect("s", "S", "Choose", options), NewFilterableList("f", "F", "Choose", options)} {
			step.(framework.SizedStep).SetSize(18, 4)
			assertBounds(t, step.View(), 18, 4)
			_, _, result := step.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if result != framework.StepContinue {
				t.Fatal("empty/disabled list confirmed")
			}
		}
	}
}

func TestRowBudgetCountsDescriptions(t *testing.T) {
	options := make([]framework.Option, 30)
	for i := range options {
		options[i] = framework.Option{Label: fmt.Sprintf("row-%02d", i), Description: "second row"}
	}
	list := NewSingleSelect("s", "S", "", options)
	list.SetSize(40, 10)
	list.SetCursor(12)
	view := list.View()
	assertBounds(t, view, 40, 10)
	if !strings.Contains(ansi.Strip(view), "> row-12") || !strings.Contains(view, "more above") || !strings.Contains(view, "more below") {
		t.Fatalf("missing focus/scroll indicators:\n%s", view)
	}
	if n := strings.Count(view, "row-"); n != 4 {
		t.Fatalf("expected four two-row options in eight-row budget; got %d:\n%s", n, view)
	}
}

func TestCreateOptionAndFilterSurviveResize(t *testing.T) {
	list := NewFilterableList("s", "S", "Choose", nil).WithCreateFromFilter(func(s string) string { return "+ Create " + s })
	list.Update(tea.PasteMsg{Content: strings.Repeat("feature", 15)})
	for _, width := range []int{75, 38, 115} {
		list.SetSize(width, 6)
		assertBounds(t, list.View(), width, 6)
		if !strings.Contains(list.View(), "+ Create") {
			t.Fatal("create row missing")
		}
	}
	_, _, result := list.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if result != framework.StepSubmitIfReady || list.GetSelectedValue() != strings.Repeat("feature", 15) {
		t.Fatal("resize changed creation identity")
	}
}

func TestRowBudgetDoesNotCountEmptyStyledDescriptions(t *testing.T) {
	options := make([]framework.Option, 30)
	for i := range options {
		options[i] = framework.Option{Label: fmt.Sprintf("row-%02d", i)}
	}
	for _, step := range []framework.Step{NewSingleSelect("s", "S", "", options), NewFilterableList("f", "F", "", options)} {
		step.(framework.SizedStep).SetSize(40, 10)
		step.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
		view := step.View()
		count := strings.Count(view, "row-")
		want := 8
		if step.ID() == "f" {
			want = 7
		} // filter consumes one row
		if count != want {
			t.Fatalf("empty descriptions consumed rows: got %d want %d:\n%s", count, want, view)
		}
	}
}

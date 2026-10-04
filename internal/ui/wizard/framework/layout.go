package framework

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// CompactHelper provides contextual bindings for a small terminal.
type CompactHelper interface{ CompactHelp() string }

// SizedStep receives the content area, excluding wizard chrome and help.
// Keeping sizing optional lets other steps use the framework's final bounds.
type SizedStep interface{ SetSize(width, height int) }

// Fit bounds styled text by display cells and rows without cutting ANSI sequences.
func Fit(text string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
		lines[height-1] = ansi.Truncate(lines[height-1], max(0, width-1), "") + "…"
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "…")
	}
	return strings.Join(lines, "\n")
}

// Wrap bounds prose while preserving styled Unicode text.
func Wrap(text string, width, height int) string {
	return Fit(ansi.Wrap(text, max(1, width), ""), width, height)
}

func (w *Wizard) layout() (border lipgloss.Style, width, height int, header, help string) {
	border = BorderStyle()
	compact := w.height < 18 || w.width < 60
	if compact {
		border = border.MarginTop(0).MarginBottom(0).PaddingLeft(1).PaddingRight(0)
	}
	if w.width < 8 || w.height < 6 {
		border = lipgloss.NewStyle()
	}
	width = max(1, w.width-border.GetHorizontalFrameSize())
	innerHeight := max(1, w.height-border.GetVerticalFrameSize())
	title := TitleStyle().Render(w.title)
	if compact && w.currentStep < len(w.steps) {
		title += " · " + StepActiveStyle().Render(w.steps[w.currentStep].Title())
	}
	header = Fit(title, width, 1)
	if w.height >= 8 && w.infoLine != nil {
		if info := w.infoLine(w); info != "" {
			header += "\n" + Fit(InfoStyle().Render(info), width, 1)
		}
	}
	if !compact && !(len(w.steps) == 1 && w.skipSummary) {
		tabs := w.renderStepTabs()
		if lipgloss.Width(tabs) > width {
			active := w.summaryTitle
			if w.currentStep < len(w.steps) {
				active = w.steps[w.currentStep].Title()
			}
			tabs = StepActiveStyle().Render(active)
		}
		header += "\n\n" + Fit(tabs, width, 1)
	}
	helpText := "↑/↓ scroll • " + BindingHelp(Keys.ListBack, Keys.Confirm) + " • " + CancellationHelp(false, "")
	if w.currentStep < len(w.steps) {
		helpText = w.steps[w.currentStep].Help()
		if compact {
			if step, ok := w.steps[w.currentStep].(CompactHelper); ok {
				helpText = step.CompactHelp()
			}
		}
	}
	helpRows := min(4, max(1, innerHeight/3))
	// Extremely small screens prioritize the focused row and cancellation.
	if innerHeight < 6 {
		helpText = "enter select • ctrl+c cancel"
	}
	help = HelpStyle().MarginTop(0).Render(Wrap(helpText, width, helpRows))
	height = max(1, innerHeight-lipgloss.Height(header)-lipgloss.Height(help)-2)
	return
}

func (w *Wizard) resizeStep() {
	if w.currentStep >= len(w.steps) {
		_, width, height, _, _ := w.layout()
		lines := strings.Count(ansi.Wrap(w.renderSummary(), width, ""), "\n") + 1
		w.summaryOffset = min(w.summaryOffset, max(0, lines-height))
		return
	}
	if w.currentStep < len(w.steps) {
		_, width, height, _, _ := w.layout()
		if step, ok := w.steps[w.currentStep].(SizedStep); ok {
			step.SetSize(width, height)
		}
	}
}

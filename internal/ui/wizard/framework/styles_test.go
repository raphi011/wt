package framework

import (
	"testing"
)

// TestStyles verifies all style functions return non-zero lipgloss styles
// (i.e. they don't panic and return a usable style that renders to a non-empty string).
func TestStyles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		style func() string
	}{
		{
			name: "BorderStyle",
			style: func() string {
				return BorderStyle().Render("content")
			},
		},
		{
			name: "TitleStyle",
			style: func() string {
				return TitleStyle().Render("Title")
			},
		},
		{
			name: "StepActiveStyle",
			style: func() string {
				return StepActiveStyle().Render("Step 1")
			},
		},
		{
			name: "StepCompletedStyle",
			style: func() string {
				return StepCompletedStyle().Render("Step 1")
			},
		},
		{
			name: "StepCheckStyle",
			style: func() string {
				return StepCheckStyle().Render("✓")
			},
		},
		{
			name: "StepInactiveStyle",
			style: func() string {
				return StepInactiveStyle().Render("Step 1")
			},
		},
		{
			name: "StepArrowStyle",
			style: func() string {
				return StepArrowStyle().Render("→")
			},
		},
		{
			name: "OptionSelectedStyle",
			style: func() string {
				return OptionSelectedStyle().Render("Option")
			},
		},
		{
			name: "OptionNormalStyle",
			style: func() string {
				return OptionNormalStyle().Render("Option")
			},
		},
		{
			name: "OptionDisabledStyle",
			style: func() string {
				return OptionDisabledStyle().Render("Disabled")
			},
		},
		{
			name: "HelpStyle",
			style: func() string {
				return HelpStyle().Render("help text")
			},
		},
		{
			name: "InfoStyle",
			style: func() string {
				return InfoStyle().Render("info text")
			},
		},
		{
			name: "FilterStyle",
			style: func() string {
				return FilterStyle().Render("filter")
			},
		},
		{
			name: "FilterLabelStyle",
			style: func() string {
				return FilterLabelStyle().Render("Filter:")
			},
		},
		{
			name: "SummaryLabelStyle",
			style: func() string {
				return SummaryLabelStyle().Render("Label:")
			},
		},
		{
			name: "SummaryValueStyle",
			style: func() string {
				return SummaryValueStyle().Render("value")
			},
		},
		{
			name: "OptionDescriptionStyle",
			style: func() string {
				return OptionDescriptionStyle().Render("description")
			},
		},
		{
			name: "MatchHighlightStyle",
			style: func() string {
				return MatchHighlightStyle().Render("match")
			},
		},
		{
			name: "ErrorStyle",
			style: func() string {
				return ErrorStyle().Render("error message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.style()
			if got == "" {
				t.Errorf("%s: rendered empty string", tt.name)
			}
		})
	}
}

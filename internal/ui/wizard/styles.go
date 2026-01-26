package wizard

import "github.com/charmbracelet/lipgloss"

// Styles for wizard rendering
var (
	// BorderStyle wraps the entire wizard
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	// TitleStyle for the wizard title
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("62"))

	// StepActiveStyle for the current step tab
	StepActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	// StepCompletedStyle for completed step tabs
	StepCompletedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	// StepCheckStyle for the checkmark on completed steps
	StepCheckStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	// StepInactiveStyle for unvisited step tabs
	StepInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	// StepArrowStyle for arrows between steps
	StepArrowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// OptionSelectedStyle for the cursor-highlighted option
	OptionSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212"))

	// OptionNormalStyle for regular options
	OptionNormalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	// OptionDisabledStyle for disabled/greyed options
	OptionDisabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	// HelpStyle for help text at the bottom
	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginTop(1)

	// InfoStyle for informational text (e.g., "Selected: repo1, repo2")
	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true)

	// FilterStyle for the filter text being typed
	FilterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	// FilterLabelStyle for the "Filter:" label
	FilterLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	// SummaryLabelStyle for summary field labels
	SummaryLabelStyle = OptionNormalStyle

	// SummaryValueStyle for summary field values
	SummaryValueStyle = OptionSelectedStyle
)

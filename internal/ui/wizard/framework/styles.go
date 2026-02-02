package framework

import (
	"charm.land/lipgloss/v2"
	"github.com/raphi011/wt/internal/ui/styles"
)

// Style functions that return styles based on current theme
// These are functions instead of variables to pick up theme changes

// BorderStyle wraps the entire wizard (left border only)
func BorderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(styles.Primary).
		MarginTop(1).
		MarginBottom(1).
		PaddingLeft(2).
		PaddingRight(2)
}

// TitleStyle for the wizard title
func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Primary)
}

// StepActiveStyle for the current step tab
func StepActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Accent)
}

// StepCompletedStyle for completed step tabs
func StepCompletedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Normal)
}

// StepCheckStyle for the checkmark on completed steps
func StepCheckStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Success)
}

// StepInactiveStyle for unvisited step tabs
func StepInactiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Muted)
}

// StepArrowStyle for arrows between steps
func StepArrowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Muted)
}

// OptionSelectedStyle for the cursor-highlighted option
func OptionSelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Accent)
}

// OptionNormalStyle for regular options
func OptionNormalStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Normal)
}

// OptionDisabledStyle for disabled/greyed options
func OptionDisabledStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Muted)
}

// HelpStyle for help text at the bottom
func HelpStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Muted).
		MarginTop(1)
}

// InfoStyle for informational text (e.g., "Selected: repo1, repo2")
func InfoStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Info).
		Italic(true)
}

// FilterStyle for the filter text being typed
func FilterStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Accent).
		Bold(true)
}

// FilterLabelStyle for the "Filter:" label
func FilterLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Muted)
}

// SummaryLabelStyle for summary field labels
func SummaryLabelStyle() lipgloss.Style {
	return OptionNormalStyle()
}

// SummaryValueStyle for summary field values
func SummaryValueStyle() lipgloss.Style {
	return OptionSelectedStyle()
}

// OptionDescriptionStyle for two-row option descriptions
func OptionDescriptionStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Muted)
}

// MatchHighlightStyle for highlighting fuzzy matched characters
func MatchHighlightStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Accent).
		Bold(true).
		Underline(true)
}

// ErrorStyle for validation error messages
func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(styles.Error)
}

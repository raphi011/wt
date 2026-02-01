// Package styles provides shared lipgloss styles for UI components.
//
// This package centralizes color definitions and styling to ensure
// visual consistency across all UI components (static, progress,
// prompt, and wizard packages).
package styles

import "github.com/charmbracelet/lipgloss"

// Primary colors used throughout the UI
var (
	// Primary is the main accent color (cyan/teal)
	Primary lipgloss.TerminalColor = lipgloss.Color("62")

	// Accent is the highlight color for selected/active items (pink)
	Accent lipgloss.TerminalColor = lipgloss.Color("212")

	// Success is used for checkmarks and positive outcomes (green)
	Success lipgloss.TerminalColor = lipgloss.Color("82")

	// Error is used for error messages (red)
	Error lipgloss.TerminalColor = lipgloss.Color("196")

	// Muted is used for disabled/inactive text (gray)
	Muted lipgloss.TerminalColor = lipgloss.Color("240")

	// Normal is the standard text color (light gray)
	Normal lipgloss.TerminalColor = lipgloss.Color("252")

	// Info is used for informational text (gray)
	Info lipgloss.TerminalColor = lipgloss.Color("244")
)

// Common styles
var (
	// Bold applies bold formatting
	Bold = lipgloss.NewStyle().Bold(true)

	// Italic applies italic formatting
	Italic = lipgloss.NewStyle().Italic(true)

	// PrimaryStyle applies the primary color
	PrimaryStyle = lipgloss.NewStyle().Foreground(Primary)

	// AccentStyle applies the accent color with bold
	AccentStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	// SuccessStyle applies the success color
	SuccessStyle = lipgloss.NewStyle().Foreground(Success)

	// ErrorStyle applies the error color
	ErrorStyle = lipgloss.NewStyle().Foreground(Error)

	// MutedStyle applies the muted color
	MutedStyle = lipgloss.NewStyle().Foreground(Muted)

	// NormalStyle applies the normal text color
	NormalStyle = lipgloss.NewStyle().Foreground(Normal)

	// InfoStyle applies the info color with italic
	InfoStyle = lipgloss.NewStyle().
			Foreground(Info).
			Italic(true)
)

// Border styles
var (
	// RoundedBorder creates a rounded border with primary color
	RoundedBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 2)
)

// Text highlighting styles
var (
	// HighlightStyle for highlighting matched characters (pink, bold, underline)
	HighlightStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true).
			Underline(true)
)

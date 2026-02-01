package styles

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/raphi011/wt/internal/config"
)

// Theme defines the color palette for UI components
type Theme struct {
	Primary lipgloss.TerminalColor // main accent color (borders, titles)
	Accent  lipgloss.TerminalColor // highlight color (selected items)
	Success lipgloss.TerminalColor // success indicators (checkmarks)
	Error   lipgloss.TerminalColor // error messages
	Muted   lipgloss.TerminalColor // disabled/inactive text
	Normal  lipgloss.TerminalColor // standard text
	Info    lipgloss.TerminalColor // informational text
}

// Preset themes
var (
	// DefaultTheme is the default color scheme
	DefaultTheme = Theme{
		Primary: lipgloss.Color("62"),  // cyan/teal
		Accent:  lipgloss.Color("212"), // pink/magenta
		Success: lipgloss.Color("82"),  // green
		Error:   lipgloss.Color("196"), // red
		Muted:   lipgloss.Color("240"), // dark gray
		Normal:  lipgloss.Color("252"), // light gray
		Info:    lipgloss.Color("244"), // gray
	}

	// DraculaTheme is based on the Dracula color scheme
	DraculaTheme = Theme{
		Primary: lipgloss.Color("#bd93f9"), // purple
		Accent:  lipgloss.Color("#ff79c6"), // pink
		Success: lipgloss.Color("#50fa7b"), // green
		Error:   lipgloss.Color("#ff5555"), // red
		Muted:   lipgloss.Color("#6272a4"), // comment
		Normal:  lipgloss.Color("#f8f8f2"), // foreground
		Info:    lipgloss.Color("#8be9fd"), // cyan
	}

	// NordTheme is based on the Nord color scheme
	NordTheme = Theme{
		Primary: lipgloss.Color("#88c0d0"), // nord8 (frost cyan)
		Accent:  lipgloss.Color("#b48ead"), // nord15 (aurora purple)
		Success: lipgloss.Color("#a3be8c"), // nord14 (aurora green)
		Error:   lipgloss.Color("#bf616a"), // nord11 (aurora red)
		Muted:   lipgloss.Color("#4c566a"), // nord3 (polar night)
		Normal:  lipgloss.Color("#eceff4"), // nord6 (snow storm)
		Info:    lipgloss.Color("#81a1c1"), // nord9 (frost blue)
	}

	// GruvboxTheme is based on the Gruvbox color scheme (dark)
	GruvboxTheme = Theme{
		Primary: lipgloss.Color("#83a598"), // blue
		Accent:  lipgloss.Color("#d3869b"), // purple
		Success: lipgloss.Color("#b8bb26"), // green
		Error:   lipgloss.Color("#fb4934"), // red
		Muted:   lipgloss.Color("#665c54"), // gray
		Normal:  lipgloss.Color("#ebdbb2"), // foreground
		Info:    lipgloss.Color("#8ec07c"), // aqua
	}

	// CatppuccinFrappeTheme is based on Catppuccin Frappé
	CatppuccinFrappeTheme = Theme{
		Primary: lipgloss.Color("#8caaee"), // blue
		Accent:  lipgloss.Color("#f4b8e4"), // pink
		Success: lipgloss.Color("#a6d189"), // green
		Error:   lipgloss.Color("#e78284"), // red
		Muted:   lipgloss.Color("#626880"), // overlay0
		Normal:  lipgloss.Color("#c6d0f5"), // text
		Info:    lipgloss.Color("#99d1db"), // teal
	}

	// CatppuccinMochaTheme is based on Catppuccin Mocha
	CatppuccinMochaTheme = Theme{
		Primary: lipgloss.Color("#89b4fa"), // blue
		Accent:  lipgloss.Color("#f5c2e7"), // pink
		Success: lipgloss.Color("#a6e3a1"), // green
		Error:   lipgloss.Color("#f38ba8"), // red
		Muted:   lipgloss.Color("#6c7086"), // overlay0
		Normal:  lipgloss.Color("#cdd6f4"), // text
		Info:    lipgloss.Color("#94e2d5"), // teal
	}
)

// themePresets maps theme names to their definitions
var themePresets = map[string]Theme{
	"default":           DefaultTheme,
	"dracula":           DraculaTheme,
	"nord":              NordTheme,
	"gruvbox":           GruvboxTheme,
	"catppuccin-frappe": CatppuccinFrappeTheme,
	"catppuccin-mocha":  CatppuccinMochaTheme,
}

// currentTheme holds the active theme
var currentTheme = DefaultTheme

// Current returns the current theme
func Current() Theme {
	return currentTheme
}

// Init initializes the theme from config
// Call this after loading config and before displaying any UI
func Init(cfg config.ThemeConfig) {
	theme := DefaultTheme

	// Start with preset if specified
	if cfg.Name != "" {
		if preset, ok := themePresets[cfg.Name]; ok {
			theme = preset
		} else {
			// Unknown theme name - log warning and use default
			fmt.Fprintf(os.Stderr, "Warning: unknown theme %q, using default (available: %s)\n",
				cfg.Name, strings.Join(config.ValidThemeNames, ", "))
		}
	}

	// Override individual colors if specified
	if cfg.Primary != "" {
		theme.Primary = lipgloss.Color(cfg.Primary)
	}
	if cfg.Accent != "" {
		theme.Accent = lipgloss.Color(cfg.Accent)
	}
	if cfg.Success != "" {
		theme.Success = lipgloss.Color(cfg.Success)
	}
	if cfg.Error != "" {
		theme.Error = lipgloss.Color(cfg.Error)
	}
	if cfg.Muted != "" {
		theme.Muted = lipgloss.Color(cfg.Muted)
	}
	if cfg.Normal != "" {
		theme.Normal = lipgloss.Color(cfg.Normal)
	}
	if cfg.Info != "" {
		theme.Info = lipgloss.Color(cfg.Info)
	}

	currentTheme = theme

	// Update the global style variables
	applyTheme(theme)

	// Set nerdfont symbols
	SetNerdfont(cfg.Nerdfont)
}

// applyTheme updates all global style variables to use the given theme
func applyTheme(t Theme) {
	// Update color variables
	Primary = t.Primary
	Accent = t.Accent
	Success = t.Success
	Error = t.Error
	Muted = t.Muted
	Normal = t.Normal
	Info = t.Info

	// Update style variables
	PrimaryStyle = lipgloss.NewStyle().Foreground(t.Primary)
	AccentStyle = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	SuccessStyle = lipgloss.NewStyle().Foreground(t.Success)
	ErrorStyle = lipgloss.NewStyle().Foreground(t.Error)
	MutedStyle = lipgloss.NewStyle().Foreground(t.Muted)
	NormalStyle = lipgloss.NewStyle().Foreground(t.Normal)
	InfoStyle = lipgloss.NewStyle().Foreground(t.Info).Italic(true)

	// Update border styles
	RoundedBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2)

	// Update highlight style
	HighlightStyle = lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true).
		Underline(true)
}

// GetPreset returns a theme preset by name, or nil if not found
func GetPreset(name string) *Theme {
	if t, ok := themePresets[name]; ok {
		return &t
	}
	return nil
}

// PresetNames returns a list of available preset names
func PresetNames() []string {
	return config.ValidThemeNames
}

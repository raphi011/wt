package styles

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/raphi011/wt/internal/config"
)

// Theme defines the color palette for UI components
type Theme struct {
	Primary color.Color // main accent color (borders, titles)
	Accent  color.Color // highlight color (selected items)
	Success color.Color // success indicators (checkmarks)
	Error   color.Color // error messages
	Muted   color.Color // disabled/inactive text
	Normal  color.Color // standard text
	Info    color.Color // informational text
	Warning color.Color // warning indicators (stale items)
}

// themeFamily groups light and dark variants of a theme
type themeFamily struct {
	Light *Theme // nil if no light variant
	Dark  *Theme // nil if no dark variant
}

// Preset themes - Dark variants
var (
	// DefaultTheme is the default color scheme (dark only)
	DefaultTheme = Theme{
		Primary: lipgloss.Color("62"),  // cyan/teal
		Accent:  lipgloss.Color("212"), // pink/magenta
		Success: lipgloss.Color("82"),  // green
		Error:   lipgloss.Color("196"), // red
		Muted:   lipgloss.Color("240"), // dark gray
		Normal:  lipgloss.Color("252"), // light gray
		Info:    lipgloss.Color("244"), // gray
		Warning: lipgloss.Color("214"), // orange
	}

	// DraculaTheme is based on the Dracula color scheme (dark only)
	DraculaTheme = Theme{
		Primary: lipgloss.Color("#bd93f9"), // purple
		Accent:  lipgloss.Color("#ff79c6"), // pink
		Success: lipgloss.Color("#50fa7b"), // green
		Error:   lipgloss.Color("#ff5555"), // red
		Muted:   lipgloss.Color("#6272a4"), // comment
		Normal:  lipgloss.Color("#f8f8f2"), // foreground
		Info:    lipgloss.Color("#8be9fd"), // cyan
		Warning: lipgloss.Color("#ffb86c"), // dracula orange
	}

	// NordTheme is based on the Nord color scheme (dark)
	NordTheme = Theme{
		Primary: lipgloss.Color("#88c0d0"), // nord8 (frost cyan)
		Accent:  lipgloss.Color("#b48ead"), // nord15 (aurora purple)
		Success: lipgloss.Color("#a3be8c"), // nord14 (aurora green)
		Error:   lipgloss.Color("#bf616a"), // nord11 (aurora red)
		Muted:   lipgloss.Color("#4c566a"), // nord3 (polar night)
		Normal:  lipgloss.Color("#eceff4"), // nord6 (snow storm)
		Info:    lipgloss.Color("#81a1c1"), // nord9 (frost blue)
		Warning: lipgloss.Color("#ebcb8b"), // nord13 (aurora yellow)
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
		Warning: lipgloss.Color("#fabd2f"), // gruvbox yellow
	}

	// CatppuccinMochaTheme is based on Catppuccin Mocha (dark)
	CatppuccinMochaTheme = Theme{
		Primary: lipgloss.Color("#89b4fa"), // blue
		Accent:  lipgloss.Color("#f5c2e7"), // pink
		Success: lipgloss.Color("#a6e3a1"), // green
		Error:   lipgloss.Color("#f38ba8"), // red
		Muted:   lipgloss.Color("#6c7086"), // overlay0
		Normal:  lipgloss.Color("#cdd6f4"), // text
		Info:    lipgloss.Color("#94e2d5"), // teal
		Warning: lipgloss.Color("#fab387"), // catppuccin peach
	}

	// NoneTheme renders without any colors (uses terminal defaults)
	// Formatting (bold/italic/underline) is preserved
	NoneTheme = Theme{
		Primary: lipgloss.NoColor{},
		Accent:  lipgloss.NoColor{},
		Success: lipgloss.NoColor{},
		Error:   lipgloss.NoColor{},
		Muted:   lipgloss.NoColor{},
		Normal:  lipgloss.NoColor{},
		Info:    lipgloss.NoColor{},
		Warning: lipgloss.NoColor{},
	}
)

// Preset themes - Light variants
var (
	// NordLightTheme is based on the Nord color scheme (light)
	NordLightTheme = Theme{
		Primary: lipgloss.Color("#5e81ac"), // nord10 (frost blue, darker)
		Accent:  lipgloss.Color("#b48ead"), // nord15 (aurora purple)
		Success: lipgloss.Color("#a3be8c"), // nord14 (aurora green)
		Error:   lipgloss.Color("#bf616a"), // nord11 (aurora red)
		Muted:   lipgloss.Color("#9a9a9a"), // gray
		Normal:  lipgloss.Color("#2e3440"), // nord0 (polar night)
		Info:    lipgloss.Color("#81a1c1"), // nord9 (frost blue)
		Warning: lipgloss.Color("#d08770"), // nord12 (aurora orange)
	}

	// GruvboxLightTheme is based on the Gruvbox color scheme (light)
	GruvboxLightTheme = Theme{
		Primary: lipgloss.Color("#076678"), // blue (dark for contrast)
		Accent:  lipgloss.Color("#8f3f71"), // purple (dark for contrast)
		Success: lipgloss.Color("#79740e"), // green (dark)
		Error:   lipgloss.Color("#9d0006"), // red (dark)
		Muted:   lipgloss.Color("#928374"), // gray
		Normal:  lipgloss.Color("#3c3836"), // foreground (dark)
		Info:    lipgloss.Color("#427b58"), // aqua (dark)
		Warning: lipgloss.Color("#b57614"), // gruvbox yellow dark
	}

	// CatppuccinLatteTheme is based on Catppuccin Latte (light)
	CatppuccinLatteTheme = Theme{
		Primary: lipgloss.Color("#1e66f5"), // blue
		Accent:  lipgloss.Color("#ea76cb"), // pink
		Success: lipgloss.Color("#40a02b"), // green
		Error:   lipgloss.Color("#d20f39"), // red
		Muted:   lipgloss.Color("#9ca0b0"), // overlay0
		Normal:  lipgloss.Color("#4c4f69"), // text
		Info:    lipgloss.Color("#179299"), // teal
		Warning: lipgloss.Color("#fe640b"), // catppuccin peach
	}
)

// themeFamilies maps theme family names to their light/dark variants
var themeFamilies = map[string]themeFamily{
	"none":       {Light: &NoneTheme, Dark: &NoneTheme},                       // no colors
	"default":    {Dark: &DefaultTheme},                                       // dark only
	"dracula":    {Dark: &DraculaTheme},                                       // dark only
	"nord":       {Light: &NordLightTheme, Dark: &NordTheme},                  // both variants
	"gruvbox":    {Light: &GruvboxLightTheme, Dark: &GruvboxTheme},            // both variants
	"catppuccin": {Light: &CatppuccinLatteTheme, Dark: &CatppuccinMochaTheme}, // both variants
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
	theme := selectTheme(cfg)

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
	if cfg.Warning != "" {
		theme.Warning = lipgloss.Color(cfg.Warning)
	}

	currentTheme = theme

	// Update the global style variables
	applyTheme(theme)

	// Set nerdfont symbols
	SetNerdfont(cfg.Nerdfont)
}

// selectTheme picks the appropriate theme based on config and terminal background
func selectTheme(cfg config.ThemeConfig) Theme {
	// Default mode is "auto"
	mode := cfg.Mode
	if mode == "" {
		mode = "auto"
	}

	// Get family (default if not found)
	family, ok := themeFamilies[cfg.Name]
	if !ok {
		if cfg.Name != "" {
			// Unknown theme name - log warning
			fmt.Fprintf(os.Stderr, "Warning: unknown theme %q, using default (available: %s)\n",
				cfg.Name, strings.Join(config.ValidThemeNames, ", "))
		}
		family = themeFamilies["default"]
	}

	// Determine which variant to use based on mode
	var theme *Theme
	switch mode {
	case "light":
		theme = family.Light
	case "dark":
		theme = family.Dark
	case "auto":
		// Detect terminal background using lipgloss v2
		isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stderr)
		if isDark {
			theme = family.Dark
		} else {
			theme = family.Light
		}
	default:
		// Invalid mode - log warning and use auto
		fmt.Fprintf(os.Stderr, "Warning: unknown theme mode %q, using auto (available: %s)\n",
			mode, strings.Join(config.ValidThemeModes, ", "))
		isDark := lipgloss.HasDarkBackground(os.Stdin, os.Stderr)
		if isDark {
			theme = family.Dark
		} else {
			theme = family.Light
		}
	}

	// Fall back if the requested variant doesn't exist
	if theme == nil {
		if family.Dark != nil {
			theme = family.Dark
		} else if family.Light != nil {
			theme = family.Light
		} else {
			// Should never happen, but fall back to default
			return DefaultTheme
		}
	}

	return *theme
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
	Warning = t.Warning

	// Update style variables
	PrimaryStyle = lipgloss.NewStyle().Foreground(t.Primary)
	AccentStyle = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	SuccessStyle = lipgloss.NewStyle().Foreground(t.Success)
	ErrorStyle = lipgloss.NewStyle().Foreground(t.Error)
	MutedStyle = lipgloss.NewStyle().Foreground(t.Muted)
	NormalStyle = lipgloss.NewStyle().Foreground(t.Normal)
	InfoStyle = lipgloss.NewStyle().Foreground(t.Info).Italic(true)
	WarningStyle = lipgloss.NewStyle().Foreground(t.Warning)

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
// For theme families with variants, returns the dark variant by default
func GetPreset(name string) *Theme {
	if family, ok := themeFamilies[name]; ok {
		if family.Dark != nil {
			return family.Dark
		}
		return family.Light
	}
	return nil
}

// PresetNames returns a list of available preset names (theme families)
func PresetNames() []string {
	return config.ValidThemeNames
}

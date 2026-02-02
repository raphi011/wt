package styles

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/raphi011/wt/internal/config"
)

func TestInit_DefaultTheme(t *testing.T) {
	// Initialize with empty config (should use default theme)
	Init(config.ThemeConfig{})

	theme := Current()

	// Verify default colors are set
	if theme.Primary != lipgloss.Color("62") {
		t.Errorf("expected default primary color 62, got %v", theme.Primary)
	}
	if theme.Accent != lipgloss.Color("212") {
		t.Errorf("expected default accent color 212, got %v", theme.Accent)
	}
}

func TestInit_PresetTheme(t *testing.T) {
	tests := []struct {
		name          string
		preset        string
		mode          string      // theme mode ("dark", "light", or empty for auto)
		expectedColor color.Color // primary color to check
	}{
		{"dracula dark", "dracula", "dark", lipgloss.Color("#bd93f9")},
		{"nord dark", "nord", "dark", lipgloss.Color("#88c0d0")},
		{"nord light", "nord", "light", lipgloss.Color("#5e81ac")},
		{"gruvbox dark", "gruvbox", "dark", lipgloss.Color("#83a598")},
		{"gruvbox light", "gruvbox", "light", lipgloss.Color("#076678")},
		{"catppuccin dark", "catppuccin", "dark", lipgloss.Color("#89b4fa")},
		{"catppuccin light", "catppuccin", "light", lipgloss.Color("#1e66f5")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(config.ThemeConfig{Name: tt.preset, Mode: tt.mode})

			theme := Current()
			if theme.Primary != tt.expectedColor {
				t.Errorf("expected primary color %v for theme %s mode %s, got %v",
					tt.expectedColor, tt.preset, tt.mode, theme.Primary)
			}
		})
	}

	// Reset to default
	Init(config.ThemeConfig{})
}

func TestInit_CustomColors(t *testing.T) {
	Init(config.ThemeConfig{
		Primary: "#ff0000",
		Accent:  "#00ff00",
	})

	theme := Current()

	if theme.Primary != lipgloss.Color("#ff0000") {
		t.Errorf("expected custom primary color #ff0000, got %v", theme.Primary)
	}
	if theme.Accent != lipgloss.Color("#00ff00") {
		t.Errorf("expected custom accent color #00ff00, got %v", theme.Accent)
	}

	// Reset to default
	Init(config.ThemeConfig{})
}

func TestInit_PresetWithOverride(t *testing.T) {
	// Use dracula preset but override accent color
	Init(config.ThemeConfig{
		Name:   "dracula",
		Accent: "#123456",
	})

	theme := Current()

	// Primary should be dracula's purple
	if theme.Primary != lipgloss.Color("#bd93f9") {
		t.Errorf("expected dracula primary color, got %v", theme.Primary)
	}

	// Accent should be overridden
	if theme.Accent != lipgloss.Color("#123456") {
		t.Errorf("expected custom accent color #123456, got %v", theme.Accent)
	}

	// Reset to default
	Init(config.ThemeConfig{})
}

func TestGetPreset(t *testing.T) {
	// Valid preset
	preset := GetPreset("dracula")
	if preset == nil {
		t.Error("expected dracula preset to exist")
	}

	// Invalid preset
	preset = GetPreset("nonexistent")
	if preset != nil {
		t.Error("expected nil for nonexistent preset")
	}
}

func TestPresetNames(t *testing.T) {
	names := PresetNames()

	// Theme families (not individual variants)
	expected := []string{"default", "dracula", "nord", "gruvbox", "catppuccin"}

	if len(names) != len(expected) {
		t.Errorf("expected %d preset names, got %d", len(expected), len(names))
	}

	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected preset name %s at index %d, got %s", name, i, names[i])
		}
	}
}

func TestApplyTheme_UpdatesGlobalStyles(t *testing.T) {
	// Initialize with dracula theme
	Init(config.ThemeConfig{Name: "dracula"})

	// Check that global color variables are updated
	if Primary != lipgloss.Color("#bd93f9") {
		t.Errorf("expected Primary to be updated to dracula color, got %v", Primary)
	}

	// Check that style variables are updated
	if PrimaryStyle.GetForeground() != lipgloss.Color("#bd93f9") {
		t.Errorf("expected PrimaryStyle foreground to be updated, got %v",
			PrimaryStyle.GetForeground())
	}

	// Reset to default
	Init(config.ThemeConfig{})
}

func TestInit_UnknownThemeFallsBackToDefault(t *testing.T) {
	// Unknown theme should fall back to default (with warning logged to stderr)
	Init(config.ThemeConfig{Name: "nonexistent-theme"})

	theme := Current()

	// Should use default colors
	if theme.Primary != lipgloss.Color("62") {
		t.Errorf("expected default primary color 62 for unknown theme, got %v", theme.Primary)
	}
	if theme.Accent != lipgloss.Color("212") {
		t.Errorf("expected default accent color 212 for unknown theme, got %v", theme.Accent)
	}

	// Reset to default
	Init(config.ThemeConfig{})
}

func TestInit_DarkOnlyThemeFallsBackToDark(t *testing.T) {
	// Dracula only has dark mode - requesting light should fall back to dark
	Init(config.ThemeConfig{Name: "dracula", Mode: "light"})

	theme := Current()

	// Should use dracula dark colors (no light variant available)
	if theme.Primary != lipgloss.Color("#bd93f9") {
		t.Errorf("expected dracula primary color for dark fallback, got %v", theme.Primary)
	}

	// Reset to default
	Init(config.ThemeConfig{})
}

func TestInit_ThemeModeVariants(t *testing.T) {
	// Test that themes with both variants properly select based on mode
	tests := []struct {
		name          string
		themeName     string
		mode          string
		expectedColor color.Color // expected primary color
	}{
		{"nord dark", "nord", "dark", lipgloss.Color("#88c0d0")},
		{"nord light", "nord", "light", lipgloss.Color("#5e81ac")},
		{"gruvbox dark", "gruvbox", "dark", lipgloss.Color("#83a598")},
		{"gruvbox light", "gruvbox", "light", lipgloss.Color("#076678")},
		{"catppuccin dark", "catppuccin", "dark", lipgloss.Color("#89b4fa")},
		{"catppuccin light", "catppuccin", "light", lipgloss.Color("#1e66f5")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(config.ThemeConfig{Name: tt.themeName, Mode: tt.mode})

			theme := Current()
			if theme.Primary != tt.expectedColor {
				t.Errorf("expected %s %s primary %v, got %v",
					tt.themeName, tt.mode, tt.expectedColor, theme.Primary)
			}
		})
	}

	// Reset to default
	Init(config.ThemeConfig{})
}

func TestInit_InvalidModeFallsBackToAuto(t *testing.T) {
	// Invalid mode should log warning and use auto (falls back to dark in non-TTY)
	Init(config.ThemeConfig{Name: "nord", Mode: "invalid"})

	theme := Current()

	// In test environment (non-TTY), auto detects as dark
	// Should be one of the nord variants (test passes if it doesn't crash)
	if theme.Primary != lipgloss.Color("#88c0d0") && theme.Primary != lipgloss.Color("#5e81ac") {
		t.Errorf("expected nord theme color, got %v", theme.Primary)
	}

	// Reset to default
	Init(config.ThemeConfig{})
}

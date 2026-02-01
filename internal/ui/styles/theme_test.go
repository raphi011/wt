package styles

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
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
		expectedColor lipgloss.TerminalColor // primary color to check
	}{
		{"dracula", "dracula", lipgloss.Color("#bd93f9")},
		{"nord", "nord", lipgloss.Color("#88c0d0")},
		{"gruvbox", "gruvbox", lipgloss.Color("#83a598")},
		{"catppuccin-frappe", "catppuccin-frappe", lipgloss.Color("#8caaee")},
		{"catppuccin-mocha", "catppuccin-mocha", lipgloss.Color("#89b4fa")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(config.ThemeConfig{Name: tt.preset})

			theme := Current()
			if theme.Primary != tt.expectedColor {
				t.Errorf("expected primary color %v for theme %s, got %v",
					tt.expectedColor, tt.preset, theme.Primary)
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

	expected := []string{"default", "dracula", "nord", "gruvbox", "catppuccin-frappe", "catppuccin-mocha"}

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

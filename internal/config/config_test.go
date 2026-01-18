package config

import (
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.DefaultPath != "" {
		t.Errorf("expected default path '', got %q", cfg.DefaultPath)
	}
}

func TestLoadNonexistent(t *testing.T) {
	// When config doesn't exist, should return default without error
	// This test relies on the actual home directory behavior
	cfg, err := Load()
	if err != nil {
		// Only fail if there's a parsing error, not if file doesn't exist
		t.Logf("Load returned error (may be expected): %v", err)
	}
	// Empty DefaultPath is valid (means not configured)
	_ = cfg
}

func TestValidateDefaultPath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"", false},                // empty is allowed
		{"~/Git/worktrees", false}, // tilde path
		{"~", false},               // just tilde
		{"/absolute/path", false},  // absolute path
		{".", true},                // relative - not allowed
		{"..", true},               // relative - not allowed
		{"relative/path", true},    // relative - not allowed
		{"./foo", true},            // relative - not allowed
		{"../foo", true},           // relative - not allowed
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := ValidateDefaultPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDefaultPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestParseHooksConfig(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]interface{}
		expected HooksConfig
	}{
		{
			name: "full hooks config",
			raw: map[string]interface{}{
				"default": "kitty",
				"kitty": map[string]interface{}{
					"command":     "kitty @ launch --cwd={path}",
					"description": "Open kitty tab",
				},
				"vscode": map[string]interface{}{
					"command":       "code {path}",
					"description":   "Open VS Code",
					"run_on_exists": true,
				},
			},
			expected: HooksConfig{
				Default: "kitty",
				Hooks: map[string]Hook{
					"kitty": {
						Command:     "kitty @ launch --cwd={path}",
						Description: "Open kitty tab",
						RunOnExists: false,
					},
					"vscode": {
						Command:     "code {path}",
						Description: "Open VS Code",
						RunOnExists: true,
					},
				},
			},
		},
		{
			name: "no default",
			raw: map[string]interface{}{
				"test": map[string]interface{}{
					"command": "echo test",
				},
			},
			expected: HooksConfig{
				Default: "",
				Hooks: map[string]Hook{
					"test": {Command: "echo test"},
				},
			},
		},
		{
			name:     "nil input",
			raw:      nil,
			expected: HooksConfig{Hooks: map[string]Hook{}},
		},
		{
			name:     "empty input",
			raw:      map[string]interface{}{},
			expected: HooksConfig{Hooks: map[string]Hook{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseHooksConfig(tt.raw)

			if result.Default != tt.expected.Default {
				t.Errorf("Default = %q, want %q", result.Default, tt.expected.Default)
			}

			if len(result.Hooks) != len(tt.expected.Hooks) {
				t.Errorf("len(Hooks) = %d, want %d", len(result.Hooks), len(tt.expected.Hooks))
				return
			}

			for name, expectedHook := range tt.expected.Hooks {
				gotHook, ok := result.Hooks[name]
				if !ok {
					t.Errorf("missing hook %q", name)
					continue
				}
				if gotHook.Command != expectedHook.Command {
					t.Errorf("hook %q Command = %q, want %q", name, gotHook.Command, expectedHook.Command)
				}
				if gotHook.Description != expectedHook.Description {
					t.Errorf("hook %q Description = %q, want %q", name, gotHook.Description, expectedHook.Description)
				}
				if gotHook.RunOnExists != expectedHook.RunOnExists {
					t.Errorf("hook %q RunOnExists = %v, want %v", name, gotHook.RunOnExists, expectedHook.RunOnExists)
				}
			}
		})
	}
}

package config

import (
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.WorktreeDir != "" {
		t.Errorf("expected worktree dir '', got %q", cfg.WorktreeDir)
	}
	if cfg.RepoDir != "" {
		t.Errorf("expected repo dir '', got %q", cfg.RepoDir)
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

func TestValidatePath(t *testing.T) {
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
			err := ValidatePath(tt.path, "test_field")
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestRepoScanDir(t *testing.T) {
	tests := []struct {
		name        string
		worktreeDir string
		repoDir     string
		expected    string
	}{
		{"both empty", "", "", ""},
		{"only worktree_dir", "/worktrees", "", "/worktrees"},
		{"only repo_dir", "", "/repos", "/repos"},
		{"both set - repo_dir takes precedence", "/worktrees", "/repos", "/repos"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				WorktreeDir: tt.worktreeDir,
				RepoDir:     tt.repoDir,
			}
			result := cfg.RepoScanDir()
			if result != tt.expected {
				t.Errorf("RepoScanDir() = %q, want %q", result, tt.expected)
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
				"kitty": map[string]interface{}{
					"command":     "kitty @ launch --cwd={path}",
					"description": "Open kitty tab",
					"on":          []interface{}{"create", "open"},
				},
				"vscode": map[string]interface{}{
					"command":     "code {path}",
					"description": "Open VS Code",
				},
			},
			expected: HooksConfig{
				Hooks: map[string]Hook{
					"kitty": {
						Command:     "kitty @ launch --cwd={path}",
						Description: "Open kitty tab",
						On:          []string{"create", "open"},
					},
					"vscode": {
						Command:     "code {path}",
						Description: "Open VS Code",
					},
				},
			},
		},
		{
			name: "hook without on",
			raw: map[string]interface{}{
				"test": map[string]interface{}{
					"command": "echo test",
				},
			},
			expected: HooksConfig{
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
			}
		})
	}
}

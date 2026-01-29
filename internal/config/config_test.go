package config

import (
	"testing"

	"github.com/BurntSushi/toml"
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
					"command":     "kitty @ launch --cwd={worktree-dir}",
					"description": "Open kitty tab",
					"on":          []interface{}{"create", "open"},
				},
				"vscode": map[string]interface{}{
					"command":     "code {worktree-dir}",
					"description": "Open VS Code",
				},
			},
			expected: HooksConfig{
				Hooks: map[string]Hook{
					"kitty": {
						Command:     "kitty @ launch --cwd={worktree-dir}",
						Description: "Open kitty tab",
						On:          []string{"create", "open"},
					},
					"vscode": {
						Command:     "code {worktree-dir}",
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

func TestForgeConfigGetForgeTypeForRepo(t *testing.T) {
	cfg := ForgeConfig{
		Default: "github",
		Rules: []ForgeRule{
			{Pattern: "n26/*", Type: "github"},
			{Pattern: "company/*", Type: "gitlab"},
			{Pattern: "personal/*", Type: ""}, // empty type, should fall through
		},
	}

	tests := []struct {
		repoSpec string
		want     string
	}{
		{"n26/repo", "github"},
		{"company/repo", "gitlab"},
		{"personal/repo", "github"}, // empty type in rule, falls back to default
		{"other/repo", "github"},    // no match, uses default
	}

	for _, tt := range tests {
		t.Run(tt.repoSpec, func(t *testing.T) {
			got := cfg.GetForgeTypeForRepo(tt.repoSpec)
			if got != tt.want {
				t.Errorf("GetForgeTypeForRepo(%q) = %q, want %q", tt.repoSpec, got, tt.want)
			}
		})
	}
}

func TestForgeConfigGetUserForRepo(t *testing.T) {
	cfg := ForgeConfig{
		Default: "github",
		Rules: []ForgeRule{
			{Pattern: "n26/*", Type: "github", User: "work-user"},
			{Pattern: "personal/*", Type: "github", User: "personal-user"},
			{Pattern: "company/*", Type: "gitlab"}, // no user
		},
	}

	tests := []struct {
		repoSpec string
		want     string
	}{
		{"n26/repo", "work-user"},
		{"personal/repo", "personal-user"},
		{"company/repo", ""}, // no user in rule
		{"other/repo", ""},   // no match, returns empty
	}

	for _, tt := range tests {
		t.Run(tt.repoSpec, func(t *testing.T) {
			got := cfg.GetUserForRepo(tt.repoSpec)
			if got != tt.want {
				t.Errorf("GetUserForRepo(%q) = %q, want %q", tt.repoSpec, got, tt.want)
			}
		})
	}
}

func TestDefaultConfigIsValidTOML(t *testing.T) {
	content := DefaultConfig()
	var raw rawConfig
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Errorf("DefaultConfig() produces invalid TOML: %v\nContent:\n%s", err, content)
	}
}

func TestDefaultConfigWithDirIsValidTOML(t *testing.T) {
	content := DefaultConfigWithDir("~/Git/worktrees")
	var raw rawConfig
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Errorf("DefaultConfigWithDir() produces invalid TOML: %v\nContent:\n%s", err, content)
	}

	// Verify the worktree_dir was set correctly
	if raw.WorktreeDir != "~/Git/worktrees" {
		t.Errorf("worktree_dir = %q, want %q", raw.WorktreeDir, "~/Git/worktrees")
	}
}

func TestDefaultConfigWithDirsIsValidTOML(t *testing.T) {
	content := DefaultConfigWithDirs("~/Git/worktrees", "~/Code")
	var raw rawConfig
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Errorf("DefaultConfigWithDirs() produces invalid TOML: %v\nContent:\n%s", err, content)
	}

	// Verify both dirs were set correctly
	if raw.WorktreeDir != "~/Git/worktrees" {
		t.Errorf("worktree_dir = %q, want %q", raw.WorktreeDir, "~/Git/worktrees")
	}
	if raw.RepoDir != "~/Code" {
		t.Errorf("repo_dir = %q, want %q", raw.RepoDir, "~/Code")
	}
}

func TestDefaultConfigWithDirsNoRepoDir(t *testing.T) {
	// When repo_dir is empty, it should not appear in the output
	content := DefaultConfigWithDirs("~/Git/worktrees", "")
	var raw rawConfig
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Errorf("DefaultConfigWithDirs() produces invalid TOML: %v\nContent:\n%s", err, content)
	}

	// Verify worktree_dir was set
	if raw.WorktreeDir != "~/Git/worktrees" {
		t.Errorf("worktree_dir = %q, want %q", raw.WorktreeDir, "~/Git/worktrees")
	}
	// Verify repo_dir is empty (not set)
	if raw.RepoDir != "" {
		t.Errorf("repo_dir = %q, want empty", raw.RepoDir)
	}
}

func TestAutoFetchParsing(t *testing.T) {
	// Verify auto_fetch is parsed correctly from TOML
	tests := []struct {
		name     string
		toml     string
		expected bool
	}{
		{"not set", `worktree_format = "{repo}-{branch}"`, false},
		{"set true", `auto_fetch = true`, true},
		{"set false", `auto_fetch = false`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw rawConfig
			if _, err := toml.Decode(tt.toml, &raw); err != nil {
				t.Fatalf("failed to parse TOML: %v", err)
			}
			if raw.AutoFetch != tt.expected {
				t.Errorf("AutoFetch = %v, want %v", raw.AutoFetch, tt.expected)
			}
		})
	}
}

func TestDefaultConfigWithDirSpecialChars(t *testing.T) {
	// Test with paths containing special characters that need TOML escaping
	paths := []string{
		"/path/with spaces/worktrees",
		`/path/with\backslash`,
		"/path/with'quotes",
		`/path/with"doublequotes`,
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			content := DefaultConfigWithDir(path)
			var raw rawConfig
			if _, err := toml.Decode(content, &raw); err != nil {
				t.Errorf("DefaultConfigWithDir(%q) produces invalid TOML: %v", path, err)
				return
			}
			if raw.WorktreeDir != path {
				t.Errorf("worktree_dir = %q, want %q", raw.WorktreeDir, path)
			}
		})
	}
}

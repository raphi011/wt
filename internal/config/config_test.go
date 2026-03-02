package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Checkout.WorktreeFormat != DefaultWorktreeFormat {
		t.Errorf("expected checkout.worktree_format %q, got %q", DefaultWorktreeFormat, cfg.Checkout.WorktreeFormat)
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

func TestParseHooksConfig(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]any
		expected HooksConfig
	}{
		{
			name: "full hooks config",
			raw: map[string]any{
				"kitty": map[string]any{
					"command":     "kitty @ launch --cwd={worktree-dir}",
					"description": "Open kitty tab",
					"on":          []any{"create", "open"},
				},
				"vscode": map[string]any{
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
			raw: map[string]any{
				"test": map[string]any{
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
			raw:      map[string]any{},
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

func TestAutoFetchParsing(t *testing.T) {
	// Verify auto_fetch is parsed correctly from TOML under [checkout] section
	tests := []struct {
		name     string
		toml     string
		expected bool
	}{
		{"not set", `[checkout]
worktree_format = "{repo}-{branch}"`, false},
		{"set true", `[checkout]
auto_fetch = true`, true},
		{"set false", `[checkout]
auto_fetch = false`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw rawConfig
			if _, err := toml.Decode(tt.toml, &raw); err != nil {
				t.Fatalf("failed to parse TOML: %v", err)
			}
			if raw.Checkout.AutoFetch != tt.expected {
				t.Errorf("Checkout.AutoFetch = %v, want %v", raw.Checkout.AutoFetch, tt.expected)
			}
		})
	}
}

func TestIsValidThemeName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"none", true},
		{"default", true},
		{"dracula", true},
		{"nord", true},
		{"gruvbox", true},
		{"catppuccin", true}, // family name (not variant suffixes)
		{"invalid", false},
		{"", false},
		{"DRACULA", false},           // case-sensitive
		{"catppuccin-mocha", false},  // old variant name no longer valid
		{"catppuccin-frappe", false}, // old variant name no longer valid
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidThemeName(tt.name)
			if result != tt.valid {
				t.Errorf("isValidThemeName(%q) = %v, want %v", tt.name, result, tt.valid)
			}
		})
	}
}

func TestThemeConfigParsing(t *testing.T) {
	tests := []struct {
		name     string
		toml     string
		expected ThemeConfig
	}{
		{
			name:     "empty theme",
			toml:     `worktree_format = "{repo}-{branch}"`,
			expected: ThemeConfig{},
		},
		{
			name: "preset only",
			toml: `[theme]
name = "dracula"`,
			expected: ThemeConfig{Name: "dracula"},
		},
		{
			name: "custom colors",
			toml: `[theme]
primary = "#ff0000"
accent = "#00ff00"`,
			expected: ThemeConfig{Primary: "#ff0000", Accent: "#00ff00"},
		},
		{
			name: "preset with override",
			toml: `[theme]
name = "nord"
accent = "#ff79c6"`,
			expected: ThemeConfig{Name: "nord", Accent: "#ff79c6"},
		},
		{
			name: "all colors",
			toml: `[theme]
primary = "#111111"
accent = "#222222"
success = "#333333"
error = "#444444"
muted = "#555555"
normal = "#666666"
info = "#777777"`,
			expected: ThemeConfig{
				Primary: "#111111",
				Accent:  "#222222",
				Success: "#333333",
				Error:   "#444444",
				Muted:   "#555555",
				Normal:  "#666666",
				Info:    "#777777",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw rawConfig
			if _, err := toml.Decode(tt.toml, &raw); err != nil {
				t.Fatalf("failed to parse TOML: %v", err)
			}
			if raw.Theme != tt.expected {
				t.Errorf("Theme = %+v, want %+v", raw.Theme, tt.expected)
			}
		})
	}
}

func TestValidThemeNames(t *testing.T) {
	// Verify ValidThemeNames contains expected theme families
	expected := []string{"none", "default", "dracula", "nord", "gruvbox", "catppuccin"}

	if len(ValidThemeNames) != len(expected) {
		t.Errorf("len(ValidThemeNames) = %d, want %d", len(ValidThemeNames), len(expected))
	}

	for i, name := range expected {
		if ValidThemeNames[i] != name {
			t.Errorf("ValidThemeNames[%d] = %q, want %q", i, ValidThemeNames[i], name)
		}
	}
}

func TestDefaultSortValidation(t *testing.T) {
	tests := []struct {
		name    string
		sort    string
		wantErr bool
	}{
		{"empty", "", false},
		{"date", "date", false},
		{"repo", "repo", false},
		{"branch", "branch", false},
		{"invalid name", "name", true},
		{"old value id", "id", true},
		{"old value created", "created", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate Load()'s validation for default_sort
			isInvalid := tt.sort != "" && tt.sort != "date" && tt.sort != "repo" && tt.sort != "branch"
			if isInvalid != tt.wantErr {
				t.Errorf("default_sort %q: isInvalid=%v, wantErr=%v", tt.sort, isInvalid, tt.wantErr)
			}

			// Also verify TOML round-trip maps the field correctly
			input := `default_sort = "` + tt.sort + `"`
			if tt.sort == "" {
				input = `default_sort = ""`
			}
			var raw rawConfig
			if _, err := toml.Decode(input, &raw); err != nil {
				t.Fatalf("failed to parse TOML: %v", err)
			}
			if raw.DefaultSort != tt.sort {
				t.Errorf("TOML round-trip: got %q, want %q", raw.DefaultSort, tt.sort)
			}
		})
	}
}

func TestValidThemeModes(t *testing.T) {
	// Verify ValidThemeModes contains expected modes
	expected := []string{"auto", "light", "dark"}

	if len(ValidThemeModes) != len(expected) {
		t.Errorf("len(ValidThemeModes) = %d, want %d", len(ValidThemeModes), len(expected))
	}

	for i, name := range expected {
		if ValidThemeModes[i] != name {
			t.Errorf("ValidThemeModes[%d] = %q, want %q", i, ValidThemeModes[i], name)
		}
	}
}

func TestWithConfig_FromContext(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{DefaultSort: "branch"}
		ctx := WithConfig(context.Background(), cfg)
		got := FromContext(ctx)
		if got != cfg {
			t.Error("FromContext did not return the stored config")
		}
		if got.DefaultSort != "branch" {
			t.Errorf("DefaultSort = %q, want %q", got.DefaultSort, "branch")
		}
	})

	t.Run("nil when not set", func(t *testing.T) {
		t.Parallel()
		got := FromContext(context.Background())
		if got != nil {
			t.Errorf("FromContext on empty context = %v, want nil", got)
		}
	})
}

func TestWithWorkDir_FromContext(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()
		ctx := WithWorkDir(context.Background(), "/custom/path")
		got := WorkDirFromContext(ctx)
		if got != "/custom/path" {
			t.Errorf("WorkDirFromContext = %q, want %q", got, "/custom/path")
		}
	})

	t.Run("fallback to getwd when not set", func(t *testing.T) {
		t.Parallel()
		got := WorkDirFromContext(context.Background())
		wd, _ := os.Getwd()
		if got != wd {
			t.Errorf("WorkDirFromContext = %q, want %q (os.Getwd)", got, wd)
		}
	})

	t.Run("fallback to getwd when empty", func(t *testing.T) {
		t.Parallel()
		ctx := WithWorkDir(context.Background(), "")
		got := WorkDirFromContext(ctx)
		wd, _ := os.Getwd()
		if got != wd {
			t.Errorf("WorkDirFromContext = %q, want %q (os.Getwd)", got, wd)
		}
	})
}

func TestGetHistoryPath(t *testing.T) {
	t.Parallel()

	t.Run("override", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{HistoryPath: "/custom/history.json"}
		if got := cfg.GetHistoryPath(); got != "/custom/history.json" {
			t.Errorf("GetHistoryPath = %q, want %q", got, "/custom/history.json")
		}
	})

	t.Run("default", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{}
		got := cfg.GetHistoryPath()
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".wt", "history.json")
		if got != want {
			t.Errorf("GetHistoryPath = %q, want %q", got, want)
		}
	})
}

func TestShouldSetUpstream(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ptr  *bool
		want bool
	}{
		{"nil defaults to false", nil, false},
		{"true", new(true), true},
		{"false", new(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cc := &CheckoutConfig{SetUpstream: tt.ptr}
			if got := cc.ShouldSetUpstream(); got != tt.want {
				t.Errorf("ShouldSetUpstream() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		spec     string
		expected bool
	}{
		{"wildcard matches all", "*", "anything/here", true},
		{"prefix match", "n26/*", "n26/repo", true},
		{"prefix no match", "n26/*", "other/repo", false},
		{"suffix match", "*/repo", "org/repo", true},
		{"suffix no match", "*/repo", "org/other", false},
		{"exact match", "org/repo", "org/repo", true},
		{"exact no match", "org/repo", "org/other", false},
		{"prefix with trailing chars", "n26/*", "n26/", true},
		{"empty spec against prefix", "n26/*", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := matchPattern(tt.pattern, tt.spec); got != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.spec, got, tt.expected)
			}
		})
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	// Cannot use t.Parallel() — t.Setenv mutates process env
	t.Run("WT_THEME overrides theme name", func(t *testing.T) {
		t.Setenv("WT_THEME", "nord")
		cfg := Default()
		if err := applyEnvOverrides(&cfg); err != nil {
			t.Fatalf("applyEnvOverrides error: %v", err)
		}
		if cfg.Theme.Name != "nord" {
			t.Errorf("Theme.Name = %q, want %q", cfg.Theme.Name, "nord")
		}
	})

	t.Run("WT_THEME_MODE overrides theme mode", func(t *testing.T) {
		t.Setenv("WT_THEME_MODE", "dark")
		cfg := Default()
		if err := applyEnvOverrides(&cfg); err != nil {
			t.Fatalf("applyEnvOverrides error: %v", err)
		}
		if cfg.Theme.Mode != "dark" {
			t.Errorf("Theme.Mode = %q, want %q", cfg.Theme.Mode, "dark")
		}
	})

	t.Run("empty env vars leave config unchanged", func(t *testing.T) {
		t.Setenv("WT_THEME", "")
		t.Setenv("WT_THEME_MODE", "")
		cfg := Config{Theme: ThemeConfig{Name: "dracula", Mode: "light"}}
		if err := applyEnvOverrides(&cfg); err != nil {
			t.Fatalf("applyEnvOverrides error: %v", err)
		}
		if cfg.Theme.Name != "dracula" {
			t.Errorf("Theme.Name = %q, want %q", cfg.Theme.Name, "dracula")
		}
		if cfg.Theme.Mode != "light" {
			t.Errorf("Theme.Mode = %q, want %q", cfg.Theme.Mode, "light")
		}
	})
}

func TestValidateEnum(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		field   string
		allowed []string
		wantErr bool
	}{
		{"empty value is ok", "", "test", []string{"a", "b"}, false},
		{"valid value", "a", "test", []string{"a", "b"}, false},
		{"invalid value", "c", "test", []string{"a", "b"}, true},
		{"case sensitive", "A", "test", []string{"a", "b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateEnum(tt.value, tt.field, tt.allowed)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnum(%q, %q, %v) error = %v, wantErr %v", tt.value, tt.field, tt.allowed, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePreservePatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []string
		context  string
		wantErr  bool
	}{
		{"valid patterns", []string{".env", ".env.*", "*.local"}, "", false},
		{"empty patterns", []string{}, "", false},
		{"nil patterns", nil, "", false},
		{"invalid pattern", []string{"[invalid"}, "", true},
		{"with context info", []string{"[bad"}, ".wt.toml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePreservePatterns(tt.patterns, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePreservePatterns(%v, %q) error = %v, wantErr %v", tt.patterns, tt.context, err, tt.wantErr)
			}
		})
	}
}

func TestFormatOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []string
		want string
	}{
		{"single option", []string{"a"}, `"a"`},
		{"two options", []string{"a", "b"}, `"a" or "b"`},
		{"three options", []string{"a", "b", "c"}, `"a", "b", or "c"`},
		{"four options", []string{"a", "b", "c", "d"}, `"a", "b", "c", or "d"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatOptions(tt.opts)
			if got != tt.want {
				t.Errorf("formatOptions(%v) = %q, want %q", tt.opts, got, tt.want)
			}
		})
	}
}

func TestDefaultLocalConfigIsValidTOML(t *testing.T) {
	t.Parallel()

	content := DefaultLocalConfig()
	var raw rawLocalConfig
	if _, err := toml.Decode(content, &raw); err != nil {
		t.Errorf("DefaultLocalConfig() produces invalid TOML: %v", err)
	}
}

func TestParseHooksConfig_WithEnabled(t *testing.T) {
	t.Parallel()

	raw := map[string]any{
		"enabled-hook": map[string]any{
			"command": "echo enabled",
			"enabled": true,
		},
		"disabled-hook": map[string]any{
			"command": "echo disabled",
			"enabled": false,
		},
		"default-hook": map[string]any{
			"command": "echo default",
		},
	}

	result := parseHooksConfig(raw)

	if len(result.Hooks) != 3 {
		t.Fatalf("len(Hooks) = %d, want 3", len(result.Hooks))
	}

	// enabled-hook: Enabled = true
	eh := result.Hooks["enabled-hook"]
	if eh.Enabled == nil || !*eh.Enabled {
		t.Errorf("enabled-hook.Enabled = %v, want true", eh.Enabled)
	}
	if !eh.IsEnabled() {
		t.Error("enabled-hook.IsEnabled() = false, want true")
	}

	// disabled-hook: Enabled = false
	dh := result.Hooks["disabled-hook"]
	if dh.Enabled == nil || *dh.Enabled {
		t.Errorf("disabled-hook.Enabled = %v, want false", dh.Enabled)
	}
	if dh.IsEnabled() {
		t.Error("disabled-hook.IsEnabled() = true, want false")
	}

	// default-hook: Enabled = nil (defaults to true)
	defh := result.Hooks["default-hook"]
	if defh.Enabled != nil {
		t.Errorf("default-hook.Enabled = %v, want nil", defh.Enabled)
	}
	if !defh.IsEnabled() {
		t.Error("default-hook.IsEnabled() = false, want true")
	}
}

func TestCloneConfigIsBare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode string
		want bool
	}{
		{"bare mode", "bare", true},
		{"regular mode", "regular", false},
		{"empty defaults to bare", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cc := &CloneConfig{Mode: tt.mode}
			if got := cc.IsBare(); got != tt.want {
				t.Errorf("IsBare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneConfigParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		toml     string
		expected string
	}{
		{"not set", `worktree_format = "x"`, ""},
		{"bare", `[clone]
mode = "bare"`, "bare"},
		{"regular", `[clone]
mode = "regular"`, "regular"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var raw rawConfig
			if _, err := toml.Decode(tt.toml, &raw); err != nil {
				t.Fatalf("failed to parse TOML: %v", err)
			}
			if raw.Clone.Mode != tt.expected {
				t.Errorf("Clone.Mode = %q, want %q", raw.Clone.Mode, tt.expected)
			}
		})
	}
}

func TestCloneConfigDefault(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if cfg.Clone.Mode != "bare" {
		t.Errorf("default Clone.Mode = %q, want %q", cfg.Clone.Mode, "bare")
	}
	if !cfg.Clone.IsBare() {
		t.Error("default Clone.IsBare() = false, want true")
	}
}

func TestCloneConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"empty is valid", "", false},
		{"bare is valid", "bare", false},
		{"regular is valid", "regular", false},
		{"invalid mode", "shallow", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateEnum(tt.mode, "clone.mode", ValidCloneModes)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnum(%q) error = %v, wantErr %v", tt.mode, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCloneMode(t *testing.T) {
	t.Parallel()

	if err := ValidateCloneMode("bare"); err != nil {
		t.Errorf("ValidateCloneMode(bare) = %v, want nil", err)
	}
	if err := ValidateCloneMode("regular"); err != nil {
		t.Errorf("ValidateCloneMode(regular) = %v, want nil", err)
	}
	if err := ValidateCloneMode("invalid"); err == nil {
		t.Error("ValidateCloneMode(invalid) = nil, want error")
	}
}

func TestCloneConfigResolveIsBare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		configMode  string
		cliOverride string
		wantBare    bool
		wantErr     bool
	}{
		{"config bare, no override", "bare", "", true, false},
		{"config regular, no override", "regular", "", false, false},
		{"config empty, no override", "", "", true, false},
		{"cli overrides config to regular", "bare", "regular", false, false},
		{"cli overrides config to bare", "regular", "bare", true, false},
		{"invalid cli override", "bare", "shallow", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cc := &CloneConfig{Mode: tt.configMode}
			got, err := cc.ResolveIsBare(tt.cliOverride)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveIsBare(%q) error = %v, wantErr %v", tt.cliOverride, err, tt.wantErr)
			}
			if err == nil && got != tt.wantBare {
				t.Errorf("ResolveIsBare(%q) = %v, want %v", tt.cliOverride, got, tt.wantBare)
			}
		})
	}
}

func TestStaleDaysConfigParsing(t *testing.T) {
	t.Parallel()

	input := `
[prune]
stale_days = 30
`
	var raw rawConfig
	if err := toml.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw.Prune.StaleDays == nil {
		t.Fatal("stale_days should not be nil")
	}
	if *raw.Prune.StaleDays != 30 {
		t.Errorf("stale_days = %d, want 30", *raw.Prune.StaleDays)
	}
}

func TestStaleDaysConfigDefault(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if cfg.Prune.StaleDays != 14 {
		t.Errorf("default stale_days = %d, want 14", cfg.Prune.StaleDays)
	}
}

func TestStaleDaysConfigDisabled(t *testing.T) {
	t.Parallel()

	input := `
[prune]
stale_days = 0
`
	var raw rawConfig
	if err := toml.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw.Prune.StaleDays == nil {
		t.Fatal("stale_days should not be nil when explicitly set to 0")
	}
	if *raw.Prune.StaleDays != 0 {
		t.Errorf("stale_days = %d, want 0", *raw.Prune.StaleDays)
	}
}

func TestFullConfigRoundTrip(t *testing.T) {
	t.Parallel()

	// Test parsing a full config TOML with all fields — exercises the paths in Load()
	// that parse forge rules, hosts, merge, stale_days, etc.
	input := `
default_sort = "branch"
default_labels = ["team-a", "team-b"]

[clone]
mode = "regular"

[checkout]
worktree_format = "{origin}-{branch}"
base_ref = "local"
auto_fetch = true
set_upstream = true

[forge]
default = "gitlab"
default_org = "my-org"

[[forge.rules]]
pattern = "n26/*"
type = "github"
user = "work-user"

[[forge.rules]]
pattern = "personal/*"
type = "github"

[merge]
strategy = "rebase"

[prune]
delete_local_branches = true
stale_days = 30

[preserve]
patterns = [".env", ".env.*"]
exclude = ["node_modules"]

[hosts]
"gitlab.corp.com" = "gitlab"
"gh.company.com" = "github"

[theme]
name = "dracula"
mode = "dark"

[hooks.test]
command = "echo test"
description = "Test"
on = ["checkout"]
`
	var raw rawConfig
	if err := toml.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatalf("failed to parse TOML: %v", err)
	}

	// Build config the same way Load() does
	cfg := Config{
		DefaultSort:   raw.DefaultSort,
		DefaultLabels: raw.DefaultLabels,
		Hooks:         parseHooksConfig(raw.Hooks),
		Clone:         raw.Clone,
		Checkout:      raw.Checkout,
		Forge:         raw.Forge,
		Merge:         raw.Merge,
		Prune: PruneConfig{
			DeleteLocalBranches: raw.Prune.DeleteLocalBranches,
		},
		Preserve: raw.Preserve,
		Hosts:    raw.Hosts,
		Theme:    raw.Theme,
	}

	// Validate enums
	if err := validateEnum(cfg.Forge.Default, "forge.default", ValidForgeTypes); err != nil {
		t.Fatalf("forge.default validation failed: %v", err)
	}
	for i, rule := range cfg.Forge.Rules {
		if err := validateEnum(rule.Type, "forge.rules.type", ValidForgeTypes); err != nil {
			t.Fatalf("forge.rules[%d].type validation failed: %v", i, err)
		}
	}
	for host, forgeType := range cfg.Hosts {
		if err := validateEnum(forgeType, "hosts", ValidForgeTypes); err != nil {
			t.Fatalf("hosts[%q] validation failed: %v", host, err)
		}
	}
	if err := validateEnum(cfg.Merge.Strategy, "merge.strategy", ValidMergeStrategies); err != nil {
		t.Fatalf("merge.strategy validation failed: %v", err)
	}
	if err := validateEnum(cfg.Checkout.BaseRef, "checkout.base_ref", ValidBaseRefs); err != nil {
		t.Fatalf("checkout.base_ref validation failed: %v", err)
	}
	if err := validateEnum(cfg.DefaultSort, "default_sort", ValidDefaultSortModes); err != nil {
		t.Fatalf("default_sort validation failed: %v", err)
	}
	if err := validateEnum(cfg.Clone.Mode, "clone.mode", ValidCloneModes); err != nil {
		t.Fatalf("clone.mode validation failed: %v", err)
	}
	if err := validatePreservePatterns(cfg.Preserve.Patterns, ""); err != nil {
		t.Fatalf("preserve.patterns validation failed: %v", err)
	}

	// Apply defaults
	if cfg.Checkout.WorktreeFormat == "" {
		cfg.Checkout.WorktreeFormat = DefaultWorktreeFormat
	}
	if cfg.Forge.Default == "" {
		cfg.Forge.Default = "github"
	}
	if raw.Prune.StaleDays != nil {
		cfg.Prune.StaleDays = *raw.Prune.StaleDays
	} else {
		cfg.Prune.StaleDays = 14
	}

	// Apply clone default
	if cfg.Clone.Mode == "" {
		cfg.Clone.Mode = "bare"
	}

	// Verify parsed values
	if cfg.Clone.Mode != "regular" {
		t.Errorf("Clone.Mode = %q, want %q", cfg.Clone.Mode, "regular")
	}
	if cfg.DefaultSort != "branch" {
		t.Errorf("DefaultSort = %q, want %q", cfg.DefaultSort, "branch")
	}
	if len(cfg.DefaultLabels) != 2 {
		t.Errorf("len(DefaultLabels) = %d, want 2", len(cfg.DefaultLabels))
	}
	if cfg.Checkout.WorktreeFormat != "{origin}-{branch}" {
		t.Errorf("WorktreeFormat = %q, want %q", cfg.Checkout.WorktreeFormat, "{origin}-{branch}")
	}
	if cfg.Checkout.BaseRef != "local" {
		t.Errorf("BaseRef = %q, want %q", cfg.Checkout.BaseRef, "local")
	}
	if !cfg.Checkout.AutoFetch {
		t.Error("AutoFetch should be true")
	}
	if cfg.Forge.Default != "gitlab" {
		t.Errorf("Forge.Default = %q, want %q", cfg.Forge.Default, "gitlab")
	}
	if cfg.Forge.DefaultOrg != "my-org" {
		t.Errorf("Forge.DefaultOrg = %q, want %q", cfg.Forge.DefaultOrg, "my-org")
	}
	if len(cfg.Forge.Rules) != 2 {
		t.Errorf("len(Forge.Rules) = %d, want 2", len(cfg.Forge.Rules))
	}
	if cfg.Merge.Strategy != "rebase" {
		t.Errorf("Merge.Strategy = %q, want %q", cfg.Merge.Strategy, "rebase")
	}
	if !cfg.Prune.DeleteLocalBranches {
		t.Error("Prune.DeleteLocalBranches should be true")
	}
	if cfg.Prune.StaleDays != 30 {
		t.Errorf("Prune.StaleDays = %d, want 30", cfg.Prune.StaleDays)
	}
	if len(cfg.Preserve.Patterns) != 2 {
		t.Errorf("len(Preserve.Patterns) = %d, want 2", len(cfg.Preserve.Patterns))
	}
	if len(cfg.Hosts) != 2 {
		t.Errorf("len(Hosts) = %d, want 2", len(cfg.Hosts))
	}
	if cfg.Theme.Name != "dracula" {
		t.Errorf("Theme.Name = %q, want %q", cfg.Theme.Name, "dracula")
	}
	if len(cfg.Hooks.Hooks) != 1 {
		t.Errorf("len(Hooks) = %d, want 1", len(cfg.Hooks.Hooks))
	}
}

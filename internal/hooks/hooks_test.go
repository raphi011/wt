package hooks

import (
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
)

func TestSubstitutePlaceholders(t *testing.T) {
	ctx := Context{
		Path:     "/home/user/worktrees/repo-branch",
		Branch:   "feature-branch",
		Repo:     "myrepo",
		Folder:   "repo",
		MainRepo: "/home/user/repo",
		Trigger:  "checkout",
	}

	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "single placeholder",
			command:  "code {path}",
			expected: "code /home/user/worktrees/repo-branch",
		},
		{
			name:     "multiple placeholders",
			command:  "cd {path} && echo {branch}",
			expected: "cd /home/user/worktrees/repo-branch && echo feature-branch",
		},
		{
			name:     "all placeholders",
			command:  "{path} {branch} {repo} {folder} {main-repo} {trigger}",
			expected: "/home/user/worktrees/repo-branch feature-branch myrepo repo /home/user/repo checkout",
		},
		{
			name:     "no placeholders",
			command:  "echo hello",
			expected: "echo hello",
		},
		{
			name:     "repeated placeholder",
			command:  "{path} and {path}",
			expected: "/home/user/worktrees/repo-branch and /home/user/worktrees/repo-branch",
		},
		{
			name:     "trigger placeholder",
			command:  "echo triggered by {trigger}",
			expected: "echo triggered by checkout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstitutePlaceholders(tt.command, ctx)
			if result != tt.expected {
				t.Errorf("SubstitutePlaceholders(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

func TestSubstitutePlaceholders_SpecialChars(t *testing.T) {
	tests := []struct {
		name     string
		ctx      Context
		command  string
		expected string
	}{
		{
			name: "path with spaces",
			ctx: Context{
				Path: "/home/user/my documents/worktree",
			},
			command:  "code {path}",
			expected: "code /home/user/my documents/worktree",
		},
		{
			name: "branch with special chars",
			ctx: Context{
				Branch: "feature/test-branch",
			},
			command:  "echo {branch}",
			expected: "echo feature/test-branch",
		},
		{
			name: "value with single quotes",
			ctx: Context{
				Path: "/home/user/it's a path",
			},
			command:  "code {path}",
			expected: "code /home/user/it's a path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstitutePlaceholders(tt.command, tt.ctx)
			if result != tt.expected {
				t.Errorf("SubstitutePlaceholders(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

func TestSelectHooks(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"kitty": {
				Command:     "kitty @ launch --cwd={path}",
				Description: "Open kitty tab",
				On:          []string{"checkout"},
			},
			"vscode": {
				Command:     "code {path}",
				Description: "Open VS Code",
				// no On - only runs via explicit --hook
			},
			"pr-setup": {
				Command: "npm install",
				On:      []string{"pr"},
			},
		},
	}

	tests := []struct {
		name        string
		hookFlags   []string
		noHook      bool
		cmdType     CommandType
		expectCount int
		expectNames []string
		expectError bool
	}{
		{
			name:        "hook with on=checkout runs for checkout",
			cmdType:     CommandCheckout,
			expectCount: 1,
			expectNames: []string{"kitty"},
		},
		{
			name:        "hook with on=pr runs for pr",
			cmdType:     CommandPR,
			expectCount: 1,
			expectNames: []string{"pr-setup"},
		},
		{
			name:        "explicit hook runs regardless of on condition",
			hookFlags:   []string{"vscode"},
			cmdType:     CommandCheckout,
			expectCount: 1,
			expectNames: []string{"vscode"},
		},
		{
			name:        "multiple explicit hooks",
			hookFlags:   []string{"vscode", "kitty"},
			cmdType:     CommandCheckout,
			expectCount: 2,
			expectNames: []string{"vscode", "kitty"},
		},
		{
			name:        "no-hook skips all",
			noHook:      true,
			cmdType:     CommandCheckout,
			expectCount: 0,
		},
		{
			name:        "unknown hook errors",
			hookFlags:   []string{"nonexistent"},
			cmdType:     CommandCheckout,
			expectError: true,
		},
		{
			name:        "one unknown hook in list errors",
			hookFlags:   []string{"vscode", "nonexistent"},
			cmdType:     CommandCheckout,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := SelectHooks(hooksConfig, tt.hookFlags, tt.noHook, tt.cmdType)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(matches) != tt.expectCount {
				t.Errorf("expected %d hooks, got %d", tt.expectCount, len(matches))
				return
			}

			for i, expectedName := range tt.expectNames {
				if i >= len(matches) {
					break
				}
				if matches[i].Name != expectedName {
					t.Errorf("expected name %q at position %d, got %q", expectedName, i, matches[i].Name)
				}
			}
		})
	}
}

func TestSelectHooks_NoOnCondition(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"vscode": {Command: "code {path}"}, // no On - only via --hook
		},
	}

	// Without explicit --hook, hooks without "on" don't run
	matches, err := SelectHooks(hooksConfig, nil, false, CommandCheckout)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no hooks when no 'on' condition, got %d", len(matches))
	}

	// With explicit --hook, it runs
	matches, err = SelectHooks(hooksConfig, []string{"vscode"}, false, CommandCheckout)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 hook with explicit --hook, got %d", len(matches))
	}
}

func TestSelectHooks_EmptyConfig(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{},
	}

	matches, err := SelectHooks(hooksConfig, nil, false, CommandCheckout)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no hooks with empty config, got %d", len(matches))
	}
}

func TestSelectHooks_OnCondition(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"editor": {
				Command: "code {path}",
				On:      []string{"checkout"},
			},
			"pr-setup": {
				Command: "npm install && code {path}",
				On:      []string{"pr"},
			},
			"universal": {
				Command: "echo {path}",
				// On is empty - only runs via --hook
			},
		},
	}

	tests := []struct {
		name        string
		cmdType     CommandType
		expectCount int
		expectNames []string
	}{
		{
			name:        "on=checkout runs for checkout",
			cmdType:     CommandCheckout,
			expectCount: 1,
			expectNames: []string{"editor"},
		},
		{
			name:        "on=pr runs for pr command",
			cmdType:     CommandPR,
			expectCount: 1,
			expectNames: []string{"pr-setup"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := SelectHooks(hooksConfig, nil, false, tt.cmdType)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(matches) != tt.expectCount {
				t.Errorf("expected %d hooks, got %d", tt.expectCount, len(matches))
				return
			}

			for i, expectedName := range tt.expectNames {
				if matches[i].Name != expectedName {
					t.Errorf("expected name %q at position %d, got %q", expectedName, i, matches[i].Name)
				}
			}
		})
	}
}

func TestSelectHooks_OnConditionNoMatch(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"pr-only": {
				Command: "npm install",
				On:      []string{"pr"},
			},
		},
	}

	// Hook with on=pr doesn't match "create"
	matches, err := SelectHooks(hooksConfig, nil, false, CommandCheckout)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no hooks when on condition doesn't match, got %d", len(matches))
	}
}

func TestSelectHooks_MultipleMatches(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"editor": {
				Command: "code {path}",
				On:      []string{"checkout"},
			},
			"setup": {
				Command: "npm install",
				On:      []string{"checkout"},
			},
		},
	}

	// Both hooks match "checkout", should return both
	matches, err := SelectHooks(hooksConfig, nil, false, CommandCheckout)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(matches))
	}
}

func TestSelectHooks_OnAll(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"universal": {
				Command: "notify-send {branch}",
				On:      []string{"all"},
			},
		},
	}

	// "all" should match all command types
	for _, cmdType := range []CommandType{CommandCheckout, CommandPR, CommandPrune} {
		matches, err := SelectHooks(hooksConfig, nil, false, cmdType)
		if err != nil {
			t.Errorf("unexpected error for %s: %v", cmdType, err)
		}
		if len(matches) != 1 {
			t.Errorf("expected 1 hook for %s with on=all, got %d", cmdType, len(matches))
		}
	}
}

func TestSelectHooks_PruneCommand(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"cleanup": {
				Command:     "echo 'Removed {branch}'",
				Description: "Log removal",
				On:          []string{"prune"},
			},
			"editor": {
				Command: "code {path}",
				On:      []string{"checkout"},
			},
		},
	}

	// Prune hook runs for prune command
	matches, err := SelectHooks(hooksConfig, nil, false, CommandPrune)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 hook for prune, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].Name != "cleanup" {
		t.Errorf("expected cleanup hook, got %s", matches[0].Name)
	}

	// Prune hook does NOT run for checkout command
	matches, err = SelectHooks(hooksConfig, nil, false, CommandCheckout)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 1 || matches[0].Name != "editor" {
		t.Errorf("expected only editor hook for checkout, got %v", matches)
	}
}

func TestParseEnv(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		expected    map[string]string
		expectError bool
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: map[string]string{},
		},
		{
			name:  "single key=value",
			input: []string{"prompt=hello world"},
			expected: map[string]string{
				"prompt": "hello world",
			},
		},
		{
			name:  "multiple key=value",
			input: []string{"prompt=hello", "mode=ask", "verbose=true"},
			expected: map[string]string{
				"prompt":  "hello",
				"mode":    "ask",
				"verbose": "true",
			},
		},
		{
			name:  "value with equals sign",
			input: []string{"expr=1+1=2"},
			expected: map[string]string{
				"expr": "1+1=2",
			},
		},
		{
			name:  "empty value",
			input: []string{"empty="},
			expected: map[string]string{
				"empty": "",
			},
		},
		{
			name:        "missing equals sign",
			input:       []string{"invalid"},
			expectError: true,
		},
		{
			name:        "empty key",
			input:       []string{"=value"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseEnv(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d entries, got %d", len(tt.expected), len(result))
				return
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("expected %q=%q, got %q=%q", k, v, k, result[k])
				}
			}
		})
	}
}

func TestParseEnvWithStdin(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		expected    map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: map[string]string{},
		},
		{
			name:  "regular key=value without stdin",
			input: []string{"prompt=hello world"},
			expected: map[string]string{
				"prompt": "hello world",
			},
		},
		{
			name:  "multiple regular key=value",
			input: []string{"mode=ask", "verbose=true"},
			expected: map[string]string{
				"mode":    "ask",
				"verbose": "true",
			},
		},
		{
			name:  "value with equals sign",
			input: []string{"expr=1+1=2"},
			expected: map[string]string{
				"expr": "1+1=2",
			},
		},
		{
			name:        "missing equals sign",
			input:       []string{"invalid"},
			expectError: true,
			errorMsg:    "invalid env format",
		},
		{
			name:        "empty key",
			input:       []string{"=value"},
			expectError: true,
			errorMsg:    "key cannot be empty",
		},
		{
			name:        "stdin requested but not piped",
			input:       []string{"content=-"},
			expectError: true,
			errorMsg:    "stdin not piped",
		},
		{
			name:        "mixed regular and stdin (stdin not available)",
			input:       []string{"mode=ask", "content=-"},
			expectError: true,
			errorMsg:    "stdin not piped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseEnvWithStdin(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d entries, got %d", len(tt.expected), len(result))
				return
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("expected %q=%q, got %q=%q", k, v, k, result[k])
				}
			}
		})
	}
}

func TestSubstitutePlaceholders_EnvVariables(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		ctx      Context
		expected string
	}{
		{
			name:    "env variable substitution",
			command: "echo {prompt}",
			ctx: Context{
				Env: map[string]string{"prompt": "hello world"},
			},
			expected: "echo hello world",
		},
		{
			name:    "env variable with default - value provided",
			command: "echo {prompt:-default message}",
			ctx: Context{
				Env: map[string]string{"prompt": "custom message"},
			},
			expected: "echo custom message",
		},
		{
			name:    "env variable with default - no value",
			command: "echo {prompt:-default message}",
			ctx: Context{
				Env: map[string]string{},
			},
			expected: "echo default message",
		},
		{
			name:    "env variable with default - nil env",
			command: "echo {prompt:-fallback}",
			ctx: Context{
				Env: nil,
			},
			expected: "echo fallback",
		},
		{
			name:    "env variable without default - missing",
			command: "echo {prompt}",
			ctx: Context{
				Env: map[string]string{},
			},
			expected: "echo ",
		},
		{
			name:    "multiple env variables",
			command: "cmd --mode={mode} --prompt={prompt}",
			ctx: Context{
				Env: map[string]string{"mode": "ask", "prompt": "help me"},
			},
			expected: "cmd --mode=ask --prompt=help me",
		},
		{
			name:    "mixed static and env placeholders",
			command: "claude --cwd={path} {prompt:-help}",
			ctx: Context{
				Path: "/home/user/worktree",
				Env:  map[string]string{"prompt": "implement feature"},
			},
			expected: "claude --cwd=/home/user/worktree implement feature",
		},
		{
			name:    "env variable with special characters",
			command: "echo {msg}",
			ctx: Context{
				Env: map[string]string{"msg": "it's a test"},
			},
			expected: "echo it's a test",
		},
		{
			name:    "env variable with empty default",
			command: "cmd {opt:-}",
			ctx: Context{
				Env: map[string]string{},
			},
			expected: "cmd ",
		},
		{
			name:    "underscore in variable name",
			command: "echo {my_var}",
			ctx: Context{
				Env: map[string]string{"my_var": "value"},
			},
			expected: "echo value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstitutePlaceholders(tt.command, tt.ctx)
			if result != tt.expected {
				t.Errorf("SubstitutePlaceholders(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

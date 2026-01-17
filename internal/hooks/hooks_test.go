package hooks

import (
	"testing"

	"github.com/raphaelgruber/wt/internal/config"
)

func TestSubstitutePlaceholders(t *testing.T) {
	ctx := Context{
		Path:     "/home/user/worktrees/repo-branch",
		Branch:   "feature-branch",
		Repo:     "myrepo",
		Folder:   "repo",
		MainRepo: "/home/user/repo",
	}

	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "single placeholder",
			command:  "code {path}",
			expected: "code '/home/user/worktrees/repo-branch'",
		},
		{
			name:     "multiple placeholders",
			command:  "cd {path} && echo {branch}",
			expected: "cd '/home/user/worktrees/repo-branch' && echo 'feature-branch'",
		},
		{
			name:     "all placeholders",
			command:  "{path} {branch} {repo} {folder} {main-repo}",
			expected: "'/home/user/worktrees/repo-branch' 'feature-branch' 'myrepo' 'repo' '/home/user/repo'",
		},
		{
			name:     "no placeholders",
			command:  "echo hello",
			expected: "echo hello",
		},
		{
			name:     "repeated placeholder",
			command:  "{path} and {path}",
			expected: "'/home/user/worktrees/repo-branch' and '/home/user/worktrees/repo-branch'",
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

func TestSubstitutePlaceholders_ShellEscaping(t *testing.T) {
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
			expected: "code '/home/user/my documents/worktree'",
		},
		{
			name: "branch with special chars",
			ctx: Context{
				Branch: "feature/test-branch",
			},
			command:  "echo {branch}",
			expected: "echo 'feature/test-branch'",
		},
		{
			name: "value with single quotes",
			ctx: Context{
				Path: "/home/user/it's a path",
			},
			command:  "code {path}",
			expected: "code '/home/user/it'\\''s a path'",
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

func TestSelectHook(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Default: "kitty",
		Hooks: map[string]config.Hook{
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
	}

	tests := []struct {
		name          string
		hookFlag      string
		noHook        bool
		alreadyExists bool
		expectHook    bool
		expectName    string
		expectError   bool
	}{
		{
			name:       "default hook runs",
			expectHook: true,
			expectName: "kitty",
		},
		{
			name:       "explicit hook overrides default",
			hookFlag:   "vscode",
			expectHook: true,
			expectName: "vscode",
		},
		{
			name:       "no-hook skips all",
			noHook:     true,
			expectHook: false,
		},
		{
			name:        "unknown hook errors",
			hookFlag:    "nonexistent",
			expectError: true,
		},
		{
			name:          "run_on_exists=false skips on existing",
			alreadyExists: true,
			expectHook:    false,
		},
		{
			name:          "run_on_exists=true runs on existing",
			hookFlag:      "vscode",
			alreadyExists: true,
			expectHook:    true,
			expectName:    "vscode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook, name, err := SelectHook(hooksConfig, tt.hookFlag, tt.noHook, tt.alreadyExists)

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

			if tt.expectHook {
				if hook == nil {
					t.Error("expected hook, got nil")
					return
				}
				if name != tt.expectName {
					t.Errorf("expected name %q, got %q", tt.expectName, name)
				}
			} else {
				if hook != nil {
					t.Errorf("expected nil hook, got %+v", hook)
				}
			}
		})
	}
}

func TestSelectHook_NoDefault(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Default: "", // no default
		Hooks: map[string]config.Hook{
			"vscode": {Command: "code {path}"},
		},
	}

	hook, _, err := SelectHook(hooksConfig, "", false, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if hook != nil {
		t.Errorf("expected nil hook when no default, got %+v", hook)
	}
}

func TestSelectHook_EmptyConfig(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{},
	}

	hook, _, err := SelectHook(hooksConfig, "", false, false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if hook != nil {
		t.Errorf("expected nil hook with empty config, got %+v", hook)
	}
}

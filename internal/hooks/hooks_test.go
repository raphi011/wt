package hooks

import (
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
		Trigger:  "create",
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
			command:  "{path} {branch} {repo} {folder} {main-repo} {trigger}",
			expected: "'/home/user/worktrees/repo-branch' 'feature-branch' 'myrepo' 'repo' '/home/user/repo' 'create'",
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
		{
			name:     "trigger placeholder",
			command:  "echo triggered by {trigger}",
			expected: "echo triggered by 'create'",
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

func TestSelectHooks(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"kitty": {
				Command:     "kitty @ launch --cwd={path}",
				Description: "Open kitty tab",
				On:          []string{"create", "open"},
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
		hookFlag    string
		noHook      bool
		cmdType     CommandType
		expectCount int
		expectNames []string
		expectError bool
	}{
		{
			name:        "hook with on=create runs for create",
			cmdType:     CommandCreate,
			expectCount: 1,
			expectNames: []string{"kitty"},
		},
		{
			name:        "hook with on=create,open runs for open",
			cmdType:     CommandOpen,
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
			hookFlag:    "vscode",
			cmdType:     CommandCreate,
			expectCount: 1,
			expectNames: []string{"vscode"},
		},
		{
			name:        "no-hook skips all",
			noHook:      true,
			cmdType:     CommandCreate,
			expectCount: 0,
		},
		{
			name:        "unknown hook errors",
			hookFlag:    "nonexistent",
			cmdType:     CommandCreate,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := SelectHooks(hooksConfig, tt.hookFlag, tt.noHook, tt.cmdType)

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
	matches, err := SelectHooks(hooksConfig, "", false, CommandCreate)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no hooks when no 'on' condition, got %d", len(matches))
	}

	// With explicit --hook, it runs
	matches, err = SelectHooks(hooksConfig, "vscode", false, CommandCreate)
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

	matches, err := SelectHooks(hooksConfig, "", false, CommandCreate)
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
				On:      []string{"create", "open"},
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
			name:        "on=create,open runs for create",
			cmdType:     CommandCreate,
			expectCount: 1,
			expectNames: []string{"editor"},
		},
		{
			name:        "on=create,open runs for open",
			cmdType:     CommandOpen,
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
			matches, err := SelectHooks(hooksConfig, "", false, tt.cmdType)
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
	matches, err := SelectHooks(hooksConfig, "", false, CommandCreate)
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
				On:      []string{"create"},
			},
			"setup": {
				Command: "npm install",
				On:      []string{"create"},
			},
		},
	}

	// Both hooks match "create", should return both
	matches, err := SelectHooks(hooksConfig, "", false, CommandCreate)
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
	for _, cmdType := range []CommandType{CommandCreate, CommandOpen, CommandPR, CommandTidy} {
		matches, err := SelectHooks(hooksConfig, "", false, cmdType)
		if err != nil {
			t.Errorf("unexpected error for %s: %v", cmdType, err)
		}
		if len(matches) != 1 {
			t.Errorf("expected 1 hook for %s with on=all, got %d", cmdType, len(matches))
		}
	}
}

func TestSelectHooks_TidyCommand(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"cleanup": {
				Command:     "echo 'Removed {branch}'",
				Description: "Log removal",
				On:          []string{"tidy"},
			},
			"editor": {
				Command: "code {path}",
				On:      []string{"create", "open"},
			},
		},
	}

	// Tidy hook runs for tidy command
	matches, err := SelectHooks(hooksConfig, "", false, CommandTidy)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 hook for tidy, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].Name != "cleanup" {
		t.Errorf("expected cleanup hook, got %s", matches[0].Name)
	}

	// Tidy hook does NOT run for create command
	matches, err = SelectHooks(hooksConfig, "", false, CommandCreate)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 1 || matches[0].Name != "editor" {
		t.Errorf("expected only editor hook for create, got %v", matches)
	}
}

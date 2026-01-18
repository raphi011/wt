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

func TestSelectHooks(t *testing.T) {
	hooksConfig := config.HooksConfig{
		Hooks: map[string]config.Hook{
			"kitty": {
				Command:     "kitty @ launch --cwd={path}",
				Description: "Open kitty tab",
				On:          []string{"create", "open"},
				RunOnExists: false,
			},
			"vscode": {
				Command:     "code {path}",
				Description: "Open VS Code",
				RunOnExists: true,
				// no On - only runs via explicit --hook
			},
			"pr-setup": {
				Command: "npm install",
				On:      []string{"pr"},
			},
		},
	}

	tests := []struct {
		name          string
		hookFlag      string
		noHook        bool
		alreadyExists bool
		cmdType       CommandType
		expectCount   int
		expectNames   []string
		expectError   bool
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
		{
			name:          "run_on_exists=false skips on existing worktree",
			alreadyExists: true,
			cmdType:       CommandCreate,
			expectCount:   0,
		},
		{
			name:          "run_on_exists=true runs on existing worktree",
			hookFlag:      "vscode",
			alreadyExists: true,
			cmdType:       CommandCreate,
			expectCount:   1,
			expectNames:   []string{"vscode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := SelectHooks(hooksConfig, tt.hookFlag, tt.noHook, tt.alreadyExists, tt.cmdType)

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
	matches, err := SelectHooks(hooksConfig, "", false, false, CommandCreate)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no hooks when no 'on' condition, got %d", len(matches))
	}

	// With explicit --hook, it runs
	matches, err = SelectHooks(hooksConfig, "vscode", false, false, CommandCreate)
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

	matches, err := SelectHooks(hooksConfig, "", false, false, CommandCreate)
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
			matches, err := SelectHooks(hooksConfig, "", false, false, tt.cmdType)
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
	matches, err := SelectHooks(hooksConfig, "", false, false, CommandCreate)
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
	matches, err := SelectHooks(hooksConfig, "", false, false, CommandCreate)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(matches))
	}
}

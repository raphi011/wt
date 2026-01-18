package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/raphi011/wt/internal/config"
)

// shellQuote escapes a string for safe use in shell commands.
// It wraps the value in single quotes and escapes any embedded single quotes.
func shellQuote(s string) string {
	// Single quotes preserve everything literally except single quotes themselves.
	// To include a single quote, we end the quoted string, add an escaped quote, and restart.
	// e.g., "it's" becomes 'it'\''s'
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// CommandType identifies which command is triggering the hook
type CommandType string

const (
	CommandCreate CommandType = "create"
	CommandOpen   CommandType = "open"
	CommandPR     CommandType = "pr"
)

// Context holds the values for placeholder substitution
type Context struct {
	Path     string // absolute worktree path
	Branch   string // branch name
	Repo     string // repo name from git origin
	Folder   string // main repo folder name
	MainRepo string // main repo path
}

// HookMatch represents a hook that matched the current command
type HookMatch struct {
	Hook *config.Hook
	Name string
}

// SelectHooks determines which hooks to run based on config and CLI flags.
// Returns all matching hooks. If hookName is specified, only that hook runs.
// Otherwise, all hooks with matching "on" conditions run.
// Returns nil slice if no hooks should run, error if specified hook doesn't exist.
func SelectHooks(cfg config.HooksConfig, hookName string, noHook bool, alreadyExists bool, cmdType CommandType) ([]HookMatch, error) {
	if noHook {
		return nil, nil
	}

	// If explicit hook specified, use it directly (ignores "on" condition)
	if hookName != "" {
		hook, exists := cfg.Hooks[hookName]
		if !exists {
			return nil, fmt.Errorf("unknown hook %q", hookName)
		}
		// Check run_on_exists
		if alreadyExists && !hook.RunOnExists {
			return nil, nil
		}
		return []HookMatch{{Hook: &hook, Name: hookName}}, nil
	}

	// Find all hooks with matching "on" conditions
	return findMatchingHooks(cfg, cmdType, alreadyExists), nil
}

// findMatchingHooks returns all hooks that have the command type in their "on" list.
// Hooks without "on" are skipped (they only run via explicit --hook=name).
func findMatchingHooks(cfg config.HooksConfig, cmdType CommandType, alreadyExists bool) []HookMatch {
	var matches []HookMatch

	for name, hook := range cfg.Hooks {
		// Only include hooks with explicit "on" conditions that match
		if len(hook.On) > 0 && hookMatchesCommand(hook, cmdType) && (!alreadyExists || hook.RunOnExists) {
			hookCopy := hook
			matches = append(matches, HookMatch{Hook: &hookCopy, Name: name})
		}
	}

	return matches
}

// hookMatchesCommand returns true if cmdType is in the hook's "on" list.
// Special value "all" matches all command types.
func hookMatchesCommand(hook config.Hook, cmdType CommandType) bool {
	for _, cmd := range hook.On {
		if cmd == "all" || cmd == string(cmdType) {
			return true
		}
	}
	return false
}

// Run executes the hook command with variable substitution
func Run(hook *config.Hook, ctx Context) error {
	cmd := SubstitutePlaceholders(hook.Command, ctx)

	// Execute via shell for complex commands (pipes, &&, etc.)
	shellCmd := exec.Command("sh", "-c", cmd)
	shellCmd.Dir = ctx.Path
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr
	shellCmd.Stdin = os.Stdin

	return shellCmd.Run()
}

// SubstitutePlaceholders replaces {placeholder} with shell-quoted values from Context.
// Values are properly escaped to prevent command injection.
func SubstitutePlaceholders(command string, ctx Context) string {
	replacements := map[string]string{
		"{path}":      shellQuote(ctx.Path),
		"{branch}":    shellQuote(ctx.Branch),
		"{repo}":      shellQuote(ctx.Repo),
		"{folder}":    shellQuote(ctx.Folder),
		"{main-repo}": shellQuote(ctx.MainRepo),
	}

	result := command
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

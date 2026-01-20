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
	CommandAdd   CommandType = "add"
	CommandPR    CommandType = "pr"
	CommandPrune CommandType = "prune"
	CommandMerge CommandType = "merge"
)

// Context holds the values for placeholder substitution
type Context struct {
	Path     string // absolute worktree path
	Branch   string // branch name
	Repo     string // repo name from git origin
	Folder   string // main repo folder name
	MainRepo string // main repo path
	Trigger  string // command that triggered the hook (add, pr, prune, merge)
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
func SelectHooks(cfg config.HooksConfig, hookName string, noHook bool, cmdType CommandType) ([]HookMatch, error) {
	if noHook {
		return nil, nil
	}

	// If explicit hook specified, use it directly (ignores "on" condition)
	if hookName != "" {
		hook, exists := cfg.Hooks[hookName]
		if !exists {
			return nil, fmt.Errorf("unknown hook %q", hookName)
		}
		return []HookMatch{{Hook: &hook, Name: hookName}}, nil
	}

	// Find all hooks with matching "on" conditions
	return findMatchingHooks(cfg, cmdType), nil
}

// findMatchingHooks returns all hooks that have the command type in their "on" list.
// Hooks without "on" are skipped (they only run via explicit --hook=name).
func findMatchingHooks(cfg config.HooksConfig, cmdType CommandType) []HookMatch {
	var matches []HookMatch

	for name, hook := range cfg.Hooks {
		// Only include hooks with explicit "on" conditions that match
		if len(hook.On) > 0 && hookMatchesCommand(hook, cmdType) {
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

// RunAll runs all matched hooks with the given context.
// Prints "No hooks matched" if matches is empty, otherwise runs each hook.
// Returns on first error.
func RunAll(matches []HookMatch, ctx Context) error {
	return runAll(matches, ctx, ctx.Path)
}

// runAll runs all matched hooks with a custom working directory.
func runAll(matches []HookMatch, ctx Context, workDir string) error {
	if len(matches) == 0 {
		fmt.Println("No hooks matched")
		return nil
	}

	for _, match := range matches {
		if err := runHook(match.Name, match.Hook, ctx, workDir); err != nil {
			return fmt.Errorf("hook %q failed: %w", match.Name, err)
		}
	}
	return nil
}

// RunAllNonFatal runs all matched hooks, logging failures as warnings instead of returning errors.
// Prints "No hooks matched" if matches is empty.
func RunAllNonFatal(matches []HookMatch, ctx Context, workDir string) {
	if len(matches) == 0 {
		fmt.Println("No hooks matched")
		return
	}

	for _, match := range matches {
		if err := runHook(match.Name, match.Hook, ctx, workDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: hook %q failed: %v\n", match.Name, err)
		}
	}
}

// RunForEach runs all matched hooks for a single item (e.g., one worktree in a batch).
// Logs failures as warnings with branch context. Does NOT print "no hooks matched".
func RunForEach(matches []HookMatch, ctx Context, workDir string) {
	for _, match := range matches {
		if err := runHook(match.Name, match.Hook, ctx, workDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: hook %q failed for %s: %v\n", match.Name, ctx.Branch, err)
		}
	}
}

// RunSingle runs a single hook by name with the given context.
// Used by `wt hook run` to execute a specific hook manually.
func RunSingle(name string, hook *config.Hook, ctx Context) error {
	return runHook(name, hook, ctx, ctx.Path)
}

// runHook executes a single hook with variable substitution.
func runHook(name string, hook *config.Hook, ctx Context, workDir string) error {
	fmt.Printf("Running hook '%s'...\n", name)

	cmd := SubstitutePlaceholders(hook.Command, ctx)

	shellCmd := exec.Command("sh", "-c", cmd)
	shellCmd.Dir = workDir
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr
	shellCmd.Stdin = os.Stdin

	if err := shellCmd.Run(); err != nil {
		return err
	}

	if hook.Description != "" {
		fmt.Printf("  ✓ %s\n", hook.Description)
	}
	return nil
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
		"{trigger}":   shellQuote(ctx.Trigger),
	}

	result := command
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

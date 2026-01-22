package hooks

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/mattn/go-isatty"
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
	CommandCd    CommandType = "cd"
)

// Context holds the values for placeholder substitution
type Context struct {
	Path     string            // absolute worktree path
	Branch   string            // branch name
	Repo     string            // repo name from git origin
	Folder   string            // main repo folder name
	MainRepo string            // main repo path
	Trigger  string            // command that triggered the hook (add, pr, prune, merge)
	Env      map[string]string // custom variables from -e key=value flags
	DryRun   bool              // if true, print command instead of executing
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
// Used by `wt hook` to execute a specific hook manually.
func RunSingle(name string, hook *config.Hook, ctx Context) error {
	return runHook(name, hook, ctx, ctx.Path)
}

// runHook executes a single hook with variable substitution.
func runHook(name string, hook *config.Hook, ctx Context, workDir string) error {
	cmd := SubstitutePlaceholders(hook.Command, ctx)

	if ctx.DryRun {
		fmt.Printf("[dry-run] %s: %s\n", name, cmd)
		return nil
	}

	fmt.Printf("Running hook '%s'...\n", name)

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

// ParseEnv parses a slice of "key=value" strings into a map.
// Returns an error if any entry doesn't contain "=".
func ParseEnv(envSlice []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, e := range envSlice {
		idx := strings.Index(e, "=")
		if idx == -1 {
			return nil, fmt.Errorf("invalid env format %q: expected KEY=VALUE", e)
		}
		key := e[:idx]
		value := e[idx+1:]
		if key == "" {
			return nil, fmt.Errorf("invalid env format %q: key cannot be empty", e)
		}
		result[key] = value
	}
	return result, nil
}

// readStdinIfPiped reads all content from stdin if it's piped (not a TTY).
// Returns empty string and nil if stdin is a TTY (interactive).
func readStdinIfPiped() (string, error) {
	if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		return "", nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("failed to read stdin: %w", err)
	}
	return string(data), nil
}

// ParseEnvWithStdin parses a slice of "key=value" strings into a map.
// If any value is "-", reads stdin content and assigns it to all such keys.
// Returns an error if stdin is requested but not piped or empty.
func ParseEnvWithStdin(envSlice []string) (map[string]string, error) {
	result := make(map[string]string)
	var stdinKeys []string

	// First pass: parse all entries and identify stdin keys
	for _, e := range envSlice {
		idx := strings.Index(e, "=")
		if idx == -1 {
			return nil, fmt.Errorf("invalid env format %q: expected KEY=VALUE", e)
		}
		key := e[:idx]
		value := e[idx+1:]
		if key == "" {
			return nil, fmt.Errorf("invalid env format %q: key cannot be empty", e)
		}
		if value == "-" {
			stdinKeys = append(stdinKeys, key)
		} else {
			result[key] = value
		}
	}

	// If any keys need stdin, read it once
	if len(stdinKeys) > 0 {
		content, err := readStdinIfPiped()
		if err != nil {
			return nil, err
		}
		if content == "" {
			return nil, fmt.Errorf("stdin not piped: KEY=- requires piped input")
		}
		for _, key := range stdinKeys {
			result[key] = content
		}
	}

	return result, nil
}

// envPlaceholderRegex matches {key}, {key:raw}, or {key:-default} patterns for env variables.
// This is used after static replacements to expand custom env placeholders.
// Supported formats:
//   - {key}           - value is shell-quoted
//   - {key:raw}       - value is used as-is (no quoting)
//   - {key:-default}  - value is shell-quoted, uses default if key not set
var envPlaceholderRegex = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)(?:(:raw)|:-([^}]*))?\}`)

// SubstitutePlaceholders replaces {placeholder} with shell-quoted values from Context.
// Values are properly escaped to prevent command injection.
//
// Static placeholders: {path}, {branch}, {repo}, {folder}, {main-repo}, {trigger}
// Env placeholders (from Context.Env):
//   - {key}         - shell-quoted value
//   - {key:raw}     - unquoted value (for embedding in existing quotes)
//   - {key:-default} - shell-quoted value with default if key missing
func SubstitutePlaceholders(command string, ctx Context) string {
	// First, handle static replacements
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

	// Then, handle env placeholders with optional defaults: {key}, {key:raw}, or {key:-default}
	result = envPlaceholderRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Parse the match to extract key, raw flag, and optional default
		submatch := envPlaceholderRegex.FindStringSubmatch(match)
		if submatch == nil {
			return match
		}
		key := submatch[1]
		isRaw := submatch[2] == ":raw"
		defaultVal := submatch[3] // empty string if no default specified

		// Look up value in env map
		if ctx.Env != nil {
			if val, ok := ctx.Env[key]; ok {
				if isRaw {
					return val
				}
				return shellQuote(val)
			}
		}

		// Use default if specified, otherwise empty string
		if isRaw {
			return defaultVal
		}
		return shellQuote(defaultVal)
	})

	return result
}

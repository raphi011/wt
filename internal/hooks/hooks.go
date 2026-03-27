package hooks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/ui/styles"
)

// CommandType identifies which command is triggering the hook
type CommandType string

const (
	CommandCheckout CommandType = "checkout"
	CommandPrune    CommandType = "prune"
	CommandMerge    CommandType = "merge"
	CommandRun      CommandType = "run"
)

// Phase constants for hook execution timing.
const (
	PhaseBefore = "before"
	PhaseAfter  = "after"
)

// Action constants for checkout subtypes and manual invocation.
const (
	ActionCreate = "create"
	ActionOpen   = "open"
	ActionPR     = "pr"
	ActionManual = "manual"
)

// Context holds the values for placeholder substitution
type Context struct {
	WorktreeDir string            // absolute worktree path
	RepoDir     string            // absolute main repo path
	Branch      string            // branch name
	Repo        string            // registered repo name (as shown in wt repo list)
	Trigger     string            // command that triggered the hook (checkout, prune, merge, run)
	Action      string            // checkout subtype: create, open, pr, manual (for wt hook)
	Phase       string            // "before" or "after"
	Env         map[string]string // custom variables from --arg key=value flags
	DryRun      bool              // if true, print command instead of executing
}

// HookMatch represents a hook that matched the current command
type HookMatch struct {
	Hook *config.Hook
	Name string
}

// SelectHooks determines which hooks to run based on config and CLI flags.
// Returns all matching hooks. If hookNames are specified, those hooks run.
// Otherwise, all hooks with matching "on" conditions run.
// Returns nil slice if no hooks should run, error if any specified hook doesn't exist.
func SelectHooks(cfg config.HooksConfig, hookNames []string, noHook bool, cmdType CommandType, subtype, phase string) ([]HookMatch, error) {
	if noHook {
		return nil, nil
	}

	// If explicit hooks specified, use them directly (ignores "on" condition).
	// Only return explicit hooks in the "after" phase to avoid running them twice
	// (once in before-hooks and once in after-hooks).
	if len(hookNames) > 0 {
		if phase == PhaseBefore {
			return nil, nil
		}
		var matches []HookMatch
		for _, hookName := range hookNames {
			hook, exists := cfg.Hooks[hookName]
			if !exists {
				return nil, fmt.Errorf("unknown hook %q", hookName)
			}
			matches = append(matches, HookMatch{Hook: &hook, Name: hookName})
		}
		return matches, nil
	}

	// Find all hooks with matching "on" conditions
	return findMatchingHooks(cfg, cmdType, subtype, phase), nil
}

// findMatchingHooks returns all hooks that have the command type in their "on" list.
// Hooks without "on" are skipped (they only run via explicit --hook=name).
// Results are sorted alphabetically by hook name for deterministic execution order.
func findMatchingHooks(cfg config.HooksConfig, cmdType CommandType, subtype, phase string) []HookMatch {
	var matches []HookMatch

	for name, hook := range cfg.Hooks {
		// Only include hooks with explicit "on" conditions that match
		if len(hook.On) > 0 && hookMatchesCommand(hook, cmdType, subtype, phase) {
			hookCopy := hook
			matches = append(matches, HookMatch{Hook: &hookCopy, Name: name})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name < matches[j].Name
	})

	return matches
}

// hookMatchesCommand returns true if cmdType/subtype/phase match any of the hook's "on" triggers.
func hookMatchesCommand(hook config.Hook, cmdType CommandType, subtype, phase string) bool {
	for _, on := range hook.On {
		parsed, err := ParseTrigger(on)
		if err != nil {
			continue // already validated at config load time
		}
		if parsed.Phase == phase && parsed.Matches(string(cmdType), subtype) {
			return true
		}
	}
	return false
}

// RunAllNonFatal runs all matched hooks, logging failures as warnings instead of returning errors.
// Hooks run in alphabetical order by name (determined by SelectHooks).
// Prints "No hooks matched" if matches is empty.
func RunAllNonFatal(goCtx context.Context, matches []HookMatch, ctx Context, workDir string) {
	l := log.FromContext(goCtx)

	if len(matches) == 0 {
		l.Printf("No hooks matched\n")
		return
	}

	for _, match := range matches {
		if err := runHook(goCtx, match.Name, match.Hook, ctx, workDir); err != nil {
			l.Printf("Warning: hook %q failed: %v\n", match.Name, err)
		}
	}
}

// RunForEach runs all matched hooks for a single item (e.g., one worktree in a batch).
// Hooks run in alphabetical order by name (determined by SelectHooks).
// Logs failures as warnings with branch context. Does NOT print "no hooks matched".
func RunForEach(goCtx context.Context, matches []HookMatch, ctx Context, workDir string) {
	l := log.FromContext(goCtx)

	for _, match := range matches {
		if err := runHook(goCtx, match.Name, match.Hook, ctx, workDir); err != nil {
			l.Printf("Warning: hook %q failed for %s: %v\n", match.Name, ctx.Branch, err)
		}
	}
}

// RunBeforeHooks runs all matched hooks for a before phase, aborting on first failure.
func RunBeforeHooks(goCtx context.Context, matches []HookMatch, ctx Context, workDir string) error {
	l := log.FromContext(goCtx)
	for _, match := range matches {
		if err := runHook(goCtx, match.Name, match.Hook, ctx, workDir); err != nil {
			l.Printf("Hook %q failed — aborting operation for %s\n", match.Name, ctx.Branch)
			return err
		}
	}
	return nil
}

// RunSingle runs a single hook by name with the given context.
// Used by `wt hook` to execute a specific hook manually.
func RunSingle(goCtx context.Context, name string, hook *config.Hook, ctx Context) error {
	return runHook(goCtx, name, hook, ctx, ctx.WorktreeDir)
}

// runHook executes a single hook with variable substitution.
func runHook(goCtx context.Context, name string, hook *config.Hook, ctx Context, workDir string) error {
	l := log.FromContext(goCtx)
	cmd := SubstitutePlaceholders(hook.Command, ctx)

	if ctx.DryRun {
		l.Printf("[dry-run] %s: %s\n", name, cmd)
		return nil
	}

	desc := hook.Description
	if desc == "" {
		desc = name
	}
	l.Printf("%s\n", styles.PrimaryStyle.Render(fmt.Sprintf("Running %s...", desc)))

	shell, args := shellCommand(cmd)
	shellCmd := exec.Command(shell, args...)
	shellCmd.Dir = workDir
	shellCmd.Stdin = os.Stdin
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr

	if err := shellCmd.Run(); err != nil {
		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return fmt.Errorf("command failed (exit %d): %s", exitCode, cmd)
	}

	l.Debug("hook completed", "name", name)
	return nil
}

// ReadStdinIfPiped reads all content from stdin if it's piped (not a TTY).
// Returns empty string and nil if stdin is a TTY (interactive).
func ReadStdinIfPiped() (string, error) {
	if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		return "", nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("failed to read stdin: %w", err)
	}
	return string(data), nil
}

// NeedsStdin returns true if any env entry has "-" as value
func NeedsStdin(envSlice []string) bool {
	for _, e := range envSlice {
		_, value, ok := strings.Cut(e, "=")
		if ok && value == "-" {
			return true
		}
	}
	return false
}

// ParseEnvWithStdin parses a slice of "key=value" strings into a map.
// If any value is "-", reads stdin content and assigns it to all such keys.
// Returns an error if stdin is requested but not piped or empty.
func ParseEnvWithStdin(envSlice []string) (map[string]string, error) {
	// Read stdin if needed
	var stdinContent string
	if NeedsStdin(envSlice) {
		var err error
		stdinContent, err = ReadStdinIfPiped()
		if err != nil {
			return nil, err
		}
		if stdinContent == "" {
			return nil, fmt.Errorf("stdin not piped: KEY=- requires piped input")
		}
	}
	return ParseEnvWithCachedStdin(envSlice, stdinContent)
}

// ParseEnvWithCachedStdin parses a slice of "key=value" or bare "key" strings into a map,
// using pre-read stdin content for any "-" values.
// Bare keys without "=" are treated as boolean flags with value "true".
// Returns an error if stdin is needed but stdinContent is empty.
func ParseEnvWithCachedStdin(envSlice []string, stdinContent string) (map[string]string, error) {
	result := make(map[string]string)

	for _, e := range envSlice {
		key, value, ok := strings.Cut(e, "=")
		if !ok {
			// Bare key without "=" - treat as boolean flag
			if e == "" {
				return nil, fmt.Errorf("invalid env format %q: key cannot be empty", e)
			}
			result[e] = "true"
			continue
		}
		if key == "" {
			return nil, fmt.Errorf("invalid env format %q: key cannot be empty", e)
		}
		if value == "-" {
			if stdinContent == "" {
				return nil, fmt.Errorf("stdin not piped: KEY=- requires piped input")
			}
			result[key] = stdinContent
		} else {
			result[key] = value
		}
	}

	return result, nil
}

// envPlaceholderRegex matches {key}, {key:-default}, and {key:+text} patterns for env variables.
// This is used after static replacements to expand custom env placeholders.
// Supported formats:
//   - {key}           - value from --arg key=value
//   - {key:-default}  - value with default if key not set
//   - {key:+text}     - conditional: expands to text if key is set and non-empty
var envPlaceholderRegex = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)(?::([-+])([^}]*))?\}`)

// SubstitutePlaceholders replaces {placeholder} with values from Context.
//
// Static placeholders: {worktree-dir}, {repo-dir}, {branch}, {repo}, {trigger}
// Env placeholders (from Context.Env via --arg key=value or --arg key):
//   - {key}           - value from --arg key=value
//   - {key:-default}  - value with default if key not set
//   - {key:+text}     - expands to text if key is set and non-empty, otherwise empty
func SubstitutePlaceholders(command string, ctx Context) string {
	// First, handle static replacements
	replacements := map[string]string{
		"{worktree-dir}": ctx.WorktreeDir,
		"{repo-dir}":     ctx.RepoDir,
		"{branch}":       ctx.Branch,
		"{repo}":         ctx.Repo,
		"{trigger}":      ctx.Trigger,
		"{action}":       ctx.Action,
		"{phase}":        ctx.Phase,
	}

	result := command
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// Then, handle env placeholders: {key}, {key:-default}, {key:+text}
	result = envPlaceholderRegex.ReplaceAllStringFunc(result, func(match string) string {
		submatch := envPlaceholderRegex.FindStringSubmatch(match)
		if submatch == nil {
			return match
		}
		key := submatch[1]
		operator := submatch[2] // "-", "+", or "" (no operator)
		operand := submatch[3]  // text after the operator

		// Look up value in env map
		val, isSet := "", false
		if ctx.Env != nil {
			val, isSet = ctx.Env[key]
		}

		switch operator {
		case "+":
			// {key:+text} - if key is set and non-empty, expand to text; otherwise empty
			if isSet && val != "" {
				return operand
			}
			return ""
		case "-":
			// {key:-default} - if key is set, use value; otherwise use default
			if isSet {
				return val
			}
			return operand
		default:
			// {key} - if key is set, use value; otherwise empty string
			if isSet {
				return val
			}
			return ""
		}
	})

	return result
}

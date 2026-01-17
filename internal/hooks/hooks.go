package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/raphaelgruber/wt/internal/config"
)

// shellQuote escapes a string for safe use in shell commands.
// It wraps the value in single quotes and escapes any embedded single quotes.
func shellQuote(s string) string {
	// Single quotes preserve everything literally except single quotes themselves.
	// To include a single quote, we end the quoted string, add an escaped quote, and restart.
	// e.g., "it's" becomes 'it'\''s'
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// Context holds the values for placeholder substitution
type Context struct {
	Path     string // absolute worktree path
	Branch   string // branch name
	Repo     string // repo name from git origin
	Folder   string // main repo folder name
	MainRepo string // main repo path
}

// SelectHook determines which hook to run based on config and CLI flags
// Returns nil if no hook should run, error if specified hook doesn't exist
func SelectHook(cfg config.HooksConfig, hookName string, noHook bool, alreadyExists bool) (*config.Hook, string, error) {
	if noHook {
		return nil, "", nil
	}

	name := cfg.Default
	if hookName != "" {
		name = hookName
	}

	if name == "" {
		return nil, "", nil
	}

	hook, exists := cfg.Hooks[name]
	if !exists {
		return nil, "", fmt.Errorf("unknown hook %q", name)
	}

	// Check run_on_exists
	if alreadyExists && !hook.RunOnExists {
		return nil, "", nil
	}

	return &hook, name, nil
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

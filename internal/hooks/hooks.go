package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/raphaelgruber/wt/internal/config"
)

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
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr
	shellCmd.Stdin = os.Stdin

	return shellCmd.Run()
}

// SubstitutePlaceholders replaces {placeholder} with values from Context
func SubstitutePlaceholders(command string, ctx Context) string {
	replacements := map[string]string{
		"{path}":      ctx.Path,
		"{branch}":    ctx.Branch,
		"{repo}":      ctx.Repo,
		"{folder}":    ctx.Folder,
		"{main-repo}": ctx.MainRepo,
	}

	result := command
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

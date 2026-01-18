package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Hook defines a post-create hook
type Hook struct {
	Command     string   `toml:"command"`
	Description string   `toml:"description"`
	RunOnExists bool     `toml:"run_on_exists"` // default false
	On          []string `toml:"on"`            // commands this hook runs on (empty = only via --hook)
}

// HooksConfig holds hook-related configuration
type HooksConfig struct {
	Hooks map[string]Hook `toml:"-"` // parsed from [hooks.NAME] sections
}

// Config holds the wt configuration
type Config struct {
	DefaultPath    string      `toml:"default_path"`
	WorktreeFormat string      `toml:"worktree_format"`
	Hooks          HooksConfig `toml:"-"` // custom parsing needed
}

// DefaultWorktreeFormat is the default format for worktree folder names
const DefaultWorktreeFormat = "{git-origin}-{branch-name}"

// Default returns the default configuration
func Default() Config {
	return Config{
		DefaultPath:    "",
		WorktreeFormat: DefaultWorktreeFormat,
	}
}

// ValidateDefaultPath checks that the path is absolute or starts with ~
// Returns error if path is relative (like "." or "..")
func ValidateDefaultPath(path string) error {
	if path == "" {
		return nil // Empty is allowed (means not configured)
	}
	// Allow ~ paths
	if len(path) >= 1 && path[0] == '~' {
		return nil
	}
	// Must be absolute
	if !filepath.IsAbs(path) {
		return fmt.Errorf("default_path must be absolute or start with ~, got: %q", path)
	}
	return nil
}

// configPath returns the path to the config file
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "wt", "config.toml"), nil
}

// rawConfig is used for initial TOML parsing before processing hooks
type rawConfig struct {
	DefaultPath    string                 `toml:"default_path"`
	WorktreeFormat string                 `toml:"worktree_format"`
	Hooks          map[string]interface{} `toml:"hooks"`
}

// Load reads config from ~/.config/wt/config.toml
// Returns Default() if file doesn't exist (no error)
// Returns error only if file exists but is invalid
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Default(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Default(), fmt.Errorf("failed to read config file: %w", err)
	}

	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Default(), fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg := Config{
		DefaultPath:    raw.DefaultPath,
		WorktreeFormat: raw.WorktreeFormat,
		Hooks:          parseHooksConfig(raw.Hooks),
	}

	// Validate default_path (must be absolute or start with ~)
	if err := ValidateDefaultPath(cfg.DefaultPath); err != nil {
		return Default(), err
	}

	// Use defaults for empty values
	if cfg.WorktreeFormat == "" {
		cfg.WorktreeFormat = DefaultWorktreeFormat
	}

	return cfg, nil
}

// parseHooksConfig extracts HooksConfig from raw TOML map
// Handles [hooks.NAME] sections
func parseHooksConfig(raw map[string]interface{}) HooksConfig {
	hc := HooksConfig{
		Hooks: make(map[string]Hook),
	}

	if raw == nil {
		return hc
	}

	for key, value := range raw {
		// Hook definitions are tables
		if hookMap, ok := value.(map[string]interface{}); ok {
			hook := Hook{}
			if cmd, ok := hookMap["command"].(string); ok {
				hook.Command = cmd
			}
			if desc, ok := hookMap["description"].(string); ok {
				hook.Description = desc
			}
			if roe, ok := hookMap["run_on_exists"].(bool); ok {
				hook.RunOnExists = roe
			}
			if on, ok := hookMap["on"].([]interface{}); ok {
				for _, v := range on {
					if s, ok := v.(string); ok {
						hook.On = append(hook.On, s)
					}
				}
			}
			hc.Hooks[key] = hook
		}
	}

	return hc
}

const defaultConfig = `# wt configuration

# Base directory for new worktrees (required for default behavior)
# Must be an absolute path or start with ~ (no relative paths like "." or "..")
# Examples: "/Users/you/Git/worktrees" or "~/Git/worktrees"
# default_path = "~/Git/worktrees"

# Worktree folder naming format
# Available placeholders:
#   {git-origin}   - repo name extracted from git remote origin URL
#   {branch-name}  - the branch name as provided
#   {folder-name}  - actual folder name of the git repo on disk
# Example: "{folder-name}_{branch-name}" creates "my-repo_feature-branch"
worktree_format = "{git-origin}-{branch-name}"

# Hooks - run commands after worktree creation
# Use --hook=name to run a specific hook, --no-hook to skip all hooks
#
# Hooks with "on" run automatically for matching commands.
# Hooks without "on" only run when explicitly called with --hook=name.
#
# [hooks.kitty]
# command = "kitty @ launch --type=tab --cwd={path}"
# description = "Open new kitty tab"
# on = ["create", "open"]  # auto-run for create and open commands
#
# [hooks.pr-setup]
# command = "cd {path} && npm install && code {path}"
# description = "Install deps and open editor"
# on = ["pr"]  # auto-run when opening PRs
#
# [hooks.vscode]
# command = "code {path}"
# description = "Open VS Code"
# # no "on" - only runs via --hook=vscode
#
# Available "on" values: "create", "open", "pr"
#
# Other options:
#   run_on_exists = false  # skip if worktree already existed (default: false)
#
# Hooks run with working directory set to the worktree path.
# Available placeholders:
#   {path}      - absolute worktree path
#   {branch}    - branch name
#   {repo}      - repo name from git origin
#   {folder}    - main repo folder name
#   {main-repo} - main repo path
`

// Init creates a default config file at ~/.config/wt/config.toml
// If force is true, overwrites existing file
// Returns the path to the created file
func Init(force bool) (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}

	// Check if file already exists (skip if force)
	if !force {
		if _, err := os.Stat(path); err == nil {
			return "", errors.New("config file already exists: " + path)
		}
	}

	// Create directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	// Write default config
	if err := os.WriteFile(path, []byte(defaultConfig), 0644); err != nil {
		return "", err
	}

	return path, nil
}

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
	Command     string `toml:"command"`
	Description string `toml:"description"`
	RunOnExists bool   `toml:"run_on_exists"` // default false
}

// HooksConfig holds hook-related configuration
type HooksConfig struct {
	Default string          `toml:"default"` // name of default hook (empty = none)
	Hooks   map[string]Hook `toml:"-"`       // parsed from [hooks.NAME] sections
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
		DefaultPath:    ".",
		WorktreeFormat: DefaultWorktreeFormat,
	}
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

	// Use defaults for empty values
	if cfg.DefaultPath == "" {
		cfg.DefaultPath = "."
	}
	if cfg.WorktreeFormat == "" {
		cfg.WorktreeFormat = DefaultWorktreeFormat
	}

	return cfg, nil
}

// parseHooksConfig extracts HooksConfig from raw TOML map
// Handles [hooks] with "default" key and [hooks.NAME] sections
func parseHooksConfig(raw map[string]interface{}) HooksConfig {
	hc := HooksConfig{
		Hooks: make(map[string]Hook),
	}

	if raw == nil {
		return hc
	}

	for key, value := range raw {
		if key == "default" {
			if s, ok := value.(string); ok {
				hc.Default = s
			}
			continue
		}

		// Other keys are hook definitions (tables)
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
			hc.Hooks[key] = hook
		}
	}

	return hc
}

const defaultConfig = `# wt configuration

# Base directory for new worktrees
# Options: ".", "..", or absolute path like "~/Git/worktrees"
default_path = "."

# Worktree folder naming format
# Available placeholders:
#   {git-origin}   - repo name extracted from git remote origin URL
#   {branch-name}  - the branch name as provided
#   {folder-name}  - actual folder name of the git repo on disk
# Example: "{folder-name}_{branch-name}" creates "my-repo_feature-branch"
worktree_format = "{git-origin}-{branch-name}"

# Hooks - run commands after worktree creation
# Set default = "hookname" to run a hook by default
# Use --hook=name to run a specific hook, --no-hook to skip
#
# [hooks]
# default = "kitty"  # which hook runs by default (empty = none)
#
# [hooks.kitty]
# command = "kitty @ launch --type=tab --cwd={path}"
# description = "Open new kitty tab"
#
# [hooks.vscode]
# command = "code {path}"
# description = "Open VS Code"
#
# [hooks.setup]
# command = "cd {path} && npm install && code {path}"
# description = "Install deps and open editor"
# run_on_exists = false  # don't run if worktree already existed
#
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

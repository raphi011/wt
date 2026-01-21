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
	On          []string `toml:"on"` // commands this hook runs on (empty = only via --hook)
}

// HooksConfig holds hook-related configuration
type HooksConfig struct {
	Hooks map[string]Hook `toml:"-"` // parsed from [hooks.NAME] sections
}

// CloneRule maps a pattern to a forge
type CloneRule struct {
	Pattern string `toml:"pattern"` // glob pattern like "n26/*" or "company/*"
	Forge   string `toml:"forge"`   // "github" or "gitlab"
}

// MergeConfig holds merge-related configuration
type MergeConfig struct {
	Strategy string `toml:"strategy"` // "squash", "rebase", or "merge"
}

// CloneConfig holds clone-related configuration
type CloneConfig struct {
	Forge string      `toml:"forge"` // default forge: "github" or "gitlab"
	Org   string      `toml:"org"`   // default organization when not specified
	Rules []CloneRule `toml:"rules"` // pattern-based rules
}

// Config holds the wt configuration
type Config struct {
	WorktreeDir    string            `toml:"worktree_dir"`
	RepoDir        string            `toml:"repo_dir"` // optional: where to find repos for -r/-l
	WorktreeFormat string            `toml:"worktree_format"`
	Hooks          HooksConfig       `toml:"-"` // custom parsing needed
	Clone          CloneConfig       `toml:"clone"`
	Merge          MergeConfig       `toml:"merge"`
	Hosts          map[string]string `toml:"hosts"` // domain -> forge type mapping
}

// RepoScanDir returns the directory to scan for repositories.
// Returns RepoDir if set, otherwise falls back to WorktreeDir.
func (c *Config) RepoScanDir() string {
	if c.RepoDir != "" {
		return c.RepoDir
	}
	return c.WorktreeDir
}

// DefaultWorktreeFormat is the default format for worktree folder names
const DefaultWorktreeFormat = "{git-origin}-{branch-name}"

// Default returns the default configuration
func Default() Config {
	return Config{
		WorktreeDir:    "",
		RepoDir:        "",
		WorktreeFormat: DefaultWorktreeFormat,
		Clone: CloneConfig{
			Forge: "github", // backwards compatible default
		},
	}
}

// ValidatePath checks that the path is absolute or starts with ~
// Returns error if path is relative (like "." or "..")
func ValidatePath(path, fieldName string) error {
	if path == "" {
		return nil // Empty is allowed (means not configured)
	}
	// Allow ~ paths
	if len(path) >= 1 && path[0] == '~' {
		return nil
	}
	// Must be absolute
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s must be absolute or start with ~, got: %q", fieldName, path)
	}
	return nil
}

// expandPath expands ~ to the user's home directory
func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand ~: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	if path == "~" {
		return os.UserHomeDir()
	}
	return path, nil
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
	WorktreeDir    string                 `toml:"worktree_dir"`
	RepoDir        string                 `toml:"repo_dir"`
	WorktreeFormat string                 `toml:"worktree_format"`
	Hooks          map[string]interface{} `toml:"hooks"`
	Clone          CloneConfig            `toml:"clone"`
	Merge          MergeConfig            `toml:"merge"`
	Hosts          map[string]string      `toml:"hosts"`
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
		WorktreeDir:    raw.WorktreeDir,
		RepoDir:        raw.RepoDir,
		WorktreeFormat: raw.WorktreeFormat,
		Hooks:          parseHooksConfig(raw.Hooks),
		Clone:          raw.Clone,
		Merge:          raw.Merge,
		Hosts:          raw.Hosts,
	}

	// Validate worktree_dir (must be absolute or start with ~)
	if err := ValidatePath(cfg.WorktreeDir, "worktree_dir"); err != nil {
		return Default(), err
	}

	// Validate repo_dir (must be absolute or start with ~)
	if err := ValidatePath(cfg.RepoDir, "repo_dir"); err != nil {
		return Default(), err
	}

	// Expand ~ in worktree_dir (shell doesn't expand in config files)
	if cfg.WorktreeDir != "" {
		expanded, err := expandPath(cfg.WorktreeDir)
		if err != nil {
			return Default(), fmt.Errorf("expand worktree_dir: %w", err)
		}
		cfg.WorktreeDir = expanded
	}

	// Expand ~ in repo_dir
	if cfg.RepoDir != "" {
		expanded, err := expandPath(cfg.RepoDir)
		if err != nil {
			return Default(), fmt.Errorf("expand repo_dir: %w", err)
		}
		cfg.RepoDir = expanded
	}

	// Validate clone.forge (only "github", "gitlab", or empty allowed)
	if cfg.Clone.Forge != "" && cfg.Clone.Forge != "github" && cfg.Clone.Forge != "gitlab" {
		return Default(), fmt.Errorf("invalid clone.forge %q: must be \"github\" or \"gitlab\"", cfg.Clone.Forge)
	}

	// Validate hosts (only "github" or "gitlab" allowed)
	for host, forgeType := range cfg.Hosts {
		if forgeType != "github" && forgeType != "gitlab" {
			return Default(), fmt.Errorf("invalid forge type %q for host %q: must be \"github\" or \"gitlab\"", forgeType, host)
		}
	}

	// Validate merge.strategy (only "squash", "rebase", "merge", or empty allowed)
	if cfg.Merge.Strategy != "" && cfg.Merge.Strategy != "squash" && cfg.Merge.Strategy != "rebase" && cfg.Merge.Strategy != "merge" {
		return Default(), fmt.Errorf("invalid merge.strategy %q: must be \"squash\", \"rebase\", or \"merge\"", cfg.Merge.Strategy)
	}

	// Use defaults for empty values
	if cfg.WorktreeFormat == "" {
		cfg.WorktreeFormat = DefaultWorktreeFormat
	}
	if cfg.Clone.Forge == "" {
		cfg.Clone.Forge = "github"
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

// GetForgeForRepo returns the forge name for a given repo spec (e.g., "org/repo")
// Matches against clone rules in order, returns default if no match
func (c *CloneConfig) GetForgeForRepo(repoSpec string) string {
	for _, rule := range c.Rules {
		if matchPattern(rule.Pattern, repoSpec) {
			return rule.Forge
		}
	}
	return c.Forge
}

// matchPattern checks if repoSpec matches the pattern
// Supports simple glob patterns: * matches any sequence of characters
// Examples: "n26/*" matches "n26/foo", "company/*" matches "company/bar"
func matchPattern(pattern, repoSpec string) bool {
	// Simple glob matching - split on *
	if pattern == "*" {
		return true
	}

	// Handle prefix match like "n26/*"
	if len(pattern) > 1 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(repoSpec) >= len(prefix) && repoSpec[:len(prefix)] == prefix
	}

	// Handle suffix match like "*/repo"
	if len(pattern) > 1 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(repoSpec) >= len(suffix) && repoSpec[len(repoSpec)-len(suffix):] == suffix
	}

	// Exact match
	return pattern == repoSpec
}

const defaultConfig = `# wt configuration

# Base directory for new worktrees
# Must be an absolute path or start with ~ (no relative paths like "." or "..")
# Examples: "/Users/you/Git/worktrees" or "~/Git/worktrees"
# worktree_dir = "~/Git/worktrees"

# Optional: directory where repositories are stored (for -r/-l repo lookup)
# If not set, uses worktree_dir for repo scanning
# Useful when repos live in ~/Code but worktrees go to ~/Git/worktrees
# repo_dir = "~/Code"

# Worktree folder naming format
# Available placeholders:
#   {git-origin}   - repo name extracted from git remote origin URL
#   {branch-name}  - the branch name as provided
#   {folder-name}  - actual folder name of the git repo on disk
# Example: "{folder-name}_{branch-name}" creates "my-repo_feature-branch"
worktree_format = "{git-origin}-{branch-name}"

# Hooks - run commands after worktree creation/removal
# Use --hook=name to run a specific hook, --no-hook to skip all hooks
#
# Hooks with "on" run automatically for matching commands.
# Hooks without "on" only run when explicitly called with --hook=name.
#
# [hooks.kitty]
# command = "kitty @ launch --type=tab --cwd={path}"
# description = "Open new kitty tab"
# on = ["add"]  # auto-run for add command
#
# [hooks.pr-setup]
# command = "cd {path} && npm install && code {path}"
# description = "Install deps and open editor"
# on = ["pr"]  # auto-run when opening PRs
#
# [hooks.cleanup]
# command = "echo 'Removed {branch} from {repo}'"
# description = "Log removed branches"
# on = ["prune"]  # auto-run when removing worktrees
#
# [hooks.vscode]
# command = "code {path}"
# description = "Open VS Code"
# # no "on" - only runs via --hook=vscode
#
# Available "on" values: "add", "pr", "prune", "merge", "all"
#
# Hooks run with working directory set to the worktree path.
# For "prune" hooks, working directory is the main repo (worktree is deleted).
#
# Available placeholders:
#   {path}      - absolute worktree path
#   {branch}    - branch name
#   {repo}      - repo name from git origin
#   {folder}    - main repo folder name
#   {main-repo} - main repo path
#   {trigger}   - command that triggered the hook (add, pr, prune, merge)
#   {key}       - custom variable passed via --arg key=value
#   {key:-def}  - custom variable with default value if not provided
#
# Custom variables example:
# [hooks.claude]
# command = "kitty @ launch --cwd={path} -- claude {prompt:-help me}"
# Run with: wt hook claude --arg prompt="implement feature X"

# Clone settings - configure forge and default org for cloning repos
# Used by "wt pr clone" when cloning a new repository
#
# [clone]
# forge = "github"    # default forge when no rule matches (github or gitlab)
# org = "my-org"      # default org when repo specified without org/ prefix
#
# [[clone.rules]]
# pattern = "n26/*"     # glob pattern (* matches anything)
# forge = "github"
#
# [[clone.rules]]
# pattern = "company/*"
# forge = "gitlab"
#
# Rules are matched in order; first match wins.
# Supported forges: "github" (gh CLI), "gitlab" (glab CLI)

# Merge settings for "wt pr merge"
# [merge]
# strategy = "squash"  # squash, rebase, or merge (default: squash)
#                      # Note: rebase is not supported on GitLab

# Host mappings - for self-hosted GitHub Enterprise or GitLab instances
# Maps custom domains to forge type for automatic detection
#
# [hosts]
# "github.mycompany.com" = "github"   # GitHub Enterprise
# "gitlab.internal.corp" = "gitlab"   # Self-hosted GitLab
# "code.company.com" = "gitlab"       # Another GitLab instance
#
# Note: You must also authenticate with the respective CLI:
#   gh auth login --hostname github.mycompany.com
#   glab auth login --hostname gitlab.internal.corp
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

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"
)

// Context keys for dependency injection
type cfgKey struct{}
type workDirKey struct{}

// WithConfig returns a new context with the config stored in it.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, cfgKey{}, cfg)
}

// FromContext returns the config from context.
// Returns nil if no config is stored.
func FromContext(ctx context.Context) *Config {
	if cfg, ok := ctx.Value(cfgKey{}).(*Config); ok {
		return cfg
	}
	return nil
}

// WithWorkDir returns a new context with the working directory stored in it.
func WithWorkDir(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, workDirKey{}, dir)
}

// WorkDirFromContext returns the working directory from context.
// Falls back to os.Getwd() if not stored or empty.
func WorkDirFromContext(ctx context.Context) string {
	if dir, ok := ctx.Value(workDirKey{}).(string); ok && dir != "" {
		return dir
	}
	wd, _ := os.Getwd()
	return wd
}

// LocalConfigFileName is the name of the per-repo local config file
const LocalConfigFileName = ".wt.toml"

// Hook defines a post-create hook
type Hook struct {
	Command     string   `toml:"command"`
	Description string   `toml:"description"`
	On          []string `toml:"on"`      // commands this hook runs on (empty = only via --hook)
	Enabled     *bool    `toml:"enabled"` // nil = true (default); false disables a global hook locally
}

// IsEnabled returns whether the hook is enabled (defaults to true when Enabled is nil)
func (h *Hook) IsEnabled() bool {
	if h.Enabled == nil {
		return true
	}
	return *h.Enabled
}

// HooksConfig holds hook-related configuration
type HooksConfig struct {
	Hooks map[string]Hook `toml:"-"` // parsed from [hooks.NAME] sections
}

// ForgeRule maps a pattern to forge settings
type ForgeRule struct {
	Pattern string `toml:"pattern"` // glob pattern like "n26/*" or "company/*"
	Type    string `toml:"type"`    // "github" or "gitlab"
	User    string `toml:"user"`    // optional: gh/glab username for auth
}

// ForgeConfig holds forge-related configuration
type ForgeConfig struct {
	Default    string      `toml:"default"`     // default forge type
	DefaultOrg string      `toml:"default_org"` // default org for clone
	Rules      []ForgeRule `toml:"rules"`
}

// MergeConfig holds merge-related configuration
type MergeConfig struct {
	Strategy string `toml:"strategy"` // "squash", "rebase", or "merge"
}

// PruneConfig holds prune-related configuration
type PruneConfig struct {
	DeleteLocalBranches bool `toml:"delete_local_branches"`
}

// ListConfig holds list-related configuration
type ListConfig struct {
	StaleDays int `toml:"stale_days"` // days after which worktrees are highlighted as stale (0 = disabled)
}

// PreserveConfig holds file preservation settings for worktree creation.
// Matching git-ignored files are copied from an existing worktree into new ones.
type PreserveConfig struct {
	Patterns []string `toml:"patterns"` // Glob patterns matched against file basenames
	Exclude  []string `toml:"exclude"`  // Path segments to exclude (e.g., "node_modules")
}

// CheckoutConfig holds checkout-related configuration
type CheckoutConfig struct {
	WorktreeFormat string `toml:"worktree_format"` // Template for worktree folder names
	BaseRef        string `toml:"base_ref"`        // "local" or "remote" (default: "remote")
	AutoFetch      bool   `toml:"auto_fetch"`      // Fetch from origin before checkout
	SetUpstream    *bool  `toml:"set_upstream"`    // Auto-set upstream tracking (default: true)
}

// ThemeConfig holds theme/color configuration for interactive UI
type ThemeConfig struct {
	Name     string `toml:"name"`     // preset name: "none", "default", "dracula", "nord", "gruvbox", "catppuccin"
	Mode     string `toml:"mode"`     // theme mode: "auto", "light", "dark" (default: "auto")
	Primary  string `toml:"primary"`  // main accent color (borders, titles)
	Accent   string `toml:"accent"`   // highlight color (selected items)
	Success  string `toml:"success"`  // success indicators (checkmarks)
	Error    string `toml:"error"`    // error messages
	Muted    string `toml:"muted"`    // disabled/inactive text
	Normal   string `toml:"normal"`   // standard text
	Info     string `toml:"info"`     // informational text
	Warning  string `toml:"warning"`  // warning indicators (stale items)
	Nerdfont bool   `toml:"nerdfont"` // use nerd font symbols (default: false)
}

// Config holds the wt configuration
type Config struct {
	RegistryPath  string            `toml:"-"`              // Override ~/.wt/repos.json path (for testing)
	HistoryPath   string            `toml:"-"`              // Override ~/.wt/history.json path (for testing)
	DefaultSort   string            `toml:"default_sort"`   // "date", "repo", "branch" (default: "date")
	DefaultLabels []string          `toml:"default_labels"` // labels for newly registered repos
	Hooks         HooksConfig       `toml:"-"`              // custom parsing needed
	Checkout      CheckoutConfig    `toml:"checkout"`       // checkout settings
	Forge         ForgeConfig       `toml:"forge"`
	Merge         MergeConfig       `toml:"merge"`
	Prune         PruneConfig       `toml:"prune"`
	List          ListConfig        `toml:"list"`     // list display settings
	Preserve      PreserveConfig    `toml:"preserve"` // file preservation for new worktrees
	Hosts         map[string]string `toml:"hosts"`    // domain -> forge type mapping
	Theme         ThemeConfig       `toml:"theme"`    // UI theme/colors for interactive mode
}

// DefaultWorktreeFormat is the default format for worktree folder names
const DefaultWorktreeFormat = "{repo}-{branch}"

// GetHistoryPath returns the effective history file path.
// Returns HistoryPath if set (for testing), otherwise returns default ~/.wt/history.json.
func (c *Config) GetHistoryPath() string {
	if c.HistoryPath != "" {
		return c.HistoryPath
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wt", "history.json")
}

// ShouldSetUpstream returns true if upstream tracking should be set (default: false)
func (c *CheckoutConfig) ShouldSetUpstream() bool {
	if c.SetUpstream == nil {
		return false // Default to false
	}
	return *c.SetUpstream
}

// Default returns the default configuration
func Default() Config {
	return Config{
		Checkout: CheckoutConfig{
			WorktreeFormat: DefaultWorktreeFormat,
		},
		Forge: ForgeConfig{
			Default: "github",
		},
		List: ListConfig{
			StaleDays: 14,
		},
	}
}

// configPath returns the path to the config file
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wt", "config.toml"), nil
}

// rawConfig is used for initial TOML parsing before processing hooks
type rawConfig struct {
	DefaultSort   string         `toml:"default_sort"`
	DefaultLabels []string       `toml:"default_labels"`
	Hooks         map[string]any `toml:"hooks"`
	Checkout      CheckoutConfig `toml:"checkout"`
	Forge         ForgeConfig    `toml:"forge"`
	Merge         MergeConfig    `toml:"merge"`
	Prune         PruneConfig    `toml:"prune"`
	List          struct {
		StaleDays *int `toml:"stale_days"`
	} `toml:"list"`
	Preserve PreserveConfig    `toml:"preserve"`
	Hosts    map[string]string `toml:"hosts"`
	Theme    ThemeConfig       `toml:"theme"`
}

// Load reads config from ~/.config/wt/config.toml
// Returns Default() if file doesn't exist (no error)
// Returns error only if file exists but is invalid
// Environment variables override config file values:
// - WT_THEME overrides theme.name
// - WT_THEME_MODE overrides theme.mode (auto, light, dark)
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Default(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := Default()
			// Apply env vars even when no config file exists
			if err := applyEnvOverrides(&cfg); err != nil {
				return Default(), err
			}
			return cfg, nil
		}
		return Default(), fmt.Errorf("failed to read config file: %w", err)
	}

	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Default(), fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg := Config{
		DefaultSort:   raw.DefaultSort,
		DefaultLabels: raw.DefaultLabels,
		Hooks:         parseHooksConfig(raw.Hooks),
		Checkout:      raw.Checkout,
		Forge:         raw.Forge,
		Merge:         raw.Merge,
		Prune:         raw.Prune,
		Preserve:      raw.Preserve,
		Hosts:         raw.Hosts,
		Theme:         raw.Theme,
	}

	// Validate enum fields
	if err := validateEnum(cfg.Forge.Default, "forge.default", ValidForgeTypes); err != nil {
		return Default(), err
	}
	for i, rule := range cfg.Forge.Rules {
		if err := validateEnum(rule.Type, fmt.Sprintf("forge.rules[%d].type", i), ValidForgeTypes); err != nil {
			return Default(), err
		}
	}
	for host, forgeType := range cfg.Hosts {
		if err := validateEnum(forgeType, fmt.Sprintf("hosts[%q]", host), ValidForgeTypes); err != nil {
			return Default(), err
		}
	}
	if err := validateEnum(cfg.Merge.Strategy, "merge.strategy", ValidMergeStrategies); err != nil {
		return Default(), err
	}
	if err := validateEnum(cfg.Checkout.BaseRef, "checkout.base_ref", ValidBaseRefs); err != nil {
		return Default(), err
	}
	if err := validateEnum(cfg.DefaultSort, "default_sort", ValidDefaultSortModes); err != nil {
		return Default(), err
	}
	if err := validatePreservePatterns(cfg.Preserve.Patterns, ""); err != nil {
		return Default(), err
	}

	// Note: theme.name is validated at runtime with a warning, not an error

	// Use defaults for empty values
	if cfg.Checkout.WorktreeFormat == "" {
		cfg.Checkout.WorktreeFormat = DefaultWorktreeFormat
	}
	if cfg.Forge.Default == "" {
		cfg.Forge.Default = "github"
	}
	if raw.List.StaleDays != nil {
		cfg.List.StaleDays = *raw.List.StaleDays
	} else {
		cfg.List.StaleDays = 14
	}

	// Apply env var overrides (after loading config file)
	if err := applyEnvOverrides(&cfg); err != nil {
		return Default(), err
	}

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(cfg *Config) error {
	// WT_THEME overrides theme.name
	if envTheme := os.Getenv("WT_THEME"); envTheme != "" {
		cfg.Theme.Name = envTheme
	}

	// WT_THEME_MODE overrides theme.mode
	if envMode := os.Getenv("WT_THEME_MODE"); envMode != "" {
		cfg.Theme.Mode = envMode
	}

	return nil
}

// parseHooksConfig extracts HooksConfig from raw TOML map
// Handles [hooks.NAME] sections
func parseHooksConfig(raw map[string]any) HooksConfig {
	hc := HooksConfig{
		Hooks: make(map[string]Hook),
	}

	if raw == nil {
		return hc
	}

	for key, value := range raw {
		// Hook definitions are tables
		if hookMap, ok := value.(map[string]any); ok {
			hook := Hook{}
			if cmd, ok := hookMap["command"].(string); ok {
				hook.Command = cmd
			}
			if desc, ok := hookMap["description"].(string); ok {
				hook.Description = desc
			}
			if on, ok := hookMap["on"].([]any); ok {
				for _, v := range on {
					if s, ok := v.(string); ok {
						hook.On = append(hook.On, s)
					}
				}
			}
			if enabled, ok := hookMap["enabled"].(bool); ok {
				hook.Enabled = &enabled
			}
			hc.Hooks[key] = hook
		}
	}

	return hc
}

// GetForgeTypeForRepo returns the forge type for a given repo spec (e.g., "org/repo")
// Matches against rules in order, returns default if no match
func (c *ForgeConfig) GetForgeTypeForRepo(repoSpec string) string {
	for _, rule := range c.Rules {
		if matchPattern(rule.Pattern, repoSpec) && rule.Type != "" {
			return rule.Type
		}
	}
	return c.Default
}

// GetUserForRepo returns the gh/glab username for a repo spec
// Matches against rules in order, returns empty string if no match (use active account)
func (c *ForgeConfig) GetUserForRepo(repoSpec string) string {
	for _, rule := range c.Rules {
		if matchPattern(rule.Pattern, repoSpec) {
			return rule.User
		}
	}
	return ""
}

// ValidThemeNames is the list of supported theme presets (families)
var ValidThemeNames = []string{"none", "default", "dracula", "nord", "gruvbox", "catppuccin"}

// ValidThemeModes is the list of supported theme modes
var ValidThemeModes = []string{"auto", "light", "dark"}

// isValidThemeName checks if the theme name is a known preset
func isValidThemeName(name string) bool {
	return slices.Contains(ValidThemeNames, name)
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

// defaultConfig is the full default config template
const defaultConfig = `# wt configuration

# Checkout settings - controls worktree creation behavior
[checkout]
# Worktree folder naming format
# Available placeholders:
#   {repo}    - folder name of git repo
#   {branch}  - the branch name as provided
#   {origin}  - repo name from git origin URL (falls back to {repo})
# Example: "{origin}_{branch}" creates "origin-name_feature-branch"
worktree_format = "{repo}-{branch}"

# Base ref mode for new branches (wt checkout -b)
# Controls which ref to use when creating new branches:
#   "remote" - use origin/<branch> (default, ensures up-to-date base)
#   "local"  - use local <branch> (faster, but may be stale)
# base_ref = "remote"

# Auto-fetch from origin before checkout
# For new branches (-b): fetches the base branch (or --base if specified)
# For existing branches: fetches the target branch
# Same as always passing --fetch flag
# auto_fetch = false

# Auto-set upstream tracking when checking out branches (default: false)
# When true and origin exists:
#   - For new branches (-b): pushes branch to origin, then sets upstream
#   - For existing branches: sets upstream if origin/<branch> exists
# This enables git push/pull without specifying remote.
# set_upstream = false

# Default sort order for 'wt list'
# Available values: "date", "repo", "branch"
#   "date"    - sort by commit date, newest first (default)
#   "repo"    - sort by repository name
#   "branch"  - sort by branch name
# default_sort = "date"

# List display settings
# [list]
# stale_days = 14  # Days before a worktree is highlighted as stale (0 = disabled, default: 14)

# Hooks - run commands after worktree creation/removal
# Use --hook=name to run a specific hook, --no-hook to skip all hooks
#
# Hooks with "on" run automatically for matching commands.
# Hooks without "on" only run when explicitly called with --hook=name.
#
# Available "on" values: "checkout", "pr", "prune", "merge", "all"
#
# Hooks run with working directory set to the worktree path.
# For "prune" hooks, working directory is the main repo (worktree is deleted).
#
# Available placeholders:
#   {worktree-dir} - absolute worktree path
#   {repo-dir}     - absolute main repo path
#   {branch}       - branch name
#   {repo}         - folder name of git repo
#   {origin}       - repo name from git origin (falls back to {repo})
#   {trigger}      - command that triggered the hook (checkout, pr, prune, merge)
#   {key}          - custom variable passed via --arg key=value
#   {key:-def}     - custom variable with default value if not provided
#
# === Editor Examples ===
#
# VS Code - open worktree in VS Code
# [hooks.code]
# command = "code {worktree-dir}"
# description = "Open in VS Code"
# on = ["checkout"]
#
# IntelliJ IDEA - open worktree in IDEA
# [hooks.idea]
# command = "idea {worktree-dir}"
# description = "Open in IntelliJ IDEA"
# on = ["checkout"]
#
# === AI Assistant Examples ===
#
# Claude Code - start Claude in the worktree (interactive)
# [hooks.claude]
# command = "cd {worktree-dir} && claude"
# description = "Start Claude Code session"
#
# Claude Code with custom prompt
# [hooks.claude-task]
# command = "cd {worktree-dir} && claude -p {prompt}"
# description = "Run Claude with a task"
# Run with: wt hook claude-task --arg prompt="implement feature X"
#
# Claude Code in new terminal tab (kitty example)
# [hooks.claude-tab]
# command = "kitty @ launch --type=tab --cwd={worktree-dir} -- claude"
# description = "Open Claude in new tab"
# on = ["checkout"]
#
# === Other Examples ===
#
# Setup hook - install dependencies after checkout
# [hooks.setup]
# command = "npm install"
# description = "Install dependencies"
# on = ["checkout", "pr"]
#
# Cleanup notification
# [hooks.cleanup]
# command = "echo 'Removed {branch} from {repo}'"
# description = "Log removed branches"
# on = ["prune"]

# Preserve settings - auto-copy git-ignored files into new worktrees
# Copies matching files from an existing worktree (preferring the default branch) into newly created ones.
# Only git-ignored files are considered. Existing files are never overwritten.
# Use --no-preserve on checkout to skip for a single invocation.
#
# [preserve]
# patterns = [".env", ".env.*", ".envrc", "docker-compose.override.yml"]
# exclude = ["node_modules", ".cache", "vendor"]

# Forge settings - configure forge type, default org, and multi-account auth
# Used for PR operations and "wt pr checkout <number> org/repo" when cloning
#
# [forge]
# default = "github"     # default forge type (github or gitlab)
# default_org = "my-org" # default org when repo specified without org/ prefix
#
# [[forge.rules]]
# pattern = "n26/*"           # glob pattern (* matches anything)
# type = "github"             # forge type for matching repos
# user = "raphaelgrubern26"   # gh/glab user for authentication (optional)
#
# [[forge.rules]]
# pattern = "raphi011/*"
# type = "github"
# user = "raphi011"           # different gh account for personal repos
#
# [[forge.rules]]
# pattern = "company/*"
# type = "gitlab"
# # user omitted - uses default active glab account
#
# Rules are matched in order; first match wins.
# The "user" field enables multi-account support for gh CLI.
# Use "gh auth status" to see available accounts.
# Supported forges: "github" (gh CLI), "gitlab" (glab CLI)

# Merge settings for "wt pr merge"
# [merge]
# strategy = "squash"  # squash, rebase, or merge (default: squash)
#                      # Note: rebase is not supported on GitLab

# Prune settings for "wt prune"
# [prune]
# delete_local_branches = false  # Delete local branches after worktree removal

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

# Theme settings - customize colors for interactive wizards
# Available presets: "none", "default", "dracula", "nord", "gruvbox", "catppuccin"
# Some themes have light/dark variants that are auto-selected based on terminal
#
# [theme]
# name = "catppuccin"  # use a preset theme family
# mode = "auto"        # "auto" (detect terminal), "light", or "dark"
#
# Or customize individual colors (hex or 256-color codes):
# [theme]
# primary = "#89b4fa"  # borders, titles (Catppuccin blue)
# accent = "#f5c2e7"   # selected items (Catppuccin pink)
# success = "#a6e3a1"  # checkmarks (Catppuccin green)
# error = "#f38ba8"    # error messages (Catppuccin red)
# muted = "#6c7086"    # disabled text (Catppuccin overlay0)
# normal = "#cdd6f4"   # standard text (Catppuccin text)
# info = "#94e2d5"     # info text (Catppuccin teal)
# warning = "#fab387"  # warning text (Catppuccin peach)
#
# You can also use a preset and override specific colors:
# [theme]
# name = "nord"
# mode = "dark"        # force dark variant
# accent = "#ff79c6"   # override just the accent color
#
# Enable nerd font symbols for enhanced icons (requires a nerd font):
# nerdfont = true
`

// DefaultConfig returns the default configuration content.
func DefaultConfig() string {
	return defaultConfig
}

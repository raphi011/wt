package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/BurntSushi/toml"

	"github.com/raphi011/wt/internal/statepath"
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

// Hook defines a hook: a shell command run before or after a wt operation
type Hook struct {
	Command     string   `toml:"command" json:"command,omitempty"`
	Description string   `toml:"description" json:"description,omitempty"`
	On          []string `toml:"on" json:"on,omitempty"`           // commands this hook runs on (empty = only via --hook)
	Enabled     *bool    `toml:"enabled" json:"enabled,omitempty"` // nil = true (default); false disables a global hook locally
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

// MarshalJSON encodes the hooks as an object keyed by hook name
func (h HooksConfig) MarshalJSON() ([]byte, error) {
	if h.Hooks == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(h.Hooks)
}

// ForgeRule maps a pattern to forge settings
type ForgeRule struct {
	Pattern string `toml:"pattern" json:"pattern,omitempty"` // glob pattern like "n26/*" or "company/*"
	Type    string `toml:"type" json:"type,omitempty"`       // "github" or "gitlab"
	User    string `toml:"user" json:"user,omitempty"`       // optional: gh/glab username for auth
}

// ForgeConfig holds forge-related configuration
type ForgeConfig struct {
	Default    string      `toml:"default" json:"default,omitempty"`         // default forge type
	DefaultOrg string      `toml:"default_org" json:"default_org,omitempty"` // default org for clone
	Rules      []ForgeRule `toml:"rules" json:"rules,omitempty"`
}

// MergeConfig holds merge-related configuration
type MergeConfig struct {
	Strategy string `toml:"strategy" json:"strategy,omitempty"` // "squash", "rebase", or "merge"
}

// PruneConfig holds prune-related configuration
type PruneConfig struct {
	DeleteLocalBranches bool `toml:"delete_local_branches" json:"delete_local_branches"`
	StaleDays           int  `toml:"stale_days" json:"stale_days"` // days after which worktrees are highlighted as stale and eligible for --stale pruning (0 = disabled)
}

// PreserveConfig holds file preservation settings for worktree creation.
// Listed paths are symlinked from the repo root into new worktrees.
type PreserveConfig struct {
	Paths []string `toml:"paths" json:"paths,omitempty"` // Relative paths from repo root to symlink (e.g., ".env", "config/.env")
}

// CloneConfig holds clone-related configuration
type CloneConfig struct {
	Mode string `toml:"mode" json:"mode,omitempty"` // "bare" or "regular" (default: "regular")
}

// IsBare returns true if the clone mode is bare.
func (c *CloneConfig) IsBare() bool {
	return c.Mode == "bare"
}

// CheckoutConfig holds checkout-related configuration
type CheckoutConfig struct {
	WorktreeFormat string `toml:"worktree_format" json:"worktree_format,omitempty"` // Template for worktree folder names
	BaseRef        string `toml:"base_ref" json:"base_ref,omitempty"`               // "local" or "remote" (default: "remote")
	AutoFetch      bool   `toml:"auto_fetch" json:"auto_fetch"`                     // Fetch from origin before checkout
	SetUpstream    *bool  `toml:"set_upstream" json:"set_upstream,omitempty"`       // Auto-set upstream tracking (default: false)
}

// ThemeConfig holds theme/color configuration for interactive UI
type ThemeConfig struct {
	Name     string `toml:"name" json:"name,omitempty"`       // preset name: "none", "default", "dracula", "nord", "gruvbox", "catppuccin"
	Mode     string `toml:"mode" json:"mode,omitempty"`       // theme mode: "auto", "light", "dark" (default: "auto")
	Primary  string `toml:"primary" json:"primary,omitempty"` // main accent color (borders, titles)
	Accent   string `toml:"accent" json:"accent,omitempty"`   // highlight color (selected items)
	Success  string `toml:"success" json:"success,omitempty"` // success indicators (checkmarks)
	Error    string `toml:"error" json:"error,omitempty"`     // error messages
	Muted    string `toml:"muted" json:"muted,omitempty"`     // disabled/inactive text
	Normal   string `toml:"normal" json:"normal,omitempty"`   // standard text
	Info     string `toml:"info" json:"info,omitempty"`       // informational text
	Warning  string `toml:"warning" json:"warning,omitempty"` // warning indicators (stale items)
	Nerdfont bool   `toml:"nerdfont" json:"nerdfont"`         // use nerd font symbols (default: false)
}

// Config holds the wt configuration
type Config struct {
	RegistryPath  string            `toml:"-" json:"-"`                                     // Override ~/.wt/repos.json path (for testing)
	HistoryPath   string            `toml:"-" json:"-"`                                     // Override ~/.wt/history.json path (for testing)
	DefaultSort   string            `toml:"default_sort" json:"default_sort,omitempty"`     // "date", "repo", "branch" (default: "date")
	DefaultLabels []string          `toml:"default_labels" json:"default_labels,omitempty"` // labels for newly registered repos
	Hooks         HooksConfig       `toml:"-" json:"hooks"`                                 // custom parsing needed
	Clone         CloneConfig       `toml:"clone" json:"clone"`                             // clone settings
	Checkout      CheckoutConfig    `toml:"checkout" json:"checkout"`                       // checkout settings
	Forge         ForgeConfig       `toml:"forge" json:"forge"`
	Merge         MergeConfig       `toml:"merge" json:"merge"`
	Prune         PruneConfig       `toml:"prune" json:"prune"`
	Preserve      PreserveConfig    `toml:"preserve" json:"preserve"`     // file preservation for new worktrees
	Hosts         map[string]string `toml:"hosts" json:"hosts,omitempty"` // domain -> forge type mapping
	Theme         ThemeConfig       `toml:"theme" json:"theme"`           // UI theme/colors for interactive mode
	Warnings      []string          `toml:"-" json:"-"`                   // unknown keys found in the global config file
}

// DefaultWorktreeFormat is the default format for worktree folder names
const DefaultWorktreeFormat = ".worktrees/{branch}"

// GetWtDir returns the effective wt config directory path.
// Returns filepath.Dir(RegistryPath) if set (for testing), otherwise returns default ~/.wt/.
func (c *Config) GetWtDir() (string, error) {
	dir, err := statepath.Dir(c.RegistryPath)
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return dir, nil
}

// GetHistoryPath returns the effective history file path.
// Returns HistoryPath if set (for testing), otherwise returns default ~/.wt/history.json.
func (c *Config) GetHistoryPath() (string, error) {
	path, err := statepath.History(c.HistoryPath)
	if err != nil {
		return "", fmt.Errorf("cannot determine history path: %w", err)
	}
	return path, nil
}

// GetPRCachePath returns the effective PR cache file path (prs.json in the wt dir).
func (c *Config) GetPRCachePath() (string, error) {
	path, err := statepath.PRCache(c.RegistryPath)
	if err != nil {
		return "", fmt.Errorf("cannot determine PR cache path: %w", err)
	}
	return path, nil
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
		Clone: CloneConfig{
			Mode: "regular",
		},
		Checkout: CheckoutConfig{
			WorktreeFormat: DefaultWorktreeFormat,
		},
		Forge: ForgeConfig{
			Default: "github",
		},
		Prune: PruneConfig{
			StaleDays: 14,
		},
	}
}

// rawConfig is used for initial TOML parsing before processing hooks
type rawConfig struct {
	DefaultSort   string         `toml:"default_sort"`
	DefaultLabels []string       `toml:"default_labels"`
	Hooks         map[string]any `toml:"hooks"`
	Clone         CloneConfig    `toml:"clone"`
	Checkout      CheckoutConfig `toml:"checkout"`
	Forge         ForgeConfig    `toml:"forge"`
	Merge         MergeConfig    `toml:"merge"`
	Prune         struct {
		DeleteLocalBranches bool `toml:"delete_local_branches"`
		StaleDays           *int `toml:"stale_days"`
	} `toml:"prune"`
	Preserve PreserveConfig    `toml:"preserve"`
	Hosts    map[string]string `toml:"hosts"`
	Theme    ThemeConfig       `toml:"theme"`
}

// Load reads config from ~/.wt/config.toml
// Returns Default() if file doesn't exist (no error)
// Returns error only if file exists but is invalid
// Environment variables override config file values:
// - WT_THEME overrides theme.name
// - WT_THEME_MODE overrides theme.mode (auto, light, dark)
func Load() (Config, error) {
	path, err := statepath.Config("")
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
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		return Default(), fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg := Config{
		Warnings:      unknownKeyWarnings(md, raw.Hooks, path),
		DefaultSort:   raw.DefaultSort,
		DefaultLabels: raw.DefaultLabels,
		Hooks:         parseHooksConfig(raw.Hooks),
		Clone:         raw.Clone,
		Checkout:      raw.Checkout,
		Forge:         raw.Forge,
		Merge:         raw.Merge,
		Prune: PruneConfig{
			DeleteLocalBranches: raw.Prune.DeleteLocalBranches,
		},
		Preserve: raw.Preserve,
		Hosts:    raw.Hosts,
		Theme:    raw.Theme,
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
	if err := validateEnum(cfg.Clone.Mode, "clone.mode", ValidCloneModes); err != nil {
		return Default(), err
	}
	if err := validateEnum(cfg.DefaultSort, "default_sort", ValidDefaultSortModes); err != nil {
		return Default(), err
	}
	if err := validatePreservePaths(cfg.Preserve.Paths, ""); err != nil {
		return Default(), err
	}
	if err := ValidateHookTriggers(cfg.Hooks.Hooks); err != nil {
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
	if cfg.Clone.Mode == "" {
		cfg.Clone.Mode = "regular"
	}
	if raw.Prune.StaleDays != nil {
		cfg.Prune.StaleDays = *raw.Prune.StaleDays
	} else {
		cfg.Prune.StaleDays = 14
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

// hookKeys are the keys allowed in a [hooks.NAME] table
var hookKeys = []string{"command", "description", "on", "enabled"}

// unknownKeyWarnings returns one warning per key in a config file that wt does
// not know. An unknown table is reported once, without its keys. Hooks decode
// into a generic map, which md cannot check, so their keys are checked here.
func unknownKeyWarnings(md toml.MetaData, hooks map[string]any, path string) []string {
	var unknown []toml.Key
	for _, key := range md.Undecoded() {
		if key[0] == "hooks" {
			continue
		}
		inUnknownTable := slices.ContainsFunc(unknown, func(table toml.Key) bool {
			return len(table) < len(key) && slices.Equal(table, key[:len(table)])
		})
		if !inUnknownTable {
			unknown = append(unknown, key)
		}
	}
	for name, value := range hooks {
		hookMap, ok := value.(map[string]any)
		if !ok {
			unknown = append(unknown, toml.Key{"hooks", name})
			continue
		}
		for key := range hookMap {
			if !slices.Contains(hookKeys, key) {
				unknown = append(unknown, toml.Key{"hooks", name, key})
			}
		}
	}

	var warnings []string
	for _, key := range unknown {
		warnings = append(warnings, fmt.Sprintf("unknown key %q in %s (ignored)", key.String(), path))
	}
	slices.Sort(warnings)
	return warnings
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

# Default sort order for 'wt list'
# Available values: "date", "repo", "branch"
#   "date"    - sort by commit date, newest first (default)
#   "repo"    - sort by repository name
#   "branch"  - sort by branch name
# default_sort = "date"

# Labels applied to newly auto-registered repos
# default_labels = ["work"]

# Clone settings - controls how repos are cloned by "wt repo clone" and "wt pr checkout"
# [clone]
# Clone mode: "bare" or "regular" (default)
#   bare    - clone into .git/ directory, worktrees as siblings
#   regular - standard git clone with working tree at root (default)
# mode = "regular"

# Checkout settings - controls worktree creation behavior
[checkout]
# Worktree folder naming format
# Available placeholders:
#   {repo}    - registered repo name (as shown in wt repo list)
#   {branch}  - the branch name as provided
worktree_format = ".worktrees/{branch}"

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

# Hooks - run commands when creating, removing, or merging worktrees
# Use --hook=name to run a specific hook, --no-hook to skip all hooks
#
# Hooks with "on" run automatically for matching commands.
# Hooks without "on" only run when explicitly called with --hook=name or wt hook.
# With --hook=name only the named hooks run, in the phase their "on" names for
# the command (after the command, if it names none).
#
# Trigger syntax: [before:|after:]trigger[:subtype]
#   Triggers: checkout, prune, merge, all
#   Subtypes (checkout only): create, open, pr
#   Timing: before (can cancel operation), after (default)
#
# Examples:
#   on = ["checkout"]              - all checkouts (after)
#   on = ["checkout:pr"]           - PR checkouts only
#   on = ["before:prune"]          - pre-prune guard (can abort deletion)
#   on = ["before:checkout:pr"]    - before PR checkout only
#   on = ["all"]                   - all triggers (after)
#
# Before-hooks: non-zero exit aborts the operation.
# After-hooks: failures are logged as warnings.
#
# Hooks run with working directory set to the worktree path.
# For "checkout" before-hooks, working directory is the main repo if the worktree is created (it does not exist yet).
# For "prune" after-hooks, working directory is the main repo (worktree is deleted).
# For "prune" before-hooks, working directory is the worktree (still exists).
# For "merge" after-hooks, working directory is the main repo.
#
# Available placeholders:
#   {worktree-dir}      - absolute worktree path
#   {repo-dir}          - absolute main repo path
#   {branch}            - branch name
#   {repo}              - registered repo name
#   {trigger}           - command trigger (checkout, prune, merge, run)
#   {action}            - checkout subtype (create, open, pr, manual)
#   {phase}             - hook timing (before, after)
#   {config-dir}        - absolute path to ~/.wt/ config directory
#   {pr-number}         - PR/MR number (empty for non-PR checkouts)
#   {pr-repo}           - forge repo path, e.g. owner/repo (empty for non-PR checkouts)
#   {key}               - custom variable passed via --arg key=value
#   {key:-def}          - custom variable with default
#   {key:+text}         - conditional: includes text only if key is set
#
# === Editor Examples ===
#
# VS Code - open worktree in VS Code
# [hooks.code]
# command = "code '{worktree-dir}'"
# description = "Open in VS Code"
# on = ["checkout"]
#
# IntelliJ IDEA - open worktree in IDEA
# [hooks.idea]
# command = "idea '{worktree-dir}'"
# description = "Open in IntelliJ IDEA"
# on = ["checkout"]
#
# === AI Assistant Examples ===
#
# Claude Code - start Claude in the worktree
# [hooks.claude]
# command = "claude"
# description = "Start Claude Code session"
#
# Claude Code with custom prompt
# [hooks.claude-task]
# command = "claude -p '{prompt}'"
# description = "Run Claude with a task"
# Run with: wt hook claude-task --arg prompt="implement feature X"
#
# Claude Code with conditional flags (use -a skip to skip permissions)
# [hooks.claude-auto]
# command = "claude {skip:+--dangerously-skip-permissions} -p '{prompt:-help}'"
# description = "Run Claude with optional permission skip"
# Run with: wt hook claude-auto -a skip -a prompt="implement feature X"
#
# Claude Code in new terminal tab (kitty example)
# [hooks.claude-tab]
# command = "kitty @ launch --type=tab --cwd='{worktree-dir}' -- claude"
# description = "Open Claude in new tab"
# on = ["checkout"]
#
# === Other Examples ===
#
# Setup hook - install dependencies after checkout
# [hooks.setup]
# command = "npm install"
# description = "Install dependencies"
# on = ["checkout"]
#
# Cleanup notification
# [hooks.cleanup]
# command = "echo 'Removed {branch} from {repo}'"
# description = "Log removed branches"
# on = ["prune"]

# Preserve settings - symlink files from repo root into new worktrees
# Listed paths (relative to repo root) are symlinked into newly created worktrees.
# Edits in any worktree are instantly visible in all others.
# Use --no-preserve on checkout to skip for a single invocation.
#
# [preserve]
# paths = [".env", ".envrc"]

# Forge settings - configure forge type, default org, and multi-account auth
# Used for PR operations and "wt repo clone org/repo"
#
# [forge]
# default = "github"     # default forge type (github or gitlab)
# default_org = "my-org" # default org when repo specified without org/ prefix
#
# [[forge.rules]]
# pattern = "work-org/*"      # glob pattern (* matches anything)
# type = "github"             # forge type for matching repos
# user = "work-account"       # gh/glab user for authentication (optional)
#
# [[forge.rules]]
# pattern = "my-user/*"
# type = "github"
# user = "personal-account"   # different gh account for personal repos
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

# Prune settings for "wt prune" and stale worktree highlighting
# [prune]
# delete_local_branches = false  # Delete local branches after worktree removal
# stale_days = 14                # Days before a worktree is highlighted as stale and eligible for --stale pruning (0 = disabled, default: 14)

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

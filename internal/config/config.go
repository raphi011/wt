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

// CheckoutConfig holds checkout-related configuration
type CheckoutConfig struct {
	WorktreeFormat string `toml:"worktree_format"` // Template for worktree folder names
	BaseRef        string `toml:"base_ref"`        // "local" or "remote" (default: "remote")
	AutoFetch      bool   `toml:"auto_fetch"`      // Fetch before creating new branches
	SetUpstream    *bool  `toml:"set_upstream"`    // Auto-set upstream tracking (default: true)
}

// ThemeConfig holds theme/color configuration for interactive UI
type ThemeConfig struct {
	Name     string `toml:"name"`     // preset name: "default", "dracula", "nord", "gruvbox"
	Primary  string `toml:"primary"`  // main accent color (borders, titles)
	Accent   string `toml:"accent"`   // highlight color (selected items)
	Success  string `toml:"success"`  // success indicators (checkmarks)
	Error    string `toml:"error"`    // error messages
	Muted    string `toml:"muted"`    // disabled/inactive text
	Normal   string `toml:"normal"`   // standard text
	Info     string `toml:"info"`     // informational text
	Nerdfont bool   `toml:"nerdfont"` // use nerd font symbols (default: false)
}

// Config holds the wt configuration
type Config struct {
	WorktreeDir   string            `toml:"worktree_dir"`
	RepoDir       string            `toml:"repo_dir"`       // optional: where to find repos for -r/-l
	DefaultSort   string            `toml:"default_sort"`   // "id", "repo", "branch", "commit" (default: "id")
	DefaultLabels []string          `toml:"default_labels"` // labels for newly registered repos
	Hooks         HooksConfig       `toml:"-"`              // custom parsing needed
	Checkout      CheckoutConfig    `toml:"checkout"`       // checkout settings
	Forge         ForgeConfig       `toml:"forge"`
	Merge         MergeConfig       `toml:"merge"`
	Prune         PruneConfig       `toml:"prune"`
	Hosts         map[string]string `toml:"hosts"` // domain -> forge type mapping
	Theme         ThemeConfig       `toml:"theme"` // UI theme/colors for interactive mode
}

// RepoScanDir returns the directory to scan for repositories.
// Returns RepoDir if set, otherwise falls back to WorktreeDir.
func (c *Config) RepoScanDir() string {
	if c.RepoDir != "" {
		return c.RepoDir
	}
	return c.WorktreeDir
}

// GetAbsWorktreeDir returns the absolute worktree directory, defaulting to cwd.
func (c *Config) GetAbsWorktreeDir() (string, error) {
	dir := c.WorktreeDir
	if dir == "" {
		dir = "."
	}
	return filepath.Abs(dir)
}

// DefaultWorktreeFormat is the default format for worktree folder names
const DefaultWorktreeFormat = "{repo}-{branch}"

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
		WorktreeDir: "",
		RepoDir:     "",
		Checkout: CheckoutConfig{
			WorktreeFormat: DefaultWorktreeFormat,
		},
		Forge: ForgeConfig{
			Default: "github",
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
	return filepath.Join(home, ".wt", "config.toml"), nil
}

// rawConfig is used for initial TOML parsing before processing hooks
type rawConfig struct {
	WorktreeDir   string                 `toml:"worktree_dir"`
	RepoDir       string                 `toml:"repo_dir"`
	DefaultSort   string                 `toml:"default_sort"`
	DefaultLabels []string               `toml:"default_labels"`
	Hooks         map[string]interface{} `toml:"hooks"`
	Checkout      CheckoutConfig         `toml:"checkout"`
	Forge         ForgeConfig            `toml:"forge"`
	Merge         MergeConfig            `toml:"merge"`
	Prune         PruneConfig            `toml:"prune"`
	Hosts         map[string]string      `toml:"hosts"`
	Theme         ThemeConfig            `toml:"theme"`
}

// Load reads config from ~/.config/wt/config.toml
// Returns Default() if file doesn't exist (no error)
// Returns error only if file exists but is invalid
// Environment variables override config file values:
// - WT_WORKTREE_DIR overrides worktree_dir
// - WT_REPO_DIR overrides repo_dir
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
		WorktreeDir:   raw.WorktreeDir,
		RepoDir:       raw.RepoDir,
		DefaultSort:   raw.DefaultSort,
		DefaultLabels: raw.DefaultLabels,
		Hooks:         parseHooksConfig(raw.Hooks),
		Checkout:      raw.Checkout,
		Forge:         raw.Forge,
		Merge:         raw.Merge,
		Prune:         raw.Prune,
		Hosts:         raw.Hosts,
		Theme:         raw.Theme,
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

	// Validate forge.default (only "github", "gitlab", or empty allowed)
	if cfg.Forge.Default != "" && cfg.Forge.Default != "github" && cfg.Forge.Default != "gitlab" {
		return Default(), fmt.Errorf("invalid forge.default %q: must be \"github\" or \"gitlab\"", cfg.Forge.Default)
	}

	// Validate forge.rules
	for i, rule := range cfg.Forge.Rules {
		if rule.Type != "" && rule.Type != "github" && rule.Type != "gitlab" {
			return Default(), fmt.Errorf("invalid forge.rules[%d].type %q: must be \"github\" or \"gitlab\"", i, rule.Type)
		}
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

	// Validate base_ref (only "local", "remote", or empty allowed)
	if cfg.Checkout.BaseRef != "" && cfg.Checkout.BaseRef != "local" && cfg.Checkout.BaseRef != "remote" {
		return Default(), fmt.Errorf("invalid checkout.base_ref %q: must be \"local\" or \"remote\"", cfg.Checkout.BaseRef)
	}

	// Validate default_sort (only "id", "repo", "branch", "commit", or empty allowed)
	if cfg.DefaultSort != "" && cfg.DefaultSort != "id" && cfg.DefaultSort != "repo" && cfg.DefaultSort != "branch" && cfg.DefaultSort != "commit" {
		return Default(), fmt.Errorf("invalid default_sort %q: must be \"id\", \"repo\", \"branch\", or \"commit\"", cfg.DefaultSort)
	}

	// Note: theme.name is validated at runtime with a warning, not an error

	// Use defaults for empty values
	if cfg.Checkout.WorktreeFormat == "" {
		cfg.Checkout.WorktreeFormat = DefaultWorktreeFormat
	}
	if cfg.Forge.Default == "" {
		cfg.Forge.Default = "github"
	}

	// Apply env var overrides (after loading config file)
	if err := applyEnvOverrides(&cfg); err != nil {
		return Default(), err
	}

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(cfg *Config) error {
	// WT_WORKTREE_DIR overrides worktree_dir
	if envDir := os.Getenv("WT_WORKTREE_DIR"); envDir != "" {
		if err := ValidatePath(envDir, "WT_WORKTREE_DIR"); err != nil {
			return err
		}
		expanded, err := expandPath(envDir)
		if err != nil {
			return fmt.Errorf("expand WT_WORKTREE_DIR: %w", err)
		}
		cfg.WorktreeDir = expanded
	}

	// WT_REPO_DIR overrides repo_dir
	if envDir := os.Getenv("WT_REPO_DIR"); envDir != "" {
		if err := ValidatePath(envDir, "WT_REPO_DIR"); err != nil {
			return err
		}
		expanded, err := expandPath(envDir)
		if err != nil {
			return fmt.Errorf("expand WT_REPO_DIR: %w", err)
		}
		cfg.RepoDir = expanded
	}

	return nil
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

// ValidThemeNames is the list of supported theme presets
var ValidThemeNames = []string{"default", "dracula", "nord", "gruvbox", "catppuccin-frappe", "catppuccin-mocha"}

// isValidThemeName checks if the theme name is a known preset
func isValidThemeName(name string) bool {
	for _, valid := range ValidThemeNames {
		if name == valid {
			return true
		}
	}
	return false
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

// defaultConfigAfterWorktreeDir is the config content after the worktree_dir line
const defaultConfigAfterWorktreeDir = `
# Optional: directory where repositories are stored (for -r/-l repo lookup)
# If not set, uses worktree_dir for repo scanning
# Useful when repos live in ~/Code but worktrees go to ~/Git/worktrees
# Can also be set via WT_REPO_DIR env var (env overrides config)
# repo_dir = "~/Code"

# Checkout settings - controls worktree creation behavior
[checkout]
# Worktree folder naming format
# Available placeholders:
#   {repo}    - folder name of git repo (matches -r flag)
#   {branch}  - the branch name as provided
#   {origin}  - repo name from git origin URL (falls back to {repo})
# Example: "{origin}_{branch}" creates "origin-name_feature-branch"
worktree_format = "{repo}-{branch}"

# Base ref mode for new branches (wt checkout -b)
# Controls which ref to use when creating new branches:
#   "remote" - use origin/<branch> (default, ensures up-to-date base)
#   "local"  - use local <branch> (faster, but may be stale)
# base_ref = "remote"

# Auto-fetch before creating new branches (wt checkout -b)
# When true, fetches the base branch from origin before creating worktree
# Same as always passing --fetch flag
# auto_fetch = false

# Auto-set upstream tracking when checking out branches (default: false)
# When true and origin exists:
#   - For new branches (-b): pushes branch to origin, then sets upstream
#   - For existing branches: sets upstream if origin/<branch> exists
# This enables git push/pull without specifying remote.
# set_upstream = false

# Default sort order for 'wt list'
# Available values: "id", "repo", "branch", "commit"
#   "id"     - sort by stable worktree ID (default)
#   "repo"   - sort by repository name
#   "branch" - sort by branch name
#   "commit" - sort by most recent commit (newest first)
# default_sort = "id"

# Hooks - run commands after worktree creation/removal
# Use --hook=name to run a specific hook, --no-hook to skip all hooks
#
# Hooks with "on" run automatically for matching commands.
# Hooks without "on" only run when explicitly called with --hook=name.
#
# Available "on" values: "checkout", "pr", "prune", "merge", "cd", "all"
#
# Hooks run with working directory set to the worktree path.
# For "prune" hooks, working directory is the main repo (worktree is deleted).
#
# Available placeholders:
#   {worktree-dir} - absolute worktree path
#   {repo-dir}     - absolute main repo path
#   {branch}       - branch name
#   {repo}         - folder name of git repo (matches -r flag)
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
# Available presets: "default", "dracula", "nord", "gruvbox",
#                    "catppuccin-frappe", "catppuccin-mocha"
#
# [theme]
# name = "dracula"  # use a preset theme
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
#
# You can also use a preset and override specific colors:
# [theme]
# name = "nord"
# accent = "#ff79c6"   # override just the accent color
#
# Enable nerd font symbols for enhanced icons (requires a nerd font):
# nerdfont = true
`

// defaultConfig is the full default config template with worktree_dir commented out
const defaultConfig = `# wt configuration

# Base directory for new worktrees
# Must be an absolute path or start with ~ (no relative paths like "." or "..")
# Examples: "/Users/you/Git/worktrees" or "~/Git/worktrees"
# Can also be set via WT_WORKTREE_DIR env var (env overrides config)
# worktree_dir = "~/Git/worktrees"
` + defaultConfigAfterWorktreeDir

// Init creates a default config file at ~/.config/wt/config.toml
// If force is true, overwrites existing file
// Returns the path to the created file
func Init(worktreeDir, repoDir string, force bool) (string, error) {
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

	// Write config with worktree dir and optional repo dir
	content := DefaultConfigWithDirs(worktreeDir, repoDir)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	return path, nil
}

// DefaultConfig returns the default configuration content.
func DefaultConfig() string {
	return defaultConfig
}

// DefaultConfigWithDir returns the default configuration with worktree_dir set.
func DefaultConfigWithDir(worktreeDir string) string {
	return DefaultConfigWithDirs(worktreeDir, "")
}

// DefaultConfigWithDirs returns the default configuration with worktree_dir and optional repo_dir set.
func DefaultConfigWithDirs(worktreeDir, repoDir string) string {
	repoDirLine := ""
	if repoDir != "" {
		repoDirLine = fmt.Sprintf("\n# Directory where repositories are stored\nrepo_dir = %q\n", repoDir)
	}
	return fmt.Sprintf(`# wt configuration

# Base directory for new worktrees
worktree_dir = %q
%s`+defaultConfigAfterWorktreeDir, worktreeDir, repoDirLine)
}

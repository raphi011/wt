// Package config handles loading and validation of wt configuration.
//
// Configuration is read from ~/.config/wt/config.toml with environment
// variable overrides for directory settings.
//
// # Configuration Sources (highest priority first)
//
//   - WT_WORKTREE_DIR env var: Target directory for worktrees
//   - WT_REPO_DIR env var: Directory to scan for repositories
//   - Config file settings
//   - Default values
//
// # Key Settings
//
//   - worktree_dir: Base directory for new worktrees (must be absolute or ~/...)
//   - repo_dir: Optional separate directory for repo scanning
//   - worktree_format: Template for worktree folder names (default: "{repo}-{branch}")
//   - base_ref: "local" or "remote" for new branch base (default: "remote")
//   - default_sort: Default sort order for "wt list"
//
// # Hooks Configuration
//
// Hooks are defined in [hooks.NAME] sections:
//
//	[hooks.vscode]
//	command = "code {path}"
//	description = "Open VS Code"
//	on = ["add"]  # auto-run for add command
//
// Hooks with "on" run automatically for matching commands (add, pr, prune, merge).
// Hooks without "on" only run via explicit --hook=name flag.
//
// # Forge Configuration
//
// The [clone] section configures default forge and pattern-based rules:
//
//	[clone]
//	forge = "github"
//	[[clone.rules]]
//	pattern = "company/*"
//	forge = "gitlab"
//
// The [hosts] section maps custom domains to forge types for self-hosted instances.
//
// # Path Validation
//
// Directory paths must be absolute or start with ~ (no relative paths like "."
// or "..") to avoid confusion about the working directory.
package config

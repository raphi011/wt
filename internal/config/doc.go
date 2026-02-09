// Package config handles loading and validation of wt configuration.
//
// Configuration is read from ~/.wt/config.toml with environment
// variable overrides for directory settings.
//
// # Configuration Sources (highest priority first)
//
//   - Config file settings
//   - Default values
//
// # Key Settings
//
//   - checkout.worktree_format: Template for worktree folder names (default: "{repo}-{branch}")
//   - checkout.base_ref: "local" or "remote" for new branch base (default: "remote")
//   - checkout.auto_fetch: Fetch from origin before checkout (default: false)
//   - default_sort: Default sort order for "wt list"
//
// # Hooks Configuration
//
// Hooks are defined in [hooks.NAME] sections:
//
//	[hooks.vscode]
//	command = "code {worktree-dir}"
//	description = "Open VS Code"
//	on = ["checkout"]  # auto-run for checkout command
//
// Hooks with "on" run automatically for matching commands (checkout, pr, prune, merge, all).
// Hooks without "on" only run via explicit --hook=name flag.
//
// # Forge Configuration
//
// The [forge] section configures default forge and pattern-based rules:
//
//	[forge]
//	default = "github"
//	[[forge.rules]]
//	pattern = "company/*"
//	type = "gitlab"
//
// The [hosts] section maps custom domains to forge types for self-hosted instances.
//
// # Path Validation
//
// Directory paths must be absolute or start with ~ (no relative paths like "."
// or "..") to avoid confusion about the working directory.
package config

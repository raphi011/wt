package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// LocalConfig holds per-repo configuration overrides from .wt.toml.
// Pointer fields and zero-value strings indicate "not set" (inherit from global).
type LocalConfig struct {
	Hooks    HooksConfig    `toml:"-"` // merge by name into global
	Checkout LocalCheckout  `toml:"checkout"`
	Merge    LocalMerge     `toml:"merge"`
	Prune    LocalPrune     `toml:"prune"`
	Preserve PreserveConfig `toml:"preserve"` // appended to global
	Forge    LocalForge     `toml:"forge"`
}

// LocalCheckout holds local checkout overrides
type LocalCheckout struct {
	WorktreeFormat string `toml:"worktree_format"`
	BaseRef        string `toml:"base_ref"`
	AutoFetch      *bool  `toml:"auto_fetch"`
	SetUpstream    *bool  `toml:"set_upstream"`
}

// LocalMerge holds local merge overrides
type LocalMerge struct {
	Strategy string `toml:"strategy"`
}

// LocalPrune holds local prune overrides
type LocalPrune struct {
	DeleteLocalBranches *bool `toml:"delete_local_branches"`
}

// LocalForge holds local forge overrides
type LocalForge struct {
	Default string `toml:"default"`
}

// rawLocalConfig is used for initial TOML parsing before processing hooks
type rawLocalConfig struct {
	Hooks    map[string]any `toml:"hooks"`
	Checkout LocalCheckout  `toml:"checkout"`
	Merge    LocalMerge     `toml:"merge"`
	Prune    LocalPrune     `toml:"prune"`
	Preserve PreserveConfig `toml:"preserve"`
	Forge    LocalForge     `toml:"forge"`
}

// LoadLocal reads a per-repo .wt.toml config from the given repo path.
// Returns nil (no error) if the file doesn't exist.
// Returns an error only on parse or validation failure.
func LoadLocal(repoPath string) (*LocalConfig, error) {
	configFile := filepath.Join(repoPath, LocalConfigFileName)

	data, err := os.ReadFile(configFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read local config %s: %w", configFile, err)
	}

	var raw rawLocalConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse local config %s: %w", configFile, err)
	}

	local := &LocalConfig{
		Hooks:    parseHooksConfig(raw.Hooks),
		Checkout: raw.Checkout,
		Merge:    raw.Merge,
		Prune:    raw.Prune,
		Preserve: raw.Preserve,
		Forge:    raw.Forge,
	}

	// Validate forge.default
	if local.Forge.Default != "" && local.Forge.Default != "github" && local.Forge.Default != "gitlab" {
		return nil, fmt.Errorf("invalid forge.default %q in %s: must be \"github\" or \"gitlab\"", local.Forge.Default, configFile)
	}

	// Validate merge.strategy
	if local.Merge.Strategy != "" && local.Merge.Strategy != "squash" && local.Merge.Strategy != "rebase" && local.Merge.Strategy != "merge" {
		return nil, fmt.Errorf("invalid merge.strategy %q in %s: must be \"squash\", \"rebase\", or \"merge\"", local.Merge.Strategy, configFile)
	}

	// Validate checkout.base_ref
	if local.Checkout.BaseRef != "" && local.Checkout.BaseRef != "local" && local.Checkout.BaseRef != "remote" {
		return nil, fmt.Errorf("invalid checkout.base_ref %q in %s: must be \"local\" or \"remote\"", local.Checkout.BaseRef, configFile)
	}

	// Validate preserve patterns
	for i, pat := range local.Preserve.Patterns {
		if _, err := filepath.Match(pat, ""); err != nil {
			return nil, fmt.Errorf("invalid preserve.patterns[%d] %q in %s: %w", i, pat, configFile, err)
		}
	}

	return local, nil
}

// defaultLocalConfig is the template for wt config init --local
const defaultLocalConfig = `# wt local config (per-repo overrides)
# Place this file at the root of your bare repo.
# Settings here override the global ~/.wt/config.toml for this repo only.

# Checkout settings
# [checkout]
# worktree_format = "{branch}"
# base_ref = "remote"
# auto_fetch = false
# set_upstream = false

# Merge settings
# [merge]
# strategy = "squash"

# Prune settings
# [prune]
# delete_local_branches = false

# Preserve settings (patterns here are added to global patterns)
# [preserve]
# patterns = [".env.local"]
# exclude = ["dist"]

# Forge settings
# [forge]
# default = "github"

# Hooks - add repo-specific hooks or override global hooks
# Set enabled = false to disable a global hook for this repo
#
# [hooks.setup]
# command = "npm install"
# description = "Install dependencies"
# on = ["checkout"]
#
# [hooks.global-hook-name]
# enabled = false  # Disable this global hook for this repo
`

// DefaultLocalConfig returns the default local configuration template content.
func DefaultLocalConfig() string {
	return defaultLocalConfig
}

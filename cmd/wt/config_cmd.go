package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Manage configuration",
		Aliases: []string{"cfg"},
		GroupID: GroupConfig,
		Long: `Manage wt configuration.

Global config: ~/.wt/config.toml
Local config:  .wt.toml (in bare repo root)`,
		Example: `  wt config init          # Create default global config
  wt config init --local  # Create local repo config
  wt config show          # Show effective config
  wt config hooks         # List available hooks`,
	}

	cmd.AddCommand(newConfigInitCmd())
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigHooksCmd())

	return cmd
}

func newConfigInitCmd() *cobra.Command {
	var (
		force  bool
		stdout bool
		local  bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create default config file",
		Args:  cobra.NoArgs,
		Long: `Create default config file.

Without flags, creates global config at ~/.wt/config.toml.
With --local, creates per-repo config at .wt.toml in the current repo root.`,
		Example: `  wt config init           # Create global config
  wt config init --local   # Create local repo config
  wt config init -f        # Overwrite existing config
  wt config init -s        # Print config to stdout`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if local {
				return initLocalConfig(cmd, force, stdout)
			}
			return initGlobalConfig(force, stdout)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing config")
	cmd.Flags().BoolVarP(&stdout, "stdout", "s", false, "Print config to stdout")
	cmd.Flags().BoolVar(&local, "local", false, "Create per-repo .wt.toml instead of global config")

	return cmd
}

func initGlobalConfig(force, stdout bool) error {
	configContent := defaultConfig()

	if stdout {
		fmt.Print(configContent)
		return nil
	}

	// Get config path
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(home, ".wt", "config.toml")

	// Check if exists
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config file already exists: %s (use -f to overwrite)", configPath)
		}
	}

	// Create directory
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// Write config
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return err
	}

	fmt.Printf("Created config file: %s\n", configPath)
	return nil
}

func initLocalConfig(cmd *cobra.Command, force, stdout bool) error {
	configContent := config.DefaultLocalConfig()

	if stdout {
		fmt.Print(configContent)
		return nil
	}

	ctx := cmd.Context()
	cfg := config.FromContext(ctx)

	// Find current repo
	reg, err := registry.Load(cfg.RegistryPath)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}

	repo, err := findOrRegisterCurrentRepoFromContext(ctx, reg)
	if err != nil {
		return fmt.Errorf("not in a registered repo: %w", err)
	}

	configPath := filepath.Join(repo.Path, config.LocalConfigFileName)

	// Check if exists
	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("local config already exists: %s (use -f to overwrite)", configPath)
		}
	}

	// Write config
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return err
	}

	fmt.Printf("Created local config: %s\n", configPath)
	return nil
}

func newConfigShowCmd() *cobra.Command {
	var (
		jsonOutput bool
		repoName   string
	)

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show effective configuration",
		Args:  cobra.NoArgs,
		Long: `Show effective configuration.

When inside a repo (or with --repo), shows the merged config with source
annotations (global vs local). Otherwise shows global config only.`,
		Example: `  wt config show              # Show config (merged if in a repo)
  wt config show --repo myrepo  # Show merged config for specific repo
  wt config show --json        # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Determine if we should show merged config
			var repoPath string
			var localConfigPath string

			if repoName != "" {
				// --repo flag: look up in registry
				reg, err := registry.Load(cfg.RegistryPath)
				if err != nil {
					return fmt.Errorf("load registry: %w", err)
				}
				repo, err := reg.FindByName(repoName)
				if err != nil {
					return fmt.Errorf("repository %q not found", repoName)
				}
				repoPath = repo.Path
			} else {
				// Try current repo
				workDir := config.WorkDirFromContext(ctx)
				repoPath = git.GetCurrentRepoMainPathFrom(ctx, workDir)
			}

			// Load local config once and derive effective config from it
			var local *config.LocalConfig
			var effCfg *config.Config
			if repoPath != "" {
				localConfigPath = filepath.Join(repoPath, config.LocalConfigFileName)
				var err error
				local, err = config.LoadLocal(repoPath)
				if err != nil {
					l.Printf("Warning: failed to load local config: %v (using global config)\n", err)
				}
				effCfg = config.MergeLocal(cfg, local)
			} else {
				effCfg = cfg
			}

			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(effCfg)
			}

			fmt.Printf("Global config: ~/.wt/config.toml\n")
			if localConfigPath != "" {
				if local != nil {
					fmt.Printf("Local config:  %s\n", localConfigPath)
				} else {
					fmt.Printf("Local config:  (none)\n")
				}
			}
			fmt.Println()

			// Helper to annotate source
			source := func(isLocal bool) string {
				if isLocal {
					return " (local)"
				}
				return ""
			}

			fmt.Printf("checkout.worktree_format: %s%s\n", effCfg.Checkout.WorktreeFormat, source(local != nil && local.Checkout.WorktreeFormat != ""))
			fmt.Printf("checkout.base_ref: %s%s\n", effCfg.Checkout.BaseRef, source(local != nil && local.Checkout.BaseRef != ""))
			fmt.Printf("checkout.auto_fetch: %v%s\n", effCfg.Checkout.AutoFetch, source(local != nil && local.Checkout.AutoFetch != nil))
			fmt.Printf("checkout.set_upstream: %v%s\n", effCfg.Checkout.ShouldSetUpstream(), source(local != nil && local.Checkout.SetUpstream != nil))
			fmt.Printf("default_sort: %s\n", effCfg.DefaultSort)
			fmt.Printf("forge.default: %s%s\n", effCfg.Forge.Default, source(local != nil && local.Forge.Default != ""))
			fmt.Printf("merge.strategy: %s%s\n", effCfg.Merge.Strategy, source(local != nil && local.Merge.Strategy != ""))
			fmt.Printf("prune.delete_local_branches: %v%s\n", effCfg.Prune.DeleteLocalBranches, source(local != nil && local.Prune.DeleteLocalBranches != nil))
			fmt.Printf("hooks: %d configured\n", len(effCfg.Hooks.Hooks))
			if len(effCfg.Preserve.Patterns) > 0 {
				fmt.Printf("preserve.patterns: %v\n", effCfg.Preserve.Patterns)
			}
			if len(effCfg.Preserve.Exclude) > 0 {
				fmt.Printf("preserve.exclude: %v\n", effCfg.Preserve.Exclude)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&repoName, "repo", "", "Show config for specific repo")
	cmd.RegisterFlagCompletionFunc("repo", completeRepoNames)

	return cmd
}

func newConfigHooksCmd() *cobra.Command {
	var (
		jsonOutput bool
		repoName   string
	)

	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "List available hooks",
		Args:  cobra.NoArgs,
		Long: `List available hooks.

When inside a repo (or with --repo), shows merged hooks with source annotations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Determine repo context
			var repoPath string
			if repoName != "" {
				reg, err := registry.Load(cfg.RegistryPath)
				if err != nil {
					return fmt.Errorf("load registry: %w", err)
				}
				repo, err := reg.FindByName(repoName)
				if err != nil {
					return fmt.Errorf("repository %q not found", repoName)
				}
				repoPath = repo.Path
			} else {
				workDir := config.WorkDirFromContext(ctx)
				repoPath = git.GetCurrentRepoMainPathFrom(ctx, workDir)
			}

			// Load local config once and derive effective config from it
			var effCfg *config.Config
			var local *config.LocalConfig
			if repoPath != "" {
				var err error
				local, err = config.LoadLocal(repoPath)
				if err != nil {
					l.Printf("Warning: failed to load local config: %v (using global config)\n", err)
				}
				effCfg = config.MergeLocal(cfg, local)
			} else {
				effCfg = cfg
			}

			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(effCfg.Hooks.Hooks)
			}

			if len(effCfg.Hooks.Hooks) == 0 {
				fmt.Println("No hooks configured")
				return nil
			}

			globalHooks := cfg.Hooks.Hooks

			for name, hook := range effCfg.Hooks.Hooks {
				// Determine source
				src := "global"
				if local != nil {
					if _, inLocal := local.Hooks.Hooks[name]; inLocal {
						if _, inGlobal := globalHooks[name]; inGlobal {
							src = "local (override)"
						} else {
							src = "local"
						}
					}
				}

				fmt.Printf("%s: [%s]\n", name, src)
				fmt.Printf("  command: %s\n", hook.Command)
				if hook.Description != "" {
					fmt.Printf("  description: %s\n", hook.Description)
				}
				if len(hook.On) > 0 {
					fmt.Printf("  on: %v\n", hook.On)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&repoName, "repo", "", "Show hooks for specific repo")
	cmd.RegisterFlagCompletionFunc("repo", completeRepoNames)

	return cmd
}

// defaultConfig returns the default configuration content
func defaultConfig() string {
	return `# wt configuration
# Config location: ~/.wt/config.toml

# Default labels for newly registered repos
# default_labels = []

# Default sort order for 'wt list'
# default_sort = "date"

# Checkout settings
[checkout]
# Worktree naming format
# Placeholders: {repo}, {branch}
# Path prefixes:
#   "{branch}" or "./{branch}" = nested inside repo
#   "../{repo}-{branch}" = sibling to repo
#   "~/worktrees/{repo}-{branch}" = centralized folder
worktree_format = "{branch}"

# Base ref mode for new branches (wt checkout -b)
# "remote" = use origin/<branch> (default)
# "local" = use local <branch>
# base_ref = "remote"

# Auto-fetch from origin before checkout
# auto_fetch = false

# Hooks - run commands after worktree creation/removal
# [hooks.code]
# command = "code {worktree-dir}"
# description = "Open in VS Code"
# on = ["checkout"]

# Forge settings
# [forge]
# default = "github"
# default_org = "my-org"

# Merge settings
# [merge]
# strategy = "squash"
`
}

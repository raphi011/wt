package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/output"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Manage configuration",
		GroupID: GroupConfig,
		Long: `Manage wt configuration.

Config file: ~/.wt/config.toml`,
		Example: `  wt config init          # Create default config
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
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create default config file",
		Args:  cobra.NoArgs,
		Long:  `Create default config file at ~/.wt/config.toml`,
		Example: `  wt config init           # Create config
  wt config init -f        # Overwrite existing config
  wt config init -s        # Print config to stdout`,
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing config")
	cmd.Flags().BoolVarP(&stdout, "stdout", "s", false, "Print config to stdout")

	return cmd
}

func newConfigShowCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show effective configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := output.FromContext(cmd.Context())

			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(cfg)
			}

			fmt.Printf("Config file: ~/.wt/config.toml\n\n")
			fmt.Printf("worktree_format: %s\n", cfg.WorktreeFormat)
			fmt.Printf("base_ref: %s\n", cfg.BaseRef)
			fmt.Printf("auto_fetch: %v\n", cfg.AutoFetch)
			fmt.Printf("default_sort: %s\n", cfg.DefaultSort)
			fmt.Printf("forge.default: %s\n", cfg.Forge.Default)
			fmt.Printf("merge.strategy: %s\n", cfg.Merge.Strategy)
			fmt.Printf("hooks: %d configured\n", len(cfg.Hooks.Hooks))

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func newConfigHooksCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "List available hooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := output.FromContext(cmd.Context())

			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(cfg.Hooks.Hooks)
			}

			if len(cfg.Hooks.Hooks) == 0 {
				fmt.Println("No hooks configured")
				return nil
			}

			for name, hook := range cfg.Hooks.Hooks {
				fmt.Printf("%s:\n", name)
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

	return cmd
}

// defaultConfig returns the default configuration content
func defaultConfig() string {
	return `# wt configuration
# Config location: ~/.wt/config.toml

# Worktree naming format
# Placeholders: {repo}, {branch}
# Path prefixes:
#   "{branch}" or "./{branch}" = nested inside repo
#   "../{repo}-{branch}" = sibling to repo
#   "~/worktrees/{repo}-{branch}" = centralized folder
worktree_format = "{branch}"

# Default labels for newly registered repos
# default_labels = []

# Base ref mode for new branches (wt checkout -b)
# "remote" = use origin/<branch> (default)
# "local" = use local <branch>
# base_ref = "remote"

# Auto-fetch before creating new branches
# auto_fetch = false

# Default sort order for 'wt list'
# default_sort = "created"

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

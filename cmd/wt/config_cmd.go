package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
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
	configContent := config.DefaultConfig()

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

// resolveConfigWithSources returns the effective config along with the raw local
// config and its path (for source annotations). If repoName is set, looks up the
// repo in the registry. Otherwise tries the current working directory.
// Returns global config if no repo context is found (not inside a git repo and
// --repo not specified). Returns an error if the local config exists but cannot
// be loaded (bad TOML or permissions).
func resolveConfigWithSources(ctx context.Context, repoName string) (*config.Config, *config.LocalConfig, string, error) {
	cfg := config.FromContext(ctx)

	var repoPath string
	if repoName != "" {
		reg, err := registry.Load(cfg.RegistryPath)
		if err != nil {
			return nil, nil, "", fmt.Errorf("load registry: %w", err)
		}
		repo, err := reg.FindByName(repoName)
		if err != nil {
			return nil, nil, "", fmt.Errorf("repository %q: %w", repoName, err)
		}
		repoPath = repo.Path
	} else {
		workDir := config.WorkDirFromContext(ctx)
		repoPath = git.GetCurrentRepoMainPathFrom(ctx, workDir)
	}

	if repoPath == "" {
		return cfg, nil, "", nil
	}

	localConfigPath := filepath.Join(repoPath, config.LocalConfigFileName)
	local, err := config.LoadLocal(repoPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("local config at %s: %w", localConfigPath, err)
	}
	return config.MergeLocal(cfg, local), local, localConfigPath, nil
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
			out := output.FromContext(ctx)

			effCfg, local, localPath, err := resolveConfigWithSources(ctx, repoName)
			if err != nil {
				return err
			}

			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(effCfg)
			}

			globalCfgPath, err := effCfg.GetWtDir()
			if err != nil {
				return fmt.Errorf("resolve config directory: %w", err)
			}
			return renderConfigText(out.Writer(), effCfg, local, filepath.Join(globalCfgPath, "config.toml"), localPath)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&repoName, "repo", "", "Show config for specific repo")
	cmd.RegisterFlagCompletionFunc("repo", completeRepoNames)

	return cmd
}

// renderConfigText writes the effective config to w, grouped by section.
// Source annotations are only shown for non-default values: "(local)" when
// overridden by .wt.toml, or "(env: VAR)" when set via environment variable.
// Annotations within each section are vertically aligned.
func renderConfigText(w io.Writer, cfg *config.Config, local *config.LocalConfig, globalCfgPath, localPath string) error {
	type kv struct {
		key string
		val string
		ann string
	}

	// fprint/fprintln accumulate the first write error so all errors are checked once.
	var writeErr error
	fprint := func(format string, args ...any) {
		if writeErr == nil {
			_, writeErr = fmt.Fprintf(w, format, args...)
		}
	}
	fprintln := func(args ...any) {
		if writeErr == nil {
			_, writeErr = fmt.Fprintln(w, args...)
		}
	}

	// printAlignedLines renders key=value lines with aligned annotations.
	printAlignedLines := func(lines []kv) {
		maxKey, maxVal := 0, 0
		for _, l := range lines {
			if len(l.key) > maxKey {
				maxKey = len(l.key)
			}
			if len(l.val) > maxVal {
				maxVal = len(l.val)
			}
		}
		for _, l := range lines {
			fprint("  %-*s = %-*s  %s\n", maxKey, l.key, maxVal, l.val, l.ann)
		}
	}

	// printSection renders a header and key=value lines with aligned annotations.
	printSection := func(header string, lines []kv) {
		fprint("%s\n", header)
		printAlignedLines(lines)
		fprintln()
	}

	// withDefault returns dflt when val is empty, otherwise val. Use to fill in
	// the known effective default for fields that are not applied by Load().
	withDefault := func(val, dflt string) string {
		if val == "" {
			return dflt
		}
		return val
	}

	// src returns "(local)" if overridden, else "(global)". Use for bool/int fields
	// where the zero value is indistinguishable from an explicit global setting.
	src := func(isLocalOverride bool) string {
		if isLocalOverride {
			return "(local)"
		}
		return "(global)"
	}
	// srcStr returns "(local)" if overridden locally, "(default)" if the value is
	// the empty string (not explicitly configured), or "(global)" otherwise.
	srcStr := func(val string, isLocalOverride bool) string {
		if isLocalOverride {
			return "(local)"
		}
		if val == "" {
			return "(default)"
		}
		return "(global)"
	}
	// srcEnvStr returns "(env: VAR)" if the env var is set, "(default)" if the
	// value is empty, or "(global)" otherwise.
	srcEnvStr := func(val, envVar string) string {
		if os.Getenv(envVar) != "" {
			return fmt.Sprintf("(env: %s)", envVar)
		}
		if val == "" {
			return "(default)"
		}
		return "(global)"
	}

	// Header
	fprint("Global config: %s\n", globalCfgPath)
	if localPath != "" {
		if local != nil {
			fprint("Local config:  %s\n", localPath)
		} else {
			fprint("Local config:  (none)\n")
		}
	}
	fprintln()

	// [checkout]
	printSection("[checkout]", []kv{
		{"worktree_format", cfg.Checkout.WorktreeFormat, srcStr(cfg.Checkout.WorktreeFormat, local != nil && local.Checkout.WorktreeFormat != "")},
		{"base_ref", withDefault(cfg.Checkout.BaseRef, "remote"), srcStr(cfg.Checkout.BaseRef, local != nil && local.Checkout.BaseRef != "")},
		{"auto_fetch", fmt.Sprintf("%v", cfg.Checkout.AutoFetch), src(local != nil && local.Checkout.AutoFetch != nil)},
		{"set_upstream", fmt.Sprintf("%v", cfg.Checkout.ShouldSetUpstream()), src(local != nil && local.Checkout.SetUpstream != nil)},
	})

	// [clone]
	printSection("[clone]", []kv{
		{"mode", cfg.Clone.Mode, srcStr(cfg.Clone.Mode, local != nil && local.Clone.Mode != "")},
	})

	// [forge] — uses printAlignedLines to share alignment logic, then appends rules.
	fprint("[forge]\n")
	printAlignedLines([]kv{
		{"default", cfg.Forge.Default, srcStr(cfg.Forge.Default, local != nil && local.Forge.Default != "")},
		{"default_org", cfg.Forge.DefaultOrg, srcStr(cfg.Forge.DefaultOrg, false)},
	})
	if len(cfg.Forge.Rules) > 0 {
		fprint("  rules:\n")
		for i, rule := range cfg.Forge.Rules {
			parts := []string{
				fmt.Sprintf("pattern=%s", rule.Pattern),
				fmt.Sprintf("type=%s", rule.Type),
			}
			if rule.User != "" {
				parts = append(parts, fmt.Sprintf("user=%s", rule.User))
			}
			fprint("    [%d] %s\n", i, strings.Join(parts, "  "))
		}
	} else {
		fprint("  rules: (none)\n")
	}
	fprintln()

	// [merge]
	printSection("[merge]", []kv{
		{"strategy", withDefault(cfg.Merge.Strategy, "squash"), srcStr(cfg.Merge.Strategy, local != nil && local.Merge.Strategy != "")},
	})

	// [prune]
	printSection("[prune]", []kv{
		{"stale_days", fmt.Sprintf("%d", cfg.Prune.StaleDays), "(global)"},
		{"delete_local_branches", fmt.Sprintf("%v", cfg.Prune.DeleteLocalBranches), src(local != nil && local.Prune.DeleteLocalBranches != nil)},
	})

	// [preserve]
	patternAnn := "(global)"
	if local != nil && len(local.Preserve.Patterns) > 0 {
		patternAnn = "(local)"
	}
	excludeAnn := "(global)"
	if local != nil && len(local.Preserve.Exclude) > 0 {
		excludeAnn = "(local)"
	}
	printSection("[preserve]", []kv{
		{"patterns", "[" + strings.Join(cfg.Preserve.Patterns, ", ") + "]", patternAnn},
		{"exclude", "[" + strings.Join(cfg.Preserve.Exclude, ", ") + "]", excludeAnn},
	})

	// [hooks]
	fprint("[hooks]\n")
	if len(cfg.Hooks.Hooks) == 0 {
		fprint("  (none configured)\n")
	} else {
		fprint("  %d configured  (run 'wt config hooks' for details)\n", len(cfg.Hooks.Hooks))
	}
	fprintln()

	// [theme]
	themeLines := []kv{
		{"name", withDefault(cfg.Theme.Name, "default"), srcEnvStr(cfg.Theme.Name, "WT_THEME")},
		{"mode", withDefault(cfg.Theme.Mode, "auto"), srcEnvStr(cfg.Theme.Mode, "WT_THEME_MODE")},
		{"nerdfont", fmt.Sprintf("%v", cfg.Theme.Nerdfont), "(global)"},
	}
	if cfg.Theme.Primary != "" {
		themeLines = append(themeLines, kv{"primary", cfg.Theme.Primary, "(global)"})
	}
	if cfg.Theme.Accent != "" {
		themeLines = append(themeLines, kv{"accent", cfg.Theme.Accent, "(global)"})
	}
	if cfg.Theme.Success != "" {
		themeLines = append(themeLines, kv{"success", cfg.Theme.Success, "(global)"})
	}
	if cfg.Theme.Error != "" {
		themeLines = append(themeLines, kv{"error", cfg.Theme.Error, "(global)"})
	}
	if cfg.Theme.Muted != "" {
		themeLines = append(themeLines, kv{"muted", cfg.Theme.Muted, "(global)"})
	}
	if cfg.Theme.Normal != "" {
		themeLines = append(themeLines, kv{"normal", cfg.Theme.Normal, "(global)"})
	}
	if cfg.Theme.Info != "" {
		themeLines = append(themeLines, kv{"info", cfg.Theme.Info, "(global)"})
	}
	if cfg.Theme.Warning != "" {
		themeLines = append(themeLines, kv{"warning", cfg.Theme.Warning, "(global)"})
	}
	printSection("[theme]", themeLines)

	// [general]
	printSection("[general]", []kv{
		{"default_sort", withDefault(cfg.DefaultSort, "date"), srcStr(cfg.DefaultSort, false)},
		{"default_labels", "[" + strings.Join(cfg.DefaultLabels, ", ") + "]", "(global)"},
	})

	// [hosts]
	fprint("[hosts]\n")
	if len(cfg.Hosts) == 0 {
		fprint("  (none)\n")
	} else {
		keys := make([]string, 0, len(cfg.Hosts))
		for k := range cfg.Hosts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fprint("  %s = %s\n", k, cfg.Hosts[k])
		}
	}
	fprintln()

	return writeErr
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
			out := output.FromContext(ctx)

			effCfg, local, _, err := resolveConfigWithSources(ctx, repoName)
			if err != nil {
				return err
			}

			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(effCfg.Hooks.Hooks)
			}

			if len(effCfg.Hooks.Hooks) == 0 {
				fmt.Fprintln(out.Writer(), "No hooks configured")
				return nil
			}

			globalHooks := cfg.Hooks.Hooks

			for name, hook := range effCfg.Hooks.Hooks {
				// Determine source
				hookSrc := "global"
				if local != nil {
					if _, inLocal := local.Hooks.Hooks[name]; inLocal {
						if _, inGlobal := globalHooks[name]; inGlobal {
							hookSrc = "local (override)"
						} else {
							hookSrc = "local"
						}
					}
				}

				fmt.Fprintf(out.Writer(), "%s: [%s]\n", name, hookSrc)
				fmt.Fprintf(out.Writer(), "  command: %s\n", hook.Command)
				if hook.Description != "" {
					fmt.Fprintf(out.Writer(), "  description: %s\n", hook.Description)
				}
				if len(hook.On) > 0 {
					fmt.Fprintf(out.Writer(), "  on: %v\n", hook.On)
				}
				fmt.Fprintln(out.Writer())
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().StringVar(&repoName, "repo", "", "Show hooks for specific repo")
	cmd.RegisterFlagCompletionFunc("repo", completeRepoNames)

	return cmd
}

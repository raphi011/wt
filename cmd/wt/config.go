package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/output"
)

func (c *ConfigInitCmd) runConfigInit(ctx context.Context) error {
	out := output.FromContext(ctx)

	// Validate worktree dir (must be absolute or start with ~)
	if err := config.ValidatePath(c.WorktreeDir, "worktree_dir"); err != nil {
		return err
	}
	// Validate repo_dir if provided
	if c.RepoDir != "" {
		if err := config.ValidatePath(c.RepoDir, "repo_dir"); err != nil {
			return err
		}
	}
	if c.Stdout {
		out.Print(config.DefaultConfigWithDirs(c.WorktreeDir, c.RepoDir))
		return nil
	}
	path, err := config.Init(c.WorktreeDir, c.RepoDir, c.Force)
	if err != nil {
		return err
	}
	out.Printf("Created config file: %s\n", path)
	return nil
}

func (c *ConfigShowCmd) runConfigShow(ctx context.Context) error {
	out := output.FromContext(ctx)
	cfg := c.Config

	if c.JSON {
		// Build JSON output structure
		type forgeRuleJSON struct {
			Pattern string `json:"pattern"`
			Type    string `json:"type"`
			User    string `json:"user,omitempty"`
		}
		type forgeJSON struct {
			Default    string          `json:"default"`
			DefaultOrg string          `json:"default_org,omitempty"`
			Rules      []forgeRuleJSON `json:"rules,omitempty"`
		}
		type mergeJSON struct {
			Strategy string `json:"strategy"`
		}
		type configJSON struct {
			WorktreeDir    string            `json:"worktree_dir,omitempty"`
			RepoDir        string            `json:"repo_dir,omitempty"`
			WorktreeFormat string            `json:"worktree_format"`
			BaseRef        string            `json:"base_ref"`
			DefaultSort    string            `json:"default_sort"`
			Forge          forgeJSON         `json:"forge"`
			Merge          mergeJSON         `json:"merge"`
			Hosts          map[string]string `json:"hosts,omitempty"`
		}

		var rules []forgeRuleJSON
		for _, r := range cfg.Forge.Rules {
			rules = append(rules, forgeRuleJSON{
				Pattern: r.Pattern,
				Type:    r.Type,
				User:    r.User,
			})
		}

		baseRef := cfg.BaseRef
		if baseRef == "" {
			baseRef = "remote"
		}
		defaultSort := cfg.DefaultSort
		if defaultSort == "" {
			defaultSort = "id"
		}
		mergeStrategy := cfg.Merge.Strategy
		if mergeStrategy == "" {
			mergeStrategy = "squash"
		}

		result := configJSON{
			WorktreeDir:    cfg.WorktreeDir,
			RepoDir:        cfg.RepoDir,
			WorktreeFormat: cfg.WorktreeFormat,
			BaseRef:        baseRef,
			DefaultSort:    defaultSort,
			Forge: forgeJSON{
				Default:    cfg.Forge.Default,
				DefaultOrg: cfg.Forge.DefaultOrg,
				Rules:      rules,
			},
			Merge: mergeJSON{
				Strategy: mergeStrategy,
			},
			Hosts: cfg.Hosts,
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		out.Println(string(data))
		return nil
	}

	// Text output
	out.Printf("# Worktree folder naming format (placeholders: {repo}, {branch}, {origin})\n")
	out.Printf("worktree_format = %q\n", cfg.WorktreeFormat)
	if cfg.WorktreeDir != "" {
		out.Printf("\n# Base directory for new worktrees\n")
		out.Printf("worktree_dir = %q\n", cfg.WorktreeDir)
	}
	if cfg.RepoDir != "" {
		out.Printf("\n# Directory to scan for repos (-r/-l flags)\n")
		out.Printf("repo_dir = %q\n", cfg.RepoDir)
	}
	baseRef := cfg.BaseRef
	if baseRef == "" {
		baseRef = "remote"
	}
	out.Printf("\n# Base ref for new branches: \"remote\" (origin/<branch>) or \"local\"\n")
	out.Printf("base_ref = %q\n", baseRef)
	defaultSort := cfg.DefaultSort
	if defaultSort == "" {
		defaultSort = "id"
	}
	out.Printf("\n# Default sort order for 'wt list': id, repo, branch, commit\n")
	out.Printf("default_sort = %q\n", defaultSort)

	// Forge section
	out.Printf("\n# Forge settings for PR operations and cloning\n")
	out.Printf("[forge]\n")
	out.Printf("# Default forge: \"github\" or \"gitlab\"\n")
	out.Printf("default = %q\n", cfg.Forge.Default)
	if cfg.Forge.DefaultOrg != "" {
		out.Printf("# Default org when repo specified without org/ prefix\n")
		out.Printf("default_org = %q\n", cfg.Forge.DefaultOrg)
	}
	for _, rule := range cfg.Forge.Rules {
		out.Printf("\n# Pattern-based forge rule\n")
		out.Printf("[[forge.rules]]\n")
		out.Printf("pattern = %q\n", rule.Pattern)
		if rule.Type != "" {
			out.Printf("type = %q\n", rule.Type)
		}
		if rule.User != "" {
			out.Printf("user = %q\n", rule.User)
		}
	}

	// Merge section
	mergeStrategy := cfg.Merge.Strategy
	if mergeStrategy == "" {
		mergeStrategy = "squash"
	}
	out.Printf("\n# Merge settings for 'wt pr merge'\n")
	out.Printf("[merge]\n")
	out.Printf("# Merge strategy: squash, rebase, or merge\n")
	out.Printf("strategy = %q\n", mergeStrategy)

	// Hosts section
	if len(cfg.Hosts) > 0 {
		out.Printf("\n# Custom host mappings for self-hosted GitHub/GitLab\n")
		out.Printf("[hosts]\n")
		for host, forgeType := range cfg.Hosts {
			out.Printf("%q = %q\n", host, forgeType)
		}
	}

	return nil
}

func (c *ConfigHooksCmd) runConfigHooks(ctx context.Context) error {
	out := output.FromContext(ctx)
	cfg := c.Config
	hooksConfig := cfg.Hooks

	if c.JSON {
		type hookJSON struct {
			Name        string   `json:"name"`
			Command     string   `json:"command"`
			Description string   `json:"description,omitempty"`
			On          []string `json:"on,omitempty"`
		}

		var result []hookJSON
		for name, hook := range hooksConfig.Hooks {
			result = append(result, hookJSON{
				Name:        name,
				Command:     hook.Command,
				Description: hook.Description,
				On:          hook.On,
			})
		}

		// Sort by name for consistent output
		slices.SortFunc(result, func(a, b hookJSON) int {
			return cmp.Compare(a.Name, b.Name)
		})

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		out.Println(string(data))
		return nil
	}

	// Text output
	if len(hooksConfig.Hooks) == 0 {
		out.Println("No hooks configured.")
		out.Println("Add hooks to ~/.config/wt/config.toml (see: wt config init)")
		return nil
	}

	out.Println("Hooks:")

	// Sort hook names for consistent output
	var names []string
	for name := range hooksConfig.Hooks {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		hook := hooksConfig.Hooks[name]
		suffix := ""
		if len(hook.On) > 0 {
			suffix = fmt.Sprintf(" (on: %v)", hook.On)
		}
		if hook.Description != "" {
			out.Printf("  %-10s %s%s\n", name, hook.Description, suffix)
			out.Printf("             %s\n", hook.Command)
		} else {
			out.Printf("  %-10s %s%s\n", name, hook.Command, suffix)
		}
	}

	return nil
}

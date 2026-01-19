package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/raphi011/wt/internal/config"
)

func runConfigInit(cmd *ConfigInitCmd) error {
	path, err := config.Init(cmd.Force)
	if err != nil {
		return err
	}
	fmt.Printf("Created config file: %s\n", path)
	return nil
}

func runConfigShow(cmd *ConfigShowCmd, cfg *config.Config) error {
	if cmd.JSON {
		// Build JSON output structure
		type cloneRuleJSON struct {
			Pattern string `json:"pattern"`
			Forge   string `json:"forge"`
		}
		type cloneJSON struct {
			Forge string          `json:"forge"`
			Org   string          `json:"org,omitempty"`
			Rules []cloneRuleJSON `json:"rules,omitempty"`
		}
		type mergeJSON struct {
			Strategy string `json:"strategy,omitempty"`
		}
		type configJSON struct {
			DefaultPath    string            `json:"default_path,omitempty"`
			WorktreeFormat string            `json:"worktree_format"`
			Clone          cloneJSON         `json:"clone"`
			Merge          mergeJSON         `json:"merge,omitempty"`
			Hosts          map[string]string `json:"hosts,omitempty"`
		}

		var rules []cloneRuleJSON
		for _, r := range cfg.Clone.Rules {
			rules = append(rules, cloneRuleJSON{
				Pattern: r.Pattern,
				Forge:   r.Forge,
			})
		}

		result := configJSON{
			DefaultPath:    cfg.DefaultPath,
			WorktreeFormat: cfg.WorktreeFormat,
			Clone: cloneJSON{
				Forge: cfg.Clone.Forge,
				Org:   cfg.Clone.Org,
				Rules: rules,
			},
			Merge: mergeJSON{
				Strategy: cfg.Merge.Strategy,
			},
			Hosts: cfg.Hosts,
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Text output
	fmt.Printf("worktree_format = %q\n", cfg.WorktreeFormat)
	if cfg.DefaultPath != "" {
		fmt.Printf("default_path = %q\n", cfg.DefaultPath)
	}

	// Clone section
	fmt.Printf("\n[clone]\n")
	fmt.Printf("forge = %q\n", cfg.Clone.Forge)
	if cfg.Clone.Org != "" {
		fmt.Printf("org = %q\n", cfg.Clone.Org)
	}
	for _, rule := range cfg.Clone.Rules {
		fmt.Printf("\n[[clone.rules]]\n")
		fmt.Printf("pattern = %q\n", rule.Pattern)
		fmt.Printf("forge = %q\n", rule.Forge)
	}

	// Merge section
	if cfg.Merge.Strategy != "" {
		fmt.Printf("\n[merge]\n")
		fmt.Printf("strategy = %q\n", cfg.Merge.Strategy)
	}

	// Hosts section
	if len(cfg.Hosts) > 0 {
		fmt.Printf("\n[hosts]\n")
		for host, forgeType := range cfg.Hosts {
			fmt.Printf("%q = %q\n", host, forgeType)
		}
	}

	return nil
}

func runConfigHooks(cmd *ConfigHooksCmd, cfg *config.Config) error {
	hooksConfig := cfg.Hooks

	if cmd.JSON {
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
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Text output
	if len(hooksConfig.Hooks) == 0 {
		fmt.Println("No hooks configured.")
		fmt.Println("Add hooks to ~/.config/wt/config.toml (see: wt config init)")
		return nil
	}

	fmt.Println("Hooks:")

	// Sort hook names for consistent output
	var names []string
	for name := range hooksConfig.Hooks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		hook := hooksConfig.Hooks[name]
		suffix := ""
		if len(hook.On) > 0 {
			suffix = fmt.Sprintf(" (on: %v)", hook.On)
		}
		if hook.Description != "" {
			fmt.Printf("  %-10s %s%s\n", name, hook.Description, suffix)
			fmt.Printf("             %s\n", hook.Command)
		} else {
			fmt.Printf("  %-10s %s%s\n", name, hook.Command, suffix)
		}
	}

	return nil
}

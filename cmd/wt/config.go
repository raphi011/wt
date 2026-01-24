package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/raphi011/wt/internal/config"
)

func runConfigInit(cmd *ConfigInitCmd) error {
	// Validate worktree dir (must be absolute or start with ~)
	if err := config.ValidatePath(cmd.WorktreeDir, "worktree_dir"); err != nil {
		return err
	}
	if cmd.Stdout {
		fmt.Print(config.DefaultConfigWithDir(cmd.WorktreeDir))
		return nil
	}
	path, err := config.Init(cmd.WorktreeDir, cmd.Force)
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
			Strategy string `json:"strategy"`
		}
		type configJSON struct {
			WorktreeDir    string            `json:"worktree_dir,omitempty"`
			RepoDir        string            `json:"repo_dir,omitempty"`
			WorktreeFormat string            `json:"worktree_format"`
			BaseRef        string            `json:"base_ref"`
			DefaultSort    string            `json:"default_sort"`
			Clone          cloneJSON         `json:"clone"`
			Merge          mergeJSON         `json:"merge"`
			Hosts          map[string]string `json:"hosts,omitempty"`
		}

		var rules []cloneRuleJSON
		for _, r := range cfg.Clone.Rules {
			rules = append(rules, cloneRuleJSON{
				Pattern: r.Pattern,
				Forge:   r.Forge,
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
			Clone: cloneJSON{
				Forge: cfg.Clone.Forge,
				Org:   cfg.Clone.Org,
				Rules: rules,
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
		fmt.Println(string(data))
		return nil
	}

	// Text output
	fmt.Printf("# Worktree folder naming format (placeholders: {repo}, {branch}, {folder})\n")
	fmt.Printf("worktree_format = %q\n", cfg.WorktreeFormat)
	if cfg.WorktreeDir != "" {
		fmt.Printf("\n# Base directory for new worktrees\n")
		fmt.Printf("worktree_dir = %q\n", cfg.WorktreeDir)
	}
	if cfg.RepoDir != "" {
		fmt.Printf("\n# Directory to scan for repos (-r/-l flags)\n")
		fmt.Printf("repo_dir = %q\n", cfg.RepoDir)
	}
	baseRef := cfg.BaseRef
	if baseRef == "" {
		baseRef = "remote"
	}
	fmt.Printf("\n# Base ref for new branches: \"remote\" (origin/<branch>) or \"local\"\n")
	fmt.Printf("base_ref = %q\n", baseRef)
	defaultSort := cfg.DefaultSort
	if defaultSort == "" {
		defaultSort = "id"
	}
	fmt.Printf("\n# Default sort order for 'wt list': id, repo, branch, commit\n")
	fmt.Printf("default_sort = %q\n", defaultSort)

	// Clone section
	fmt.Printf("\n# Clone settings for 'wt clone' and 'wt pr checkout'\n")
	fmt.Printf("[clone]\n")
	fmt.Printf("# Default forge: \"github\" or \"gitlab\"\n")
	fmt.Printf("forge = %q\n", cfg.Clone.Forge)
	if cfg.Clone.Org != "" {
		fmt.Printf("# Default org when repo specified without org/ prefix\n")
		fmt.Printf("org = %q\n", cfg.Clone.Org)
	}
	for _, rule := range cfg.Clone.Rules {
		fmt.Printf("\n# Pattern-based forge rule\n")
		fmt.Printf("[[clone.rules]]\n")
		fmt.Printf("pattern = %q\n", rule.Pattern)
		fmt.Printf("forge = %q\n", rule.Forge)
	}

	// Merge section
	mergeStrategy := cfg.Merge.Strategy
	if mergeStrategy == "" {
		mergeStrategy = "squash"
	}
	fmt.Printf("\n# Merge settings for 'wt pr merge'\n")
	fmt.Printf("[merge]\n")
	fmt.Printf("# Merge strategy: squash, rebase, or merge\n")
	fmt.Printf("strategy = %q\n", mergeStrategy)

	// Hosts section
	if len(cfg.Hosts) > 0 {
		fmt.Printf("\n# Custom host mappings for self-hosted GitHub/GitLab\n")
		fmt.Printf("[hosts]\n")
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
		slices.SortFunc(result, func(a, b hookJSON) int {
			return cmp.Compare(a.Name, b.Name)
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
	slices.Sort(names)

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

package config

import "maps"

// MergeLocal merges a local per-repo config into a global config,
// returning a new Config without mutating the global.
// Returns global unchanged if local is nil.
func MergeLocal(global *Config, local *LocalConfig) *Config {
	if local == nil {
		return global
	}

	// Shallow copy global — fields not listed in LocalConfig (Hosts, Forge.Rules,
	// Theme, DefaultSort, DefaultLabels, RegistryPath, HistoryPath) are inherited
	// from global as-is. If adding new global-only fields to Config, they are
	// automatically preserved by the shallow copy and must NOT be added to LocalConfig.
	merged := *global

	// Merge hooks by name: local overrides/adds, enabled=false removes
	merged.Hooks = mergeHooks(global.Hooks, local.Hooks)

	// Merge clone (replace)
	if local.Clone.Mode != "" {
		merged.Clone.Mode = local.Clone.Mode
	}

	// Merge checkout (simple field replace for non-zero values)
	if local.Checkout.WorktreeFormat != "" {
		merged.Checkout.WorktreeFormat = local.Checkout.WorktreeFormat
	}
	if local.Checkout.BaseRef != "" {
		merged.Checkout.BaseRef = local.Checkout.BaseRef
	}
	if local.Checkout.AutoFetch != nil {
		merged.Checkout.AutoFetch = *local.Checkout.AutoFetch
	}
	if local.Checkout.SetUpstream != nil {
		merged.Checkout.SetUpstream = local.Checkout.SetUpstream
	}

	// Merge strategy (replace)
	if local.Merge.Strategy != "" {
		merged.Merge.Strategy = local.Merge.Strategy
	}

	// Merge prune (replace)
	if local.Prune.DeleteLocalBranches != nil {
		merged.Prune.DeleteLocalBranches = *local.Prune.DeleteLocalBranches
	}

	// Merge forge default (replace)
	if local.Forge.Default != "" {
		merged.Forge.Default = local.Forge.Default
	}

	// Merge preserve (append with dedup)
	if len(local.Preserve.Patterns) > 0 {
		merged.Preserve.Patterns = appendUnique(global.Preserve.Patterns, local.Preserve.Patterns)
	}
	if len(local.Preserve.Exclude) > 0 {
		merged.Preserve.Exclude = appendUnique(global.Preserve.Exclude, local.Preserve.Exclude)
	}

	return &merged
}

// mergeHooks merges local hooks into global hooks.
// Local hooks with the same name override global hooks.
// Local hooks with enabled=false remove the global hook.
func mergeHooks(global, local HooksConfig) HooksConfig {
	merged := HooksConfig{
		Hooks: make(map[string]Hook, len(global.Hooks)),
	}

	// Copy global hooks
	maps.Copy(merged.Hooks, global.Hooks)

	// Overlay local hooks
	for name, hook := range local.Hooks {
		if !hook.IsEnabled() {
			// Disable: remove from merged
			delete(merged.Hooks, name)
			continue
		}
		merged.Hooks[name] = hook
	}

	return merged
}

// appendUnique appends items from extra to base, skipping duplicates.
// Returns a new slice (never mutates base).
func appendUnique(base, extra []string) []string {
	seen := make(map[string]bool, len(base))
	for _, v := range base {
		seen[v] = true
	}

	result := make([]string, len(base))
	copy(result, base)

	for _, v := range extra {
		if !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}

	return result
}

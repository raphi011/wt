package main

import (
	"slices"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/ui/wizard/flows"
)

func TestWizardHooks(t *testing.T) {
	t.Parallel()

	cfg := config.HooksConfig{Hooks: map[string]config.Hook{
		"after":  {Command: "true", Description: "After checkout", On: []string{"after:checkout"}},
		"all":    {Command: "true", On: []string{"all"}},
		"create": {Command: "true", On: []string{"checkout:create"}},
		"guard":  {Command: "true", On: []string{"before:checkout"}},
		"manual": {Command: "true"},
		"pr":     {Command: "true", On: []string{"checkout:pr"}},
		"prune":  {Command: "true", On: []string{"prune"}},
	}}

	tests := []struct {
		name         string
		actions      []string
		wantDefaults []string
	}{
		{name: "checkout wizard", actions: []string{hooks.ActionCreate, hooks.ActionOpen}, wantDefaults: []string{"after", "all", "create", "guard"}},
		{name: "pr checkout wizard", actions: []string{hooks.ActionPR}, wantDefaults: []string{"after", "all", "guard", "pr"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			infos := wizardHooks(cfg, tt.actions...)
			if len(infos) != len(cfg.Hooks) {
				t.Fatalf("wizardHooks() returned %d hooks, want %d", len(infos), len(cfg.Hooks))
			}

			var defaults []string
			for _, info := range infos {
				if info.IsDefault {
					defaults = append(defaults, info.Name)
				}
			}
			slices.Sort(defaults)
			if !slices.Equal(defaults, tt.wantDefaults) {
				t.Errorf("default hooks = %v, want %v", defaults, tt.wantDefaults)
			}

			i := slices.IndexFunc(infos, func(info flows.HookInfo) bool { return info.Name == "after" })
			if i < 0 || infos[i].Description != "After checkout" {
				t.Errorf("hook description not carried over: %+v", infos)
			}
		})
	}
}

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigResolver_OverridesPrecedence(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		global   bool
		local    *bool
		override *bool
		want     bool
	}{
		{name: "global false"},
		{name: "global true", global: true, want: true},
		{name: "local true", local: new(true), want: true},
		{name: "local false", global: true, local: new(false)},
		{name: "explicit true overrides local false", local: new(false), override: new(true), want: true},
		{name: "explicit false overrides local true", global: true, local: new(true), override: new(false)},
		{name: "explicit false overrides global true", global: true, override: new(false)},
		{name: "explicit true overrides global false", override: new(true), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if tc.local != nil {
				content := fmt.Sprintf("[checkout]\nauto_fetch = %t\n[prune]\ndelete_local_branches = %t\n", *tc.local, *tc.local)
				if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
					t.Fatalf("write local config: %v", err)
				}
			}
			global := &Config{Checkout: CheckoutConfig{AutoFetch: tc.global}, Prune: PruneConfig{DeleteLocalBranches: tc.global}}
			cfg, err := NewResolver(global).ResolveForRepo(dir, Overrides{AutoFetch: tc.override, DeleteLocalBranches: tc.override})
			if err != nil {
				t.Fatalf("resolve config: %v", err)
			}
			if cfg.Checkout.AutoFetch != tc.want || cfg.Prune.DeleteLocalBranches != tc.want {
				t.Errorf("resolved auto_fetch=%t, delete_local_branches=%t; want both %t", cfg.Checkout.AutoFetch, cfg.Prune.DeleteLocalBranches, tc.want)
			}
		})
	}
}

func TestConfigResolver_StringOverridesPrecedence(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		global   string
		local    string
		override *string
		want     string
	}{
		{name: "unset"},
		{name: "global", global: "g", want: "g"},
		{name: "local overrides global", global: "g", local: "l", want: "l"},
		{name: "override beats local", global: "g", local: "l", override: new("o"), want: "o"},
		{name: "override beats global", global: "g", override: new("o"), want: "o"},
		{name: "empty override beats config", global: "g", local: "l", override: new("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if tc.local != "" {
				// Enum fields are validated on load, so the local file only sets the free-form one
				content := fmt.Sprintf("[checkout]\nworktree_format = %q\n", tc.local)
				if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
					t.Fatalf("write local config: %v", err)
				}
			}
			global := &Config{
				DefaultSort: tc.global,
				Merge:       MergeConfig{Strategy: tc.global},
				Clone:       CloneConfig{Mode: tc.global},
				Checkout:    CheckoutConfig{WorktreeFormat: tc.global},
			}
			cfg, err := NewResolver(global).ResolveForRepo(dir, Overrides{MergeStrategy: tc.override, DefaultSort: tc.override, CloneMode: tc.override, WorktreeFormat: tc.override})
			if err != nil {
				t.Fatalf("resolve config: %v", err)
			}
			if cfg.Checkout.WorktreeFormat != tc.want {
				t.Errorf("worktree_format = %q, want %q", cfg.Checkout.WorktreeFormat, tc.want)
			}
			// Not set in the local file: override, else global
			wantGlobal := tc.global
			if tc.override != nil {
				wantGlobal = *tc.override
			}
			if cfg.Merge.Strategy != wantGlobal || cfg.DefaultSort != wantGlobal || cfg.Clone.Mode != wantGlobal {
				t.Errorf("strategy=%q, default_sort=%q, clone.mode=%q; want all %q", cfg.Merge.Strategy, cfg.DefaultSort, cfg.Clone.Mode, wantGlobal)
			}
			if global.Merge.Strategy != tc.global || global.DefaultSort != tc.global || global.Clone.Mode != tc.global || global.Checkout.WorktreeFormat != tc.global {
				t.Error("overrides mutated the global config")
			}
		})
	}
}

func TestConfigResolver_OverridesIsolation(t *testing.T) {
	t.Parallel()

	for _, local := range []bool{false, true} {
		t.Run(fmt.Sprintf("local=%t", local), func(t *testing.T) {
			t.Parallel()
			global := &Config{Checkout: CheckoutConfig{AutoFetch: true}, Prune: PruneConfig{DeleteLocalBranches: true}}
			r := NewResolver(global)
			dir := t.TempDir()
			if local {
				if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte("[checkout]\nauto_fetch = true\n[prune]\ndelete_local_branches = true\n"), 0644); err != nil {
					t.Fatalf("write local config: %v", err)
				}
			}
			base, err := r.ConfigForRepo(dir)
			if err != nil {
				t.Fatalf("resolve base config: %v", err)
			}
			resolved, err := r.ResolveForRepo(dir, Overrides{AutoFetch: new(false), DeleteLocalBranches: new(false)})
			if err != nil {
				t.Fatalf("resolve overrides: %v", err)
			}
			if resolved == base || resolved == global {
				t.Fatal("override resolution should return an independent config")
			}
			if !base.Checkout.AutoFetch || !base.Prune.DeleteLocalBranches || !global.Checkout.AutoFetch || !global.Prune.DeleteLocalBranches {
				t.Fatal("overrides mutated the cached or global config")
			}
			resolved.Checkout.AutoFetch = true
			resolved.Prune.DeleteLocalBranches = true
			for _, repo := range []string{dir, t.TempDir()} {
				cfg, err := r.ResolveForRepo(repo, Overrides{})
				if err != nil {
					t.Fatalf("resolve without overrides: %v", err)
				}
				if !cfg.Checkout.AutoFetch || !cfg.Prune.DeleteLocalBranches {
					t.Error("overrides leaked into a later call or another repo")
				}
				cfg.Checkout.AutoFetch = false
				cfg.Prune.DeleteLocalBranches = false
			}
			cached, err := r.ConfigForRepo(dir)
			if err != nil {
				t.Fatalf("read cached config: %v", err)
			}
			if cached != base || !cached.Checkout.AutoFetch || !cached.Prune.DeleteLocalBranches {
				t.Error("resolved result mutation changed the cached config")
			}
		})
	}
}

func TestConfigResolver_ResolveGlobalIndependence(t *testing.T) {
	t.Parallel()

	global := &Config{Checkout: CheckoutConfig{AutoFetch: true}, Prune: PruneConfig{DeleteLocalBranches: true}}
	r := NewResolver(global)
	resolved := r.ResolveGlobal(Overrides{AutoFetch: new(false), DeleteLocalBranches: new(false)})
	if resolved == global || resolved.Checkout.AutoFetch || resolved.Prune.DeleteLocalBranches {
		t.Fatal("ResolveGlobal did not independently apply explicit false overrides")
	}
	if !global.Checkout.AutoFetch || !global.Prune.DeleteLocalBranches {
		t.Fatal("ResolveGlobal mutated its source")
	}
	plain := r.ResolveGlobal(Overrides{})
	plain.Checkout.AutoFetch = false
	plain.Prune.DeleteLocalBranches = false
	next := r.ResolveGlobal(Overrides{})
	if !next.Checkout.AutoFetch || !next.Prune.DeleteLocalBranches {
		t.Error("mutating a resolved global config leaked into later resolution")
	}
}

func TestConfigResolver_OverridesInvalidLocalConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte("invalid [[["), 0644); err != nil {
		t.Fatalf("write local config: %v", err)
	}
	if cfg, err := NewResolver(&Config{}).ResolveForRepo(dir, Overrides{AutoFetch: new(true), DeleteLocalBranches: new(false)}); err == nil || cfg != nil {
		t.Fatalf("expected strict local config error and nil config, got config=%v, error=%v", cfg, err)
	}
}

func TestConfigResolver_OverridesPreserveOtherSettings(t *testing.T) {
	t.Parallel()

	global := &Config{
		RegistryPath: "/registry/repos.json", HistoryPath: "/history/history.json",
		DefaultSort: "repo", DefaultLabels: []string{"backend"},
		Hosts:    map[string]string{"git.example.com": "gitlab"},
		Hooks:    HooksConfig{Hooks: map[string]Hook{}},
		Theme:    ThemeConfig{Name: "nord"},
		Checkout: CheckoutConfig{WorktreeFormat: "{repo}-{branch}", AutoFetch: true},
		Prune:    PruneConfig{DeleteLocalBranches: true, StaleDays: 21},
	}
	dir := t.TempDir()
	content := "default_sort = \"branch\"\n[hosts]\n\"git.example.com\" = \"github\"\n[checkout]\nworktree_format = \"{branch}\"\n[prune]\nstale_days = 1\n"
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write local config: %v", err)
	}
	resolved, err := NewResolver(global).ResolveForRepo(dir, Overrides{AutoFetch: new(false), DeleteLocalBranches: new(false)})
	if err != nil {
		t.Fatalf("resolve overrides: %v", err)
	}
	want := *global
	want.Checkout.WorktreeFormat = "{branch}"
	want.Checkout.AutoFetch = false
	want.Prune.DeleteLocalBranches = false
	if !reflect.DeepEqual(resolved, &want) {
		t.Errorf("resolved config = %#v, want %#v", resolved, &want)
	}
}

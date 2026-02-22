package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigResolver_Global(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{WorktreeFormat: "{repo}-{branch}"},
		Forge:    ForgeConfig{Default: "github"},
	}

	r := NewResolver(global)
	if r.Global() != global {
		t.Error("Global() should return the global config")
	}
}

func TestConfigResolver_NoLocalConfig(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{WorktreeFormat: "{repo}-{branch}"},
		Forge:    ForgeConfig{Default: "github"},
		Hooks:    HooksConfig{Hooks: map[string]Hook{}},
	}

	r := NewResolver(global)
	dir := t.TempDir() // no .wt.toml

	cfg, err := r.ConfigForRepo(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Checkout.WorktreeFormat != "{repo}-{branch}" {
		t.Errorf("worktree_format = %q, want {repo}-{branch}", cfg.Checkout.WorktreeFormat)
	}
}

func TestConfigResolver_WithLocalConfig(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{WorktreeFormat: "{repo}-{branch}"},
		Forge:    ForgeConfig{Default: "github"},
		Hooks:    HooksConfig{Hooks: map[string]Hook{}},
	}

	dir := t.TempDir()
	content := `[checkout]
worktree_format = "{branch}"

[forge]
default = "gitlab"
`
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	r := NewResolver(global)
	cfg, err := r.ConfigForRepo(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Checkout.WorktreeFormat != "{branch}" {
		t.Errorf("worktree_format = %q, want {branch}", cfg.Checkout.WorktreeFormat)
	}
	if cfg.Forge.Default != "gitlab" {
		t.Errorf("forge.default = %q, want gitlab", cfg.Forge.Default)
	}
}

func TestConfigResolver_Caching(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{WorktreeFormat: "{repo}-{branch}"},
		Forge:    ForgeConfig{Default: "github"},
		Hooks:    HooksConfig{Hooks: map[string]Hook{}},
	}

	dir := t.TempDir()
	content := `[checkout]
worktree_format = "{branch}"
`
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	r := NewResolver(global)

	cfg1, err := r.ConfigForRepo(dir)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	cfg2, err := r.ConfigForRepo(dir)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if cfg1 != cfg2 {
		t.Error("expected same pointer from cache")
	}
}

func TestConfigResolver_DifferentRepos(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{WorktreeFormat: "{repo}-{branch}"},
		Forge:    ForgeConfig{Default: "github"},
		Hooks:    HooksConfig{Hooks: map[string]Hook{}},
	}

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// dir1 has local config
	content := `[forge]
default = "gitlab"
`
	if err := os.WriteFile(filepath.Join(dir1, LocalConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// dir2 has no local config

	r := NewResolver(global)

	cfg1, err := r.ConfigForRepo(dir1)
	if err != nil {
		t.Fatalf("dir1: %v", err)
	}
	cfg2, err := r.ConfigForRepo(dir2)
	if err != nil {
		t.Fatalf("dir2: %v", err)
	}

	if cfg1.Forge.Default != "gitlab" {
		t.Errorf("dir1 forge.default = %q, want gitlab", cfg1.Forge.Default)
	}
	if cfg2.Forge.Default != "github" {
		t.Errorf("dir2 forge.default = %q, want github", cfg2.Forge.Default)
	}
}

func TestConfigResolver_InvalidLocalConfig(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{WorktreeFormat: "{repo}-{branch}"},
		Forge:    ForgeConfig{Default: "github"},
		Hooks:    HooksConfig{Hooks: map[string]Hook{}},
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte("invalid [[["), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	r := NewResolver(global)
	_, err := r.ConfigForRepo(dir)
	if err == nil {
		t.Fatal("expected error for invalid local config")
	}
}

func TestResolverContext(t *testing.T) {
	t.Parallel()

	global := &Config{Forge: ForgeConfig{Default: "github"}}
	r := NewResolver(global)

	ctx := context.Background()
	ctx = WithResolver(ctx, r)

	got := ResolverFromContext(ctx)
	if got != r {
		t.Error("expected same resolver from context")
	}
}

func TestResolverFromContext_Nil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	got := ResolverFromContext(ctx)
	if got != nil {
		t.Error("expected nil when no resolver in context")
	}
}

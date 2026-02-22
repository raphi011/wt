package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLocal_NoFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	local, err := LoadLocal(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local != nil {
		t.Fatalf("expected nil, got %+v", local)
	}
}

func TestLoadLocal_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(""), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	local, err := LoadLocal(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if local == nil {
		t.Fatal("expected non-nil local config for empty file")
	}
}

func TestLoadLocal_AllFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `
[checkout]
worktree_format = "{branch}"
base_ref = "local"
auto_fetch = true
set_upstream = true

[merge]
strategy = "rebase"

[prune]
delete_local_branches = true

[preserve]
patterns = [".env.local"]
exclude = ["dist"]

[forge]
default = "gitlab"

[hooks.setup]
command = "npm install"
description = "Install deps"
on = ["checkout"]

[hooks.lint]
command = "npm run lint"
enabled = false
`
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	local, err := LoadLocal(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if local.Checkout.WorktreeFormat != "{branch}" {
		t.Errorf("worktree_format = %q, want {branch}", local.Checkout.WorktreeFormat)
	}
	if local.Checkout.BaseRef != "local" {
		t.Errorf("base_ref = %q, want local", local.Checkout.BaseRef)
	}
	if local.Checkout.AutoFetch == nil || !*local.Checkout.AutoFetch {
		t.Errorf("auto_fetch = %v, want true", local.Checkout.AutoFetch)
	}
	if local.Checkout.SetUpstream == nil || !*local.Checkout.SetUpstream {
		t.Errorf("set_upstream = %v, want true", local.Checkout.SetUpstream)
	}
	if local.Merge.Strategy != "rebase" {
		t.Errorf("strategy = %q, want rebase", local.Merge.Strategy)
	}
	if local.Prune.DeleteLocalBranches == nil || !*local.Prune.DeleteLocalBranches {
		t.Errorf("delete_local_branches = %v, want true", local.Prune.DeleteLocalBranches)
	}
	if local.Forge.Default != "gitlab" {
		t.Errorf("forge.default = %q, want gitlab", local.Forge.Default)
	}
	if len(local.Preserve.Patterns) != 1 || local.Preserve.Patterns[0] != ".env.local" {
		t.Errorf("preserve.patterns = %v, want [.env.local]", local.Preserve.Patterns)
	}
	if len(local.Preserve.Exclude) != 1 || local.Preserve.Exclude[0] != "dist" {
		t.Errorf("preserve.exclude = %v, want [dist]", local.Preserve.Exclude)
	}

	// Check hooks
	if len(local.Hooks.Hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(local.Hooks.Hooks))
	}
	setup := local.Hooks.Hooks["setup"]
	if setup.Command != "npm install" {
		t.Errorf("setup.command = %q, want npm install", setup.Command)
	}
	lint := local.Hooks.Hooks["lint"]
	if lint.IsEnabled() {
		t.Error("lint hook should be disabled")
	}
}

func TestLoadLocal_InvalidStrategy(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `[merge]
strategy = "invalid"
`
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := LoadLocal(dir)
	if err == nil {
		t.Fatal("expected error for invalid strategy")
	}
}

func TestLoadLocal_InvalidForgeDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `[forge]
default = "bitbucket"
`
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := LoadLocal(dir)
	if err == nil {
		t.Fatal("expected error for invalid forge.default")
	}
}

func TestLoadLocal_InvalidBaseRef(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `[checkout]
base_ref = "invalid"
`
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := LoadLocal(dir)
	if err == nil {
		t.Fatal("expected error for invalid base_ref")
	}
}

func TestLoadLocal_InvalidPreservePattern(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `[preserve]
patterns = ["[invalid"]
`
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := LoadLocal(dir)
	if err == nil {
		t.Fatal("expected error for invalid preserve pattern")
	}
}

func TestLoadLocal_InvalidTOML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LocalConfigFileName), []byte("invalid toml [[["), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := LoadLocal(dir)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

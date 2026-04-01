//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/registry"
)

// TestConfigInit_Stdout tests printing default config to stdout.
//
// Scenario: User runs `wt config init --stdout`
// Expected: Default TOML config is printed (no file created)
func TestConfigInit_Stdout(t *testing.T) {
	t.Parallel()

	// config init --stdout writes to fmt.Printf (os.Stdout), so we can't capture
	// via output.Printer. We just verify no error and the command runs successfully.
	ctx := testContext(t)

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"init", "--stdout"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config init --stdout failed: %v", err)
	}
}

// TestConfigShow_Basic tests basic config display.
//
// Scenario: User runs `wt config show`
// Expected: Config fields are displayed (no error)
func TestConfigShow_Basic(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "{repo}-{branch}",
		},
		Forge: config.ForgeConfig{
			Default: "github",
		},
	}

	ctx := testContextWithConfig(t, cfg, t.TempDir())

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"show"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config show failed: %v", err)
	}
}

// TestConfigShow_JSON tests JSON output of config show.
//
// Scenario: User runs `wt config show --json`
// Expected: Valid JSON is output containing config fields
func TestConfigShow_JSON(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "{repo}-{branch}",
		},
		Forge: config.ForgeConfig{
			Default: "github",
		},
	}

	ctx, out := testContextWithConfigAndOutput(t, cfg, t.TempDir())

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"show", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config show --json failed: %v", err)
	}

	// Verify output is valid JSON
	output := out.String()
	if output == "" {
		t.Fatal("expected JSON output, got empty")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}
}

// TestConfigHooks_NoHooks tests hooks display when none are configured.
//
// Scenario: User runs `wt config hooks` with no hooks in config
// Expected: Prints "No hooks configured" (no error)
func TestConfigHooks_NoHooks(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	ctx := testContextWithConfig(t, cfg, t.TempDir())

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"hooks"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config hooks failed: %v", err)
	}
}

// TestConfigHooks_WithHooks tests hooks display when hooks are configured.
//
// Scenario: User runs `wt config hooks` with hooks in config
// Expected: Hook details are listed (no error)
func TestConfigHooks_WithHooks(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"code": {
					Command:     "code {worktree-dir}",
					Description: "Open in VS Code",
					On:          []string{"checkout"},
				},
			},
		},
	}

	ctx := testContextWithConfig(t, cfg, t.TempDir())

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"hooks"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config hooks failed: %v", err)
	}
}

// TestConfigHooks_JSON tests JSON output of hooks.
//
// Scenario: User runs `wt config hooks --json`
// Expected: Valid JSON array is output
func TestConfigHooks_JSON(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"code": {
					Command:     "code {worktree-dir}",
					Description: "Open in VS Code",
					On:          []string{"checkout"},
				},
			},
		},
	}

	ctx, out := testContextWithConfigAndOutput(t, cfg, t.TempDir())

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"hooks", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config hooks --json failed: %v", err)
	}

	output := out.String()
	if output == "" {
		t.Fatal("expected JSON output, got empty")
	}

	// hooks --json outputs a map of hook name -> hook
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	if _, ok := result["code"]; !ok {
		t.Errorf("expected hook 'code' in JSON output, got keys: %v", result)
	}
}

// TestConfigHooks_JSON_Empty tests JSON output when no hooks configured.
//
// Scenario: User runs `wt config hooks --json` with no hooks
// Expected: Valid JSON (null or empty object)
func TestConfigHooks_JSON_Empty(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	ctx, out := testContextWithConfigAndOutput(t, cfg, t.TempDir())

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"hooks", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config hooks --json failed: %v", err)
	}

	output := strings.TrimSpace(out.String())
	// Accept null or empty object
	if output != "null" && output != "{}" {
		t.Errorf("expected null or {} for empty hooks, got %q", output)
	}
}

// TestConfigShow_WithLocalConfig tests config show --json when a local .wt.toml overrides values.
//
// Scenario: User is inside a repo that has a local .wt.toml with overrides, runs `wt config show --json`
// Expected: JSON output includes the local overrides (e.g. forge.default = "gitlab")
func TestConfigShow_WithLocalConfig(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	// Write a local .wt.toml that overrides forge.default
	localCfgContent := `[forge]
default = "gitlab"
`
	localCfgPath := filepath.Join(repoPath, config.LocalConfigFileName)
	if err := os.WriteFile(localCfgPath, []byte(localCfgContent), 0644); err != nil {
		t.Fatalf("failed to write local config: %v", err)
	}

	cfg := &config.Config{
		Forge: config.ForgeConfig{
			Default: "github",
		},
	}

	// workDir is set to the repo path so config show can detect the local config
	ctx, out := testContextWithConfigAndOutput(t, cfg, repoPath)

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"show", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config show --json with local config failed: %v", err)
	}

	output := out.String()
	if output == "" {
		t.Fatal("expected JSON output, got empty")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	// Verify the local override took effect: Forge.Default should be "gitlab"
	forge, ok := result["Forge"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'Forge' key in JSON output, got: %v", result)
	}
	if forge["Default"] != "gitlab" {
		t.Errorf("expected Forge.Default = 'gitlab' (from local override), got %q", forge["Default"])
	}
}

// TestConfigInit_LocalStdout tests `config init --local --stdout`.
//
// Scenario: User runs `wt config init --local --stdout`
// Expected: Default local config TOML is printed, no error (--stdout path does not write files)
func TestConfigInit_LocalStdout(t *testing.T) {
	t.Parallel()

	// --local --stdout prints to fmt.Printf (os.Stdout) and returns immediately without
	// needing a repo or registry. We just verify no error.
	ctx := testContext(t)

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"init", "--local", "--stdout"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config init --local --stdout failed: %v", err)
	}
}

// TestConfigShow_PreservePatterns tests that preserve patterns appear in config show --json output.
//
// Scenario: Config has preserve patterns and excludes set, user runs `wt config show --json`
// Expected: JSON output includes the preserve patterns and excludes
func TestConfigShow_PreservePatterns(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Preserve: config.PreserveConfig{
			Patterns: []string{".env", ".env.local"},
			Exclude:  []string{"node_modules", "dist"},
		},
	}

	ctx, out := testContextWithConfigAndOutput(t, cfg, t.TempDir())

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"show", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config show --json with preserve patterns failed: %v", err)
	}

	output := out.String()
	if output == "" {
		t.Fatal("expected JSON output, got empty")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	preserve, ok := result["Preserve"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'Preserve' key in JSON output, got: %v", result)
	}

	patterns, ok := preserve["Patterns"].([]any)
	if !ok || len(patterns) == 0 {
		t.Errorf("expected non-empty Preserve.Patterns in JSON output, got: %v", preserve["Patterns"])
	}

	exclude, ok := preserve["Exclude"].([]any)
	if !ok || len(exclude) == 0 {
		t.Errorf("expected non-empty Preserve.Exclude in JSON output, got: %v", preserve["Exclude"])
	}
}

// TestConfigHooks_WithRepo tests `config hooks --repo <name>` with a registered repo.
//
// Scenario: A repo is registered and has hooks both globally and in its local .wt.toml.
// User runs `wt config hooks --repo myrepo`.
// Expected: No error; command resolves the repo, loads local config, and displays hooks.
func TestConfigHooks_WithRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	// Write a local .wt.toml with a hook override
	localCfgContent := `[hooks.setup]
command = "npm install"
description = "Install dependencies"
on = ["checkout"]
`
	localCfgPath := filepath.Join(repoPath, config.LocalConfigFileName)
	if err := os.WriteFile(localCfgPath, []byte(localCfgContent), 0644); err != nil {
		t.Fatalf("failed to write local config: %v", err)
	}

	// Set up registry with the repo
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"code": {
					Command:     "code {worktree-dir}",
					Description: "Open in VS Code",
					On:          []string{"checkout"},
				},
			},
		},
	}

	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"hooks", "--repo", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config hooks --repo myrepo failed: %v", err)
	}
}

// TestConfigShow_RepoFlag tests `config show --json --repo <name>` with a registered repo.
//
// Scenario: A repo is registered with a local .wt.toml. User runs `wt config show --json --repo myrepo`.
// Expected: Valid JSON output contains the merged config (including local overrides).
func TestConfigShow_RepoFlag(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	// Write a local .wt.toml that overrides checkout.worktree_format
	localCfgContent := `[checkout]
worktree_format = "custom/{branch}"
`
	localCfgPath := filepath.Join(repoPath, config.LocalConfigFileName)
	if err := os.WriteFile(localCfgPath, []byte(localCfgContent), 0644); err != nil {
		t.Fatalf("failed to write local config: %v", err)
	}

	// Set up registry with the repo
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry dir: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: ".worktrees/{branch}",
		},
	}

	ctx, out := testContextWithConfigAndOutput(t, cfg, tmpDir)

	cmd := newConfigCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"show", "--json", "--repo", "myrepo"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config show --json --repo myrepo failed: %v", err)
	}

	output := out.String()
	if output == "" {
		t.Fatal("expected JSON output, got empty")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	// Verify the local override was applied: Checkout.WorktreeFormat should be from local config
	checkout, ok := result["Checkout"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'Checkout' key in JSON output, got: %v", result)
	}
	if checkout["WorktreeFormat"] != "custom/{branch}" {
		t.Errorf("expected Checkout.WorktreeFormat = 'custom/{branch}' (from local override), got %q", checkout["WorktreeFormat"])
	}
}

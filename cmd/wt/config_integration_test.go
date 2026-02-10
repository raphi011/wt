//go:build integration

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
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

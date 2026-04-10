package config

import (
	"testing"
)

func TestMergeLocal_Nil(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{WorktreeFormat: "{repo}-{branch}"},
		Forge:    ForgeConfig{Default: "github"},
	}

	result := MergeLocal(global, nil)
	if result != global {
		t.Error("expected same pointer when local is nil")
	}
}

func TestMergeLocal_NoMutation(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{WorktreeFormat: "{repo}-{branch}"},
		Forge:    ForgeConfig{Default: "github"},
		Hooks: HooksConfig{
			Hooks: map[string]Hook{
				"test": {Command: "echo test"},
			},
		},
	}

	local := &LocalConfig{
		Checkout: LocalCheckout{WorktreeFormat: "{branch}"},
	}

	MergeLocal(global, local)

	// Verify global wasn't mutated
	if global.Checkout.WorktreeFormat != "{repo}-{branch}" {
		t.Error("global config was mutated")
	}
}

func TestMergeLocal_SimpleFieldReplace(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{
			WorktreeFormat: "{repo}-{branch}",
			BaseRef:        "remote",
			AutoFetch:      false,
		},
		Merge: MergeConfig{Strategy: "squash"},
		Prune: PruneConfig{DeleteLocalBranches: false},
		Forge: ForgeConfig{Default: "github"},
		Hooks: HooksConfig{Hooks: map[string]Hook{}},
	}

	local := &LocalConfig{
		Checkout: LocalCheckout{
			WorktreeFormat: "{branch}",
			BaseRef:        "local",
			AutoFetch:      new(true),
			SetUpstream:    new(true),
		},
		Merge: LocalMerge{Strategy: "rebase"},
		Prune: LocalPrune{DeleteLocalBranches: new(true)},
		Forge: LocalForge{Default: "gitlab"},
	}

	result := MergeLocal(global, local)

	if result.Checkout.WorktreeFormat != "{branch}" {
		t.Errorf("worktree_format = %q, want {branch}", result.Checkout.WorktreeFormat)
	}
	if result.Checkout.BaseRef != "local" {
		t.Errorf("base_ref = %q, want local", result.Checkout.BaseRef)
	}
	if !result.Checkout.AutoFetch {
		t.Error("auto_fetch should be true")
	}
	if !result.Checkout.ShouldSetUpstream() {
		t.Error("set_upstream should be true")
	}
	if result.Merge.Strategy != "rebase" {
		t.Errorf("strategy = %q, want rebase", result.Merge.Strategy)
	}
	if !result.Prune.DeleteLocalBranches {
		t.Error("delete_local_branches should be true")
	}
	if result.Forge.Default != "gitlab" {
		t.Errorf("forge.default = %q, want gitlab", result.Forge.Default)
	}
}

func TestMergeLocal_ZeroValuesPreserveGlobal(t *testing.T) {
	t.Parallel()

	global := &Config{
		Checkout: CheckoutConfig{
			WorktreeFormat: "{repo}-{branch}",
			BaseRef:        "remote",
			AutoFetch:      true,
		},
		Merge: MergeConfig{Strategy: "squash"},
		Forge: ForgeConfig{Default: "github"},
		Hooks: HooksConfig{Hooks: map[string]Hook{}},
	}

	// Empty local config — nothing should change
	local := &LocalConfig{}

	result := MergeLocal(global, local)

	if result.Checkout.WorktreeFormat != "{repo}-{branch}" {
		t.Errorf("worktree_format = %q, want {repo}-{branch}", result.Checkout.WorktreeFormat)
	}
	if result.Checkout.BaseRef != "remote" {
		t.Errorf("base_ref = %q, want remote", result.Checkout.BaseRef)
	}
	if !result.Checkout.AutoFetch {
		t.Error("auto_fetch should remain true")
	}
	if result.Merge.Strategy != "squash" {
		t.Errorf("strategy = %q, want squash", result.Merge.Strategy)
	}
	if result.Forge.Default != "github" {
		t.Errorf("forge.default = %q, want github", result.Forge.Default)
	}
}

func TestMergeLocal_CloneModeReplace(t *testing.T) {
	t.Parallel()

	global := &Config{
		Clone:    CloneConfig{Mode: "bare"},
		Checkout: CheckoutConfig{WorktreeFormat: DefaultWorktreeFormat},
		Forge:    ForgeConfig{Default: "github"},
		Hooks:    HooksConfig{Hooks: map[string]Hook{}},
	}

	local := &LocalConfig{
		Clone: LocalClone{Mode: "regular"},
	}

	result := MergeLocal(global, local)

	if result.Clone.Mode != "regular" {
		t.Errorf("clone.mode = %q, want regular", result.Clone.Mode)
	}

	// Verify global wasn't mutated
	if global.Clone.Mode != "bare" {
		t.Error("global clone.mode was mutated")
	}
}

func TestMergeLocal_CloneModePreserveGlobal(t *testing.T) {
	t.Parallel()

	global := &Config{
		Clone:    CloneConfig{Mode: "regular"},
		Checkout: CheckoutConfig{WorktreeFormat: DefaultWorktreeFormat},
		Forge:    ForgeConfig{Default: "github"},
		Hooks:    HooksConfig{Hooks: map[string]Hook{}},
	}

	// Empty local — should preserve global
	local := &LocalConfig{}

	result := MergeLocal(global, local)

	if result.Clone.Mode != "regular" {
		t.Errorf("clone.mode = %q, want regular", result.Clone.Mode)
	}
}

func TestMergeLocal_HooksMergeByName(t *testing.T) {
	t.Parallel()

	global := &Config{
		Hooks: HooksConfig{
			Hooks: map[string]Hook{
				"code":  {Command: "code {worktree-dir}", On: []string{"checkout"}},
				"setup": {Command: "npm install", On: []string{"checkout"}},
				"lint":  {Command: "npm run lint"},
			},
		},
		Checkout: CheckoutConfig{WorktreeFormat: DefaultWorktreeFormat},
		Forge:    ForgeConfig{Default: "github"},
	}

	local := &LocalConfig{
		Hooks: HooksConfig{
			Hooks: map[string]Hook{
				// Override setup with different command
				"setup": {Command: "go mod download", Description: "Go deps"},
				// Disable lint
				"lint": {Enabled: new(false)},
				// Add new hook
				"test": {Command: "go test ./...", On: []string{"checkout"}},
			},
		},
	}

	result := MergeLocal(global, local)

	// code: unchanged from global
	if result.Hooks.Hooks["code"].Command != "code {worktree-dir}" {
		t.Error("code hook should be unchanged")
	}

	// setup: overridden by local
	if result.Hooks.Hooks["setup"].Command != "go mod download" {
		t.Errorf("setup = %q, want go mod download", result.Hooks.Hooks["setup"].Command)
	}

	// lint: disabled
	if _, exists := result.Hooks.Hooks["lint"]; exists {
		t.Error("lint hook should be removed (disabled)")
	}

	// test: added by local
	if result.Hooks.Hooks["test"].Command != "go test ./..." {
		t.Errorf("test = %q, want go test ./...", result.Hooks.Hooks["test"].Command)
	}

	// Verify global hooks weren't mutated
	if _, exists := global.Hooks.Hooks["lint"]; !exists {
		t.Error("global lint hook should still exist")
	}
	if global.Hooks.Hooks["setup"].Command != "npm install" {
		t.Error("global setup hook should be unchanged")
	}
}

func TestMergeLocal_PreserveAppendDedup(t *testing.T) {
	t.Parallel()

	global := &Config{
		Preserve: PreserveConfig{
			Paths: []string{".env", ".envrc"},
		},
		Checkout: CheckoutConfig{WorktreeFormat: DefaultWorktreeFormat},
		Forge:    ForgeConfig{Default: "github"},
		Hooks:    HooksConfig{Hooks: map[string]Hook{}},
	}

	local := &LocalConfig{
		Preserve: PreserveConfig{
			Paths: []string{".envrc", ".env.local"}, // .envrc is a dup
		},
	}

	result := MergeLocal(global, local)

	// Paths: .env, .envrc, .env.local (deduped)
	expectedPaths := []string{".env", ".envrc", ".env.local"}
	if len(result.Preserve.Paths) != len(expectedPaths) {
		t.Fatalf("paths = %v, want %v", result.Preserve.Paths, expectedPaths)
	}
	for i, p := range expectedPaths {
		if result.Preserve.Paths[i] != p {
			t.Errorf("paths[%d] = %q, want %q", i, result.Preserve.Paths[i], p)
		}
	}

	// Verify global wasn't mutated
	if len(global.Preserve.Paths) != 2 {
		t.Error("global paths should be unchanged")
	}
}

func TestAppendUnique(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     []string
		extra    []string
		expected []string
	}{
		{"both empty", nil, nil, nil},
		{"base only", []string{"a", "b"}, nil, []string{"a", "b"}},
		{"extra only", nil, []string{"a", "b"}, []string{"a", "b"}},
		{"no overlap", []string{"a"}, []string{"b"}, []string{"a", "b"}},
		{"full overlap", []string{"a", "b"}, []string{"a", "b"}, []string{"a", "b"}},
		{"partial overlap", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := appendUnique(tt.base, tt.extra)
			if len(result) != len(tt.expected) {
				t.Fatalf("got %v, want %v", result, tt.expected)
			}
			for i, v := range tt.expected {
				if result[i] != v {
					t.Errorf("[%d] = %q, want %q", i, result[i], v)
				}
			}
		})
	}
}

func TestHookIsEnabled(t *testing.T) {
	t.Parallel()

	t.Run("nil defaults to true", func(t *testing.T) {
		h := Hook{Command: "test"}
		if !h.IsEnabled() {
			t.Error("nil Enabled should return true")
		}
	})

	t.Run("explicit true", func(t *testing.T) {
		h := Hook{Command: "test", Enabled: new(true)}
		if !h.IsEnabled() {
			t.Error("explicit true should return true")
		}
	})

	t.Run("explicit false", func(t *testing.T) {
		h := Hook{Command: "test", Enabled: new(false)}
		if h.IsEnabled() {
			t.Error("explicit false should return false")
		}
	})
}

# Interactive Hooks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `interactive = true` hook config option that gives TUI programs (like `claude`) full TTY control via foreground process groups, with cross-platform shell abstraction.

**Architecture:** Platform-specific shell invocation and interactive process setup isolated behind build tags (`shell_unix.go`, `shell_windows.go`). Hook execution partitions matches into non-interactive (run first) and interactive (run last). Config validation rejects `interactive = true` on before hooks.

**Tech Stack:** Go, `syscall.SysProcAttr` (Unix), `os/exec`, build tags

**Spec:** `docs/superpowers/specs/2026-03-27-interactive-hooks-design.md`

---

### Task 1: Platform shell abstraction (build-tagged files)

**Files:**
- Create: `internal/hooks/shell_unix.go`
- Create: `internal/hooks/shell_windows.go`
- Modify: `internal/hooks/hooks.go:161-166`

- [ ] **Step 1: Create `shell_unix.go`**

```go
//go:build !windows

package hooks

import (
	"os"
	"os/exec"
	"syscall"
)

// shellCommand returns the shell and arguments for running a command string.
func shellCommand(command string) (string, []string) {
	return "sh", []string{"-c", command}
}

// setInteractive configures the command to run as the foreground process group,
// giving it full TTY control. This allows TUI programs like editors and
// interactive CLIs to work correctly as hook subprocesses.
func setInteractive(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Foreground: true,
		Ctty:       int(os.Stdin.Fd()),
	}
}
```

- [ ] **Step 2: Create `shell_windows.go`**

```go
//go:build windows

package hooks

import "os/exec"

// shellCommand returns the shell and arguments for running a command string.
func shellCommand(command string) (string, []string) {
	return "cmd", []string{"/c", command}
}

// setInteractive is a no-op on Windows. Windows doesn't have Unix process
// groups or SIGTTOU/SIGTTIN signals, so regular subprocess execution with
// stdio wiring is sufficient for interactive programs.
func setInteractive(cmd *exec.Cmd) {}
```

- [ ] **Step 3: Update `runHook` to use platform functions**

In `internal/hooks/hooks.go`, replace lines 161-166:

```go
// Before:
shellCmd := exec.Command("sh", "-c", cmd)
shellCmd.Dir = workDir
shellCmd.Stdin = os.Stdin

shellCmd.Stdout = os.Stdout
shellCmd.Stderr = os.Stderr

// After:
shell, args := shellCommand(cmd)
shellCmd := exec.Command(shell, args...)
shellCmd.Dir = workDir
shellCmd.Stdin = os.Stdin
shellCmd.Stdout = os.Stdout
shellCmd.Stderr = os.Stderr

if hook.Interactive {
	setInteractive(shellCmd)
}
```

- [ ] **Step 4: Remove unused `os/exec` import if needed**

The `exec` package is still used in `runHook`, so it stays. But verify the `"os/exec"` import is still referenced — `shellCommand` returns strings, `exec.Command` is called in `runHook`. No change needed.

- [ ] **Step 5: Build and verify**

Run: `go build ./cmd/wt`
Expected: Clean build with no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/hooks/shell_unix.go internal/hooks/shell_windows.go internal/hooks/hooks.go
git commit -m "feat: add cross-platform shell abstraction and interactive hook support"
```

---

### Task 2: Config — add `Interactive` field and validation

**Files:**
- Modify: `internal/config/config.go:50-56` (Hook struct)
- Modify: `internal/config/validate.go:39-49` (ValidateHookTriggers)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for validation**

Add to `internal/config/config_test.go` after the existing `TestValidateHookTriggers` function:

```go
func TestValidateHookTriggers_InteractiveBeforeHook(t *testing.T) {
	t.Parallel()

	hooks := map[string]Hook{
		"guard": {
			Command:     "claude",
			On:          []string{"before:checkout"},
			Interactive: true,
		},
	}

	err := ValidateHookTriggers(hooks)
	if err == nil {
		t.Fatal("expected error for interactive before hook")
	}
	if !strings.Contains(err.Error(), "interactive") {
		t.Errorf("error should mention 'interactive', got: %v", err)
	}
}

func TestValidateHookTriggers_InteractiveAfterHook(t *testing.T) {
	t.Parallel()

	hooks := map[string]Hook{
		"claude": {
			Command:     "claude",
			On:          []string{"checkout"},
			Interactive: true,
		},
	}

	if err := ValidateHookTriggers(hooks); err != nil {
		t.Errorf("interactive after hook should be valid, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -run TestValidateHookTriggers_Interactive -v`
Expected: `TestValidateHookTriggers_InteractiveBeforeHook` fails (no `Interactive` field yet).

- [ ] **Step 3: Add `Interactive` field to Hook struct**

In `internal/config/config.go`, update the `Hook` struct:

```go
// Hook defines a post-create hook
type Hook struct {
	Command     string   `toml:"command"`
	Description string   `toml:"description"`
	On          []string `toml:"on"`          // commands this hook runs on (empty = only via --hook)
	Enabled     *bool    `toml:"enabled"`     // nil = true (default); false disables a global hook locally
	Interactive bool     `toml:"interactive"` // run with foreground process group for TUI programs
}
```

- [ ] **Step 4: Add validation in `ValidateHookTriggers`**

In `internal/config/validate.go`, update `ValidateHookTriggers`:

```go
// ValidateHookTriggers validates all "on" values in hook config.
func ValidateHookTriggers(hooksMap map[string]Hook) error {
	for name, hook := range hooksMap {
		for _, on := range hook.On {
			parsed, err := hooktrigger.ParseTrigger(on)
			if err != nil {
				return fmt.Errorf("invalid hook trigger %q in hook %q: %w", on, name, err)
			}
			if hook.Interactive && parsed.Phase == "before" {
				return fmt.Errorf("hook %q: interactive hooks cannot use before triggers (%s)", name, on)
			}
		}
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/config/ -run TestValidateHookTriggers -v`
Expected: All pass including the two new tests.

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/validate.go internal/config/config_test.go
git commit -m "feat: add interactive field to Hook config with before-hook validation"
```

---

### Task 3: Hook execution ordering — non-interactive before interactive

**Files:**
- Modify: `internal/hooks/hooks.go` (RunAllNonFatal, RunForEach)
- Test: `internal/hooks/hooks_test.go`

- [ ] **Step 1: Write failing test for ordering**

Add to `internal/hooks/hooks_test.go`:

```go
func TestPartitionHooks(t *testing.T) {
	t.Parallel()

	matches := []HookMatch{
		{Name: "interactive1", Hook: &config.Hook{Command: "claude", Interactive: true}},
		{Name: "regular1", Hook: &config.Hook{Command: "echo a"}},
		{Name: "interactive2", Hook: &config.Hook{Command: "vim", Interactive: true}},
		{Name: "regular2", Hook: &config.Hook{Command: "echo b"}},
	}

	nonInteractive, interactive := partitionHooks(matches)

	if len(nonInteractive) != 2 {
		t.Fatalf("expected 2 non-interactive hooks, got %d", len(nonInteractive))
	}
	if len(interactive) != 2 {
		t.Fatalf("expected 2 interactive hooks, got %d", len(interactive))
	}

	if nonInteractive[0].Name != "regular1" || nonInteractive[1].Name != "regular2" {
		t.Errorf("non-interactive hooks should be regular1, regular2; got %s, %s", nonInteractive[0].Name, nonInteractive[1].Name)
	}
	if interactive[0].Name != "interactive1" || interactive[1].Name != "interactive2" {
		t.Errorf("interactive hooks should be interactive1, interactive2; got %s, %s", interactive[0].Name, interactive[1].Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hooks/ -run TestPartitionHooks -v`
Expected: FAIL — `partitionHooks` not defined.

- [ ] **Step 3: Implement `partitionHooks`**

Add to `internal/hooks/hooks.go` before `RunAllNonFatal`:

```go
// partitionHooks splits matches into non-interactive and interactive hooks.
// Non-interactive hooks run first, interactive hooks run last.
func partitionHooks(matches []HookMatch) (nonInteractive, interactive []HookMatch) {
	for _, m := range matches {
		if m.Hook.Interactive {
			interactive = append(interactive, m)
		} else {
			nonInteractive = append(nonInteractive, m)
		}
	}
	return
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/hooks/ -run TestPartitionHooks -v`
Expected: PASS.

- [ ] **Step 5: Update `RunAllNonFatal` to use partitioning**

```go
// RunAllNonFatal runs all matched hooks, logging failures as warnings instead of returning errors.
// Non-interactive hooks run first, interactive hooks run last.
// Prints "No hooks matched" if matches is empty.
func RunAllNonFatal(goCtx context.Context, matches []HookMatch, ctx Context, workDir string) {
	l := log.FromContext(goCtx)

	if len(matches) == 0 {
		l.Printf("No hooks matched\n")
		return
	}

	nonInteractive, interactive := partitionHooks(matches)

	for _, match := range nonInteractive {
		if err := runHook(goCtx, match.Name, match.Hook, ctx, workDir); err != nil {
			l.Printf("Warning: hook %q failed: %v\n", match.Name, err)
		}
	}

	for _, match := range interactive {
		if err := runHook(goCtx, match.Name, match.Hook, ctx, workDir); err != nil {
			l.Printf("Warning: hook %q failed: %v\n", match.Name, err)
		}
	}
}
```

- [ ] **Step 6: Update `RunForEach` to use partitioning**

```go
// RunForEach runs all matched hooks for a single item (e.g., one worktree in a batch).
// Non-interactive hooks run first, interactive hooks run last.
// Logs failures as warnings with branch context. Does NOT print "no hooks matched".
func RunForEach(goCtx context.Context, matches []HookMatch, ctx Context, workDir string) {
	l := log.FromContext(goCtx)

	nonInteractive, interactive := partitionHooks(matches)

	for _, match := range nonInteractive {
		if err := runHook(goCtx, match.Name, match.Hook, ctx, workDir); err != nil {
			l.Printf("Warning: hook %q failed for %s: %v\n", match.Name, ctx.Branch, err)
		}
	}

	for _, match := range interactive {
		if err := runHook(goCtx, match.Name, match.Hook, ctx, workDir); err != nil {
			l.Printf("Warning: hook %q failed for %s: %v\n", match.Name, ctx.Branch, err)
		}
	}
}
```

Note: `RunBeforeHooks` does NOT need partitioning because `interactive = true` on before hooks is rejected at config validation time. `RunSingle` (used by `wt hook`) runs a single hook so partitioning doesn't apply.

- [ ] **Step 7: Run all hook unit tests**

Run: `go test ./internal/hooks/ -v`
Expected: All pass.

- [ ] **Step 8: Commit**

```bash
git add internal/hooks/hooks.go internal/hooks/hooks_test.go
git commit -m "feat: partition hooks to run non-interactive before interactive"
```

---

### Task 4: Integration tests

**Files:**
- Modify: `cmd/wt/checkout_integration_test.go`

- [ ] **Step 1: Add integration test — interactive hook runs after non-interactive**

Add to `cmd/wt/checkout_integration_test.go`:

```go
// TestCheckout_InteractiveHookRunsLast tests that non-interactive hooks run before interactive ones.
//
// Scenario: Two hooks match checkout — one regular, one interactive
// Expected: Regular hook runs first (creates marker 1), interactive runs second (creates marker 2)
func TestCheckout_InteractiveHookRunsLast(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "test-repo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	os.MkdirAll(filepath.Dir(regFile), 0755)

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	orderFile := filepath.Join(tmpDir, "order.txt")

	cfg := &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
			BaseRef:        "local",
		},
		Hooks: config.HooksConfig{
			Hooks: map[string]config.Hook{
				"interactive-hook": {
					Command:     "echo interactive >> " + orderFile,
					On:          []string{"checkout"},
					Interactive: true,
				},
				"regular-hook": {
					Command: "echo regular >> " + orderFile,
					On:      []string{"checkout"},
				},
			},
		},
	}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"-b", "feature"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout command failed: %v", err)
	}

	content, err := os.ReadFile(orderFile)
	if err != nil {
		t.Fatalf("failed to read order file: %v", err)
	}

	lines := strings.TrimSpace(string(content))
	if lines != "regular\ninteractive" {
		t.Errorf("expected regular before interactive, got:\n%s", lines)
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `go test -tags integration -run TestCheckout_InteractiveHookRunsLast ./cmd/wt/ -v -timeout 60s`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cmd/wt/checkout_integration_test.go
git commit -m "test: add integration test for interactive hook ordering"
```

---

### Task 5: Documentation

**Files:**
- Modify: `README.md` (~line 117-138, hooks quick-start section)
- Modify: `README.md` (~line 592, Writing Hooks section)
- Modify: `internal/config/config.go` (default config template)

- [ ] **Step 1: Update README quick-start hooks example**

Update the claude-review hook example around line 131 to show `interactive`:

```toml
# Run Claude to review PR when checking out a PR
[hooks.claude-review]
command = "claude -p 'review this PR'"
on = ["checkout:pr"]
interactive = true
```

- [ ] **Step 2: Add Interactive Hooks section to Writing Hooks**

Add after the "Hook Working Directory" section (after the `cd` note), before "Quoting Placeholders":

```markdown
### Interactive Hooks

Hooks that run TUI programs (editors, `claude`, interactive CLIs) need full terminal control. Mark them with `interactive = true`:

` ` `toml
[hooks.claude]
command = "claude"
on = ["checkout"]
interactive = true
` ` `

Interactive hooks:
- Get the foreground process group (full TTY control for raw mode, signals, rendering)
- Run **after** all non-interactive hooks for the same trigger
- Cannot be used with `before:` triggers (before hooks must return a status)
- When the interactive program exits, `wt` resumes normally

Without `interactive = true`, TUI programs may hang because they lack foreground process group access.
```

- [ ] **Step 3: Update default config template**

In `internal/config/config.go`, update the Claude hook examples in the default config template to include `interactive = true`:

```toml
# Claude Code - start Claude in the worktree (interactive)
# [hooks.claude]
# command = "claude"
# description = "Start Claude Code session"
# interactive = true
```

- [ ] **Step 4: Commit**

```bash
git add README.md internal/config/config.go
git commit -m "docs: document interactive hooks option"
```

---

### Task 6: Full verification

- [ ] **Step 1: Run unit tests**

Run: `go test ./...`
Expected: All pass.

- [ ] **Step 2: Run integration tests**

Run: `go test -tags integration ./cmd/wt/ -timeout 300s`
Expected: All pass.

- [ ] **Step 3: Build and install**

Run: `just install`
Expected: Clean install.

- [ ] **Step 4: Manual smoke test**

Add to `~/.wt/config.toml`:

```toml
[hooks.test-interactive]
command = "claude -p 'say hello and exit'"
interactive = true
```

Run: `wt hook test-interactive`
Expected: Claude starts with full TTY, runs prompt, exits, wt finishes cleanly.

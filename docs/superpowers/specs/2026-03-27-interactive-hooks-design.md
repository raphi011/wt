# Interactive Hooks

## Problem

Hooks that run interactive/TUI programs (e.g., `claude`) hang when executed as subprocesses of `wt`. The child process is not in the terminal's foreground process group, so TUI applications that need full TTY control (raw mode, signal handling) fail silently.

Non-interactive hooks (`npm install`, `echo`, `touch`) work fine as regular subprocesses.

## Design

### Config

New `interactive` boolean field on hooks:

```toml
[hooks.npm]
command = "npm install"
on = ["checkout"]

[hooks.claude-review]
command = "claude -p /pr-review-toolkit:review-pr"
on = ["checkout:pr"]
interactive = true
```

- Defaults to `false`
- Before hooks (`before:*`) cannot be `interactive` — validated at config load time. Before hooks must return a status to wt to decide whether to abort the operation.
- Multiple interactive hooks on the same trigger are allowed and run sequentially.

### Execution Behavior

When hooks are selected for a trigger:

1. Run all **non-interactive** hooks first (current subprocess behavior)
2. Run all **interactive** hooks last, sequentially, each with foreground process group
3. After each interactive hook exits, wt regains terminal control and continues

Interactive hooks get the foreground process group, giving them full TTY control (raw mode, SIGWINCH, etc.). wt stays alive as the parent and resumes after the hook exits.

### Platform Abstraction

New files in `internal/hooks/`:

**`shell_unix.go`** (`//go:build !windows`):

```go
func shellCommand(command string) (string, []string) {
    return "sh", []string{"-c", command}
}

func setInteractive(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Foreground: true,
        Ctty:       int(os.Stdin.Fd()),
    }
}
```

**`shell_windows.go`** (`//go:build windows`):

```go
func shellCommand(command string) (string, []string) {
    return "cmd", []string{"/c", command}
}

func setInteractive(cmd *exec.Cmd) {
    // No-op on Windows. Windows doesn't have Unix process groups
    // or SIGTTOU/SIGTTIN, so regular subprocess with stdio wiring
    // is sufficient for interactive programs.
}
```

This also makes the existing `sh -c` invocation cross-platform.

### Changes to `runHook`

`runHook` in `hooks.go` uses the platform functions instead of hardcoding `sh -c`:

```go
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

### Changes to Hook Selection / Running

Callers that run multiple hooks need to partition and order them:

- `RunAllNonFatal` / `RunForEach` / `RunBeforeHooks`: partition matches into non-interactive and interactive, run non-interactive first, then interactive
- Or: new `RunWithInteractive(ctx, matches, hookCtx, workDir)` function that handles the partitioning and ordering

### Validation

At config load time:
- Before hooks with `interactive = true` produce a validation error
- Specifically: any hook where `on` contains a `before:*` trigger and `interactive = true` is invalid

### Config Struct Change

```go
type Hook struct {
    Command     string   `toml:"command"`
    Description string   `toml:"description"`
    On          []string `toml:"on"`
    Enabled     *bool    `toml:"enabled"`
    Interactive bool     `toml:"interactive"` // NEW
}
```

## Files to Modify

- `internal/config/config.go` — add `Interactive` field to `Hook`, add validation
- `internal/hooks/hooks.go` — update `runHook` to use platform functions and `setInteractive`, update run functions for ordering
- `internal/hooks/shell_unix.go` — new file, shell command and interactive setup for Unix
- `internal/hooks/shell_windows.go` — new file, shell command and no-op interactive for Windows
- `internal/hooks/hooks_test.go` — unit tests for partitioning and ordering
- `cmd/wt/checkout_integration_test.go` — integration test for interactive hook execution
- `README.md` — document `interactive` option
- `internal/config/config.go` — update default config template with interactive example

## Testing

- Unit: `shellCommand` returns correct shell per platform
- Unit: `setInteractive` sets `SysProcAttr` on Unix (build-tag test)
- Unit: hook partitioning — non-interactive before interactive
- Unit: before hook + `interactive = true` produces validation error
- Integration: interactive hook runs a program that needs TTY and exits cleanly
- Integration: non-interactive hooks run before interactive hook when both match

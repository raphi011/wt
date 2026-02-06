# CLI Best Practices Audit

This document identifies CLI best practices that `wt` could adopt to improve scripting compatibility, user experience, and standards compliance.

## References

- [Command Line Interface Guidelines](https://clig.dev/)
- [NO_COLOR Standard](https://no-color.org/)
- [12 Factor CLI Apps](https://medium.com/@jdxcode/12-factor-cli-apps-dd3c227a0e46)

---

## Critical Issues

### 1. NO_COLOR Environment Variable Support

**Current:** Colors always applied regardless of environment.

**Best Practice:** Respect the `NO_COLOR` standard:

```go
if os.Getenv("NO_COLOR") != "" {
    // Disable all colors
}
```

**Impact:** Breaks CI/CD pipelines, log aggregation, and accessibility preferences.

---

### 2. TTY Detection

**Current:** Colors output even when stdout is piped or redirected.

**Best Practice:** Only apply colors when output is a terminal:

```go
import "github.com/mattn/go-isatty"

if isatty.IsTerminal(os.Stdout.Fd()) {
    // Apply colors
}
```

**Impact:** ANSI codes in piped output break parsing and automation.

---

### 3. Signal Handling (SIGINT/SIGTERM)

**Current:** No signal handling; long operations can't be gracefully interrupted.

**Best Practice:** Handle interrupts to clean up properly:

```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
```

**Impact:** Risk of orphaned processes or incomplete state when killed mid-operation.

---

### 4. Semantic Exit Codes

**Current:** Only uses 0 (success) and 1 (all errors).

**Best Practice:** Use semantic exit codes per sysexits.h:

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 64 | Usage error (invalid arguments) |
| 65 | Data error (invalid input) |
| 69 | Unavailable (service/resource missing) |
| 70 | Internal software error |

**Impact:** Scripts can't distinguish recoverable errors from user mistakes.

---

### 5. Default Command Exit Code

**Current:** Running `wt` with no arguments shows help and exits 1.

**Best Practice:** Exit 0 when showing help (user requested information, not an error).

```go
// cmd/wt/main.go ~line 118
default:
    p.WriteHelp(os.Stdout)
    os.Exit(0)  // Not an error
```

---

## High Impact Improvements

### 6. Verbosity Flags

**Missing:** `--quiet`, `--verbose`, `--debug` flags.

```go
type Args struct {
    Quiet   bool `arg:"-q,--quiet" help:"suppress non-error output"`
    Verbose bool `arg:"-v,--verbose" help:"show detailed output"`
}
```

**Use cases:**
- `--quiet` for scripts that only care about exit codes
- `--verbose` for debugging workflows

---

### 7. Consistent JSON Output

**Current:** Only `list` and `config hooks` support `--json`.

**Needed:** Add `--json` to all commands that produce structured output:

| Command | Status |
|---------|--------|
| `list` | Supported |
| `config hooks` | Supported |
| `create` | Missing |
| `open` | Missing |
| `tidy` | Missing |
| `pr open` | Missing |
| `mv` | Missing |

Example output for `wt create --json`:
```json
{"path": "/home/user/repo-feature", "branch": "feature", "created": true}
```

---

### 8. Consistent `--dry-run` Flag

**Current:** Only `tidy` and `mv` support `--dry-run`.

**Needed:** Add to all write operations:

| Command | Status |
|---------|--------|
| `tidy` | Supported |
| `mv` | Supported |
| `create` | Missing |
| `open` | Missing |
| `pr open` | Missing |

---

### 9. Additional Environment Variables

**Current:** Only `WT_DEFAULT_PATH` is supported.

**Suggested additions:**

| Variable | Purpose |
|----------|---------|
| `NO_COLOR` | Disable colors (standard) |
| `PAGER` | Custom pager for long output |
| `WT_DEBUG` | Enable debug output |
| `WT_CONFIG` | Override config file location |

---

## Medium Impact Improvements

### 10. Pager Support

**Current:** Long tables print directly to terminal.

**Best Practice:** Use pager for large outputs:

```go
pager := os.Getenv("PAGER")
if pager == "" {
    pager = "less -R"
}
```

---

### 11. Progress Indicators for All Long Operations

**Current:** Only `tidy` shows a spinner.

**Needed:** Add progress feedback to:
- `create` (while creating worktree)
- `pr open` (while fetching/cloning)
- `mv` (during file operations)

---

### 12. Stderr for Informational Messages

**Current:** Status messages go to stdout, mixing with data.

**Best Practice:**
- stdout = data (for piping)
- stderr = status messages, progress, errors

```bash
# Should work cleanly:
wt list --json | jq '.[] | .branch'
```

---

### 13. Dependency Check Command

**Missing:** No way to verify required tools are available.

**Suggested:**
```bash
$ wt doctor
git:  found (2.43.0)
gh:   found (2.40.0)
glab: not found (optional, needed for GitLab)
```

---

## Implementation Priority

### Phase 1: Quick Wins
- [ ] NO_COLOR support
- [ ] TTY detection for colors
- [ ] Fix default exit code (0 for help)
- [ ] Add `--quiet` flag

### Phase 2: Scripting Support
- [ ] Semantic exit codes
- [ ] JSON output for all commands
- [ ] Consistent `--dry-run`
- [ ] Stderr for info messages

### Phase 3: Polish
- [ ] Signal handling
- [ ] Pager support
- [ ] Progress indicators everywhere
- [ ] `wt doctor` command
- [ ] Additional env vars

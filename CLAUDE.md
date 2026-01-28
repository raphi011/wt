# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`wt` is a Go CLI tool for managing git worktrees with GitHub/GitLab MR integration.

## Build & Development

```bash
# Build
make build          # Creates ./wt binary
go build ./cmd/wt   # Same as above

# Install to ~/go/bin
make install
go install ./cmd/wt

# Run tests
make test
go test ./...

# Clean
make clean
```

## Architecture

### Package Structure

```
cmd/wt/main.go           - CLI entry point with kong subcommands
internal/git/            - Git operations via exec.Command
  ├── worktree.go        - Create/list/remove worktrees
  └── repo.go            - Branch info, merge status, diff stats
internal/forge/          - Git hosting service abstraction (GitHub/GitLab)
  ├── forge.go           - Forge interface and MR cache
  ├── detect.go          - Auto-detect forge from remote URL
  ├── github.go          - GitHub implementation (gh CLI)
  └── gitlab.go          - GitLab implementation (glab CLI)
internal/resolve/        - Target resolution for commands
  └── resolve.go         - ByID resolver (numeric ID lookup)
internal/log/            - Context-aware logging (stderr)
  └── log.go             - Logger with verbose mode
internal/output/         - Context-aware output (stdout)
  └── output.go          - Printer for primary data output
internal/ui/             - Terminal UI components
  ├── table.go           - Lipgloss table formatting with colors
  ├── spinner.go         - Bubbletea spinner
  └── interactive.go     - Interactive prompts (confirm, text input, list select)
```

### Key Design Decisions

**Shell out to git/gh/glab CLI** - All git and forge operations use `os/exec.Command` to call CLI tools directly. This is simpler and more reliable than Go libraries. Changes are isolated to `internal/git/` and `internal/forge/`.

**Forge abstraction** - The `internal/forge/` package provides a common interface for GitHub and GitLab. Platform is auto-detected from the remote URL. Both `gh` (GitHub) and `glab` (GitLab) CLIs are supported.

**Worktree naming convention** - Worktrees are created as `<repo-name>-<branch>` (e.g., `wt-feature-branch`). The repo name is extracted from git origin URL.

**Path handling** - Directory configuration is done via config file or environment variables (no `-d` flag). The tool fails if the directory doesn't exist (no automatic mkdir). Configuration sources (highest priority first):
- `WT_WORKTREE_DIR` env var - target directory for worktrees
- `WT_REPO_DIR` env var - directory to scan for repos
- `worktree_dir` in config file
- `repo_dir` in config file
- Falls back to current directory if unset

**MR/PR status** - Uses `gh pr list` or `glab mr list` to fetch merge request info (auto-detected). States: merged, open, closed.

### Dependencies

- **CLI parsing**: `github.com/alecthomas/kong` - Struct-based arg parsing with subcommands and auto-dispatch
- **UI**: `github.com/charmbracelet/lipgloss` - Terminal styling
- **UI**: `github.com/charmbracelet/lipgloss/table` - Table component
- **UI**: `github.com/charmbracelet/bubbles/spinner` - Spinner component
- **External**: Requires `git` in PATH; `gh` CLI for GitHub repos, `glab` CLI for GitLab repos

### Development Guidelines

**Target Resolution Pattern** - Commands that operate on worktrees use `--number` (`-n`) flag with `internal/resolve.ByID()`:

- **ID or repo/label**: `wt exec`, `wt hook` - require `-n <id>`, or `-r <repo>`/`-l <label>` (exec/hook support multiple); these flags are mutually exclusive
- **ID or repo or label**: `wt cd` - require `-n <id>`, `-r <repo>`, or `-l <label>` (mutually exclusive)
- **Optional ID**: `wt note`, `wt pr create`, `wt pr merge`, `wt prune` - when inside worktree, defaults to current branch; outside requires `-n` (prune supports multiple)
- **Special case**: `wt checkout` - inside repo uses branch name; outside repo requires `-r <repo>` or `-l <label>` to specify target repos; use `-i` for interactive mode

Commands using this pattern: `wt exec`, `wt cd`, `wt note set/get/clear`, `wt hook`, `wt pr create`, `wt pr merge`, `wt prune`

**Keep completions in sync** - **IMPORTANT**: When adding or modifying CLI flags, you MUST update the shell completion scripts in `cmd/wt/completions.go`. This file contains completions for fish, bash, and zsh. Search for existing flags of the same command to find where to add the new flag in each shell format.

**Reuse flags consistently** - When adding flags that serve the same purpose across commands, use identical names/shortcuts. Standard flags:
- `-n, --number` - worktree number for targeting
- `-i, --interactive` - interactive mode (wt checkout)
- `-r, --repository` - repository name for targeting (wt checkout, list, exec, cd, hook)
- `-l, --label` - target repos by label (wt checkout, list, exec, hook)
- `-d, --dry-run` - preview without making changes
- `-f, --force` - force operation (override safety checks)
- `-c, --include-clean` - include clean worktrees (0 commits, no changes)
- `-g, --global` - operate on all repos (not just current)
- `-a, --arg` - set hook variable KEY=VALUE (repeatable)
- `--json` - output as JSON
- `--hook` / `--no-hook` - control hook execution (for checkout, pr checkout, prune)

**Never modify git internal files directly** - Always use git CLI commands via `exec.Command`. Never read/write `.git/` directory contents, `.git` files in worktrees, or git refs directly. Use `git worktree repair` for fixing broken links, `git worktree prune` for cleanup.

**Interactive Mode (`-i` flag)** - When implementing interactive wizard mode for commands:

1. **Respect explicit CLI arguments** - If a flag is passed explicitly (e.g., `--hook`, `--no-hook`, `-r`), skip that wizard step entirely. Don't allow the user to change values that were explicitly set.

2. **Show all values in summary** - The final summary should display both wizard-selected and CLI-provided values. Don't distinguish between them visually.

3. **Skip irrelevant steps** - Skip steps when there are no options available (e.g., no hooks configured) or when the step doesn't apply based on previous selections.

4. **Pre-select sensible defaults** - Pre-select "default" options (e.g., hooks with `on=["checkout"]`), pre-select current repo when inside a git repo.

5. **Handle empty selections** - If a multi-select step has no selections, translate to the appropriate flag (e.g., no hooks selected → `--no-hook`).

6. **File structure** - Create wizard UI in `internal/ui/<command>_wizard.go`. Define `<Command>Options` struct for wizard output and `<Command>WizardParams` struct for wizard input. The wizard returns options which the command applies to its flags.

**Forge Feature Parity** - Any feature that involves forge operations (PRs, cloning, etc.) MUST support both GitHub and GitLab. Always:
- Add methods to the `Forge` interface first
- Implement in both `github.go` and `gitlab.go`
- Handle platform-specific limitations explicitly (e.g., GitLab doesn't support rebase merge via CLI)
- Never call `gh` or `glab` directly outside `internal/forge/`

**Branch Workflow** - All changes must be made in a feature branch and merged through a PR. Never commit directly to main.

**Pre-v1.0 Breaking Changes** - Until v1.0, backwards incompatible changes to CLI commands, flags, config format, and cache files are allowed without migration code. Users are expected to update their config manually (see README disclaimer).

### Dependency Injection Pattern

**Deps struct** - Stable configuration embedded in command structs:
```go
type Deps struct {
    Config  *config.Config `kong:"-"`
    WorkDir string         `kong:"-"`
}

type CheckoutCmd struct {
    Deps  // embedded
    Branch string `arg:""`
}
```

**context.Context** - Request-scoped values passed to functions:
- `log.FromContext(ctx)` → Logger (writes to stderr)
- `output.FromContext(ctx)` → Printer (writes to stdout)

**Convention**: Always name the logger variable `l`:
```go
func (c *CheckoutCmd) runCheckout(ctx context.Context) error {
    l := log.FromContext(ctx)
    out := output.FromContext(ctx)
    l.Debug("creating worktree")
    out.Println(result)
}
```

**stdout vs stderr**:
- stdout: Primary output (data, tables, paths, JSON)
- stderr: Diagnostics (logs, progress, errors)

This allows piping: `cd $(wt cd -n 1)` works because logs go to stderr.

### Integration Tests

Integration tests are in `cmd/wt/*_integration_test.go` with build tag `//go:build integration`.

**Dependency Injection in Tests** - Tests inject dependencies using the `Deps` struct and create a context with `testContext(t)` or `testContextWithOutput(t)`:
```go
func runCheckoutCommand(t *testing.T, workDir string, cfg *config.Config, cmd *CheckoutCmd) error {
    cmd.Deps = Deps{Config: cfg, WorkDir: workDir}
    ctx := testContext(t)
    return cmd.runCheckout(ctx)
}
```

**macOS Symlink Resolution** - On macOS, `t.TempDir()` returns paths like `/var/folders/...` but git commands may return `/private/var/folders/...` (the resolved symlink). Always use `resolvePath(t, t.TempDir())` helper to resolve symlinks before path comparisons. This helper is defined in `integration_test_helpers.go`.

**All integration tests use `t.Parallel()`** for concurrent execution.

### Commit Messages

Follow **Conventional Commits** for GoReleaser changelog grouping:

| Type | Changelog Group | Example |
|------|-----------------|---------|
| `feat:` | ✨ Features | `feat: add list command` |
| `fix:` | 🐛 Bug Fixes | `fix: crash on empty dir` |
| `docs:` | 📚 Documentation | `docs: update readme` |
| `chore:` | (excluded) | `chore: update deps` |
| `test:` | (excluded) | `test: add unit tests` |

Format: `type(scope)!: description` - scope optional, `!` for breaking changes.

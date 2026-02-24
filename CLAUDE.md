# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`wt` is a Go CLI tool for managing git worktrees with GitHub/GitLab MR integration.

## Build & Development

```bash
# Build
just build          # Creates ./wt binary
go build ./cmd/wt   # Same as above

# Install to ~/go/bin (+ shell completions + git hooks)
just install
go install ./cmd/wt

# Run tests
just test                # Unit tests only
just test-integration    # Integration tests (needs WT_TEST_GITHUB_REPO)
go test ./...

# Other
just clean               # Remove built binary
just snapshot            # GoReleaser snapshot build
just testdoc             # Generate docs/TESTS.md from test names
```

## Architecture

### Package Structure

```
cmd/wt/                  - CLI commands and entry point (cobra)
  ├── main.go            - Entry point
  ├── root.go            - Root command setup and context injection
  ├── targets.go         - Target resolution (scope:branch parsing, repo/label matching)
  └── *_cmd.go           - Individual command implementations
internal/cmd/            - Command execution helpers (exec.Command wrappers)
internal/config/         - Configuration loading (TOML), local config, resolver
internal/forge/          - Git hosting service abstraction (GitHub/GitLab)
  ├── forge.go           - Forge interface
  ├── detect.go          - Auto-detect forge from remote URL
  ├── github.go          - GitHub implementation (gh CLI)
  └── gitlab.go          - GitLab implementation (glab CLI)
internal/git/            - Git operations via exec.Command
  ├── worktree.go        - Create/list/remove worktrees
  ├── repo.go            - Branch info, merge status, diff stats
  ├── load.go            - Parallel worktree data loading
  ├── check.go           - Git state checks
  ├── migrate.go         - Worktree migration helpers
  ├── notes.go           - Git notes operations
  ├── stash.go           - Stash operations
  └── exec.go            - Low-level exec.Command runner
internal/history/        - Directory navigation history tracking
internal/hooks/          - Hook execution system
internal/log/            - Context-aware logging (stderr)
internal/output/         - Context-aware output (stdout)
internal/prcache/        - PR cache (simple JSON at ~/.wt/prs.json)
internal/preserve/       - Worktree preservation logic (protect branches from pruning)
internal/registry/       - Repository registry (tracks managed repos)
internal/storage/        - Generic JSON file storage (used by registry, prcache, history)
internal/worktree/       - Worktree path formatting (template expansion)
internal/ui/             - Terminal UI components
  ├── static/            - Non-interactive output (table formatting)
  ├── progress/          - Progress indicators (spinner, progress bar)
  ├── prompt/            - Simple interactive prompts (confirm)
  ├── styles/            - Shared lipgloss styles, themes, symbols
  └── wizard/            - Interactive wizard framework
      ├── framework/     - Core wizard orchestration (Wizard, Step interface)
      ├── flows/         - Command-specific wizard implementations
      └── steps/         - Reusable step components (FilterableList, SingleSelect, etc.)
```

### Key Design Decisions

**Shell out to git/gh/glab CLI** - All git and forge operations use `os/exec.Command` to call CLI tools directly. This is simpler and more reliable than Go libraries. Changes are isolated to `internal/git/` and `internal/forge/`.

**Forge abstraction** - The `internal/forge/` package provides a common interface for GitHub and GitLab. Platform is auto-detected from the remote URL. Both `gh` (GitHub) and `glab` (GitLab) CLIs are supported.

**Worktree naming convention** - Worktrees are created as `<repo-name>-<branch>` (e.g., `wt-feature-branch`). The repo name is extracted from git origin URL.

**Path handling** - Worktree placement is controlled by `checkout.worktree_format` (template with `{repo}`, `{branch}`, `{origin}` placeholders). Repos are explicitly registered via `wt repo add` and tracked in the registry (`~/.wt/repos.json`).

**MR/PR status** - Uses `gh pr list` or `glab mr list` to fetch merge request info (auto-detected). States: merged, open, closed.

### Dependencies

- **CLI parsing**: `github.com/spf13/cobra` - CLI framework with subcommands and flag parsing
- **UI**: `charm.land/lipgloss/v2` - Terminal styling
- **UI**: `charm.land/bubbles/v2` - UI components (spinner, progress)
- **UI**: `charm.land/bubbletea/v2` - TUI framework (wizard flows)
- **External**: Requires `git` in PATH; `gh` CLI for GitHub repos, `glab` CLI for GitLab repos

### Development Guidelines

**Go source files use tabs for indentation** - All `.go` files in this repo use tabs (not spaces). When using the `Edit` tool to match existing code, be aware that the `Read` tool displays tabs as spaces in its output. If an `Edit` replacement fails with "string not found", the likely cause is a tabs-vs-spaces mismatch. Use `cat -v -e -t` via Bash to see the raw whitespace characters before retrying.

**Use `new(expr)` for pointer literals** - Go 1.26+ supports `new(true)`, `new(false)`, `new("value")` etc. to create pointers to values. Use this instead of helper functions like `boolPtr(b bool) *bool`. The pre-commit hook runs `go fix` which inlines these helpers automatically, so never introduce `boolPtr`/`stringPtr`/`intPtr` helpers.

**Target Resolution Pattern** - Commands use a unified `[scope:]branch` positional argument pattern where `scope` can be a repo name OR label:

- **Worktree targeting** (most commands): `wt cd`, `wt exec`, `wt checkout`, `wt prune`, `wt hook`, `wt note`
  - `branch` - searches current repo, or all repos for existing branches
  - `repo:branch` - targets specific repo
  - `label:branch` - targets all repos with that label (resolved after checking repo names)

- **Repo targeting**: `wt pr create/merge/view`, `wt list`, `wt label`, `wt repo list`
  - `wt list [scope...]` - positional args for repo name or label filtering
  - `wt label add/remove/list/clear <label> [scope...]` - positional scope args
  - `wt repo list [label...]` - positional args for label filtering
  - `wt pr checkout [repo] <number>` - optional repo as first positional arg
  - `wt pr create/merge/view [repo]` - optional repo positional arg

**Resolution order for `scope:branch`:**
1. Try to match scope as repo name
2. If no repo match, try to match as label (returns multiple repos)

**Keep completions in sync** - **IMPORTANT**: When adding or modifying CLI flags, you MUST update the shell completion scripts in `cmd/wt/completions.go`. This file contains completions for fish, bash, and zsh. Search for existing flags of the same command to find where to add the new flag in each shell format.

**Reuse flags consistently** - When adding flags that serve the same purpose across commands, use identical names/shortcuts. Standard flags:
- `-i, --interactive` - interactive mode (wt checkout, wt cd, wt prune)
- `-d, --dry-run` - preview without making changes
- `-f, --force` - force operation (override safety checks)
- `-c, --include-clean` - include clean worktrees (0 commits, no changes)
- `-g, --global` - operate on all repos (not just current)
- `-a, --arg` - set hook variable KEY=VALUE (repeatable)
- `--json` - output as JSON
- `--hook` / `--no-hook` - control hook execution (for checkout, pr checkout, prune)

**Note**: Commands use positional args for repo/label targeting (see "Repo targeting" above).

**Never modify git internal files directly** - Always use git CLI commands via `exec.Command`. Never read/write `.git/` directory contents, `.git` files in worktrees, or git refs directly. Use `git worktree repair` for fixing broken links, `git worktree prune` for cleanup.

**Never ignore errors** - All errors must be handled explicitly. Never use `_ = someFunc()` or call functions without checking their return error. In tests, use `t.Fatalf` for setup errors. In production code, either return the error or log it with context if it's truly non-fatal.

**Always use `config.WorkDirFromContext(ctx)` for the working directory** - Never call `os.Getwd()` in command implementations. The working directory is captured once in `root.go` and stored in context. Use `config.WorkDirFromContext(ctx)` to retrieve it. For git operations that need a path, use `git.GetCurrentRepoMainPathFrom(ctx, workDir)`. Direct `os.Getwd()` calls break parallel test isolation.

**Avoid magic strings** - Use named constants for repeated string values (e.g., PR states, merge strategies). Define constants in the package that owns the concept (e.g., `forge.PRStateMerged` for PR states). Never compare against string literals scattered across files.

**Interactive Mode (`-i` flag)** - When implementing interactive wizard mode for commands:

1. **Respect explicit CLI arguments** - If a flag is passed explicitly (e.g., `--hook`, `--no-hook`, `-r`), skip that wizard step entirely. Don't allow the user to change values that were explicitly set.

2. **Show all values in summary** - The final summary should display both wizard-selected and CLI-provided values. Don't distinguish between them visually.

3. **Skip irrelevant steps** - Skip steps when there are no options available (e.g., no hooks configured) or when the step doesn't apply based on previous selections.

4. **Pre-select sensible defaults** - Pre-select "default" options (e.g., hooks with `on=["checkout"]`), pre-select current repo when inside a git repo.

5. **Handle empty selections** - If a multi-select step has no selections, translate to the appropriate flag (e.g., no hooks selected → `--no-hook`).

6. **File structure** - Create wizard flows in `internal/ui/wizard/flows/<command>.go`. Define `<Command>Options` struct for wizard output and `<Command>WizardParams` struct for wizard input. The wizard returns options which the command applies to its flags.

**Forge Feature Parity** - Any feature that involves forge operations (PRs, cloning, etc.) MUST support both GitHub and GitLab. Always:
- Add methods to the `Forge` interface first
- Implement in both `github.go` and `gitlab.go`
- Handle platform-specific limitations explicitly (e.g., GitLab doesn't support rebase merge via CLI)
- Never call `gh` or `glab` directly outside `internal/forge/`

**Branch Workflow** - All changes must be made in a feature branch and merged through a PR. Never commit directly to main.

**Pull Request Template** - Always use the PR template at `.github/pull_request_template.md`. Include:
- Brief summary at the top (no header)
- Breaking changes (delete section if none)
- Count of unit/integration tests added

**Pre-v1.0 Breaking Changes** - Until v1.0, backwards incompatible changes to CLI commands, flags, config format, and cache files are allowed without migration code. Users are expected to update their config manually (see README disclaimer).

### Dependency Injection Pattern

**Global variables** - Shared state initialized in `root.go`:
```go
var (
    // Global flags
    verbose bool
    quiet   bool
)
```

**context.Context** - Request-scoped values passed to functions:
- `config.FromContext(ctx)` → Config (global configuration)
- `config.ResolverFromContext(ctx)` → ConfigResolver (per-repo config resolution with caching)
- `config.WorkDirFromContext(ctx)` → Working directory
- `log.FromContext(ctx)` → Logger (writes to stderr)
- `output.FromContext(ctx)` → Printer (writes to stdout)

**Per-repo config resolution** - Commands that operate on a specific repo should use the `ConfigResolver` to get effective config (global merged with local `.wt.toml`):
```go
resolver := config.ResolverFromContext(ctx)
effCfg, err := resolver.ConfigForRepo(repo.Path)
if err != nil {
    l.Printf("Warning: failed to load local config: %v\n", err)
    effCfg = resolver.Global()
}
```
Use `effCfg` for repo-specific settings (hooks, checkout, merge, prune, preserve, forge.default). Use `config.FromContext(ctx)` for global-only settings (RegistryPath, DefaultSort, DefaultLabels, forge.rules, hosts, theme).

**Convention**: Always name the logger variable `l`:
```go
func runCheckout(ctx context.Context) error {
    l := log.FromContext(ctx)
    out := output.FromContext(ctx)
    l.Debug("creating worktree")
    out.Println(result)
}
```

**stdout vs stderr**:
- stdout: Primary output (data, tables, paths, JSON)
- stderr: Diagnostics (logs, progress, errors)

This allows piping: `cd $(wt cd --number 1)` works because logs go to stderr.

### Integration Tests

Integration tests are in `cmd/wt/*_integration_test.go` with build tag `//go:build integration`.

**When writing or modifying tests, use the `integration-test-writer` agent** (`.claude/agents/integration-test-writer.md`) which covers:
- **Integration tests**: Complete template, parallel test safety, registry/workDir isolation
- **Wizard/interactive tests**: Step unit tests, wizard orchestration, keyMsg helpers

Key points for integration tests:
- All tests MUST use `t.Parallel()` as first statement
- Never use `os.Setenv("HOME", ...)` or `os.Chdir()` - use `cfg.RegistryPath` and `workDir` instead
- Always use `resolvePath(t, t.TempDir())` for macOS symlink resolution

Key points for wizard tests:
- Test steps by calling `Update()` directly with synthetic `tea.KeyPressMsg`
- Use `keyMsg("enter")` helper to create key events
- Use `updateStep[T]()` generic helper for type-safe step updates
- For TextInputStep, call `Init()` before typing (to focus the input)

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

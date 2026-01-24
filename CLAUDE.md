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
cmd/wt/main.go           - CLI entry point with go-arg subcommands
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
internal/ui/             - Terminal UI components
  ├── table.go           - Lipgloss table formatting with colors
  └── spinner.go         - Bubbletea spinner
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

**Target Resolution Pattern** - Commands that operate on worktrees use `--id` (`-i`) flag with `internal/resolve.ByID()`:

- **ID or repo/label**: `wt exec`, `wt hook` - require `-i <id>`, or `-r <repo>`/`-l <label>` (exec/hook support multiple); these flags are mutually exclusive
- **ID or repo or label**: `wt cd` - require `-i <id>`, `-r <repo>`, or `-l <label>` (mutually exclusive)
- **Optional ID**: `wt note`, `wt pr create`, `wt pr merge`, `wt prune` - when inside worktree, defaults to current branch; outside requires `-i` (prune supports multiple)
- **Special case**: `wt add` - inside repo uses branch name; outside repo requires `-r <repo>` or `-l <label>` to specify target repos

Commands using this pattern: `wt exec`, `wt cd`, `wt note set/get/clear`, `wt hook`, `wt pr create`, `wt pr merge`, `wt prune`

**Keep completions/config in sync** - When CLI commands, flags, or subcommands change, always update the shell completion scripts (fish, bash, zsh in `cmd/wt/main.go`) and any config generation commands to match.

**Reuse flags consistently** - When adding flags that serve the same purpose across commands, use identical names/shortcuts. Standard flags:
- `-i, --id` - worktree ID for targeting
- `-r, --repository` - repository name for targeting (wt add, list, exec, cd, hook)
- `-l, --label` - target repos by label (wt add, list, exec, hook)
- `-n, --dry-run` - preview without making changes
- `-f, --force` - force operation (override safety checks)
- `-c, --include-clean` - include clean worktrees (0 commits, no changes)
- `-g, --global` - operate on all repos (not just current)
- `-a, --arg` - set hook variable KEY=VALUE (repeatable)
- `--json` - output as JSON
- `--hook` / `--no-hook` - control hook execution (for add, pr checkout, prune)

**Forge Feature Parity** - Any feature that involves forge operations (PRs, cloning, etc.) MUST support both GitHub and GitLab. Always:
- Add methods to the `Forge` interface first
- Implement in both `github.go` and `gitlab.go`
- Handle platform-specific limitations explicitly (e.g., GitLab doesn't support rebase merge via CLI)
- Never call `gh` or `glab` directly outside `internal/forge/`

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

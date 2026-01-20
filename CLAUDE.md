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
  └── spinner.go         - Bubbletea spinner (unused currently)
```

### Key Design Decisions

**Shell out to git/gh/glab CLI** - All git and forge operations use `os/exec.Command` to call CLI tools directly. This is simpler and more reliable than Go libraries. Changes are isolated to `internal/git/` and `internal/forge/`.

**Forge abstraction** - The `internal/forge/` package provides a common interface for GitHub and GitLab. Platform is auto-detected from the remote URL. Both `gh` (GitHub) and `glab` (GitLab) CLIs are supported.

**Worktree naming convention** - Worktrees are created as `<repo-name>-<branch>` (e.g., `wt-feature-branch`). The repo name is extracted from git origin URL.

**Path handling** - User must specify base directory for `wt add`. The tool fails if the directory doesn't exist (no automatic mkdir). Common patterns:
- `wt add . branch` - Add in current dir
- `wt add .. branch` - Add next to repo
- `wt add ~/Git/worktrees branch` - Add in central location

**MR/PR status** - Uses `gh pr list` or `glab mr list` to fetch merge request info (auto-detected). States: merged, open, closed.

### CLI Commands

- `wt add <branch>` - Add worktree for existing branch (inside repo)
- `wt add -b <branch>` - Add worktree for new branch
- `wt add -i <id>` - Open worktree by ID (outside repo)
- `wt prune` - Remove merged+clean worktrees, show table with PR status (use -r/--refresh to fetch latest)
- `wt prune -i <id>` - Remove specific worktree by ID
- `wt list [--json]` - List worktrees in directory
- `wt show [-i <id>]` - Show detailed status for a worktree (commits, changes, PR info)
- `wt exec -i <id> [-i <id>...] -- <cmd>` - Run command in worktree(s) by ID
- `wt cd -i <id>` - Print worktree path by ID
- `wt mv` - Move worktrees to different directory
- `wt note set/get/clear [-i <id>]` - Manage branch notes (optional ID outside worktree)
- `wt hook <hook> [-i <id>...]` - Run configured hook by name (multi-ID supported)
- `wt pr open <number> [repo]` - Create worktree for GitHub PR
- `wt pr clone <number> <repo>` - Clone repo and create worktree for PR
- `wt pr merge [-i <id>]` - Merge PR and clean up worktree
- `wt config init` - Create default config file
- `wt config show` - Show effective configuration
- `wt config hooks` - List available hooks
- `wt completion <shell>` - Generate shell completions (fish, bash, zsh)

### Shell Completions

The tool includes built-in completion scripts for fish, bash, and zsh:

```bash
# Fish
wt completion fish > ~/.config/fish/completions/wt.fish

# Bash
wt completion bash > ~/.local/share/bash-completion/completions/wt

# Zsh (then add ~/.zfunc to fpath in .zshrc)
wt completion zsh > ~/.zfunc/_wt
```

Completions provide context-aware suggestions for branches, directories, and flags.

### Dependencies

- **CLI parsing**: `github.com/alecthomas/kong` - Struct-based arg parsing with subcommands and auto-dispatch
- **UI**: `github.com/charmbracelet/lipgloss` - Terminal styling
- **UI**: `github.com/charmbracelet/bubbles/table` - Table component
- **External**: Requires `git` in PATH; `gh` CLI for GitHub repos, `glab` CLI for GitLab repos

### Development Guidelines

**Target Resolution Pattern** - Commands that operate on worktrees use `--id` (`-i`) flag with `internal/resolve.ByID()`:

- **Required ID**: `wt exec`, `wt cd` - always require `-i <id>` (exec supports multiple)
- **Optional ID**: `wt note`, `wt hook`, `wt pr merge`, `wt prune` - when inside worktree, defaults to current branch; outside requires `-i` (hook/prune support multiple)
- **Special case**: `wt add` - inside repo uses branch name positional arg; outside repo requires `-i <id>`

Commands using this pattern: `wt add` (outside repo), `wt exec`, `wt cd`, `wt note set/get/clear`, `wt hook`, `wt pr merge`, `wt prune`

**Keep completions/config in sync** - When CLI commands, flags, or subcommands change, always update the shell completion scripts (fish, bash, zsh in `cmd/wt/main.go`) and any config generation commands to match.

**Reuse flags consistently** - When adding flags that serve the same purpose across commands, use identical names/shortcuts. Standard flags:
- `-i, --id` - worktree ID for targeting
- `-d, --dir` - target directory (with `env:WT_DEFAULT_PATH`)
- `-n, --dry-run` - preview without making changes
- `-f, --force` - force operation (override safety checks)
- `-c, --include-clean` - include clean worktrees (0 commits, no changes)
- `--json` - output as JSON
- `--hook` / `--no-hook` - control hook execution (for add, pr open, prune)

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

### Releasing

Releases are automated via GoReleaser in CI. **Do not use `gh release create` manually** - it won't generate the proper changelog.

**Important**: This repo requires push access to `raphi011/wt`. If using a work machine with different GitHub credentials, you may need to switch accounts or configure SSH keys for the personal account.

```bash
# 1. Check current version
git tag --sort=-v:refname | head -1

# 2. Determine next version based on commits since last tag
git log $(git describe --tags --abbrev=0)..HEAD --oneline

# 3. Ensure all changes are committed and pushed
git push origin main

# 4. Create annotated tag and push (triggers GoReleaser CI)
git tag -a v0.X.0 -m "v0.X.0"
git push origin v0.X.0
```

Version bumping:
- `feat:` commits → minor version bump (v0.1.0 → v0.2.0)
- `fix:` commits only → patch version bump (v0.1.0 → v0.1.1)
- Breaking changes (`!`) → major version bump (v0.1.0 → v1.0.0)

GoReleaser will:
- Build binaries for darwin/linux (amd64/arm64)
- Generate changelog from conventional commits
- Create GitHub release with assets
- Update Homebrew tap (cask in `raphi011/homebrew-tap`)

### Homebrew Distribution

Uses **casks** (not formulas) via `homebrew_casks` in `.goreleaser.yaml`. This is correct because:
- Formulas build from source, casks install pre-compiled binaries
- GoReleaser produces pre-compiled binaries → casks are semantically correct
- GoReleaser deprecated `brews` in v2.10, recommends `homebrew_casks`

Install: `brew install --cask raphi011/tap/wt`

Tap repo: `raphi011/homebrew-tap` with cask in `Casks/wt.rb`

### Testing Locally

The tool must be run from within a git repository for `wt add -b` to work (needs origin URL). For testing:

```bash
cd ~/Git/wt  # Must be in a git repo
./wt add -b .. test-branch  # Creates ~/Git/wt-test-branch
```

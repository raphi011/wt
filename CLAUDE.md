# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`wt` is a Go CLI tool for managing git worktrees with GitHub PR integration. It replaces complex fish shell functions with a clean, portable binary.

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
internal/github/pr.go    - PR status via gh CLI
internal/ui/             - Terminal UI components
  ├── table.go           - Lipgloss table formatting with colors
  └── spinner.go         - Bubbletea spinner (unused currently)
```

### Key Design Decisions

**Shell out to git/gh CLI** - All git and GitHub operations use `os/exec.Command` to call `git` and `gh` CLI directly. This is simpler and more reliable than Go git libraries. If swapping to a library is needed, changes are isolated to `internal/git/` and `internal/github/`.

**Worktree naming convention** - Worktrees are created as `<repo-name>-<branch>` (e.g., `wt-feature-branch`). The repo name is extracted from git origin URL.

**Path handling** - User must specify base directory for `wt create`. The tool fails if the directory doesn't exist (no automatic mkdir). Common patterns:
- `wt create . branch` - Create in current dir
- `wt create .. branch` - Create next to repo
- `wt create ~/Git/worktrees branch` - Create in central location

**PR status** - Uses `gh pr list --json` to fetch PR info. Nerd font icons: 󰜘 (merged), 󰜛 (open), 󰅖 (closed).

### CLI Commands

- `wt create <path> <branch>` - Create worktree at path/<repo>-<branch>
- `wt clean [path]` - Remove merged+clean worktrees, show table with PR status
- `wt list [--json]` - List worktrees in directory
- `wt completion fish` - Generate fish shell completions

### Fish Completions

The tool includes a built-in fish completion script accessed via `wt completion fish`. Install with:
```bash
wt completion fish > ~/.config/fish/completions/wt.fish
```

Completions provide context-aware suggestions for paths, branches, and flags.

### Dependencies

- **CLI parsing**: `github.com/alexflint/go-arg` - Struct-based arg parsing with subcommands
- **UI**: `github.com/charmbracelet/lipgloss` - Terminal styling
- **UI**: `github.com/charmbracelet/bubbles/table` - Table component
- **External**: Requires `git` and `gh` CLI in PATH

### Testing Locally

The tool must be run from within a git repository for `wt create` to work (needs origin URL). For testing:

```bash
cd ~/Git/wt  # Must be in a git repo
./wt create .. test-branch  # Creates ~/Git/wt-test-branch
```

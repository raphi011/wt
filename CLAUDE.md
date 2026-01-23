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

**Path handling** - Directory configuration is done via config file or environment variables (no `-d` flag). The tool fails if the directory doesn't exist (no automatic mkdir). Configuration sources (highest priority first):
- `WT_WORKTREE_DIR` env var - target directory for worktrees
- `WT_REPO_DIR` env var - directory to scan for repos
- `worktree_dir` in config file
- `repo_dir` in config file
- Falls back to current directory if unset

**MR/PR status** - Uses `gh pr list` or `glab mr list` to fetch merge request info (auto-detected). States: merged, open, closed.

### CLI Commands

- `wt add <branch>` - Add worktree for existing branch (inside repo)
- `wt add -b <branch>` - Add worktree for new branch
- `wt add -b <branch> -r <repo> [-r <repo>...]` - Create branch across multiple repos (by name)
- `wt add -b <branch> -l <label> [-l <label>...]` - Create branch across repos with label
- `wt prune` - Remove merged+clean worktrees, show table with PR status (use -R/--refresh to fetch latest)
- `wt prune -i <id>` - Remove specific worktree by ID
- `wt list [-r <repo>] [-l <label>] [-s id|repo|branch|commit] [--json]` - List worktrees in directory
- `wt repos [-l <label>] [-s name|branch|worktrees|label] [--json]` - List repositories in directory
- `wt show [-i <id>]` - Show detailed status for a worktree (commits, changes, PR info)
- `wt exec -i <id> [-i <id>...] -- <cmd>` - Run command in worktree(s) by ID
- `wt exec -r <repo> [-l <label>] -- <cmd>` - Run command in repo(s) by name/label
- `wt cd -i <id>` - Print worktree path by ID
- `wt cd -r <repo>` - Print repo path by name
- `wt cd -l <label>` - Print repo path by label (must match exactly one repo)
- `wt mv` - Move worktrees to configured directory (from env/config)
- `wt note set/get/clear [-i <id>]` - Manage branch notes (optional ID outside worktree)
- `wt label add/remove/list/clear` - Manage repository labels (stored in git config as wt.labels)
- `wt hook <hook> [-i <id>...]` - Run configured hook by name (multi-ID supported)
- `wt hook <hook> -r <repo> [-l <label>]` - Run hook in repo(s) by name/label
- `wt pr checkout <number> [org/repo]` - Create worktree for PR (clones repo if org/repo provided)
- `wt pr checkout <number> -r <repo>` - Create worktree for PR from local repo by name
- `wt pr create --title "..." [--body "..."]` - Create PR for current branch
- `wt pr view [-i <id>] [-w]` - View PR details or open in browser
- `wt pr merge [-i <id>]` - Merge PR and clean up worktree
- `wt config init` - Create default config file
- `wt config show` - Show effective configuration
- `wt config hooks` - List available hooks
- `wt doctor` - Diagnose and report cache issues
- `wt doctor --fix` - Auto-fix recoverable cache issues
- `wt doctor --reset` - Rebuild cache from scratch (loses IDs)
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

**Environment Variables** - Directory configuration via env vars (override config file):
- `WT_WORKTREE_DIR` - target directory for worktrees
- `WT_REPO_DIR` - directory to scan for repos

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

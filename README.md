# wt

<div align="center">

[![CI](https://img.shields.io/github/actions/workflow/status/raphi011/wt/ci.yml?branch=main&label=tests&labelColor=333333&color=666666)](https://github.com/raphi011/wt/actions/workflows/ci.yml)
[![codecov](https://img.shields.io/codecov/c/github/raphi011/wt?labelColor=333333&color=666666)](https://codecov.io/gh/raphi011/wt)
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-555555.svg?labelColor=333333&color=666666)](https://goreportcard.com/report/github.com/raphi011/wt)
[![MIT License](https://img.shields.io/badge/License-MIT-555555.svg?labelColor=333333&color=666666)](LICENSE)
[![Downloads](https://img.shields.io/github/downloads/raphi011/wt/total?labelColor=333333&color=666666)](https://github.com/raphi011/wt/releases)
[![Last Commit](https://img.shields.io/github/last-commit/raphi011/wt?labelColor=333333&color=666666)](https://github.com/raphi011/wt/commits/main)
[![Commit Activity](https://img.shields.io/github/commit-activity/m/raphi011/wt?labelColor=333333&color=666666)](https://github.com/raphi011/wt/graphs/commit-activity)

</div>

Git worktree manager with GitHub/GitLab integration.

## Why wt

Git worktrees let you work on multiple branches simultaneously without stashing or switching—great for juggling a feature branch and a hotfix, or running multiple AI agent sessions in parallel.

But worktrees can pile up fast. You end up with a dozen directories, can't remember which ones are already merged, and need custom scripts to open your editor, create terminal tabs, or clean up stale checkouts.

`wt` solves this:
- **Hooks** auto-run commands when creating/opening worktrees (open editor, spawn terminal tab)
- **Prune** removes merged worktrees and shows PR/MR status so you know what's safe to delete
- **PR checkout** opens pull requests in worktrees for easier code review

## ⚠️ Pre-1.0 Notice

This project may include breaking command & configuration changes until v1.0 is released. Once v1 is released, backwards compatibility will be maintained.

If something breaks:
- Delete `~/.wt/prs.json` (PR cache)
- Compare your config with `wt config init -s` and update to match newer config format

## Install

```bash
# Homebrew (macOS/Linux)
brew install --cask raphi011/tap/wt

# Go
go install github.com/raphi011/wt/cmd/wt@latest
```

Requires `git` in PATH. For GitHub repos: `gh` CLI. For GitLab repos: `glab` CLI.

## Getting Started

### 1. Create Config

```bash
wt config init            # Create ~/.wt/config.toml
wt config init -s         # Print default config to stdout (for review)
```

### 2. Register Repos

```bash
# Register a repo you already have cloned
wt repo add ~/path/to/myrepo

# Or clone and register a new repo
wt repo clone git@github.com:org/repo.git
```

Repos are also auto-registered the first time you run `wt checkout` inside one.

### 3. Create a Worktree

```bash
wt checkout -b new-branch    # Create worktree with new branch (from default branch)
```

### 4. List Worktrees

```bash
wt list -g
```

You're ready to go! See Scenarios below for common workflows.

## Interactive Mode

For guided worktree creation, use the `-i` flag:

```bash
wt checkout -i
```

This launches a step-by-step wizard that guides you through the checkout process.

## Scenarios

### Starting a New Feature

```bash
# Create worktree with new branch (from origin/main)
wt checkout -b feature-login

# Create from a different base branch
wt checkout -b feature-login --base develop

# Fetch base branch before creating (ensures up-to-date base)
wt checkout -b feature-login -f

# Fetch target branch from origin before checkout
wt checkout feature-login -f

# Stash local changes and apply them to the new worktree
wt checkout -b feature-login -s

# Add a note to remember what you're working on
wt checkout -b feature-login --note "Implementing OAuth flow"

# Target a specific repo from any directory (repo:branch syntax)
wt checkout -b myrepo:feature-login

# Combine with other flags
wt checkout -b myrepo:feature-login --base develop -f
```

With hooks configured, your editor opens automatically:

```toml
# ~/.wt/config.toml
[hooks.vscode]
command = "code '{worktree-dir}'"
on = ["checkout"]
```

### Reviewing a Pull Request

```bash
# Checkout PR from current repo
wt pr checkout 123

# Checkout PR from a different local repo (by name)
wt pr checkout backend-api 123

# Clone repo you don't have locally and checkout PR
wt pr checkout org/new-repo 456

# Specify forge type when auto-detection fails
wt pr checkout 123 --forge gitlab
```

View PR details or open in browser:

```bash
wt pr view               # Show PR details
wt pr view -w            # Open PR in browser
wt pr view myrepo        # View PR for specific repo
```

After review, merge and clean up in one command:

```bash
wt pr merge              # Uses squash by default
wt pr merge -s rebase    # Or specify strategy
wt pr merge --keep       # Merge but keep worktree
```

### Creating a Pull Request

```bash
# Create PR for current branch
wt pr create --title "Add login feature"

# With description
wt pr create --title "Fix bug" --body "Fixes issue #123"

# Read body from file (great for templates)
wt pr create --title "Add feature" --body-file=pr.md

# Create as draft
wt pr create --title "WIP: Refactor auth" --draft

# Create and open in browser
wt pr create --title "Ready for review" -w

# By repo name (when outside worktree)
wt pr create --title "Add feature" myrepo
```

### Cleaning Up

```bash
# See what worktrees exist
wt list

# Remove merged worktrees (uses cached PR status)
wt prune

# Refresh PR status from GitHub/GitLab first
wt prune -R

# Preview what would be removed
wt prune -d

# Verbose dry-run: see what's skipped and why
wt prune -d -v

# Clear cached PR data and re-fetch
wt prune --reset-cache

# Also delete local branches after removal
wt prune --delete-branches

# Keep local branches even if config says delete
wt prune --no-delete-branches

# Remove specific branch worktree
wt prune feature-login -f

# Remove worktree from specific repo
wt prune myrepo:feature-login -f
```

### Working Across Multiple Repos

Register and label repos for batch operations:

```bash
# Register a repo (from inside the repo)
wt repo add .

# Register a repo by path
wt repo add ~/path/to/myrepo

# Clone and register a new repo
wt repo clone git@github.com:org/repo.git

# Migrate an existing repo to bare structure (for faster worktrees)
wt repo make-bare ./myrepo

# Unregister a repo
wt repo remove myrepo

# List all repos
wt repo list
wt repo list backend    # Filter by label
```

Label your repos for batch operations:

```bash
# Add labels to repos
cd ~/Git/backend-api && wt label add backend
cd ~/Git/auth-service && wt label add backend
cd ~/Git/web-app && wt label add frontend

# List labels
wt label list         # Labels for current repo
wt label list -g      # All labels across repos

# Clear labels from a repo
wt label clear

# Create same branch across all backend repos (using label prefix)
wt checkout -b backend:feature-auth

# Or target specific repo by name
wt checkout -b backend-api:feature-auth

# Run command across worktrees
wt exec main -- git status              # In all repos' main worktree
wt exec backend-api:main -- make test   # In specific repo's worktree
```

### Quick Navigation

> **Note:** `wt cd` prints the path but can't change your shell directory. Add the shell wrapper from [Shell Integration](#shell-integration) to use `wt cd` directly.

```bash
# Jump to most recently accessed worktree
wt cd

# Jump to worktree by branch name
wt cd feature-auth

# Jump to worktree in specific repo (if branch exists in multiple repos)
wt cd backend-api:feature-auth

# Interactive fuzzy search
wt cd -i

# Run command in worktree
wt exec -- git status                   # In current worktree
wt exec myrepo:main -- code .
```

### Running Hooks Manually

```bash
# Run a hook on current worktree
wt hook vscode

# Run multiple hooks
wt hook vscode kitty

# Run on specific worktree (repo:branch format)
wt hook vscode -- myrepo:feature

# Run across worktrees by label
wt hook build -- backend:main

# Pass custom variables
wt hook claude --arg prompt="implement feature X"

# Preview command without executing
wt hook vscode -d
```

### Branch Notes

```bash
# Set a note (visible in list/prune output)
wt note set "WIP: fixing auth timeout issue"

# Get current note
wt note get

# Clear note
wt note clear

# Set note on specific worktree (repo:branch format)
wt note set "Ready for review" myrepo:feature
```

## Configuration

Config file: `~/.wt/config.toml`

```bash
wt config init               # Create default config
wt config init -s            # Print config to stdout
wt config show               # Show effective config
wt config hooks              # List configured hooks
```

### Basic Settings

```toml
# Default sort order for list: "created", "repo", "branch"
default_sort = "created"

# Labels applied to newly auto-registered repos
# default_labels = ["work"]

[checkout]
# Folder naming: {repo}, {branch}, {origin}
worktree_format = "{repo}-{branch}"

# Base ref for new branches: "remote" (default) or "local"
# - "remote": branches from origin/<base> (ensures latest remote state)
# - "local": branches from local <base> (useful for offline work)
base_ref = "remote"

# Auto-fetch from origin before checkout (default: false)
# Note: with base_ref="local" and an explicit --base, --fetch is skipped (warns) since fetch doesn't affect local refs
auto_fetch = true

# Auto-set upstream tracking (default: false)
# set_upstream = false

[prune]
# Delete local branches after worktree removal (default: false)
# delete_local_branches = false
```

**Base branch resolution (`--base` flag):**

| `--base` value | `base_ref` config | Branch created from |
|----------------|-------------------|---------------------|
| (none) | remote | `origin/<default>` (main/master) |
| (none) | local | local default branch |
| `develop` | remote | `origin/develop` |
| `develop` | local | local `develop` |
| `origin/develop` | (overridden) | `origin/develop` |
| `upstream/main` | (overridden) | `upstream/main` |

Explicit remote refs (`origin/branch`, `upstream/branch`) always override `base_ref` config.

**Fetch behavior (`--fetch` / `auto_fetch`):**

| Scenario | Fetch behavior |
|----------|----------------|
| `--base origin/develop` | Fetches `develop` from `origin` |
| `--base upstream/main` | Fetches `main` from `upstream` |
| `--base develop` + `base_ref=remote` | Fetches `develop` from `origin` |
| `--base develop` + `base_ref=local` | **Skipped with warning** |

### Hooks

```toml
[hooks.vscode]
command = "code '{worktree-dir}'"
description = "Open VS Code"
on = ["checkout", "pr"]  # Auto-run for these commands

[hooks.kitty]
command = "kitty @ launch --type=tab --cwd='{worktree-dir}'"
description = "Open new kitty tab"
on = ["checkout"]

[hooks.cleanup]
command = "echo 'Removed {branch}'"
on = ["prune"]

[hooks.claude]
command = "kitty @ launch --cwd='{worktree-dir}' -- claude '{prompt:-help me}'"
description = "Open Claude with prompt"
# No "on" = only runs via: wt hook claude --arg prompt="..."
```

**Hook triggers:** `checkout`, `pr`, `prune`, `merge`, `all` (see `on` field in config)

**Placeholders:** `{worktree-dir}`, `{repo-dir}`, `{branch}`, `{repo}`, `{origin}`, `{trigger}`, `{key}`, `{key:-default}`

### Forge Settings

Configure forge detection and multi-account auth for PR operations:

```toml
[forge]
default = "github"      # Default forge
default_org = "my-company"  # Default org (allows: wt pr checkout repo 123)

[[forge.rules]]
pattern = "company/*"
type = "gitlab"

[[forge.rules]]
pattern = "work-org/*"
type = "github"
user = "work-account"  # Use specific gh account for matching repos
```

### Merge Settings

```toml
[merge]
strategy = "squash"  # squash, rebase, or merge
```

### Preserve Settings

Automatically copy git-ignored files (`.env`, `.envrc`, etc.) from an existing worktree when checking out a new one with `wt checkout`. Useful for keeping local configuration in sync across worktrees.

```toml
[preserve]
patterns = [".env", ".env.*", ".envrc", "docker-compose.override.yml"]
exclude = ["node_modules", ".cache", "vendor"]
```

- **patterns** — glob patterns matched against file basenames; only git-ignored files are considered
- **exclude** — path segments to skip (any component match excludes the file)

Files are copied from the worktree on the default branch, e.g. `main` (or the first available worktree). Existing files are never overwritten. Symlinks are skipped. Use `--no-preserve` on `wt checkout` to skip.

### Self-Hosted Instances

```toml
[hosts]
"github.mycompany.com" = "github"
"gitlab.internal.corp" = "gitlab"
```

### Theming

Customize the interactive UI with preset themes or custom colors:

```toml
[theme]
# Use a preset theme
name = "dracula"  # none, default, dracula, nord, gruvbox, catppuccin

# Theme mode: "auto" (detect terminal), "light", or "dark"
mode = "auto"

# Use nerd font symbols (requires a nerd font installed)
nerdfont = true
```

Override individual colors with hex codes or ANSI color numbers:

```toml
[theme]
name = "nord"       # Start with a preset
primary = "#88c0d0" # Override specific colors
accent = "#b48ead"
```

Available color keys: `primary`, `accent`, `success`, `error`, `muted`, `normal`, `info`.

## Writing Hooks

Hooks are shell commands executed via `sh -c`. Placeholders like `{worktree-dir}` are replaced with raw text before the command runs — no automatic escaping or quoting is applied.

### Quoting Placeholders

Since values are substituted as-is, paths with spaces or special characters will break unquoted placeholders:

```toml
# Breaks if path contains spaces
[hooks.unsafe]
command = "code {worktree-dir}"

# Safe — single quotes protect the value
[hooks.safe]
command = "code '{worktree-dir}'"
```

> **Note:** Single quotes protect against spaces and most special characters, but not against values containing literal single quotes. This is a limitation of raw text substitution.

The same applies to all placeholders (`{repo-dir}`, `{branch}`, `{repo}`, `{origin}`, `{trigger}`) and custom `--arg` variables:

```toml
[hooks.claude]
command = "claude '{prompt:-help me}'"
```

### Multiline Hooks

Use TOML triple-quoted strings for multi-step hooks:

```toml
[hooks.setup]
command = '''
cd '{worktree-dir}'
npm install
npm run build
'''
on = ["checkout"]
```

**Important:** Without `set -e`, intermediate failures are silent — only the exit code of the **last** command determines whether the hook succeeds or fails. Use `set -e` to fail fast:

```toml
[hooks.setup]
command = '''
set -e
cd '{worktree-dir}'
npm install
npm run build
'''
on = ["checkout"]
```

### Piping Content via Stdin

Use `--arg key=-` to pipe stdin into a variable:

```bash
echo "implement auth" | wt hook claude --arg prompt=-
```

Multiple keys can read from the same stdin (all keys receive identical content):

```bash
cat spec.md | wt hook claude --arg prompt=- --arg context=-
```

## Integration with gh-dash

`wt` works great with [gh-dash](https://github.com/dlvhdr/gh-dash). Add a keybinding to checkout PRs as worktrees:

```yaml
# ~/.config/gh-dash/config.yml
keybindings:
  prs:
    - key: O
      command: wt pr checkout {{.RepoName}} {{.PrNumber}}
```

Press `O` to checkout PR → hooks auto-open your editor.

## Shell Integration

`wt cd` prints the worktree path to stdout but can't change your shell's directory on its own. `wt init` outputs a shell wrapper that intercepts `wt cd` and performs the actual `cd`.

```bash
# Bash - add to ~/.bashrc
eval "$(wt init bash)"

# Zsh - add to ~/.zshrc
eval "$(wt init zsh)"

# Fish - add to ~/.config/fish/config.fish
wt init fish | source
```

## Shell Completions

Completions are installed automatically when using Homebrew. For manual installs:

```bash
# Fish
wt completion fish > ~/.config/fish/completions/wt.fish

# Bash
wt completion bash > ~/.local/share/bash-completion/completions/wt

# Zsh (add ~/.zfunc to fpath in .zshrc)
wt completion zsh > ~/.zfunc/_wt
```

## Development

```bash
just build    # Build ./wt binary
just test     # Run tests
just install  # Install to ~/go/bin (+ shell completions)
```

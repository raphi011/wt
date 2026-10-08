# wt

<div align="center">

[![Tests](https://github.com/raphi011/wt/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/raphi011/wt/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/raphi011/wt/branch/main/graph/badge.svg)](https://codecov.io/gh/raphi011/wt)
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
- Compare your config with `wt config init --stdout` and update to match newer config format

## Install

```bash
# Homebrew (macOS/Linux)
brew install raphi011/tap/wt

# Go
go install github.com/raphi011/wt/cmd/wt@latest
```

Supported on macOS and Linux.

Requires `git` in PATH. PR/MR features need the forge CLI, installed and authenticated: `gh` for GitHub (`gh auth login`), `glab` for GitLab (`glab auth login`).

## Getting Started

### 1. Shell Integration

Set up shell completions and the shell wrapper so that `wt cd` can change your directory. See [Shell Integration](#shell-integration) for instructions.

### 2. Create Config

```bash
wt config init            # Create ~/.wt/config.toml
wt config init --stdout   # Print default config to stdout (for review)
```

The most important setting is `checkout.worktree_format` — it controls where worktrees are placed. The format supports `{repo}` and `{branch}` placeholders, and the path prefix determines placement:

```toml
[checkout]
# Nested with subfolder (default): ~/Git/myrepo/.worktrees/feature-branch
worktree_format = ".worktrees/{branch}"

# Nested inside repo: ~/Git/myrepo/myrepo-feature-branch
worktree_format = "{repo}-{branch}"

# Sibling to repo dir: ~/Git/myrepo-feature-branch
worktree_format = "../{repo}-{branch}"

# Centralized folder: ~/worktrees/myrepo-feature-branch
worktree_format = "~/worktrees/{repo}-{branch}"

# Absolute path: /tmp/worktrees/myrepo-feature-branch
worktree_format = "/tmp/worktrees/{repo}-{branch}"
```

Slashes in a branch name become dashes in the path: `feat/login` is placed at `.worktrees/feat-login`.

### 3. Register Repos

```bash
# Register a repo you already have cloned
wt repo add ~/path/to/myrepo

# Or clone and register a new repo (clones into ./repo)
wt repo clone git@github.com:org/repo.git
```

There are two repo layouts:

- **regular** (default): a standard clone. The default branch is checked out at the repo root, other branches are worktrees. Pick this if other tools expect a normal clone at the repo path.
- **bare**: the git data lives in `.git` and the repo root has no working tree. Every branch, including the default one, is a worktree. Pick this if no branch should be special.

Use `--clone-mode bare` (or `clone.mode` in config) to clone bare; `wt repo convert --clone-mode bare|regular` switches an existing repo.

Repos are also auto-registered the first time you run `wt checkout` inside one.

### 4. Create a Worktree

```bash
# From inside a registered repo
wt checkout -b new-branch              # Create worktree with new branch (from default branch)

# From anywhere — target a repo by name (as shown in wt repo list)
wt checkout -b myrepo:new-branch
```

### 5. Configure Hooks

Hooks run automatically when creating, opening, merging, or removing worktrees. Add them to `~/.wt/config.toml`:

```toml
# Open VS Code after every checkout
[hooks.vscode]
command = "code '{worktree-dir}'"
on = ["checkout"]

# Open a new terminal tab (kitty example)
[hooks.kitty]
command = "kitty @ launch --type=tab --cwd='{worktree-dir}'"
on = ["checkout"]

# Run Claude to review PR when checking out a PR
[hooks.claude-review]
command = "claude -p 'review this PR'"
on = ["checkout:pr"]

# Manual-only hook — only runs via: wt hook claude --arg prompt="..."
[hooks.claude]
command = "claude '{prompt:-help me}'"
```

See [Hooks](docs/hooks.md) for triggers, the full placeholder reference and advanced patterns.

### 6. List Worktrees

```bash
wt list       # Current repo
wt list -g    # All registered repos
```

```text
REPO    BRANCH      COMMIT   AGE          PR  NOTE
myrepo  feat/login  a72d0b5  2 hours ago      Implementing OAuth flow
myrepo  main        a72d0b5  2 hours ago
```

The `PR` column shows the cached PR/MR state; `wt list -R` refreshes it from GitHub/GitLab.

You're ready to go! `checkout`, `cd`, `prune` and `pr checkout` also support `-i` for an interactive wizard.

## Scenarios

### Starting a New Feature

```bash
# Create worktree with new branch (from origin/main)
wt checkout -b feature-login

# Create from a different base branch
wt checkout -b feature-login --base develop

# Fetch base branch before creating (ensures up-to-date base)
wt checkout -b feature-login --fetch

# Fetch target branch from origin before checkout
wt checkout feature-login --fetch

# Stash local changes and apply them to the new worktree
wt checkout -b feature-login --autostash

# Add a note to remember what you're working on
wt checkout -b feature-login --note "Implementing OAuth flow"

# Target a specific repo from any directory (repo:branch syntax)
wt checkout -b myrepo:feature-login

# Combine with other flags
wt checkout -b myrepo:feature-login --base develop --fetch
```

### Reviewing a Pull Request

```bash
# Select an open PR interactively from registered repositories
wt pr checkout -i
wt pr checkout -i backend-api

# Checkout PR from current repo
wt pr checkout 123

# Checkout PR from a different local repo (by name)
wt pr checkout backend-api 123

# Checkout PR from a registered repo matched by its remote (org/repo)
wt pr checkout org/repo 123

# Clone repo you don't have locally and checkout PR
wt pr checkout --clone org/new-repo 456

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
wt pr merge              # Uses squash by default; removes worktree and source branch
wt pr merge --strategy rebase    # Or specify strategy
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

# Remove worktrees whose PR is merged (current repo; -g for all repos)
wt prune

# Also remove stale worktrees (last commit older than prune.stale_days, no open PR)
wt prune --stale

# Refresh PR status from GitHub/GitLab first
wt prune -R

# Preview what would be removed
wt prune -d

# Dry-run that also prints the git and forge commands being run
wt prune -d -v

# Also delete local branches after removal
wt prune --delete-branches

# Keep local branches even if config says delete
wt prune --no-delete-branches

# Remove specific branch worktree
wt prune feature-login -f

# Remove worktree from specific repo
wt prune myrepo:feature-login -f
```

A worktree with uncommitted changes (modified, staged or untracked files) is never removed without `-f`. A named worktree whose PR is not merged also needs `-f`.

If the PR cache contains corrupt JSON, commands warn and preserve the file.
Run `wt list --reset-cache` to clear it; `wt list -R` can display fresh PR status without overwriting the corrupt cache.

### Working Across Multiple Repos

Label repos for batch operations:

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
wt exec main -- git status              # In the current repo's main worktree
wt exec -g main -- git status           # In all repos' main worktree
wt exec backend-api:main -- make test   # In specific repo's worktree
```

A branch without a `repo:` or `label:` scope resolves the same way in every command:

- Inside a repo it means the current repo.
- Outside a repo, or with `-g`, all repos are searched.
- `wt cd`, `wt checkout` and `wt diff` act on one worktree and fail if the branch is in several repos.
- `wt exec`, `wt hook`, `wt note` and `wt prune` act on every match, but only with `-g` or a label scope.

### Quick Navigation

> **Note:** `wt cd` prints the path but can't change your shell directory. Add the shell wrapper from [Shell Integration](#shell-integration) to use `wt cd` directly.

```bash
# Jump to most recently accessed worktree
wt cd

# Jump to worktree by branch name (current repo)
wt cd feature-auth

# Search all repos for the branch
wt cd -g feature-auth

# Jump to worktree in specific repo (if branch exists in multiple repos)
wt cd backend-api:feature-auth

# Interactive fuzzy search (current repo; -g for all repos)
wt cd -i

# Copy worktree path to clipboard
wt cd --copy feature-auth

# Run command in worktree
wt exec -- git status                   # In current worktree
wt exec myrepo:main -- code .
```

`wt exec` attempts every selected worktree even if a command fails, then reports
which worktrees failed. With one worktree it returns the command's exit code;
with multiple worktrees it returns 1 if any command fails. A command that cannot
start returns 1. Use `wt exec -- make test && deploy` to deploy only after success.

### Reviewing Changes

```bash
wt diff                        # What a PR would contain (vs origin/<default-branch>)
wt diff myrepo:feature-auth    # Diff another worktree
wt diff --working              # Uncommitted changes only
wt diff --stat                 # Diffstat summary (or --name-only)
wt diff --base origin/develop  # Different base
wt diff --tool delta           # Use a specific pager
```

### Running Hooks Manually

```bash
# Run a hook on current worktree
wt hook vscode

# Run on specific worktree ([scope:]branch format)
wt hook myrepo:feature vscode

# Run across worktrees by label
wt hook backend:main build

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
wt note set myrepo:feature "Ready for review"
```

## Configuration

Global config: `~/.wt/config.toml`
Local config: `.wt.toml` (in repo root, overrides global settings for that repo)

```bash
wt config init               # Create default global config
wt config init --local       # Create per-repo .wt.toml
wt config show               # Show effective config (merged if in a repo)
wt config hooks              # List hooks with source annotations
```

The generated config file documents every setting. Reference:

- [Configuration](docs/configuration.md): checkout, prune, clone, forge, merge, preserve, self-hosted instances, theming, per-repo config
- [Hooks](docs/hooks.md): triggers, placeholders, working directory, execution order, quoting, stdin
- [Command reference](docs/commands.md): every command, alias and flag

`wt` keeps its files in `~/.wt/`:

| File | Content |
|------|---------|
| `config.toml` | Global configuration |
| `repos.json` | Registered repos and their labels |
| `history.json` | Recently accessed worktrees (used by `wt cd`) |
| `prs.json` | Cached PR/MR status |

## Shell Integration

### Shell Wrapper

`wt cd` prints the worktree path to stdout but can't change your shell's directory on its own. `wt init` outputs a shell wrapper that intercepts `wt cd` and performs the actual `cd`.

```bash
# Fish - add to ~/.config/fish/config.fish
wt init fish | source

# Bash - add to ~/.bashrc
eval "$(wt init bash)"

# Zsh - add to ~/.zshrc
eval "$(wt init zsh)"
```

### Shell Completions

Completions are installed automatically when using Homebrew. For manual installs:

```bash
# Fish
wt completion fish > ~/.config/fish/completions/wt.fish

# Bash
wt completion bash > ~/.local/share/bash-completion/completions/wt

# Zsh — ensure ~/.zfunc exists and is on fpath, then generate
mkdir -p ~/.zfunc
echo 'fpath=(~/.zfunc $fpath)' >> ~/.zshrc  # add once, before compinit
wt completion zsh > ~/.zfunc/_wt
```

## Troubleshooting

- **`wt cd` prints a path instead of changing directory**: the shell wrapper is not loaded, see [Shell Wrapper](#shell-wrapper).
- **The `PR` column is empty or `wt prune` removes nothing**: PR status is read from a local cache. Refresh it with `wt list -R` or `wt prune -R`. This needs an authenticated `gh` or `glab` (`gh auth status`, `glab auth status`).
- **The wrong forge is used** (self-hosted instance, GitLab repo treated as GitHub): map the host or org in the config, see [Self-Hosted Instances](docs/configuration.md#self-hosted-instances) and [Forge Settings](docs/configuration.md#forge-settings).
- **PR status looks wrong**: `wt list --reset-cache -R` clears the cache and fetches again.
- **A repo is not found by name**: `wt repo list` shows the registered names; register a repo with `wt repo add <path>`.
- **Something else fails**: add `-v` to any command to see the `git`, `gh` and `glab` commands it runs.

## Integration with gh-dash

`wt` works great with [gh-dash](https://github.com/dlvhdr/gh-dash). Add a keybinding to checkout PRs as worktrees:

```yaml
# ~/.config/gh-dash/config.yml
keybindings:
  prs:
    - key: O
      command: wt pr checkout {{.RepoName}} {{.PrNumber}}
```

Press `O` to checkout PR → hooks auto-open your editor. The repo must be registered; add `--clone` to clone unregistered repos.

## Development

```bash
just build    # Build ./wt binary
just test     # Run tests
just docs     # Regenerate docs/commands.md after changing commands or flags
just install  # Install to ~/go/bin (+ shell completions + git hooks)
```

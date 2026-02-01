# wt

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
wt config init ~/Git/worktrees           # worktrees dir only
wt config init ~/Git/worktrees ~/Code    # with separate repo dir
```

This creates `~/.config/wt/config.toml` with your worktree directory (and optionally repo directory).

### 2. List Worktrees

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

# Fetch latest before creating (ensures up-to-date base)
wt checkout -b feature-login -f

# Stash local changes and apply them to the new worktree
wt checkout -b feature-login -s

# Add a note to remember what you're working on
wt checkout -b feature-login --note "Implementing OAuth flow"
```

With hooks configured, your editor opens automatically:

```toml
# ~/.config/wt/config.toml
[hooks.vscode]
command = "code {worktree-dir}"
on = ["checkout"]
```

### Reviewing a Pull Request

```bash
# Checkout PR from current repo
wt pr checkout 123

# Checkout PR from a different local repo (by name)
wt pr checkout 123 -r backend-api

# Clone repo you don't have locally and checkout PR
wt pr checkout 456 org/new-repo

# Specify forge type when auto-detection fails
wt pr checkout 123 --forge gitlab
```

View PR details or open in browser:

```bash
wt pr view               # Show PR details
wt pr view -w            # Open PR in browser
wt pr view -r myrepo     # View PR for specific repo
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
wt pr create --title "Add feature" -r myrepo
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

# Remove specific branch worktree
wt prune --branch feature-login -f
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
wt repo
wt repo -l backend    # Filter by label
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

# Create same branch across all backend repos
wt checkout -b feature-auth -l backend

# Or target specific repos by name
wt checkout -b feature-auth -r backend-api -r auth-service

# Run command across repos
wt exec -l backend -- git status
wt exec -r backend-api -r auth-service -- make test
```

### Quick Navigation

```bash
# Jump to repo by name
cd $(wt cd -r backend-api)

# Jump to repo by label (must match exactly one)
cd $(wt cd -l backend)

# Interactive fuzzy search
cd $(wt cd -i)

# Run command in repo
wt exec -r myrepo -- git status
wt exec -r myrepo -- code .
```

### Running Hooks Manually

```bash
# Run a hook on current worktree
wt hook vscode

# Run on specific repo
wt hook vscode -r myrepo

# Run multiple hooks
wt hook vscode kitty

# Run across repos by label
wt hook build -l backend

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

# Set note in specific repo
wt note set "Ready for review" -r myrepo
```

## Configuration

Config file: `~/.config/wt/config.toml`

```bash
wt config init ~/Git/worktrees           # worktrees dir only
wt config init ~/Git/worktrees ~/Code    # with separate repo dir
wt config show                           # Show effective config
wt config hooks                          # List configured hooks
```

### Basic Settings

```toml
# Directory for new worktrees (must be absolute or start with ~)
worktree_dir = "~/Git/worktrees"

# Where repos live (for -r/-l lookup, defaults to worktree_dir)
repo_dir = "~/Git"

# Default sort order for list: "id", "repo", "branch", "commit"
default_sort = "id"

[checkout]
# Folder naming: {repo}, {branch}, {origin}
worktree_format = "{repo}-{branch}"

# Base ref for new branches: "remote" (default) or "local"
base_ref = "remote"

# Auto-fetch base branch before creating new branches (default: false)
auto_fetch = true
```

### Hooks

```toml
[hooks.vscode]
command = "code {worktree-dir}"
description = "Open VS Code"
on = ["checkout", "pr"]  # Auto-run for these commands

[hooks.kitty]
command = "kitty @ launch --type=tab --cwd={worktree-dir}"
description = "Open new kitty tab"
on = ["checkout"]

[hooks.cleanup]
command = "echo 'Removed {branch}'"
on = ["prune"]

[hooks.claude]
command = "kitty @ launch --cwd={worktree-dir} -- claude {prompt:-help me}"
description = "Open Claude with prompt"
# No "on" = only runs via: wt hook claude --arg prompt="..."
```

**Hook triggers:** `checkout`, `pr`, `prune`, `merge`, `all`

**Placeholders:** `{worktree-dir}`, `{repo-dir}`, `{branch}`, `{repo}`, `{origin}`, `{trigger}`, `{key}`, `{key:-default}`

### Forge Settings

Configure forge detection and multi-account auth for PR operations:

```toml
[forge]
default = "github"      # Default forge
default_org = "my-company"  # Default org (allows: wt pr checkout 123 repo)

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
name = "dracula"  # default, dracula, nord, gruvbox, catppuccin-frappe, catppuccin-mocha

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

## Integration with gh-dash

`wt` works great with [gh-dash](https://github.com/dlvhdr/gh-dash). Add a keybinding to checkout PRs as worktrees:

```yaml
# ~/.config/gh-dash/config.yml
keybindings:
  prs:
    - key: O
      command: wt pr checkout {{.PrNumber}} {{.RepoName}}
```

Press `O` to checkout PR → hooks auto-open your editor.

## Shell Completions

```bash
# Fish
wt completion fish > ~/.config/fish/completions/wt.fish

# Bash
wt completion bash > ~/.local/share/bash-completion/completions/wt

# Zsh (add ~/.zfunc to fpath in .zshrc)
wt completion zsh > ~/.zfunc/_wt
```

## Command Reference

Common commands have short aliases (shown in `wt --help`): `co`, `ls`, `p`, `r`, `x`.

| Command | Description |
|---------|-------------|
| `wt checkout` | Checkout worktree for branch |
| `wt list` | List worktrees |
| `wt prune` | Remove merged worktrees |
| `wt repo` | Manage repositories |
| `wt pr checkout` | Checkout PR (clones if needed) |
| `wt pr create` | Create PR for current branch |
| `wt pr view` | View PR details or open in browser |
| `wt pr merge` | Merge PR and clean up |
| `wt exec` | Run command in worktree |
| `wt cd` | Print worktree/repo path |
| `wt note` | Manage branch notes |
| `wt label` | Manage repo labels |
| `wt hook` | Run configured hook |
| `wt config` | Manage configuration |
| `wt init` | Output shell wrapper for cd |
| `wt completion` | Generate shell completions |

Run `wt <command> --help` for detailed usage.

## Development

```bash
make build    # Build ./wt binary
make test     # Run tests
make install  # Install to ~/go/bin
```

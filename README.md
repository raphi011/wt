![Logo](./logo.png)

# wt

Git worktree manager with GitHub/GitLab integration.

## Why wt

Git worktrees let you work on multiple branches simultaneously without stashing or switching—great for juggling a feature branch and a hotfix, or running multiple AI agent sessions in parallel.

But worktrees can pile up fast. You end up with a dozen directories, can't remember which ones are already merged, and need custom scripts to open your editor, create terminal tabs, or clean up stale checkouts.

`wt` solves this:
- **Hooks** auto-run commands when creating/opening worktrees (open editor, spawn terminal tab)
- **Prune** removes merged worktrees and shows PR/MR status so you know what's safe to delete
- **PR checkout** opens pull requests in worktrees for easier code review

## Install

```bash
# Homebrew (macOS/Linux)
brew install --cask raphi011/tap/wt

# Go
go install github.com/raphi011/wt/cmd/wt@latest
```

Requires `git` in PATH. For GitHub repos: `gh` CLI. For GitLab repos: `glab` CLI.

## Quick Start

```bash
# Create config (optional but recommended)
wt config init

# Start working on a new feature
wt add -b my-feature

# Review a PR
wt pr open 123

# Clean up merged worktrees
wt prune
```

## Scenarios

### Starting a New Feature

```bash
# Create worktree with new branch (from origin/main)
wt add -b feature-login

# Create from a different base branch
wt add -b feature-login --base develop

# Fetch latest before creating (ensures up-to-date base)
wt add -b feature-login -f

# Add a note to remember what you're working on
wt add -b feature-login --note "Implementing OAuth flow"
```

With hooks configured, your editor opens automatically:

```toml
# ~/.config/wt/config.toml
[hooks.vscode]
command = "code {path}"
on = ["add"]
```

### Reviewing a Pull Request

```bash
# Checkout PR from current repo
wt pr open 123

# Checkout PR from a different repo (searches in worktree_dir)
wt pr open 123 backend-api

# Clone repo you don't have locally and checkout PR
wt pr clone 456 org/new-repo
```

After review, merge and clean up in one command:

```bash
wt pr merge              # Uses squash by default
wt pr merge -s rebase    # Or specify strategy
wt pr merge --keep       # Merge but keep worktree
```

### Cleaning Up

```bash
# See what worktrees exist
wt list

# Show detailed status for a specific worktree
wt show -i 3

# Remove merged worktrees (uses cached PR status)
wt prune

# Refresh PR status from GitHub/GitLab first
wt prune -R

# Preview what would be removed
wt prune -n

# Also remove worktrees with 0 commits (stale checkouts)
wt prune -c

# Remove specific worktree by ID
wt prune -i 3

# Force remove even if not merged or dirty
wt prune -i 3 -f
```

### Working Across Multiple Repos

Label your repos for batch operations:

```bash
# Add labels to repos
cd ~/Git/backend-api && wt label add backend
cd ~/Git/auth-service && wt label add backend
cd ~/Git/web-app && wt label add frontend

# Create same branch across all backend repos
wt add -b feature-auth -l backend -d ~/Git

# Or target specific repos by name
wt add -b feature-auth -r backend-api -r auth-service -d ~/Git

# Run command across repos
wt exec -l backend -- git status
wt exec -r backend-api -r auth-service -- make test

# List repos and their labels
wt repos
wt repos -l backend
```

### Quick Navigation

```bash
# Jump to worktree by ID
cd $(wt cd -i 3)

# Jump to repo by name
cd $(wt cd -r backend-api)

# Jump to main repo (not worktree)
cd $(wt cd -i 3 -p)

# Run command in worktree without switching
wt exec -i 3 -- git status
wt exec -i 3 -- code .
```

### Running Hooks Manually

```bash
# Run a hook on current worktree
wt hook vscode

# Run on specific worktree
wt hook vscode -i 3

# Run multiple hooks
wt hook vscode kitty

# Run across repos by label
wt hook build -l backend

# Pass custom variables
wt hook claude --arg prompt="implement feature X"

# Preview command without executing
wt hook vscode -n
```

### Branch Notes

```bash
# Set a note (visible in list/prune output)
wt note set "WIP: fixing auth timeout issue"

# Get current note
wt note get

# Clear note
wt note clear

# Set note by worktree ID
wt note set "Ready for review" -i 3
```

### Moving Worktrees / Migrating to wt

Already have worktrees scattered around? Use `wt mv` to consolidate them:

```bash
# Preview what would be moved
wt mv -d ~/Git/worktrees -n

# Move all worktrees from current directory to central location
wt mv -d ~/Git/worktrees

# Move and rename to consistent format
wt mv -d ~/Git/worktrees --format={repo}-{branch}

# Force move even if worktrees have uncommitted changes
wt mv -d ~/Git/worktrees -f
```

This updates git's worktree tracking automatically—no manual fixup needed.

## Configuration

Config file: `~/.config/wt/config.toml`

```bash
wt config init    # Create default config
wt config show    # Show effective config
wt config hooks   # List configured hooks
```

### Basic Settings

```toml
# Directory for new worktrees (must be absolute or start with ~)
worktree_dir = "~/Git/worktrees"

# Where repos live (for -r/-l lookup, defaults to worktree_dir)
repo_dir = "~/Git"

# Folder naming: {repo}, {branch}, {folder}
worktree_format = "{repo}-{branch}"

# Base ref for new branches: "remote" (default) or "local"
base_ref = "remote"
```

### Hooks

```toml
[hooks.vscode]
command = "code {path}"
description = "Open VS Code"
on = ["add", "pr"]  # Auto-run for these commands

[hooks.kitty]
command = "kitty @ launch --type=tab --cwd={path}"
description = "Open new kitty tab"
on = ["add"]

[hooks.cleanup]
command = "echo 'Removed {branch}'"
on = ["prune"]

[hooks.claude]
command = "kitty @ launch --cwd={path} -- claude {prompt:-help me}"
description = "Open Claude with prompt"
# No "on" = only runs via: wt hook claude --arg prompt="..."
```

**Hook triggers:** `add`, `pr`, `prune`, `merge`, `all`

**Placeholders:** `{path}`, `{branch}`, `{repo}`, `{folder}`, `{main-repo}`, `{trigger}`, `{key}`, `{key:-default}`

### Clone Rules

Configure forge detection for `wt pr clone`:

```toml
[clone]
forge = "github"      # Default forge
org = "my-company"    # Default org (allows: wt pr clone 123 repo)

[[clone.rules]]
pattern = "company/*"
forge = "gitlab"
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

## Integration with gh-dash

`wt` works great with [gh-dash](https://github.com/dlvhdr/gh-dash). Add a keybinding to checkout PRs as worktrees:

```yaml
# ~/.config/gh-dash/config.yml
keybindings:
  prs:
    - key: O
      command: wt pr open {{.PrNumber}} {{.RepoName}}
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

| Command | Description |
|---------|-------------|
| `wt add` | Add worktree for branch |
| `wt list` | List worktrees |
| `wt show` | Show worktree details |
| `wt prune` | Remove merged worktrees |
| `wt repos` | List repositories |
| `wt pr open` | Checkout PR from local repo |
| `wt pr clone` | Clone repo and checkout PR |
| `wt pr merge` | Merge PR and clean up |
| `wt exec` | Run command in worktree |
| `wt cd` | Print worktree/repo path |
| `wt mv` | Move worktrees |
| `wt note` | Manage branch notes |
| `wt label` | Manage repo labels |
| `wt hook` | Run configured hook |
| `wt config` | Manage configuration |
| `wt completion` | Generate shell completions |

Run `wt <command> --help` for detailed usage.

## Development

```bash
make build    # Build ./wt binary
make test     # Run tests
make install  # Install to ~/go/bin
```

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

## Getting Started

### 1. Create Config

```bash
wt config init ~/Git/worktrees
```

This creates `~/.config/wt/config.toml` with your worktree directory.

### 2. Migrate Existing Repos (Optional)

If you have repos and worktrees scattered around, consolidate them:

```bash
wt mv -n          # Preview what would be moved
wt mv             # Move to configured directories
```

Scans the current directory for git repos and worktrees. Worktrees are moved to `worktree_dir`, repos to `repo_dir` (if configured, otherwise `worktree_dir`).

### 3. List Worktrees

```bash
wt list -g
```

You're ready to go! See Scenarios below for common workflows.

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
wt pr checkout 123

# Checkout PR from a different local repo (by name)
wt pr checkout 123 -r backend-api

# Clone repo you don't have locally and checkout PR
wt pr checkout 456 org/new-repo
```

View PR details or open in browser:

```bash
wt pr view               # Show PR details
wt pr view -w            # Open PR in browser
wt pr view -i 3          # By worktree ID
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

# By worktree ID (when outside worktree)
wt pr create --title "Add feature" -i 3
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

# Verbose dry-run: see what's skipped and why
wt prune -n -v

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
wt add -b feature-auth -l backend

# Or target specific repos by name
wt add -b feature-auth -r backend-api -r auth-service

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
# Preview what would be moved (destination from config)
wt mv -n

# Move all worktrees to configured worktree_dir
wt mv

# Move and rename to consistent format
wt mv --format={repo}-{branch}

# Force move even if worktrees have uncommitted changes
wt mv -f
```

This updates git's worktree tracking automatically—no manual fixup needed.

### Diagnosing Issues

If worktrees get into a bad state (broken links, stale cache entries):

```bash
# Check for issues
wt doctor

# Auto-fix recoverable issues
wt doctor --fix

# Rebuild cache from scratch (worktrees get new IDs)
wt doctor --reset
```

## Configuration

Config file: `~/.config/wt/config.toml`

```bash
wt config init ~/Git/worktrees  # Create config with worktree dir
wt config show                  # Show effective config
wt config hooks                 # List configured hooks
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

# Default sort order for list: "id", "repo", "branch", "commit"
default_sort = "id"
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

Configure forge detection for `wt pr checkout` when cloning new repos:

```toml
[clone]
forge = "github"      # Default forge
org = "my-company"    # Default org (allows: wt pr checkout 123 repo)

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

Common commands have short aliases (shown in `wt --help`): `a`, `ls`, `s`, `p`, `r`, `x`.

| Command | Description |
|---------|-------------|
| `wt add` | Add worktree for branch |
| `wt list` | List worktrees |
| `wt show` | Show worktree details |
| `wt prune` | Remove merged worktrees |
| `wt repos` | List repositories |
| `wt pr checkout` | Checkout PR (clones if needed) |
| `wt pr create` | Create PR for current branch |
| `wt pr view` | View PR details or open in browser |
| `wt pr merge` | Merge PR and clean up |
| `wt exec` | Run command in worktree |
| `wt cd` | Print worktree/repo path |
| `wt mv` | Move worktrees |
| `wt note` | Manage branch notes |
| `wt label` | Manage repo labels |
| `wt hook` | Run configured hook |
| `wt config` | Manage configuration |
| `wt doctor` | Diagnose and repair cache |
| `wt completion` | Generate shell completions |

Run `wt <command> --help` for detailed usage.

## Development

```bash
make build    # Build ./wt binary
make test     # Run tests
make install  # Install to ~/go/bin
```

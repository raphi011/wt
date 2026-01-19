# wt

Git worktree manager with GitHub/GitLab integration.

## Table of Contents

- [Why wt](#why-wt)
- [Install](#install)
- [Usage](#usage)
- [Configuration](#configuration)
- [Hook Examples](#hook-examples)
- [Shell Completions](#shell-completions)
- [Development](#development)

## Why wt

Git worktrees let you work on multiple branches simultaneously without stashing or switching—great for juggling a feature branch and a hotfix, or running multiple AI agent sessions in parallel.

But worktrees can pile up fast. You end up with a dozen directories, can't remember which ones are already merged, and need custom scripts to open your editor, create terminal tabs, or clean up stale checkouts.

`wt` solves this:
- **Hooks** auto-run commands when creating/opening worktrees (open editor, spawn terminal tab)
- **Tidy** removes merged worktrees and shows PR/MR status so you know what's safe to delete
- **PR checkout** opens pull requests in worktrees for easier code review

## Install

**Homebrew (macOS/Linux):**
```bash
brew install raphi011/tap/wt
```

**Go:**
```bash
go install github.com/raphi011/wt/cmd/wt@latest
```

Requires `git` in PATH. For GitHub repos: `gh` CLI. For GitLab repos: `glab` CLI.

## Usage

```bash
# Create worktree (creates new branch if needed)
wt create feature-branch              # in cwd
wt create feature-branch -d ~/Git     # in specific dir

# Open worktree for existing local branch
wt open existing-branch               # in cwd
wt open existing-branch -d ~/Git      # in specific dir

# Open worktree for a GitHub PR or GitLab MR
wt pr open 123                        # PR/MR from current repo
wt pr open 123 myrepo                 # find repo by name in cwd
wt pr open 123 org/repo               # clone if not found locally
wt pr open 123 -d ~/Git               # specify base directory

# Hooks (auto-run based on "on" config, or explicit)
wt create branch                      # runs hooks with on=["create"]
wt create branch --hook=vscode        # run specific hook
wt create branch --no-hook            # skip all hooks

# Tidy up merged worktrees
wt tidy                               # in cwd
wt tidy -d ~/Git/worktrees            # in specific dir
wt tidy -n                            # dry run
wt tidy -c                            # also remove clean (0 commits ahead)
wt tidy --refresh-pr                  # force refresh PR cache
wt tidy --no-hook                     # skip post-removal hooks

# List worktrees
wt list                               # in cwd (filters to current repo if in one)
wt list -d ~/Git/worktrees
wt list --json

# Move worktrees to another directory
wt mv -d ~/Git/worktrees              # move all worktrees from cwd to dir
wt mv -d ~/Git --format={branch-name} # move and rename using format
wt mv --dry-run -d ~/Git              # preview what would be moved
wt mv -f -d ~/Git                     # force move dirty worktrees

# Configuration
wt config init                        # create ~/.config/wt/config.toml
wt config hooks                       # list configured hooks
```

## Configuration

Config file: `~/.config/wt/config.toml`

```toml
# Must be absolute path or start with ~ (no relative paths)
default_path = "~/Git/worktrees"

# Worktree folder naming format
# Placeholders: {git-origin}, {branch-name}, {folder-name}
worktree_format = "{git-origin}-{branch-name}"

[hooks.kitty]
command = "kitty @ launch --type=tab --cwd={path}"
description = "Open new kitty tab"
on = ["create", "open"]  # auto-run for create/open

[hooks.pr-review]
command = "cd {path} && npm install && code {path}"
description = "Setup PR for review"
on = ["pr"]  # auto-run when opening PRs

[hooks.cleanup]
command = "echo 'Removed {branch} from {repo}'"
description = "Log removed branches"
on = ["tidy"]  # auto-run when removing worktrees

[hooks.vscode]
command = "code {path}"
description = "Open VS Code"
# no "on" - only runs via --hook=vscode
```

### Worktree Format Placeholders

| Placeholder | Value |
|-------------|-------|
| `{git-origin}` | Repo name from `git remote get-url origin` |
| `{branch-name}` | Branch name as provided |
| `{folder-name}` | Folder name of the git repo on disk |

### Hook Options

| Option | Description |
|--------|-------------|
| `command` | Shell command to run (required) |
| `description` | Human-readable description |
| `on` | Commands to auto-run on: `["create", "open", "pr", "tidy", "all"]` (empty = only via `--hook`) |
| `run_on_exists` | Run even if worktree already existed (default: false) |

### Hook Placeholders

| Placeholder | Value |
|-------------|-------|
| `{path}` | Absolute worktree path |
| `{branch}` | Branch name |
| `{repo}` | Repo name from origin |
| `{folder}` | Main repo folder name |
| `{main-repo}` | Main repo path |
| `{trigger}` | Command that triggered the hook (create, open, pr, tidy) |

### Clone Rules (for `wt pr open`)

When cloning a repo via `wt pr open org/repo`, configure which forge to use:

```toml
[clone]
default = "github"  # or "gitlab"

[[clone.rules]]
pattern = "company/*"
forge = "gitlab"

[[clone.rules]]
pattern = "oss/*"
forge = "github"
```

Rules are matched in order; first match wins. Supports glob patterns with `*`.

## Hook Examples

### VS Code

```toml
[hooks.vscode]
command = "code {path}"
description = "Open worktree in VS Code"
on = ["create", "open", "pr"]
```

### tmux

```toml
[hooks.tmux]
command = "tmux new-window -c {path} -n {branch}"
description = "Open new tmux window in worktree"
on = ["create", "open"]
```

### gh dash

`wt` works well with [gh dash](https://github.com/dlvhdr/gh-dash) for reviewing PRs. Configure a keybinding to open PRs as worktrees:

```yaml
# ~/.config/gh-dash/config.yml
keybindings:
  prs:
    - key: O
      command: wt pr open {{.PrNumber}} {{.RepoName}}
```

Combined with hooks, you get a seamless workflow: press `O` in gh dash to checkout the PR as a worktree and automatically open it in your editor.

## Shell Completions

```bash
wt completion fish > ~/.config/fish/completions/wt.fish
```

## Development

```bash
make build    # build ./wt binary
make test     # run tests
make install  # install to ~/go/bin
```

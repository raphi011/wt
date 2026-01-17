# wt

Git worktree manager with GitHub PR integration.

## Install

```bash
go install github.com/raphaelgruber/wt/cmd/wt@latest
```

Requires `git` and `gh` CLI in PATH.

## Usage

```bash
# Create worktree
wt create feature-branch          # use default dir
wt create feature-branch -d ..    # next to repo
wt create branch -d ~/worktrees   # specific location

# Create with hooks
wt create branch                  # runs default hook
wt create branch --hook=vscode    # run specific hook
wt create branch --no-hook        # skip hooks

# Cleanup merged worktrees
wt clean                          # current dir
wt clean ~/worktrees              # specific dir
wt clean -n                       # dry run

# List worktrees
wt list
wt list --json

# Configuration
wt config init                    # create ~/.config/wt/config.toml
wt config hooks                   # list configured hooks
```

## Configuration

Config file: `~/.config/wt/config.toml`

```toml
default_path = "."
worktree_format = "{git-origin}-{branch-name}"

[hooks]
default = "kitty"

[hooks.kitty]
command = "kitty @ launch --type=tab --cwd={path}"
description = "Open new kitty tab"

[hooks.vscode]
command = "code {path}"
description = "Open VS Code"
```

### Hook Placeholders

| Placeholder | Value |
|-------------|-------|
| `{path}` | Worktree path |
| `{branch}` | Branch name |
| `{repo}` | Repo name from origin |
| `{folder}` | Main repo folder |
| `{main-repo}` | Main repo path |

## Shell Completions

```bash
wt completion fish > ~/.config/fish/completions/wt.fish
```

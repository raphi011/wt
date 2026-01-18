# wt

Git worktree manager with GitHub PR integration.

## Install

```bash
go install github.com/raphaelgruber/wt/cmd/wt@latest
```

Requires `git` and `gh` CLI in PATH.

## Usage

```bash
# Create worktree (creates new branch if needed)
wt create feature-branch              # in cwd
wt create feature-branch -d ~/Git     # in specific dir

# Open worktree for existing local branch
wt open existing-branch               # in cwd
wt open existing-branch -d ~/Git      # in specific dir

# Open worktree for a GitHub PR
wt pr open 123                        # PR from current repo
wt pr open 123 myrepo                 # find repo by name in cwd
wt pr open 123 org/repo               # clone if not found locally
wt pr open 123 -d ~/Git               # specify base directory

# Hooks (auto-run based on "on" config, or explicit)
wt create branch                      # runs hooks with on=["create"]
wt create branch --hook=vscode        # run specific hook
wt create branch --no-hook            # skip all hooks

# Cleanup merged worktrees
wt clean                              # in cwd
wt clean -d ~/Git/worktrees           # in specific dir
wt clean -n                           # dry run
wt clean -e                           # also remove empty (0 commits ahead)
wt clean --refresh-pr                 # force refresh PR cache

# List worktrees
wt list                               # in cwd (filters to current repo if in one)
wt list -d ~/Git/worktrees
wt list --json

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
| `on` | Commands to auto-run on: `["create", "open", "pr"]` (empty = only via `--hook`) |
| `run_on_exists` | Run even if worktree already existed (default: false) |

### Hook Placeholders

| Placeholder | Value |
|-------------|-------|
| `{path}` | Absolute worktree path |
| `{branch}` | Branch name |
| `{repo}` | Repo name from origin |
| `{folder}` | Main repo folder name |
| `{main-repo}` | Main repo path |

## Integration with gh dash

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

# Configuration

Global config: `~/.wt/config.toml`
Local config: `.wt.toml` (in repo root)

```bash
wt config init               # Create default global config
wt config init --local       # Create per-repo .wt.toml
wt config init --stdout      # Print config to stdout
wt config show               # Show effective config (merged if in a repo)
wt config show myrepo        # Show effective config for specific repo
wt config hooks              # List hooks with source annotations
```

## Basic Settings

```toml
# Default sort order for list: "date", "repo", "branch"
default_sort = "date"

# Labels applied to newly auto-registered repos
# default_labels = ["work"]

[checkout]
# Folder naming: {repo}, {branch}
worktree_format = ".worktrees/{branch}"

# Base ref for new branches: "remote" (default) or "local"
# - "remote": branches from origin/<base> (ensures latest remote state)
# - "local": branches from local <base> (useful for offline work)
base_ref = "remote"

# Auto-fetch from origin before checkout (default: false)
# Note: with base_ref="local" and an explicit --base, --fetch is skipped (warns) since fetch doesn't affect local refs
# auto_fetch = false

# Auto-set upstream tracking (default: false)
# New branches (-b) are pushed to origin first; existing branches only get
# an upstream if origin/<branch> exists
# set_upstream = false

[prune]
# Delete local branches after worktree removal (default: false)
# delete_local_branches = false
# Days before a worktree's commit age is highlighted as stale and eligible
# for `wt prune --stale` (default: 14, 0 = disabled)
# stale_days = 14

[clone]
# How `wt repo clone` and `wt pr checkout --clone` clone: "regular" (default) or "bare"
# mode = "regular"
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

## Hooks

See [Hooks](hooks.md).

## Forge Settings

Configure forge detection and multi-account auth for PR operations:

```toml
[forge]
default = "github"      # Default forge
default_org = "my-company"  # Default org (allows: wt repo clone repo)

[[forge.rules]]
pattern = "company/*"
type = "gitlab"

[[forge.rules]]
pattern = "work-org/*"
type = "github"
user = "work-account"  # Use specific gh account for matching repos
```

## Merge Settings

```toml
[merge]
strategy = "squash"  # squash, rebase, or merge (rebase is not supported on GitLab)
```

## Preserve Settings

Symlink files from the repo root into new worktrees created with `wt checkout` or `wt pr checkout`. Useful for keeping local configuration (`.env`, `.envrc`, etc.) in sync across worktrees — edits in any worktree are instantly visible in all others.

```toml
[preserve]
paths = [".env", ".envrc"]
```

- **paths** — relative paths from the repo root to symlink (e.g., `".env"`, `"config/.env"`)

Paths that don't exist in the repo root are silently skipped. Existing files in the target worktree are never overwritten. Use `--no-preserve` on `wt checkout` or `wt pr checkout` to skip.

## Self-Hosted Instances

```toml
[hosts]
"github.mycompany.com" = "github"
"gitlab.internal.corp" = "gitlab"
```

Authenticate the forge CLI against the host as well:

```bash
gh auth login --hostname github.mycompany.com
glab auth login --hostname gitlab.internal.corp
```

## Theming

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

Available color keys: `primary`, `accent`, `success`, `error`, `muted`, `normal`, `info`, `warning`.

The `WT_THEME` and `WT_THEME_MODE` environment variables override `theme.name` and `theme.mode`.

## Per-Repo Config

Place a `.wt.toml` file in your repo root to override global settings for that repo:

```bash
wt config init --local       # Creates .wt.toml in current repo root
```

Local settings merge with global config — unset fields inherit from global. Available overrides:

```toml
# .wt.toml — per-repo overrides

[checkout]
worktree_format = "{branch}"  # replaces global
base_ref = "local"            # replaces global
auto_fetch = true             # replaces global
set_upstream = true           # replaces global

[clone]
mode = "bare"                 # replaces global

[merge]
strategy = "rebase"           # replaces global

[prune]
delete_local_branches = true  # replaces global

[forge]
default = "gitlab"            # replaces global

[preserve]
paths = [".env.local"]        # appended to global (deduplicated)

# Hooks merge by name — add new hooks or override global ones
[hooks.setup]
command = "go mod download"
on = ["checkout"]

# Disable a global hook for this repo
[hooks.npm-install]
enabled = false
```

**Not overridable** (global-only): `default_sort`, `default_labels`, `prune.stale_days`, `forge.default_org`, `forge.rules`, `hosts`, `theme`.

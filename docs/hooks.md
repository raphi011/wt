# Hooks

Hooks are shell commands that run before or after `wt` operations. They are defined under `[hooks.<name>]` in `~/.wt/config.toml` or a repo's `.wt.toml` (see [Per-Repo Config](configuration.md#per-repo-config)). Each hook has a `command`, optional `description`, and optional `on` triggers. See [Getting Started](../README.md#5-configure-hooks) for examples; `wt config hooks` lists the hooks in effect.

## Triggers

Syntax for the `on` field: `[before:|after:]trigger[:subtype]`

| Trigger | Subtypes | Description |
|---------|----------|-------------|
| `checkout` | `create`, `open`, `pr` | Worktree checkout |
| `prune` | — | Worktree removal |
| `merge` | — | PR merge |
| `all` | — | Matches all triggers |

**Timing prefix:**

| Prefix | Default | Description |
|--------|---------|-------------|
| *(none)* | `after` | Runs after the operation; a failure is logged as a warning |
| `after:` | — | Explicit after (same as no prefix) |
| `before:` | — | Runs before the operation; non-zero exit aborts |

**Examples:**

```toml
on = ["checkout"]              # All checkouts (after)
on = ["checkout:pr"]           # PR checkouts only
on = ["before:prune"]          # Pre-prune guard (can abort)
on = ["before:checkout:pr"]    # Before PR checkout only
on = ["checkout", "merge"]     # Multiple triggers
```

Hooks without `on` only run when invoked explicitly via `wt hook <name>` or `--hook <name>`.

With `--hook <name>` only the named hooks run. A named hook runs in the phase its `on` names for the command (`before:checkout` runs before a checkout and can abort it); without a matching `on` it runs after the command.

## Placeholders

Substituted in the hook `command` before execution:

| Placeholder | Description |
|-------------|-------------|
| `{worktree-dir}` | Absolute path to the worktree |
| `{repo-dir}` | Absolute path to the main repo (bare root or `.git` parent) |
| `{branch}` | Branch name |
| `{repo}` | Repo name (as registered in `wt repo list`) |
| `{trigger}` | Command that triggered the hook (`checkout`, `prune`, `merge`, `run`) |
| `{action}` | Checkout subtype: `create`, `open`, `pr`, or `manual` (for `wt hook`); empty for `prune` and `merge` |
| `{phase}` | Hook timing: `before` or `after` |
| `{config-dir}` | Absolute path to the wt config directory (`~/.wt/`) |
| `{pr-number}` | PR/MR number (empty for non-PR checkouts) |
| `{pr-repo}` | Forge repo path, e.g. `owner/repo` (empty for non-PR checkouts) |
| `{key}` | Custom variable from `--arg key=value` (empty if unset) |
| `{key:-default}` | Custom variable with fallback value if unset |
| `{key:+text}` | Expands to `text` if key is set and non-empty, otherwise empty |

**Args:** Pass `--arg key=value` or `--arg key` (bare boolean, sets to `"true"`)

## Execution

Hooks are executed via `sh -c`. Placeholders like `{worktree-dir}` are replaced with raw text before the command runs — no automatic escaping or quoting is applied.

## Hook Working Directory

Hooks run with a working directory that depends on the command and phase:

| Command | `before` CWD | `after` CWD |
|---------|-------------|------------|
| `checkout`, worktree is created | Repo root (worktree does not exist yet) | Worktree directory |
| `checkout`, worktree exists | Worktree directory | Worktree directory |
| `prune` | Worktree directory (still exists) | Repo root (worktree deleted) |
| `merge` | Current directory | Repo root |

The `checkout` rows also apply to `wt pr checkout`. Before hooks of a checkout run before anything is fetched or created, so a failing before hook leaves no worktree behind. `{worktree-dir}` then holds the path the worktree will get.

Since the working directory is already set, `cd '{worktree-dir}'` is unnecessary in after-checkout hooks. For other commands, use `{worktree-dir}` or `{repo-dir}` placeholders if you need a specific directory.

## Hook Execution Order

Hooks run in **alphabetical order** by name. Use naming prefixes to control ordering:

```toml
[hooks.01-install]
command = "npm install"
on = ["checkout"]

[hooks.02-lint]
command = "npm run lint"
on = ["checkout"]

[hooks.99-open-editor]
command = "code '{worktree-dir}'"
on = ["checkout"]
```

TUI programs (editors, `claude`, interactive CLIs) work as hooks because they inherit the terminal's stdin/stdout/stderr. Place them last alphabetically so non-interactive hooks complete first.

## Quoting Placeholders

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

The same applies to all placeholders (`{repo-dir}`, `{branch}`, `{repo}`, `{trigger}`) and custom `--arg` variables:

```toml
[hooks.claude]
command = "claude '{prompt:-help me}'"
```

## Conditional Placeholders

Use `{key:+text}` to include text only when an arg is set (and non-empty). This is useful for optional flags:

```toml
[hooks.claude]
command = "claude {skip:+--dangerously-skip-permissions} -p '{prompt:-help}'"
```

```bash
# Without skip — flag omitted
wt hook claude -a prompt="implement auth"
# → claude  -p 'implement auth'

# With skip — flag included (bare -a key sets value to "true")
wt hook claude -a skip -a prompt="implement auth"
# → claude --dangerously-skip-permissions -p 'implement auth'
```

## Multiline Hooks

Use TOML triple-quoted strings for multi-step hooks:

```toml
[hooks.setup]
command = '''
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
npm install
npm run build
'''
on = ["checkout"]
```

## Piping Content via Stdin

Use `--arg key=-` to pipe stdin into a variable:

```bash
echo "implement auth" | wt hook claude --arg prompt=-
```

Multiple keys can read from the same stdin (all keys receive identical content):

```bash
cat spec.md | wt hook claude --arg prompt=- --arg context=-
```

# wt Release Checklist

This checklist ensures all functionality works correctly and documentation is consistent before a release.

**Review Process:** For each section, Claude does initial review, user verifies, then tick off when both agree.

---

## Quick Reference

- [ ] [1. Core Commands](#1-core-commands)
- [ ] [2. PR Commands](#2-pr-commands)
- [ ] [3. Utility Commands](#3-utility-commands)
- [ ] [4. Configuration Commands](#4-configuration-commands)
- [ ] [5. Configuration System](#5-configuration-system)
- [ ] [6. Hook System](#6-hook-system)
- [ ] [7. Shell Completions](#7-shell-completions)
- [ ] [8. Documentation Consistency](#8-documentation-consistency)
- [ ] [9. Build & Release](#9-build--release)

---

## 1. Core Commands

### 1.1 `wt add`

- [ ] **Basic add** - `wt add <branch>` creates worktree for existing branch
- [ ] **New branch** - `wt add -b <branch>` creates new branch and worktree
- [ ] **Directory flag** - `-d/--dir` overrides default path
- [ ] **Multi-repo by name** - `-r <repo> [-r <repo>...]` targets multiple repos
- [ ] **Multi-repo by label** - `-l <label> [-l <label>...]` targets repos with label
- [ ] **Note flag** - `--note` sets branch note on creation
- [ ] **Hook control** - `--hook`, `--no-hook`, `-a/--arg` work correctly
- [ ] **Error handling** - proper errors for missing branch, invalid dir, etc.

### 1.2 `wt list`

- [ ] **Basic list** - shows worktrees with stable IDs
- [ ] **JSON output** - `--json` produces valid JSON
- [ ] **Global flag** - `-g/--global` shows all repos
- [ ] **Sort options** - `-s {id|repo|branch}` sorts correctly
- [ ] **Refresh flag** - `-r/--refresh` fetches latest PR status
- [ ] **Directory flag** - `-d/--dir` scans correct directory

### 1.3 `wt show`

- [ ] **Current worktree** - works without `-i` inside worktree
- [ ] **By ID** - `-i <id>` shows specific worktree
- [ ] **JSON output** - `--json` produces valid JSON
- [ ] **Refresh flag** - `-r/--refresh` fetches latest PR status
- [ ] **Shows commits** - displays commits ahead/behind
- [ ] **Shows changes** - displays uncommitted changes
- [ ] **Shows PR info** - displays PR/MR status if exists

### 1.4 `wt prune`

- [ ] **Interactive prune** - shows table, confirms before removing
- [ ] **By ID** - `-i <id>` removes specific worktree(s)
- [ ] **Dry run** - `-n/--dry-run` previews without removing
- [ ] **Force** - `-f/--force` removes even if dirty/unmerged
- [ ] **Include clean** - `-c/--include-clean` also removes clean worktrees
- [ ] **Global** - `-g/--global` prunes all repos
- [ ] **Refresh** - `-r/--refresh` fetches latest PR status first
- [ ] **Reset cache** - `--reset-cache` clears cache and resets IDs
- [ ] **Hook control** - `--hook`, `--no-hook`, `-a/--arg` work correctly

---

## 2. PR Commands

### 2.1 `wt pr checkout`

- [ ] **Open by number** - `wt pr checkout <number>` from inside repo
- [ ] **Open with repo** - `wt pr checkout <number> <repo>` finds repo locally
- [ ] **Hook control** - `--hook`, `--no-hook`, `-a/--arg` work correctly
- [ ] **GitHub support** - works with `gh` CLI
- [ ] **GitLab support** - works with `glab` CLI
- [ ] **Local repo flag** - `-r/--repository` finds repo by name
- [ ] **Clone mode** - positional `org/repo` clones and creates worktree
- [ ] **Org default** - respects `[forge] default_org` config in clone mode
- [ ] **Forge flag** - `--forge {github|gitlab}` overrides detection in clone mode
- [ ] **Clone rules** - respects pattern-based forge routing in clone mode
- [ ] **Note flag** - `--note` sets branch note
- [ ] **Mutual exclusion** - errors if both positional `org/repo` and `-r` flag used

### 2.2 `wt pr view`

- [ ] **View current** - works without `-i` inside worktree
- [ ] **View by ID** - `-i <id>` views specific worktree's PR
- [ ] **Web flag** - `-w/--web` opens PR in browser
- [ ] **GitHub support** - works with `gh pr view`
- [ ] **GitLab support** - works with `glab mr view`

### 2.3 `wt pr merge`

- [ ] **Merge current** - works without `-i` inside worktree
- [ ] **Merge by ID** - `-i <id>` merges specific worktree's PR
- [ ] **Strategy flag** - `-s {squash|rebase|merge}` uses correct strategy
- [ ] **Keep flag** - `-k/--keep` preserves worktree after merge
- [ ] **Hook control** - `--hook`, `--no-hook`, `-a/--arg` work correctly
- [ ] **Cleanup** - removes worktree and branch after merge (unless --keep)

---

## 3. Utility Commands

### 3.1 `wt exec`

- [ ] **Single ID** - `wt exec -i <id> -- <cmd>` runs in one worktree
- [ ] **Multiple IDs** - `-i <id> -i <id>` runs in multiple worktrees
- [ ] **Requires ID** - fails appropriately when `-i` not provided
- [ ] **Command execution** - command runs with correct working directory
- [ ] **Exit codes** - propagates command exit codes

### 3.2 `wt cd`

- [ ] **Print path** - `wt cd -i <id>` prints worktree path
- [ ] **Project flag** - `-p/--project` prints main repo path instead
- [ ] **Requires ID** - fails appropriately when `-i` not provided
- [ ] **Shell integration** - output usable in `$(wt cd -i X)` syntax

### 3.3 `wt mv`

- [ ] **Move worktrees** - moves all worktrees to new directory
- [ ] **Format flag** - `--format` renames during move
- [ ] **Dry run** - `-n/--dry-run` previews without moving
- [ ] **Force** - `-f/--force` moves dirty worktrees
- [ ] **Git worktree repair** - updates git worktree paths

### 3.4 `wt note`

- [ ] **Set note** - `wt note set "text"` sets note on current branch
- [ ] **Get note** - `wt note get` retrieves note
- [ ] **Clear note** - `wt note clear` removes note
- [ ] **By ID** - `-i <id>` operates on specific worktree
- [ ] **Default subcommand** - `wt note` defaults to `get`

### 3.5 `wt label`

- [ ] **Add label** - `wt label add <label>` adds to current repo
- [ ] **Remove label** - `wt label remove <label>` removes from repo
- [ ] **List labels** - `wt label list` shows repo's labels
- [ ] **List all** - `-a/--all` lists labels from all repos in dir
- [ ] **Clear labels** - `wt label clear` removes all labels
- [ ] **Stored in git config** - labels persist in `wt.labels`

### 3.6 `wt hook`

- [ ] **Run by name** - `wt hook <name>` runs specific hook
- [ ] **Multiple hooks** - `wt hook <name1> <name2>` runs multiple
- [ ] **By ID** - `-i <id>` runs in specific worktree
- [ ] **Multiple IDs** - `-i <id> -i <id>` runs for multiple worktrees
- [ ] **Arg flag** - `-a/--arg KEY=VALUE` passes variables
- [ ] **Stdin support** - `-a KEY=-` reads from stdin
- [ ] **Unknown hook error** - proper error for non-existent hook

---

## 4. Configuration Commands

### 4.1 `wt config init`

- [ ] **Creates config** - creates `~/.config/wt/config.toml`
- [ ] **No overwrite** - fails if file exists (without --force)
- [ ] **Force flag** - `-f/--force` overwrites existing
- [ ] **Stdout flag** - `-s/--stdout` prints config to stdout instead of file
- [ ] **Template quality** - generated config has good comments/examples

### 4.2 `wt config show`

- [ ] **Shows config** - displays effective configuration
- [ ] **JSON output** - `--json` produces valid JSON
- [ ] **All sections** - shows all non-empty config sections

### 4.3 `wt config hooks`

- [ ] **Lists hooks** - shows all configured hooks
- [ ] **JSON output** - `--json` produces valid JSON
- [ ] **Shows metadata** - displays command, description, triggers

### 4.4 `wt completion`

- [ ] **Fish** - `wt completion fish` generates valid fish script
- [ ] **Bash** - `wt completion bash` generates valid bash script
- [ ] **Zsh** - `wt completion zsh` generates valid zsh script
- [ ] **Error for invalid** - fails for unsupported shells

---

## 5. Configuration System

### 5.1 Config Loading

- [ ] **Default location** - reads from `~/.config/wt/config.toml`
- [ ] **Missing file OK** - no error when file doesn't exist
- [ ] **Parse errors** - reports TOML syntax errors clearly
- [ ] **Tilde expansion** - `~` in paths expanded to home dir

### 5.2 Config Options

- [ ] **default_path** - used when `-d` not specified
- [ ] **worktree_format** - controls folder naming with placeholders
- [ ] **forge.default** - default forge for `pr checkout` clone mode
- [ ] **forge.default_org** - default org for repo names
- [ ] **forge.rules** - pattern-based forge routing works
- [ ] **forge.rules[].user** - multi-account auth with gh CLI
- [ ] **merge.strategy** - default merge strategy
- [ ] **hosts** - custom domain to forge mapping

### 5.3 Config Validation

- [ ] **default_path** - rejects relative paths (`.`, `..`)
- [ ] **forge.default** - only accepts github/gitlab/empty
- [ ] **merge.strategy** - only accepts squash/rebase/merge/empty
- [ ] **hosts values** - only accepts github/gitlab

---

## 6. Hook System

### 6.1 Hook Configuration

- [ ] **command** - shell command executes correctly
- [ ] **description** - shows in `config hooks` output
- [ ] **on triggers** - `add`, `pr`, `prune`, `merge`, `all`
- [ ] **Manual-only** - hooks without `on` only run via `--hook`

### 6.2 Hook Placeholders

- [ ] **{path}** - worktree absolute path
- [ ] **{branch}** - branch name
- [ ] **{repo}** - repo name from origin
- [ ] **{folder}** - main repo folder name
- [ ] **{main-repo}** - main repo path
- [ ] **{trigger}** - triggering command name
- [ ] **Shell quoting** - values properly escaped

### 6.3 Custom Variables

- [ ] **-a KEY=VALUE** - passed to hook command
- [ ] **{key}** placeholder - substituted in command
- [ ] **{key:-default}** - default value syntax
- [ ] **KEY=-** - reads value from stdin

### 6.4 Hook Execution

- [ ] **Working directory** - correct for each trigger type
- [ ] **Error propagation** - hook failures reported
- [ ] **Multiple hooks** - all matching hooks run
- [ ] **--no-hook** - skips all hooks

---

## 7. Shell Completions

### 7.1 Fish Completions

- [ ] **Commands complete** - all commands suggested
- [ ] **Flags complete** - all flags for each command
- [ ] **Branch completion** - branch names complete
- [ ] **ID completion** - worktree IDs complete
- [ ] **Repo completion** - repo names complete
- [ ] **Label completion** - label names complete
- [ ] **Hook completion** - hook names complete

### 7.2 Bash Completions

- [ ] **Commands complete** - all commands suggested
- [ ] **Flags complete** - all flags for each command
- [ ] **Dynamic completion** - branches, IDs, repos, labels, hooks

### 7.3 Zsh Completions

- [ ] **Commands complete** - all commands suggested
- [ ] **Flags complete** - all flags for each command
- [ ] **Dynamic completion** - branches, IDs, repos, labels, hooks

---

## 8. Documentation Consistency

### 8.1 README.md

- [ ] **All commands listed** - every command has usage example
- [ ] **Flags documented** - all important flags shown
- [ ] **Config sections** - all config options explained
- [ ] **Hook examples** - practical hook examples included
- [ ] **Completion setup** - all shells documented

### 8.2 CLAUDE.md

- [ ] **Commands accurate** - CLI commands section up to date
- [ ] **Flags documented** - standard flags list accurate
- [ ] **Architecture current** - package structure matches code

### 8.3 Help Text

- [ ] **-h/--help** - all commands have help
- [ ] **Examples** - Help() methods have examples
- [ ] **Consistent** - help matches actual behavior

### 8.4 Config Template

- [ ] **All options** - template shows all config options
- [ ] **Good comments** - explains what each option does
- [ ] **Working examples** - example hooks are valid

---

## 9. Build & Release

### 9.1 Build

- [ ] **make build** - produces working binary
- [ ] **make test** - all tests pass
- [ ] **make install** - installs to ~/go/bin

### 9.2 GoReleaser

- [ ] **Config valid** - `.goreleaser.yaml` parses
- [ ] **Targets correct** - darwin/linux, amd64/arm64
- [ ] **Changelog groups** - conventional commits grouped correctly

### 9.3 Homebrew

- [ ] **Cask config** - homebrew_casks section correct
- [ ] **Tap repo** - raphi011/homebrew-tap exists and accessible

---

## Section Details

Each section below provides context for the checklist items above.

### Section 1: Core Commands

**wt add** creates worktrees for branches. Key behaviors:
- Inside repo: uses current repo's origin
- Outside repo with `-r`: targets named repos in default_path
- Outside repo with `-l`: targets repos with matching labels
- Directory resolution: flag > env WT_DEFAULT_PATH > config > cwd

**wt list** shows worktrees with stable IDs. IDs persist across sessions via `.wt-cache.json`.

**wt show** displays detailed info: commits ahead/behind, uncommitted changes, PR/MR status.

**wt prune** removes worktrees. Safety checks: merged status, clean working directory. PR status fetched from GitHub/GitLab.

### Section 2: PR Commands

**wt pr checkout** checks out a PR, cloning the repo first if needed. Use positional `org/repo` for clone mode, or `-r` flag for local repo.

**wt pr view** shows PR details or opens in browser. Uses `-w/--web` flag for browser.

**wt pr merge** merges the PR via forge CLI and cleans up the worktree.

Forge detection: auto from remote URL, can override with `--forge` or config rules.

### Section 3: Utility Commands

**wt exec** runs shell commands in worktree(s). Useful for bulk operations.

**wt cd** prints path for shell scripting: `cd $(wt cd -i 3)`

**wt mv** relocates worktrees and repairs git worktree paths.

**wt note** stores annotations on branches via git config.

**wt label** tags repos for multi-repo targeting with `wt add -l`.

**wt hook** manually triggers configured hooks.

### Section 4: Configuration Commands

**wt config init** creates the config file with extensive comments.

**wt config show** displays the effective config (merges defaults).

**wt config hooks** lists hooks for discoverability.

**wt completion** generates shell-specific completion scripts.

### Section 5: Configuration System

Config file at `~/.config/wt/config.toml` using TOML format.

Key validation rules:
- `default_path` must be absolute (or ~-prefixed)
- Forge values: "github" or "gitlab" only
- Strategy values: "squash", "rebase", "merge" only

### Section 6: Hook System

Hooks run shell commands with placeholder substitution.

Automatic triggers: hooks with `on = ["add"]` run automatically.
Manual triggers: hooks without `on` only run via `--hook=name`.

Working directory varies by trigger:
- add/pr/merge: worktree path
- prune: main repo (worktree already deleted)

### Section 7: Shell Completions

Completions generated from Go code in `cmd/wt/main.go`.

Dynamic completions fetch: branches (git), IDs (cache), repos (directory scan), labels (git config), hooks (config file).

### Section 8: Documentation Consistency

Three documentation sources must stay in sync:
1. README.md - user-facing
2. CLAUDE.md - developer-facing
3. Built-in help text - CLI users

Common issues: new flags not documented, removed features still listed.

### Section 9: Build & Release

GoReleaser handles builds and releases on tag push.

Homebrew uses casks (pre-compiled binaries), not formulas (source builds).

Conventional commits drive changelog grouping: feat/fix/docs/etc.

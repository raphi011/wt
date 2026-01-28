# wt Release Testing Checklist

Manual testing reference for pre-release verification. Test each command and flag combination.

> ❌ = unreviewed | ✅ = reviewed

---

## Core Commands

| | Command | Expected Behavior | Watch For |
|-|---------|-------------------|-----------|
| ❌ | `wt checkout <branch>` | Creates worktree for existing branch | Fails if branch doesn't exist |
| ❌ | `wt checkout -b <branch>` | Creates new branch + worktree from main/master | Base branch detection |
| ❌ | `wt checkout -b <branch> --base develop` | Creates from specified base | Custom base branch |
| ❌ | `wt checkout -b <branch> -f` | Fetches base before creating | Network errors |
| ❌ | `wt checkout -b <branch> -s` | Stashes changes, applies to new worktree | Stash apply conflicts |
| ❌ | `wt checkout -r repo1 -r repo2 <branch>` | Multi-repo by name | All repos found |
| ❌ | `wt checkout -l backend <branch>` | Multi-repo by label | Label matching |
| ❌ | `wt checkout --note "text" <branch>` | Sets branch note | Note persists |
| ❌ | `wt checkout --hook=myhook <branch>` | Runs specific hook | Hook found, executes |
| ❌ | `wt checkout --no-hook <branch>` | Skips all hooks | No hook runs |
| ❌ | `wt checkout -i` | Interactive wizard | All prompts work |
| ❌ | `wt list` | Shows worktrees with stable IDs | Inside repo: filters to current |
| ❌ | `wt list --json` | Valid JSON output | Parseable JSON |
| ❌ | `wt list -g` | Shows all repos' worktrees | Global scope |
| ❌ | `wt list -s {id\|repo\|branch\|commit}` | Sorts by column | Correct ordering |
| ❌ | `wt list -R` | Fetches origin + PR status | Network errors, cache update |
| ❌ | `wt list -r repo1,repo2` | Filters by repo name | Multiple filters |
| ❌ | `wt list -l backend` | Filters by label | Label matching |
| ❌ | `wt show` | Shows current worktree details | Inside worktree only |
| ❌ | `wt show -n <id>` | Shows specific worktree | ID resolution |
| ❌ | `wt show -r myrepo` | Shows repo's current branch | Repo resolution |
| ❌ | `wt show -R` | Refreshes PR status | API call |
| ❌ | `wt show --json` | Valid JSON output | Parseable JSON |
| ❌ | `wt prune` | Removes merged worktrees | Uses cached PR info |
| ❌ | `wt prune -R` | Refreshes PR status first | Network, then prune |
| ❌ | `wt prune -d` | Dry-run preview | No actual removal |
| ❌ | `wt prune -d -v` | Dry-run with skip reasons | Shows why not pruned |
| ❌ | `wt prune -n 1 -f` | Force removes by ID | Requires -f |
| ❌ | `wt prune -n 1 -n 2 -f` | Multiple IDs | All removed |
| ❌ | `wt prune -g` | Prunes all repos | Global scope |
| ❌ | `wt prune --reset-cache` | Clears cache, resets IDs | IDs renumbered from 1 |
| ❌ | `wt prune --hook=cleanup` | Runs specific hook | Hook after each removal |
| ❌ | `wt prune --no-hook` | Skips all hooks | No hook runs |
| ❌ | `wt prune -i` | Interactive selection | Multi-select works |
| ❌ | `wt repos` | Lists repos in directory | Correct dir scanned |
| ❌ | `wt repos -l backend` | Filters by label | Label matching |
| ❌ | `wt repos -s {name\|branch\|worktrees\|label}` | Sorts by column | Correct ordering |
| ❌ | `wt repos --json` | Valid JSON output | Parseable JSON |

---

## PR Commands

| | Command | Expected Behavior | Watch For |
|-|---------|-------------------|-----------|
| ❌ | `wt pr checkout <num>` | Checks out PR branch | Inside repo only |
| ❌ | `wt pr checkout <num> -r myrepo` | Checks out from named repo | Repo found |
| ❌ | `wt pr checkout <num> org/repo` | Clones repo, checks out PR | Clone mode |
| ❌ | `wt pr checkout <num> org/repo --forge=gitlab` | Uses GitLab | GitLab CLI works |
| ❌ | `wt pr checkout <num> --note "text"` | Sets branch note | Note persists |
| ❌ | `wt pr checkout --hook=myhook` | Runs specific hook | Hook executes |
| ❌ | `wt pr checkout --no-hook` | Skips hooks | No hook runs |
| ❌ | `wt pr checkout -i` | Interactive PR selection | List loads |
| ❌ | `wt pr checkout -i -r myrepo` | Interactive from repo | Scoped to repo |
| ❌ | `wt pr view` | Shows PR details | Inside worktree |
| ❌ | `wt pr view -n <id>` | Shows by worktree ID | ID resolution |
| ❌ | `wt pr view -r myrepo` | Shows by repo name | Repo resolution |
| ❌ | `wt pr view -w` | Opens in browser | Browser opens |
| ❌ | `wt pr merge` | Merges, removes worktree+branch | Full cleanup |
| ❌ | `wt pr merge -n <id>` | Merges by worktree ID | ID resolution |
| ❌ | `wt pr merge -r myrepo` | Merges by repo name | Repo resolution |
| ❌ | `wt pr merge -s squash` | Uses squash strategy | Strategy applied |
| ❌ | `wt pr merge -s rebase` | Uses rebase strategy | **GitLab: not supported** |
| ❌ | `wt pr merge -k` | Keeps worktree+branch | No cleanup |
| ❌ | `wt pr merge --hook=post-merge` | Runs specific hook | Hook executes |
| ❌ | `wt pr merge --no-hook` | Skips hooks | No hook runs |
| ❌ | `wt pr create -t "Title"` | Creates PR | Inside worktree |
| ❌ | `wt pr create -t "Title" -b "Body"` | With body text | Body set |
| ❌ | `wt pr create -t "Title" --body-file=pr.md` | Body from file | File read |
| ❌ | `wt pr create -t "Title" --base develop` | Custom base branch | Base used |
| ❌ | `wt pr create -t "Title" --draft` | Creates draft PR | Draft status |
| ❌ | `wt pr create -t "Title" -w` | Opens in browser | Browser opens |
| ❌ | `wt pr create -t "Title" -n <id>` | By worktree ID | ID resolution |
| ❌ | `wt pr create -t "Title" -r myrepo` | By repo name | Repo resolution |

---

## Utility Commands

| | Command | Expected Behavior | Watch For |
|-|---------|-------------------|-----------|
| ❌ | `wt exec -n <id> -- <cmd>` | Runs cmd in worktree | Correct working dir |
| ❌ | `wt exec -n 1 -n 2 -- git status` | Multiple worktrees | All executed |
| ❌ | `wt exec -r repo1,repo2 -- make` | By repo names | Runs in main repos |
| ❌ | `wt exec -l backend -- make` | By label | Runs in labeled repos |
| ❌ | `wt cd -n <id>` | Prints worktree path | Path to stdout |
| ❌ | `wt cd -n <id> -p` | Prints main repo path | Project path |
| ❌ | `wt cd -r myrepo` | Prints repo path | Repo found |
| ❌ | `wt cd -l backend` | Prints by label | Exactly one match required |
| ❌ | `cd $(wt cd -n 1)` | Shell integration | Logs to stderr only |
| ❌ | `wt mv` | Moves worktrees to config dir | Scans cwd |
| ❌ | `wt mv ~/old-folder` | Moves from specified path | Path scanned |
| ❌ | `wt mv --format={branch}` | Renames during move | Format applied |
| ❌ | `wt mv -d` | Dry-run preview | No actual move |
| ❌ | `wt mv -f` | Force move locked | Locked worktrees moved |
| ❌ | `wt mv -r myrepo` | Filters by repo | Only matching moved |
| ❌ | `wt note set "text"` | Sets note on current branch | Inside worktree |
| ❌ | `wt note set "text" -n <id>` | Sets by worktree ID | ID resolution |
| ❌ | `wt note get` | Gets current branch note | Prints or empty |
| ❌ | `wt note get -n <id>` | Gets by worktree ID | ID resolution |
| ❌ | `wt note clear` | Clears current branch note | Removed |
| ❌ | `wt note clear -n <id>` | Clears by worktree ID | ID resolution |
| ❌ | `wt label add backend` | Adds label to current repo | Stored in git config |
| ❌ | `wt label add backend -r api,web` | Adds to multiple repos | All updated |
| ❌ | `wt label remove backend` | Removes label | Removed |
| ❌ | `wt label list` | Lists current repo labels | Shows labels |
| ❌ | `wt label list -r api,web` | Lists for specific repos | Filtered |
| ❌ | `wt label list -g` | Lists all labels globally | All repos scanned |
| ❌ | `wt label clear` | Clears all labels | All removed |
| ❌ | `wt hook myhook` | Runs named hook | Inside worktree |
| ❌ | `wt hook myhook -n <id>` | By worktree ID | ID resolution |
| ❌ | `wt hook myhook -n 1 -n 2` | Multiple worktrees | All executed |
| ❌ | `wt hook myhook -r repo1,repo2` | By repo names | Runs in main repos |
| ❌ | `wt hook myhook -l backend` | By label | Runs in labeled repos |
| ❌ | `wt hook myhook -a KEY=VALUE` | Passes variable | Placeholder substituted |
| ❌ | `wt hook myhook -d` | Dry-run | Prints command only |

---

## Configuration Commands

| | Command | Expected Behavior | Watch For |
|-|---------|-------------------|-----------|
| ❌ | `wt config init ~/Git` | Creates config file | File at ~/.config/wt/config.toml |
| ❌ | `wt config init ~/Git -f` | Overwrites existing | Force works |
| ❌ | `wt config init ~/Git -s` | Prints to stdout | No file written |
| ❌ | `wt config show` | Shows effective config | All sections |
| ❌ | `wt config show --json` | Valid JSON output | Parseable JSON |
| ❌ | `wt config hooks` | Lists configured hooks | Shows all hooks |
| ❌ | `wt config hooks --json` | Valid JSON output | Parseable JSON |
| ❌ | `wt completion fish` | Fish completion script | Valid syntax |
| ❌ | `wt completion bash` | Bash completion script | Valid syntax |
| ❌ | `wt completion zsh` | Zsh completion script | Valid syntax |
| ❌ | `wt doctor` | Checks for issues | Reports problems |
| ❌ | `wt doctor --fix` | Auto-fixes issues | Repairs what's possible |
| ❌ | `wt doctor --reset` | Rebuilds cache | New IDs assigned |

---

## Cross-Cutting Concerns

When testing commands, verify these behaviors:

| | Concern | What to Check |
|-|---------|---------------|
| ❌ | **Hook execution** | `--hook=name` runs specific hook; `--no-hook` skips all; default `on=[]` hooks run automatically |
| ❌ | **GitHub vs GitLab** | Commands work with both `gh` and `glab` CLI; GitLab lacks rebase merge |
| ❌ | **Inside vs outside repo** | Some flags optional inside repo/worktree, required outside |
| ❌ | **Multi-repo targeting** | `-r repo1,repo2` and `-l label` work; mutual exclusion with `-n` |
| ❌ | **Interactive mode (`-i`)** | Respects explicit flags, pre-selects defaults, skips irrelevant steps |
| ❌ | **JSON output** | `--json` produces valid, parseable JSON for scripting |
| ❌ | **Stdout vs stderr** | Primary data to stdout; logs/progress to stderr (enables `$(...)` patterns) |
| ❌ | **Cache persistence** | IDs stable across runs; `--reset-cache` renumbers from 1 |
| ❌ | **Error messages** | Clear, actionable errors for invalid flags, missing args, missing repos |

# Command Reference

<!-- Generated from the command definitions by `just docs`. Do not edit. -->

- [wt checkout](#wt-checkout) - Create or open worktree for branch
- [wt list](#wt-list) - List worktrees
- [wt prune](#wt-prune) - Prune merged worktrees
- [wt repo](#wt-repo) - Manage registered repositories
  - [wt repo list](#wt-repo-list) - List registered repositories
  - [wt repo add](#wt-repo-add) - Register existing repositories
  - [wt repo clone](#wt-repo-clone) - Clone a repository and register it
  - [wt repo remove](#wt-repo-remove) - Unregister a repository
  - [wt repo convert](#wt-repo-convert) - Convert a repo between regular and bare-in-.git structure
- [wt pr](#wt-pr) - Work with PRs
  - [wt pr checkout](#wt-pr-checkout) - Checkout a PR into a worktree
  - [wt pr create](#wt-pr-create) - Create PR for worktree
  - [wt pr merge](#wt-pr-merge) - Merge PR and clean up worktree
  - [wt pr view](#wt-pr-view) - View PR details or open in browser
- [wt diff](#wt-diff) - Show worktree diff
- [wt exec](#wt-exec) - Run command in worktree(s)
- [wt cd](#wt-cd) - Print worktree path for shell scripting
- [wt note](#wt-note) - Manage branch notes
  - [wt note set](#wt-note-set) - Set a note on a branch
  - [wt note get](#wt-note-get) - Get the note for a branch
  - [wt note clear](#wt-note-clear) - Clear the note from a branch
- [wt label](#wt-label) - Manage repository labels
  - [wt label add](#wt-label-add) - Add a label to repositories
  - [wt label remove](#wt-label-remove) - Remove a label from repositories
  - [wt label list](#wt-label-list) - List labels
  - [wt label clear](#wt-label-clear) - Clear all labels from repositories
- [wt hook](#wt-hook) - Run configured hook
- [wt config](#wt-config) - Manage configuration
  - [wt config init](#wt-config-init) - Create default config file
  - [wt config show](#wt-config-show) - Show effective configuration
  - [wt config hooks](#wt-config-hooks) - List available hooks
- [wt completion](#wt-completion) - Generate completion script
- [wt init](#wt-init) - Output shell wrapper function

## Global flags

```text
  -q, --quiet     Suppress all log output
  -v, --verbose   Show external commands being executed
```

## wt checkout

Create or open worktree for branch

```text
wt checkout [[scope:]branch] [flags]
```

Aliases: `co`

```text
Create a worktree for a branch, or open it if one already exists.

Use -b to create a new branch, or omit for an existing branch.
Use -i for interactive mode to be prompted for options.

Target uses [scope:]branch format where scope can be a repo name or label:
  - Without scope: uses current repo; outside a repo, or with -g, searches
    all repos for an existing branch and errors if it is in several
  - With repo scope: targets that specific repo
  - With label scope (requires -b): targets all repos with that label
```

Examples:

```text
  wt checkout feature-branch              # Existing branch in current repo
  wt checkout -g feature-branch           # Existing branch in whichever repo has it
  wt checkout myrepo:feature              # Existing branch in myrepo
  wt checkout -b feature-branch           # Create new branch in current repo
  wt checkout -b myrepo:feature           # Create new branch in myrepo
  wt checkout -b backend:feature          # Create new branch in backend label repos
  wt checkout -i                          # Interactive mode
```

Flags:

```text
  -a, --arg strings    Set hook variable (KEY=VALUE or KEY for boolean)
  -s, --autostash      Stash changes and apply to new worktree
      --base string    Base branch to create from
  -f, --fetch          Fetch from origin before checkout
  -g, --global         Search all repos for an unscoped existing branch
  -h, --help           help for checkout
      --hook strings   Run named hook(s)
  -i, --interactive    Interactive mode
  -b, --new-branch     Create a new branch
      --no-hook        Skip hooks
      --no-preserve    Skip file preservation
      --note string    Set a note on the branch
```

## wt list

List worktrees

```text
wt list [scope...] [flags]
```

Aliases: `ls`

```text
List worktrees for registered repos.

Inside a repo: shows only that repo's worktrees. Use --global for all.
Use positional args to filter by repo name(s) or label(s).
Resolution order: repo name → label.

Worktrees are sorted by commit date (most recent first) by default.
Use --refresh-pr/-R to fetch PR status from GitHub/GitLab.
```

Examples:

```text
  wt list                      # List worktrees for current repo
  wt list --global             # List all worktrees (all repos)
  wt list myrepo               # Filter by repository name
  wt list backend              # Filter by label (if no repo named 'backend')
  wt list myrepo backend       # Filter by multiple scopes
  wt list -R                   # Refresh PR status before listing
  wt list --json               # Output as JSON
```

Flags:

```text
  -g, --global        Show all worktrees (not just current repo)
  -h, --help          help for list
      --json          Output as JSON
  -R, --refresh-pr    Refresh PR status before listing
  -s, --sort string   Sort by: date, repo, branch
```

## wt prune

Prune merged worktrees

```text
wt prune [[scope:]branch...] [flags]
```

Aliases: `p`

```text
Remove worktrees with merged PRs.

Without arguments, removes all worktrees with merged PRs in current repo.
Use --stale to also prune worktrees older than stale_days (default 14).
Use --global to prune all registered repos.
Use --interactive to select worktrees to prune.

Target specific worktrees using [scope:]branch arguments where scope can be
a repo name or label. Worktrees with a merged PR can be pruned without -f.
Use -f to prune worktrees whose PR is not yet confirmed merged.
Use -R to fetch the PR status of the targets first.
--stale and --interactive cannot be combined with targets.

Worktrees with uncommitted changes (modified, staged or untracked files) are
never removed without -f.
```

Examples:

```text
  wt prune                         # Remove worktrees with merged PRs
  wt prune --stale                 # Also prune stale worktrees
  wt prune --global                # Prune all repos
  wt prune -d                      # Dry-run: preview without removing
  wt prune -f                      # Also remove worktrees with uncommitted changes
  wt prune -i                      # Interactive mode
  wt prune feature                 # Remove merged feature worktree
  wt prune feature -R              # Refresh PR status, remove if merged
  wt prune feature -f              # Remove unmerged feature worktree
  wt prune feature -f -g           # Remove feature worktree (all repos)
  wt prune myrepo:feature -f       # Remove specific unmerged worktree
  wt prune backend:main -f         # Remove main in backend-labeled repos
```

Flags:

```text
  -a, --arg strings          Set hook variable (KEY=VALUE or KEY for boolean)
  -b, --delete-branches      Delete local branches after removal
  -d, --dry-run              Preview without removing
  -f, --force                Force remove unmerged worktrees and worktrees with uncommitted changes
  -g, --global               Prune all repos
  -h, --help                 help for prune
      --hook strings         Run named hook(s)
  -i, --interactive          Interactive mode
      --no-delete-branches   Keep local branches (overrides config)
      --no-hook              Skip hooks
  -R, --refresh-pr           Refresh PR status first
      --reset-cache          Clear all cached data
      --stale                Also prune stale worktrees (older than stale_days)
```

## wt repo

Manage registered repositories

```text
wt repo [flags]
```

Aliases: `r`

```text
Manage registered repositories.

Use subcommands to list, add, clone, remove, or convert registered repositories.
```

Examples:

```text
  wt repo list                  # List all repos
  wt repo add ~/work/my-project # Register a repo
  wt repo clone <url|org/repo>  # Clone and register a repo
  wt repo remove my-project     # Unregister a repo
  wt repo convert --clone-mode bare  # Convert to bare structure
  wt repo convert --clone-mode regular # Convert bare to regular
```

Flags:

```text
  -h, --help   help for repo
```

### wt repo list

List registered repositories

```text
wt repo list [label...] [flags]
```

Aliases: `ls`

```text
List all registered repositories.

Shows name, path, and labels.
Use positional args to filter by label(s).
```

Examples:

```text
  wt repo list                  # List all repos
  wt repo list backend          # Filter by label
  wt repo list backend frontend # Filter by multiple labels
  wt repo list --json           # Output as JSON
```

Flags:

```text
  -h, --help          help for list
      --json          Output as JSON
  -s, --sort string   Sort by: name, label (default "name")
```

### wt repo add

Register existing repositories

```text
wt repo add <path>... [flags]
```

Aliases: `a`

```text
Register existing git repositories with wt.

Repositories will be added to the registry (~/.wt/repos.json) and can then
be managed with other wt commands. Non-git directories are silently skipped.
```

Examples:

```text
  wt repo add ~/work/my-project                    # Register single repo
  wt repo add ~/work/*                             # Register all repos in directory
  wt repo add ~/work/my-project -n myproj          # Custom display name (single repo only)
  wt repo add ~/work/my-project -l work -l api     # Add labels
  wt repo add ~/work/my-project -w "./{branch}"    # Custom worktree format
```

Flags:

```text
  -h, --help                     help for add
  -l, --label strings            Labels for grouping (repeatable)
  -n, --name string              Display name (default: directory name)
  -w, --worktree-format string   Worktree format override
```

### wt repo clone

Clone a repository and register it

```text
wt repo clone <url|org/repo> [destination] [flags]
```

Aliases: `cl`

```text
Clone a git repository and register it.

By default, clones as a regular repo with a working tree at root:
  repo/
  ├── .git/    # git directory
  └── ...      # working tree files

Use --clone-mode bare (or clone.mode in config) for a bare clone (git data in .git, no working tree at root).

When cloning bare, creates a worktree for the default branch (main/master).
Use -b to specify a different branch instead.

Supports both full URLs and short-form org/repo format:
  - Full URLs use git clone directly
  - org/repo format uses gh/glab CLI (determined by forge config)
  - repo-only format uses default_org from config

If destination is not specified, clones into <repo-name> in the current directory.
```

Examples:

```text
  wt repo clone https://github.com/org/repo           # Clone via git URL
  wt repo clone git@github.com:org/repo.git           # Clone via SSH URL
  wt repo clone org/repo                              # Clone via gh/glab (uses forge config)
  wt repo clone myrepo                                # Clone with default_org
  wt repo clone org/repo --clone-mode bare            # Bare clone with default branch worktree
  wt repo clone org/repo --clone-mode bare -b develop # Bare clone with worktree for develop
  wt repo clone org/repo -l work                      # Clone with label
```

Flags:

```text
  -b, --branch string            Create initial worktree for branch (bare mode only)
      --clone-mode string        Clone mode: bare or regular (default: config)
  -h, --help                     help for clone
  -l, --label strings            Labels for grouping (repeatable)
  -n, --name string              Display name (default: directory name)
  -w, --worktree-format string   Worktree format override
```

### wt repo remove

Unregister a repository

```text
wt repo remove <repo> [flags]
```

Aliases: `rm`

```text
Unregister a repository from wt.

The repository will be removed from the registry (~/.wt/repos.json).
By default, files are kept on disk. Use --delete to also remove files.
```

Examples:

```text
  wt repo remove my-project           # Unregister, keep files
  wt repo remove my-project --delete  # Unregister and delete from disk
  wt repo remove my-project -D -f     # Delete without confirmation
```

Flags:

```text
  -D, --delete   Also delete repo and worktrees from disk
  -f, --force    Force deletion without confirmation
  -h, --help     help for remove
```

### wt repo convert

Convert a repo between regular and bare-in-.git structure

```text
wt repo convert [path] [flags]
```

```text
Convert a repository between regular and bare-in-.git structure.

Use --clone-mode to specify the target structure:

  --clone-mode bare     Convert regular → bare-in-.git
  --clone-mode regular  Convert bare-in-.git → regular

Regular → bare (--clone-mode bare):
  Before:                    After:
  myrepo/                    myrepo/
  ├── .git/  (regular)       ├── .git/  (bare repo)
  ├── src/                   │   └── worktrees/
  └── README.md              └── main/  (working tree)

Bare → regular (--clone-mode regular):
  Before:                    After:
  myrepo/                    myrepo/
  ├── .git/  (bare repo)     ├── .git/  (regular)
  │   └── worktrees/         ├── src/
  └── main/  (working tree)  └── README.md

The conversion:
- Preserves all uncommitted changes and untracked files
- Updates any existing worktrees to work with the new structure
- Registers the repository in the wt registry (if not already registered)
```

Examples:

```text
  wt repo convert --clone-mode bare               # Convert to bare in current dir
  wt repo convert --clone-mode bare ./myrepo      # Convert repo at path
  wt repo convert --clone-mode regular            # Convert bare to regular
  wt repo convert --clone-mode bare -n myapp      # Convert with custom name
  wt repo convert --clone-mode bare --dry-run     # Preview without changes
```

Flags:

```text
      --clone-mode string        Target mode: bare or regular (required)
  -d, --dry-run                  Preview conversion without making changes
  -h, --help                     help for convert
  -l, --label strings            Labels for grouping (repeatable)
  -n, --name string              Display name (default: directory name)
  -w, --worktree-format string   Worktree format override
```

## wt pr

Work with PRs

```text
wt pr [flags]
```

```text
Work with pull requests.
```

Examples:

```text
  wt pr checkout 123                # Checkout PR from current repo
  wt pr checkout myrepo 123         # Checkout PR from local repo
  wt pr checkout org/repo 123       # Checkout PR from registered repo matched by remote
  wt pr checkout --clone org/repo 123  # Clone repo and checkout PR
  wt pr create --title "Add feature"
  wt pr merge
  wt pr view
```

Flags:

```text
  -h, --help   help for pr
```

### wt pr checkout

Checkout a PR into a worktree

```text
wt pr checkout [repo] [number] [flags]
```

Aliases: `co`

```text
Checkout a PR into a worktree.

If repo contains '/', it's treated as org/repo and matched against remotes of
registered repos. Use --clone to clone the repo if no local match is found.
Otherwise, the repo argument is looked up in the local registry by name.
Use --clone-mode (with --clone) to control whether the repo is cloned as bare or regular.
Use --interactive to select an open PR from registered repositories.
```

Examples:

```text
  wt pr checkout -i                                     # Select an open PR interactively
  wt pr checkout 123                                    # PR from current repo
  wt pr checkout myrepo 123                             # PR from local repo in registry
  wt pr checkout org/repo 123                           # PR from registered repo matched by remote
  wt pr checkout --clone org/repo 123                   # Clone repo and checkout PR
  wt pr checkout --clone --clone-mode regular org/repo 123  # Regular clone + checkout
```

Flags:

```text
  -a, --arg strings         Set hook variable (KEY=VALUE or KEY for boolean)
      --clone               Clone the repo if no local match (for org/repo format)
      --clone-mode string   Clone mode: bare or regular (default: config)
      --forge string        Forge type: github or gitlab
  -h, --help                help for checkout
      --hook strings        Run named hook(s)
  -i, --interactive         Select an open PR interactively
      --no-hook             Skip hooks
      --no-preserve         Skip file preservation
      --note string         Set a note on the branch
```

### wt pr create

Create PR for worktree

```text
wt pr create [repo] [flags]
```

Aliases: `c`, `new`

```text
Create a PR for the current branch.
```

Examples:

```text
  wt pr create --title "Add feature"
  wt pr create myrepo --title "Add feature"  # Create for specific repo
  wt pr create --title "Add feature" --body "Details"
  wt pr create --title "Add feature" --draft
  wt pr create --title "Add feature" -w      # Open in browser
```

Flags:

```text
      --base string        Base branch
  -b, --body string        PR body
      --body-file string   Read body from file
      --draft              Create as draft PR
  -h, --help               help for create
  -t, --title string       PR title
  -w, --web                Open in browser after creation
```

### wt pr merge

Merge PR and clean up worktree

```text
wt pr merge [repo] [flags]
```

Aliases: `m`

```text
Merge the PR for the current branch.

Merges the PR, deletes its source branch, and removes the worktree (unless --keep).
With prune.delete_local_branches, the local branch is deleted with the worktree.
```

Examples:

```text
  wt pr merge                  # Merge current branch's PR
  wt pr merge myrepo           # Merge for specific repo
  wt pr merge --keep           # Keep worktree after merge
  wt pr merge -s rebase        # Use rebase strategy
```

Flags:

```text
  -a, --arg strings       Set hook variable (KEY=VALUE or KEY for boolean)
  -h, --help              help for merge
      --hook strings      Run named hook(s)
  -k, --keep              Keep worktree after merge
      --no-hook           Skip hooks
  -s, --strategy string   Merge strategy: squash, rebase, merge
```

### wt pr view

View PR details or open in browser

```text
wt pr view [repo] [flags]
```

Aliases: `v`

```text
View PR details for the current branch.
```

Examples:

```text
  wt pr view              # View PR details
  wt pr view myrepo       # View PR for specific repo
  wt pr view -w           # Open PR in browser
```

Flags:

```text
  -h, --help   help for view
  -w, --web    Open in browser
```

## wt diff

Show worktree diff

```text
wt diff [[scope:]branch] [flags]
```

Aliases: `d`

```text
Show the diff of a worktree branch against its base.

By default shows what a PR would contain: changes introduced on the branch
since it diverged from the default branch (e.g. origin/main).

The argument can be:
  - branch name: finds the worktree in the current repo; outside a repo,
    or with -g, searches all repos and errors if ambiguous
  - repo:branch: finds exact worktree in specified repo
  - label:branch: finds worktree in repos with that label, errors if ambiguous

With no arguments, diffs the current worktree.
```

Examples:

```text
  wt diff                        # Diff current worktree vs origin/main
  wt diff feature-x              # Diff feature-x worktree
  wt diff wt:feature-x           # Diff feature-x in wt repo
  wt diff --working              # Show uncommitted changes
  wt diff --stat                 # Show diffstat summary only
  wt diff --base origin/develop  # Diff against develop instead of main
  wt diff --tool delta           # Use delta as pager for this diff
```

Flags:

```text
      --base string   Override comparison base ref (default: origin/<default-branch>)
  -g, --global        Search all repos for an unscoped branch
  -h, --help          help for diff
      --name-only     Show only names of changed files
      --stat          Show diffstat summary only
  -t, --tool string   Override pager for this diff (e.g. delta, bat)
      --working       Show uncommitted changes (diff against HEAD)
```

## wt exec

Run command in worktree(s)

```text
wt exec [[scope:]branch...] -- <command> [flags]
```

Aliases: `x`

```text
Run a command in one or more worktrees.

Target worktrees using [scope:]branch arguments before --:
  - branch: runs in the current repo's worktree for that branch; outside a
    repo it searches all repos, and with -g it runs in every match
  - repo:branch: finds exact worktree in specified repo
  - label:branch: runs in all repos with that label

With no targets, runs in the current worktree.

All targets are attempted even if a command fails. Failures return a non-zero
exit status: the command's exit code for a single worktree, or 1 for multiple
worktrees. Commands that cannot start also return 1.
```

Examples:

```text
  wt exec -- git status                  # In current worktree
  wt exec main -- git status             # In main worktree of the current repo
  wt exec -g main -- git status          # In main worktree of every repo
  wt exec wt:main -- git status          # In main worktree of wt repo
  wt exec backend:main -- make test      # In main worktree of backend-labeled repos
  wt exec wt:main myrepo:dev -- make test  # In multiple worktrees
```

Flags:

```text
  -g, --global   Search all repos for an unscoped branch
  -h, --help     help for exec
```

## wt cd

Print worktree path for shell scripting

```text
wt cd [[scope:]branch] [flags]
```

```text
Print the path of a worktree for shell scripting.

Use with shell command substitution: cd $(wt cd feature-x)

The argument can be:
  - branch name: finds the worktree in the current repo; outside a repo,
    or with -g, searches all repos and errors if ambiguous
  - repo:branch: finds exact worktree in specified repo
  - label:branch: finds worktree in repos with that label, errors if ambiguous

With no arguments, returns the most recently accessed worktree.

Interactive mode (-i) is repo-aware: inside a repo it shows only that
repo's worktrees. Use -g to show all repos.
```

Examples:

```text
  cd $(wt cd)              # cd to most recently accessed worktree
  cd $(wt cd feature-x)    # cd to feature-x worktree of the current repo
  cd $(wt cd -g feature-x) # search all repos (error if ambiguous)
  cd $(wt cd wt:feature-x) # cd to feature-x worktree in wt repo
  cd $(wt cd -i)           # interactive: current repo's worktrees
  cd $(wt cd -i -g)        # interactive: all repos' worktrees
  wt cd --copy feature-x   # copy worktree path to clipboard
```

Flags:

```text
      --copy          Copy path to clipboard
  -g, --global        Search or show worktrees from all repos
  -h, --help          help for cd
  -i, --interactive   Interactive mode with fuzzy search
```

## wt note

Manage branch notes

```text
wt note [flags]
```

Aliases: `n`

```text
Manage notes on branches.

Notes are stored in git config and displayed in list output.

Target a worktree using [scope:]branch where scope can be a repo name or label.
If no target is specified, uses the current worktree's branch.
```

Examples:

```text
  wt note set "WIP"                    # Set note on current branch
  wt note set "WIP" main               # Set note on main (current repo)
  wt note set "WIP" main -g            # Set note on main (all repos with main)
  wt note set "WIP" myrepo:main        # Set note on main in myrepo
  wt note set "WIP" backend:feat       # Set note in all backend repos
  wt note get                          # Get note for current branch
  wt note get myrepo:feature           # Get note for specific worktree
  wt note clear                        # Clear note from current branch
```

Flags:

```text
  -h, --help   help for note
```

### wt note set

Set a note on a branch

```text
wt note set <text> [[scope:]branch] [flags]
```

Flags:

```text
  -g, --global   Search all repos for an unscoped branch
  -h, --help     help for set
```

### wt note get

Get the note for a branch

```text
wt note get [[scope:]branch] [flags]
```

Flags:

```text
  -g, --global   Search all repos for an unscoped branch
  -h, --help     help for get
```

### wt note clear

Clear the note from a branch

```text
wt note clear [[scope:]branch] [flags]
```

Flags:

```text
  -g, --global   Search all repos for an unscoped branch
  -h, --help     help for clear
```

## wt label

Manage repository labels

```text
wt label [flags]
```

Aliases: `lbl`

```text
Manage labels on repositories.

Labels are stored in the registry and can be used to target repos.
```

Examples:

```text
  wt label add backend           # Add label to current repo
  wt label add backend api       # Add label to specific repo
  wt label add backend mygroup   # Add label to repos with 'mygroup' label
  wt label remove backend        # Remove label from current repo
  wt label list                  # List labels for current repo
  wt label list -g               # List all labels
```

Flags:

```text
  -h, --help   help for label
```

### wt label add

Add a label to repositories

```text
wt label add <label> [scope...] [flags]
```

```text
Add a label to one or more repositories.

If no scope is specified, adds to the current repository.
Scopes are resolved as repo name first, then label.
```

Examples:

```text
  wt label add backend           # Add to current repo
  wt label add backend api       # Add to repo named 'api'
  wt label add backend frontend  # Add to repos with 'frontend' label
```

Flags:

```text
  -h, --help   help for add
```

### wt label remove

Remove a label from repositories

```text
wt label remove <label> [scope...] [flags]
```

```text
Remove a label from one or more repositories.

If no scope is specified, removes from the current repository.
Scopes are resolved as repo name first, then label.
```

Examples:

```text
  wt label remove backend           # Remove from current repo
  wt label remove backend api       # Remove from repo named 'api'
  wt label remove backend frontend  # Remove from repos with 'frontend' label
```

Flags:

```text
  -h, --help   help for remove
```

### wt label list

List labels

```text
wt label list [scope...] [flags]
```

```text
List labels for repositories.

If no scope is specified, lists labels for the current repository.
Scopes are resolved as repo name first, then label.
```

Examples:

```text
  wt label list           # List labels for current repo
  wt label list api       # List labels for repo named 'api'
  wt label list -g        # List all labels across repos
```

Flags:

```text
  -g, --global   List all labels across repos
  -h, --help     help for list
```

### wt label clear

Clear all labels from repositories

```text
wt label clear [scope...] [flags]
```

```text
Clear all labels from one or more repositories.

If no scope is specified, clears labels from the current repository.
Scopes are resolved as repo name first, then label.
```

Examples:

```text
  wt label clear           # Clear labels from current repo
  wt label clear api       # Clear labels from repo named 'api'
  wt label clear frontend  # Clear labels from repos with 'frontend' label
```

Flags:

```text
  -h, --help   help for clear
```

## wt hook

Run configured hook

```text
wt hook [[scope:]branch] <name> [flags]
```

Aliases: `h`

```text
Run a configured hook manually.

Hooks are defined under [hooks.<name>] in config.toml or .wt.toml. Any hook
can be run manually, with or without an "on" trigger.
When run manually, the hook always executes as an "after" hook
({phase}=after, {trigger}=run, {action}=manual).

With one argument, runs in the current worktree.
With two arguments, the first is a [scope:]branch target (scope is a repo name
or label) and the second is the hook name. A branch without scope means the
current repo; outside a repo it searches all repos, and with -g the hook runs
in every match.

Run 'wt config hooks' to list hooks, 'wt config init -s' for trigger syntax
and placeholders.
```

Examples:

```text
  wt hook code                        # Run 'code' hook in current worktree
  wt hook main code                   # Run 'code' in main worktree (current repo)
  wt hook -g main code                # Run 'code' in main worktree (all repos)
  wt hook myrepo:main code            # Run in specific repo's worktree
  wt hook backend:main code           # Run in backend label's main worktrees
  wt hook code -a prompt="do X"       # Pass custom variable
  wt hook code -d                     # Dry-run: print command without executing
```

Flags:

```text
  -a, --arg strings   Set hook variable (KEY=VALUE or KEY for boolean)
  -d, --dry-run       Print command without executing
  -g, --global        Search all repos for an unscoped branch
  -h, --help          help for hook
```

## wt config

Manage configuration

```text
wt config [flags]
```

Aliases: `cfg`

```text
Manage wt configuration.

Global config: ~/.wt/config.toml
Local config:  .wt.toml (in repo root)
```

Examples:

```text
  wt config init          # Create default global config
  wt config init --local  # Create local repo config
  wt config show          # Show effective config
  wt config hooks         # List available hooks
```

Flags:

```text
  -h, --help   help for config
```

### wt config init

Create default config file

```text
wt config init [flags]
```

```text
Create default config file.

Without flags, creates global config at ~/.wt/config.toml.
With --local, creates per-repo config at .wt.toml in the current repo root.
```

Examples:

```text
  wt config init           # Create global config
  wt config init --local   # Create local repo config
  wt config init -f        # Overwrite existing config
  wt config init -s        # Print config to stdout
```

Flags:

```text
  -f, --force    Overwrite existing config
  -h, --help     help for init
      --local    Create per-repo .wt.toml instead of global config
  -s, --stdout   Print config to stdout
```

### wt config show

Show effective configuration

```text
wt config show [flags]
```

```text
Show effective configuration.

When inside a repo (or with --repo), shows the merged config with source
annotations (global vs local). Otherwise shows global config only.
```

Examples:

```text
  wt config show              # Show config (merged if in a repo)
  wt config show --repo myrepo  # Show merged config for specific repo
  wt config show --json        # Output as JSON
```

Flags:

```text
  -h, --help          help for show
      --json          Output as JSON
      --repo string   Show config for specific repo
```

### wt config hooks

List available hooks

```text
wt config hooks [flags]
```

```text
List available hooks.

When inside a repo (or with --repo), shows merged hooks with source annotations.
```

Examples:

```text
  wt config hooks               # List hooks (merged if in a repo)
  wt config hooks --repo myrepo # List hooks for specific repo
  wt config hooks --json        # Output as JSON
```

Flags:

```text
  -h, --help          help for hooks
      --json          Output as JSON
      --repo string   Show hooks for specific repo
```

## wt completion

Generate completion script

```text
wt completion <shell> [flags]
```

```text
Generate shell completion script for bash, zsh, fish, or powershell.
```

Examples:

```text
  # Fish
  wt completion fish > ~/.config/fish/completions/wt.fish

  # Bash
  wt completion bash > ~/.local/share/bash-completion/completions/wt

  # Zsh
  wt completion zsh > ~/.zfunc/_wt
  # Then add ~/.zfunc to fpath in .zshrc
```

Flags:

```text
  -h, --help   help for completion
```

## wt init

Output shell wrapper function

```text
wt init <shell> [flags]
```

```text
Output shell wrapper function that makes 'wt cd' change directories.

Without this wrapper, 'wt cd' only prints the path (since subprocesses
cannot change the parent shell's directory). The wrapper intercepts
'wt cd' and performs the actual directory change.
```

Examples:

```text
  eval "$(wt init bash)"           # add to ~/.bashrc
  eval "$(wt init zsh)"            # add to ~/.zshrc
  wt init fish | source            # add to ~/.config/fish/config.fish
```

Flags:

```text
  -h, --help   help for init
```


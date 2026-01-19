package main

// CreateCmd creates a new worktree for a new or existing remote branch.
type CreateCmd struct {
	Branch string `arg:"positional,required" placeholder:"BRANCH" help:"branch name"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Note   string `arg:"--note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook   string `arg:"--hook" help:"run named hook instead of default"`
	NoHook bool   `arg:"--no-hook" help:"skip post-create hook"`
}

func (CreateCmd) Description() string {
	return `Create a new git worktree at <dir>/<repo>-<branch>

Creates a new branch and worktree in one step. If the branch already exists
remotely, it will be checked out instead.

Examples:
  wt create feature-branch              # Uses default path resolution
  wt create feature-branch -d ~/Git     # Specify target directory
  wt create feature-branch --no-hook    # Skip post-create hook
  wt create feature-branch --hook=ide   # Run 'ide' hook instead of default`
}

// OpenCmd opens a worktree for an existing local branch.
type OpenCmd struct {
	Branch string `arg:"positional,required" placeholder:"BRANCH|ID" help:"branch name (in repo) or worktree ID/branch (outside repo)"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Note   string `arg:"--note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook   string `arg:"--hook" help:"run named hook instead of default"`
	NoHook bool   `arg:"--no-hook" help:"skip post-create hook"`
}

func (OpenCmd) Description() string {
	return `Open a worktree for an existing local branch

Inside a git repo: opens a worktree for the specified branch.
Outside a git repo: resolves ID or branch name from worktree cache.

Examples:
  wt open feature-branch              # In repo: opens branch
  wt open feature-branch -d ~/Git     # Specify target directory
  wt open 1 -d ~/Git/worktrees        # Outside repo: by worktree ID
  wt open feature-x -d ~/Git/worktrees # Outside repo: by branch name
  wt open feature-branch --no-hook    # Skip post-create hook`
}

// TidyCmd removes merged and clean worktrees.
type TidyCmd struct {
	Dir          string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	DryRun       bool   `arg:"-n,--dry-run" help:"preview without removing"`
	IncludeClean bool   `arg:"-c,--include-clean" help:"also remove worktrees with 0 commits ahead and clean working directory"`
	Hook         string `arg:"--hook" help:"run named hook instead of default"`
	NoHook       bool   `arg:"--no-hook" help:"skip post-removal hooks"`
}

func (TidyCmd) Description() string {
	return `Tidy up merged git worktrees with PR status display

Removes worktrees where the branch is merged AND working directory is clean.
Shows a table with cached PR status. Run 'wt pr refresh' to update PR info.

Hooks with on=["tidy"] run after each worktree removal. Hooks run with
working directory set to the main repo (since worktree path is deleted).

Merge detection uses git merge-base locally, which may miss squash-merged
or rebased PRs. For accurate detection, use GitHub/GitLab where PR status
shows if the branch was merged.

Examples:
  wt pr refresh && wt tidy     # Refresh PR status, then tidy
  wt tidy                      # Remove merged worktrees (uses cached PR info)
  wt tidy -n                   # Dry-run: preview without removing
  wt tidy -d ~/Git/worktrees   # Scan specific directory
  wt tidy -c                   # Also remove clean (0-commit) worktrees
  wt tidy --no-hook            # Skip post-removal hooks
  wt tidy --hook=cleanup       # Run 'cleanup' hook instead of default`
}

// ListCmd lists worktrees in a directory.
type ListCmd struct {
	Dir  string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	JSON bool   `arg:"--json" help:"output as JSON"`
	All  bool   `arg:"-a,--all" help:"show all worktrees (not just current repo)"`
}

func (ListCmd) Description() string {
	return `List all git worktrees with stable IDs

When run inside a git repository, only shows worktrees for that repo.
Use --all to show worktrees from all repos in the directory.
IDs are stable across runs - use them with 'wt exec'.

Examples:
  wt list                      # List worktrees for current repo
  wt list --all                # List all worktrees (all repos)
  wt list -d ~/Git/worktrees   # List from specific directory
  wt list --json               # Output as JSON for scripting`
}

// ExecCmd runs a command in a worktree by ID or branch.
type ExecCmd struct {
	Target  string   `arg:"positional,required" placeholder:"ID|BRANCH" help:"worktree ID or branch name"`
	Command []string `arg:"positional" placeholder:"COMMAND" help:"command to run (after --)"`
	Dir     string   `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
}

func (ExecCmd) Description() string {
	return `Run a command in a worktree by ID or branch

Use 'wt list' to see worktree IDs. The command runs in the worktree directory.

Examples:
  wt exec 1 -- gh pr view            # By worktree ID
  wt exec feature-x -- git status    # By branch name
  wt exec 1 -- code .                # Open worktree in VS Code`
}

// NoteSetCmd sets a note on a branch.
type NoteSetCmd struct {
	Text   string `arg:"positional,required" placeholder:"TEXT" help:"note text"`
	Target string `arg:"positional" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for ID lookup"`
}

func (NoteSetCmd) Description() string {
	return `Set a note on a branch

When run inside a worktree, target is optional (defaults to current branch).
When run outside, specify a worktree ID or branch name.

The note is stored in git config (branch.<name>.description) and displayed
in 'wt list' and 'wt tidy' output.

Examples:
  wt note set "Working on login flow"   # Inside worktree, current branch
  wt note set "Working on login" 1      # By worktree ID
  wt note set "In progress" feature-x   # By branch name`
}

// NoteGetCmd gets a note from a branch.
type NoteGetCmd struct {
	Target string `arg:"positional" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for ID lookup"`
}

func (NoteGetCmd) Description() string {
	return `Get the note for a branch

When run inside a worktree, target is optional (defaults to current branch).
When run outside, specify a worktree ID or branch name.

Prints the note if set, or nothing if no note exists.

Examples:
  wt note get            # Inside worktree, current branch
  wt note get 1          # By worktree ID
  wt note get feature-x  # By branch name`
}

// NoteClearCmd clears a note from a branch.
type NoteClearCmd struct {
	Target string `arg:"positional" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for ID lookup"`
}

func (NoteClearCmd) Description() string {
	return `Clear the note from a branch

When run inside a worktree, target is optional (defaults to current branch).
When run outside, specify a worktree ID or branch name.

Removes the note from the branch. Safe to run even if no note exists.

Examples:
  wt note clear            # Inside worktree, current branch
  wt note clear 1          # By worktree ID
  wt note clear feature-x  # By branch name`
}

// NoteCmd manages branch notes.
type NoteCmd struct {
	Set   *NoteSetCmd   `arg:"subcommand:set" help:"set a note on a branch"`
	Get   *NoteGetCmd   `arg:"subcommand:get" help:"get the note for a branch"`
	Clear *NoteClearCmd `arg:"subcommand:clear" help:"clear the note from a branch"`
}

func (NoteCmd) Description() string {
	return `Manage branch notes

Notes are stored in git config and displayed in list/tidy output.
Inside a worktree, operates on current branch. Outside, specify ID or branch.

Examples:
  wt note set "Working on login"      # Inside worktree
  wt note set "Working on login" 1    # By worktree ID
  wt note set "In progress" feature-x # By branch name
  wt note get                         # Inside worktree
  wt note get feature-x               # By branch name
  wt note clear 1                     # By worktree ID`
}

// CompletionCmd generates shell completion scripts.
type CompletionCmd struct {
	Shell string `arg:"positional,required" placeholder:"SHELL" help:"shell type (fish, bash, zsh)"`
}

func (CompletionCmd) Description() string {
	return `Generate shell completion script

Outputs a completion script for the specified shell.
Redirect to the appropriate file for your shell.

Examples:
  wt completion fish > ~/.config/fish/completions/wt.fish
  wt completion bash > ~/.local/share/bash-completion/completions/wt
  wt completion zsh > ~/.zfunc/_wt  # then add ~/.zfunc to fpath`
}

// ConfigInitCmd creates the default config file.
type ConfigInitCmd struct {
	Force bool `arg:"-f,--force" help:"overwrite existing config file"`
}

func (ConfigInitCmd) Description() string {
	return `Create default config file at ~/.config/wt/config.toml
Examples:
  wt config init           # Create config if missing
  wt config init -f        # Overwrite existing config`
}

// ConfigShowCmd shows the effective configuration.
type ConfigShowCmd struct {
	JSON bool `arg:"--json" help:"output as JSON"`
}

func (ConfigShowCmd) Description() string {
	return `Show effective configuration
Examples:
  wt config show           # Show config in text format
  wt config show --json    # Output as JSON`
}

// ConfigHooksCmd lists available hooks.
type ConfigHooksCmd struct {
	JSON bool `arg:"--json" help:"output as JSON"`
}

func (ConfigHooksCmd) Description() string {
	return `List available hooks
Examples:
  wt config hooks          # Show hooks in text format
  wt config hooks --json   # Output as JSON`
}

// ConfigCmd manages wt configuration.
type ConfigCmd struct {
	Init  *ConfigInitCmd  `arg:"subcommand:init" help:"create default config file"`
	Show  *ConfigShowCmd  `arg:"subcommand:show" help:"show effective configuration"`
	Hooks *ConfigHooksCmd `arg:"subcommand:hooks" help:"list available hooks"`
}

func (ConfigCmd) Description() string {
	return `Manage wt configuration
Examples:
  wt config init           # Create default config
  wt config show           # Show effective config
  wt config hooks          # List available hooks`
}

// HookRunCmd runs a hook by name for a worktree.
type HookRunCmd struct {
	Hook   string `arg:"positional,required" placeholder:"HOOK" help:"hook name to run"`
	Target string `arg:"positional" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for target lookup"`
}

func (HookRunCmd) Description() string {
	return `Run a hook by name for a worktree

When run inside a worktree, target is optional (defaults to current worktree).
When run outside, specify a worktree ID or branch name.

Examples:
  wt hook run kitty              # Inside worktree: run for current
  wt hook run kitty 1            # By worktree ID
  wt hook run kitty feature-x    # By branch name
  wt hook run kitty 1 -d ~/Git   # Specify directory for lookup`
}

// HookCmd manages hooks.
type HookCmd struct {
	Run *HookRunCmd `arg:"subcommand:run" help:"run a hook by name"`
}

func (HookCmd) Description() string {
	return `Manage hooks
Examples:
  wt hook run kitty              # Run hook for current worktree
  wt hook run kitty 1            # Run hook for worktree ID 1`
}

// MvCmd moves worktrees to a different directory with optional renaming.
type MvCmd struct {
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"destination directory (flag > WT_DEFAULT_PATH > config)"`
	Format string `arg:"--format" placeholder:"FORMAT" help:"worktree naming format"`
	DryRun bool   `arg:"-n,--dry-run" help:"show what would be moved"`
	Force  bool   `arg:"-f,--force" help:"force move dirty worktrees"`
}

func (MvCmd) Description() string {
	return `Move worktrees to a different directory

Scans the current directory for worktrees and moves them to the destination
directory, optionally renaming them using the configured worktree format.

Examples:
  wt mv -d ~/Git/worktrees           # Move all worktrees to ~/Git/worktrees
  wt mv -d ~/Git --format={branch-name}  # Move and rename to just branch name
  wt mv --dry-run -d ~/Git           # Preview what would be moved
  wt mv -f -d ~/Git                  # Force move even if worktrees are dirty`
}

// PrOpenCmd creates a worktree for a PR from an existing local repo.
type PrOpenCmd struct {
	Number int    `arg:"positional,required" placeholder:"NUMBER" help:"PR number"`
	Repo   string `arg:"positional" placeholder:"REPO" help:"repository name to find locally"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Hook   string `arg:"--hook" help:"run named hook instead of default"`
	NoHook bool   `arg:"--no-hook" help:"skip post-create hook"`
}

func (PrOpenCmd) Description() string {
	return `Create a worktree for a PR from an existing local repo

Fetches PR metadata and creates a worktree for the branch.
Only works with repos that already exist locally. Use 'wt pr clone' to clone new repos.

Examples:
  wt pr open 123                  # PR from current repo
  wt pr open 123 myrepo           # Find "myrepo" in target directory
  wt pr open 123 org/repo         # Find "repo" locally (org is ignored)
  wt pr open 123 -d ~/Git         # Specify target directory
  wt pr open 123 --no-hook        # Skip post-create hook`
}

// PrCloneCmd clones a repo and creates a worktree for a PR.
type PrCloneCmd struct {
	Number int    `arg:"positional,required" placeholder:"NUMBER" help:"PR number"`
	Repo   string `arg:"positional,required" placeholder:"REPO" help:"repository (org/repo or repo if [clone] org configured)"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Forge  string `arg:"--forge,env:WT_FORGE" placeholder:"FORGE" help:"forge: github or gitlab (flag > env > clone rules > config)"`
	Note   string `arg:"--note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook   string `arg:"--hook" help:"run named hook instead of default"`
	NoHook bool   `arg:"--no-hook" help:"skip post-create hook"`
}

func (PrCloneCmd) Description() string {
	return `Clone a repo and create a worktree for a PR

Clones the repository if not present locally, then creates a worktree for the PR branch.
Use this for repos you don't have locally yet. Use 'wt pr open' for existing repos.

If [clone] org is configured, you can omit the org/ prefix from the repo name.

Examples:
  wt pr clone 123 org/repo              # Clone and checkout PR
  wt pr clone 123 repo                  # Use default org from config
  wt pr clone 123 org/repo -d ~/Git     # Specify target directory
  wt pr clone 123 org/repo --forge=gitlab  # Use GitLab instead of GitHub
  wt pr clone 123 org/repo --no-hook    # Skip post-create hook`
}

// PrRefreshCmd fetches PR status for worktrees.
type PrRefreshCmd struct {
	Dir string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
}

func (PrRefreshCmd) Description() string {
	return `Refresh PR status cache for all worktrees

Queries GitHub/GitLab for PR info on each worktree branch and caches the results.
Run this to update PR info shown in 'wt list' and 'wt tidy'.

Examples:
  wt pr refresh                # Refresh PR status for worktrees
  wt pr refresh -d ~/Git       # Refresh for worktrees in specific directory`
}

// PrMergeCmd merges the PR for the current branch.
type PrMergeCmd struct {
	Target   string `arg:"positional" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir      string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for target lookup"`
	Strategy string `arg:"-s,--strategy,env:WT_MERGE_STRATEGY" placeholder:"STRATEGY" help:"merge strategy: squash, rebase, or merge"`
	Keep     bool   `arg:"-k,--keep" help:"keep worktree and branch after merge"`
	Hook     string `arg:"--hook" help:"run named hook instead of default"`
	NoHook   bool   `arg:"--no-hook" help:"skip post-merge hook"`
}

func (PrMergeCmd) Description() string {
	return `Merge the PR for a branch and clean up

When run inside a worktree, target is optional (defaults to current branch).
When run outside, specify a worktree ID or branch name.

Merges the PR, removes the worktree, and deletes the local branch.
Merge strategy can be set via flag, WT_MERGE_STRATEGY env, or config.
Note: rebase strategy is not supported on GitLab.

Examples:
  wt pr merge                    # Inside worktree: merge current branch's PR
  wt pr merge 1 -d ~/Git         # By worktree ID
  wt pr merge feature-x -d ~/Git # By branch name
  wt pr merge --keep             # Merge PR only, keep worktree and branch
  wt pr merge -s rebase          # Use rebase merge strategy`
}

// PrCmd works with PRs.
type PrCmd struct {
	Open    *PrOpenCmd    `arg:"subcommand:open" help:"checkout PR from existing local repo"`
	Clone   *PrCloneCmd   `arg:"subcommand:clone" help:"clone repo and checkout PR"`
	Refresh *PrRefreshCmd `arg:"subcommand:refresh" help:"refresh PR status cache"`
	Merge   *PrMergeCmd   `arg:"subcommand:merge" help:"merge PR and clean up worktree"`
}

func (PrCmd) Description() string {
	return `Work with PRs
Examples:
  wt pr open 123             # PR from current repo
  wt pr open 123 myrepo      # PR from existing local repo
  wt pr clone 123 org/repo   # Clone repo and checkout PR
  wt pr refresh              # Refresh PR status cache
  wt pr merge                # Merge PR and clean up worktree`
}

// Args is the root command.
type Args struct {
	Create     *CreateCmd     `arg:"subcommand:create" help:"create a new worktree"`
	Open       *OpenCmd       `arg:"subcommand:open" help:"open worktree for existing branch"`
	Tidy       *TidyCmd       `arg:"subcommand:tidy" help:"tidy up merged worktrees"`
	List       *ListCmd       `arg:"subcommand:list" help:"list worktrees"`
	Exec       *ExecCmd       `arg:"subcommand:exec" help:"run command in worktree by ID"`
	Mv         *MvCmd         `arg:"subcommand:mv" help:"move worktrees to another directory"`
	Note       *NoteCmd       `arg:"subcommand:note" help:"manage branch notes"`
	Hook       *HookCmd       `arg:"subcommand:hook" help:"manage hooks"`
	Pr         *PrCmd         `arg:"subcommand:pr" help:"work with PRs"`
	Config     *ConfigCmd     `arg:"subcommand:config" help:"manage configuration"`
	Completion *CompletionCmd `arg:"subcommand:completion" help:"generate completion script"`
}

func (Args) Description() string {
	return `Git worktree manager with GitHub/GitLab integration

Manages git worktrees in a configurable base directory.
Set WT_DEFAULT_PATH or see ~/.config/wt/config.toml for options.

Examples:
  wt create feature-x              # Create worktree for new branch
  wt open existing-branch          # Create worktree for existing local branch
  wt pr open 123                   # Checkout PR as worktree
  wt list                          # List worktrees in current directory
  wt tidy                          # Remove merged worktrees
  wt tidy -n                       # Dry-run: preview what would be removed
  wt config init                   # Create default config file

Exit codes:
  0  Success
  1  Error (invalid arguments, git/gh failures, etc.)`
}

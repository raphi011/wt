package main

import "github.com/raphi011/wt/internal/config"

// Context is passed to all command Run() methods.
type Context struct {
	Config *config.Config
}

// CreateCmd creates a new worktree for a new or existing remote branch.
type CreateCmd struct {
	Branch string `arg:"" required:"" placeholder:"BRANCH" help:"branch name"`
	Dir    string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Note   string `name:"note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook   string `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook bool   `name:"no-hook" help:"skip post-create hook" xor:"hook-ctrl"`
}

func (c *CreateCmd) Help() string {
	return `Creates a new branch and worktree at <dir>/<repo>-<branch>. If the branch already exists
remotely, it will be checked out instead.

Examples:
  wt create feature-branch              # Uses default path resolution
  wt create feature-branch -d ~/Git     # Specify target directory
  wt create feature-branch --no-hook    # Skip post-create hook
  wt create feature-branch --hook=ide   # Run 'ide' hook instead of default`
}

func (c *CreateCmd) Run(ctx *Context) error {
	return runCreate(c, ctx.Config)
}

// OpenCmd opens a worktree for an existing local branch.
type OpenCmd struct {
	Branch string `arg:"" required:"" placeholder:"BRANCH|ID" help:"branch name (in repo) or worktree ID/branch (outside repo)"`
	Dir    string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Note   string `name:"note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook   string `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook bool   `name:"no-hook" help:"skip post-create hook" xor:"hook-ctrl"`
}

func (c *OpenCmd) Help() string {
	return `Inside a git repo: opens a worktree for the specified branch.
Outside a git repo: resolves ID or branch name from worktree cache.

Examples:
  wt open feature-branch              # In repo: opens branch
  wt open feature-branch -d ~/Git     # Specify target directory
  wt open 1 -d ~/Git/worktrees        # Outside repo: by worktree ID
  wt open feature-x -d ~/Git/worktrees # Outside repo: by branch name
  wt open feature-branch --no-hook    # Skip post-create hook`
}

func (c *OpenCmd) Run(ctx *Context) error {
	return runOpen(c, ctx.Config)
}

// TidyCmd removes merged and clean worktrees.
type TidyCmd struct {
	Target       string `arg:"" optional:"" placeholder:"ID|BRANCH" help:"specific worktree to remove (by ID or branch name)"`
	Dir          string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	DryRun       bool   `short:"n" name:"dry-run" negatable:"" help:"preview without removing"`
	Force        bool   `short:"f" name:"force" help:"force remove even if not merged or has uncommitted changes"`
	IncludeClean bool   `short:"c" name:"include-clean" help:"also remove worktrees with 0 commits ahead and clean working directory"`
	ResetCache   bool   `name:"reset-cache" help:"clear all cached data (PR info, worktree history) and reset IDs from 1"`
	Hook         string `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook       bool   `name:"no-hook" help:"skip post-removal hooks" xor:"hook-ctrl"`
}

func (c *TidyCmd) Help() string {
	return `Without arguments, removes all worktrees where the branch is merged AND
working directory is clean. With a target, removes only that specific worktree.

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
  wt tidy feature-x            # Remove specific worktree by branch name
  wt tidy 1                    # Remove specific worktree by ID
  wt tidy feature-x -f         # Force remove even if not merged/dirty
  wt tidy --no-hook            # Skip post-removal hooks
  wt tidy --hook=cleanup       # Run 'cleanup' hook instead of default
  wt tidy --reset-cache        # Clear PR cache and reset IDs from 1`
}

func (c *TidyCmd) Run(ctx *Context) error {
	return runTidy(c, ctx.Config)
}

// ListCmd lists worktrees in a directory.
type ListCmd struct {
	Dir  string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	JSON bool   `name:"json" help:"output as JSON"`
	All  bool   `short:"a" name:"all" help:"show all worktrees (not just current repo)"`
}

func (c *ListCmd) Help() string {
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

func (c *ListCmd) Run(ctx *Context) error {
	return runList(c)
}

// ExecCmd runs a command in a worktree by ID or branch.
type ExecCmd struct {
	Target  string   `arg:"" required:"" placeholder:"ID|BRANCH" help:"worktree ID or branch name"`
	Command []string `arg:"" optional:"" passthrough:"" placeholder:"COMMAND" help:"command to run (after --)"`
	Dir     string   `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
}

func (c *ExecCmd) Help() string {
	return `Run a command in a worktree by ID or branch

Use 'wt list' to see worktree IDs. The command runs in the worktree directory.

Examples:
  wt exec 1 -- gh pr view            # By worktree ID
  wt exec feature-x -- git status    # By branch name
  wt exec 1 -- code .                # Open worktree in VS Code`
}

func (c *ExecCmd) Run(ctx *Context) error {
	return runExec(c)
}

// CdCmd prints the path of a worktree for shell scripting.
type CdCmd struct {
	Target  string `arg:"" required:"" placeholder:"ID|BRANCH" help:"worktree ID or branch name"`
	Dir     string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"directory to scan for worktrees"`
	Project bool   `short:"p" name:"project" help:"print main repository path instead of worktree path"`
}

func (c *CdCmd) Help() string {
	return `Print the path of a worktree for shell scripting.

Examples:
  cd $(wt cd feature-branch)
  cd $(wt cd 3)
  cd $(wt cd -p feature-branch)  # cd to main repo`
}

func (c *CdCmd) Run(ctx *Context) error {
	return runCd(c)
}

// NoteSetCmd sets a note on a branch.
type NoteSetCmd struct {
	Text   string `arg:"" required:"" placeholder:"TEXT" help:"note text"`
	Target string `arg:"" optional:"" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir    string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for ID lookup"`
}

func (c *NoteSetCmd) Help() string {
	return `When run inside a worktree, target is optional (defaults to current branch).
When run outside, specify a worktree ID or branch name.

The note is stored in git config (branch.<name>.description) and displayed
in 'wt list' and 'wt tidy' output.

Examples:
  wt note set "Working on login flow"   # Inside worktree, current branch
  wt note set "Working on login" 1      # By worktree ID
  wt note set "In progress" feature-x   # By branch name`
}

func (c *NoteSetCmd) Run(ctx *Context) error {
	return runNoteSet(c)
}

// NoteGetCmd gets a note from a branch.
type NoteGetCmd struct {
	Target string `arg:"" optional:"" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir    string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for ID lookup"`
}

func (c *NoteGetCmd) Help() string {
	return `When run inside a worktree, target is optional (defaults to current branch).
When run outside, specify a worktree ID or branch name.

Prints the note if set, or nothing if no note exists.

Examples:
  wt note get            # Inside worktree, current branch
  wt note get 1          # By worktree ID
  wt note get feature-x  # By branch name`
}

func (c *NoteGetCmd) Run(ctx *Context) error {
	return runNoteGet(c)
}

// NoteClearCmd clears a note from a branch.
type NoteClearCmd struct {
	Target string `arg:"" optional:"" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir    string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for ID lookup"`
}

func (c *NoteClearCmd) Help() string {
	return `When run inside a worktree, target is optional (defaults to current branch).
When run outside, specify a worktree ID or branch name.

Removes the note from the branch. Safe to run even if no note exists.

Examples:
  wt note clear            # Inside worktree, current branch
  wt note clear 1          # By worktree ID
  wt note clear feature-x  # By branch name`
}

func (c *NoteClearCmd) Run(ctx *Context) error {
	return runNoteClear(c)
}

// NoteCmd manages branch notes.
type NoteCmd struct {
	Set   NoteSetCmd   `cmd:"" help:"Set a note on a branch"`
	Get   NoteGetCmd   `cmd:"" help:"Get the note for a branch"`
	Clear NoteClearCmd `cmd:"" help:"Clear the note from a branch"`
}

func (c *NoteCmd) Help() string {
	return `Notes are stored in git config and displayed in list/tidy output.
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
	Shell string `arg:"" required:"" placeholder:"SHELL" help:"shell type (fish, bash, zsh)" enum:"fish,bash,zsh"`
}

func (c *CompletionCmd) Help() string {
	return `Generate shell completion script

Outputs a completion script for the specified shell.
Redirect to the appropriate file for your shell.

Examples:
  wt completion fish > ~/.config/fish/completions/wt.fish
  wt completion bash > ~/.local/share/bash-completion/completions/wt
  wt completion zsh > ~/.zfunc/_wt  # then add ~/.zfunc to fpath`
}

func (c *CompletionCmd) Run(ctx *Context) error {
	return runCompletion(c)
}

// ConfigInitCmd creates the default config file.
type ConfigInitCmd struct {
	Force bool `short:"f" name:"force" negatable:"" help:"overwrite existing config file"`
}

func (c *ConfigInitCmd) Help() string {
	return `Create default config file at ~/.config/wt/config.toml
Examples:
  wt config init           # Create config if missing
  wt config init -f        # Overwrite existing config`
}

func (c *ConfigInitCmd) Run(ctx *Context) error {
	return runConfigInit(c)
}

// ConfigShowCmd shows the effective configuration.
type ConfigShowCmd struct {
	JSON bool `name:"json" help:"output as JSON"`
}

func (c *ConfigShowCmd) Help() string {
	return `Show effective configuration
Examples:
  wt config show           # Show config in text format
  wt config show --json    # Output as JSON`
}

func (c *ConfigShowCmd) Run(ctx *Context) error {
	return runConfigShow(c, ctx.Config)
}

// ConfigHooksCmd lists available hooks.
type ConfigHooksCmd struct {
	JSON bool `name:"json" help:"output as JSON"`
}

func (c *ConfigHooksCmd) Help() string {
	return `List available hooks
Examples:
  wt config hooks          # Show hooks in text format
  wt config hooks --json   # Output as JSON`
}

func (c *ConfigHooksCmd) Run(ctx *Context) error {
	return runConfigHooks(c, ctx.Config)
}

// ConfigCmd manages wt configuration.
type ConfigCmd struct {
	Init  ConfigInitCmd  `cmd:"" help:"Create default config file"`
	Show  ConfigShowCmd  `cmd:"" help:"Show effective configuration"`
	Hooks ConfigHooksCmd `cmd:"" help:"List available hooks"`
}

func (c *ConfigCmd) Help() string {
	return `Manage wt configuration
Examples:
  wt config init           # Create default config
  wt config show           # Show effective config
  wt config hooks          # List available hooks`
}

// HookRunCmd runs a hook by name for a worktree.
type HookRunCmd struct {
	Hook   string `arg:"" required:"" placeholder:"HOOK" help:"hook name to run"`
	Target string `arg:"" optional:"" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir    string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for target lookup"`
}

func (c *HookRunCmd) Help() string {
	return `When run inside a worktree, target is optional (defaults to current worktree).
When run outside, specify a worktree ID or branch name.

Examples:
  wt hook run kitty              # Inside worktree: run for current
  wt hook run kitty 1            # By worktree ID
  wt hook run kitty feature-x    # By branch name
  wt hook run kitty 1 -d ~/Git   # Specify directory for lookup`
}

func (c *HookRunCmd) Run(ctx *Context) error {
	return runHookRun(c, ctx.Config)
}

// HookCmd manages hooks.
type HookCmd struct {
	Run HookRunCmd `cmd:"" help:"Run a hook by name"`
}

func (c *HookCmd) Help() string {
	return `Examples:
  wt hook run kitty              # Run hook for current worktree
  wt hook run kitty 1            # Run hook for worktree ID 1`
}

// MvCmd moves worktrees to a different directory with optional renaming.
type MvCmd struct {
	Dir    string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"destination directory (flag > WT_DEFAULT_PATH > config)"`
	Format string `name:"format" placeholder:"FORMAT" help:"worktree naming format"`
	DryRun bool   `short:"n" name:"dry-run" negatable:"" help:"show what would be moved"`
	Force  bool   `short:"f" name:"force" negatable:"" help:"force move dirty worktrees"`
}

func (c *MvCmd) Help() string {
	return `Move worktrees to a different directory

Scans the current directory for worktrees and moves them to the destination
directory, optionally renaming them using the configured worktree format.

Examples:
  wt mv -d ~/Git/worktrees           # Move all worktrees to ~/Git/worktrees
  wt mv -d ~/Git --format={branch-name}  # Move and rename to just branch name
  wt mv --dry-run -d ~/Git           # Preview what would be moved
  wt mv -f -d ~/Git                  # Force move even if worktrees are dirty`
}

func (c *MvCmd) Run(ctx *Context) error {
	return runMv(c, ctx.Config)
}

// PrOpenCmd creates a worktree for a PR from an existing local repo.
type PrOpenCmd struct {
	Number int    `arg:"" required:"" placeholder:"NUMBER" help:"PR number"`
	Repo   string `arg:"" optional:"" placeholder:"REPO" help:"repository name to find locally"`
	Dir    string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Hook   string `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook bool   `name:"no-hook" help:"skip post-create hook" xor:"hook-ctrl"`
}

func (c *PrOpenCmd) Help() string {
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

func (c *PrOpenCmd) Run(ctx *Context) error {
	return runPrOpen(c, ctx.Config)
}

// PrCloneCmd clones a repo and creates a worktree for a PR.
type PrCloneCmd struct {
	Number int    `arg:"" required:"" placeholder:"NUMBER" help:"PR number"`
	Repo   string `arg:"" required:"" placeholder:"REPO" help:"repository (org/repo or repo if [clone] org configured)"`
	Dir    string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Forge  string `name:"forge" env:"WT_FORGE" placeholder:"FORGE" help:"forge: github or gitlab (flag > env > clone rules > config)"`
	Note   string `name:"note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook   string `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook bool   `name:"no-hook" help:"skip post-create hook" xor:"hook-ctrl"`
}

func (c *PrCloneCmd) Help() string {
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

func (c *PrCloneCmd) Run(ctx *Context) error {
	return runPrClone(c, ctx.Config)
}

// PrRefreshCmd fetches PR status for worktrees.
type PrRefreshCmd struct {
	Dir string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
}

func (c *PrRefreshCmd) Help() string {
	return `Refresh PR status cache for all worktrees

Queries GitHub/GitLab for PR info on each worktree branch and caches the results.
Run this to update PR info shown in 'wt list' and 'wt tidy'.

Examples:
  wt pr refresh                # Refresh PR status for worktrees
  wt pr refresh -d ~/Git       # Refresh for worktrees in specific directory`
}

func (c *PrRefreshCmd) Run(ctx *Context) error {
	return runPrRefresh(c, ctx.Config)
}

// PrMergeCmd merges the PR for the current branch.
type PrMergeCmd struct {
	Target   string `arg:"" optional:"" placeholder:"ID|BRANCH" help:"worktree ID or branch (optional in worktree)"`
	Dir      string `short:"d" name:"dir" env:"WT_DEFAULT_PATH" placeholder:"DIR" help:"worktree directory for target lookup"`
	Strategy string `short:"s" name:"strategy" env:"WT_MERGE_STRATEGY" placeholder:"STRATEGY" help:"merge strategy: squash, rebase, or merge"`
	Keep     bool   `short:"k" name:"keep" help:"keep worktree and branch after merge"`
	Hook     string `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook   bool   `name:"no-hook" help:"skip post-merge hook" xor:"hook-ctrl"`
}

func (c *PrMergeCmd) Help() string {
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

func (c *PrMergeCmd) Run(ctx *Context) error {
	return runPrMerge(c, ctx.Config)
}

// PrCmd works with PRs.
type PrCmd struct {
	Open    PrOpenCmd    `cmd:"" help:"Checkout PR from existing local repo"`
	Clone   PrCloneCmd   `cmd:"" help:"Clone repo and checkout PR"`
	Refresh PrRefreshCmd `cmd:"" help:"Refresh PR status cache"`
	Merge   PrMergeCmd   `cmd:"" help:"Merge PR and clean up worktree"`
}

func (c *PrCmd) Help() string {
	return `Examples:
  wt pr open 123             # PR from current repo
  wt pr open 123 myrepo      # PR from existing local repo
  wt pr clone 123 org/repo   # Clone repo and checkout PR
  wt pr refresh              # Refresh PR status cache
  wt pr merge                # Merge PR and clean up worktree`
}

// VersionFlag is used to show version info.
type VersionFlag bool

// CLI is the root command.
type CLI struct {
	// Core commands (ungrouped - shown first)
	Create CreateCmd `cmd:"" help:"Create a new worktree"`
	Open   OpenCmd   `cmd:"" help:"Open worktree for existing branch"`
	List   ListCmd   `cmd:"" default:"withargs" help:"List worktrees"`
	Tidy   TidyCmd   `cmd:"" help:"Tidy up merged worktrees"`

	// PR commands
	Pr PrCmd `cmd:"" help:"Work with PRs" group:"pr"`

	// Utility commands
	Exec ExecCmd `cmd:"" help:"Run command in worktree by ID" group:"util"`
	Cd   CdCmd   `cmd:"" help:"Print worktree path" group:"util"`
	Mv   MvCmd   `cmd:"" help:"Move worktrees to another directory" group:"util"`
	Note NoteCmd `cmd:"" help:"Manage branch notes" group:"util"`
	Hook HookCmd `cmd:"" help:"Manage hooks" group:"util"`

	// Configuration commands
	Config     ConfigCmd     `cmd:"" help:"Manage configuration" group:"config"`
	Completion CompletionCmd `cmd:"" help:"Generate completion script" group:"config"`

	Version VersionFlag `name:"version" help:"Show version"`
}

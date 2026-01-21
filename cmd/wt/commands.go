package main

import "github.com/raphi011/wt/internal/config"

// Context is passed to all command Run() methods.
type Context struct {
	Config *config.Config
}

// AddCmd adds a worktree for an existing or new branch.
type AddCmd struct {
	Branch     string   `arg:"" optional:"" placeholder:"BRANCH" help:"branch name"`
	Repository []string `short:"r" name:"repository" sep:"," help:"repository name(s) to create worktree in (repeatable, comma-separated)"`
	Label      []string `short:"l" name:"label" sep:"," help:"target repos by label (repeatable, comma-separated)"`
	Dir        string   `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	NewBranch  bool     `short:"b" name:"new-branch" help:"create a new branch"`
	Note       string   `name:"note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook       string   `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook     bool     `name:"no-hook" help:"skip post-add hook" xor:"hook-ctrl"`
	Env        []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

func (c *AddCmd) Help() string {
	return `Use -b to create a new branch, or omit for an existing branch.
Use -r to target repos by name, -l to target repos by label.

Examples:
  wt add feature-branch              # Existing branch in current repo
  wt add -b feature-branch           # Create new branch in current repo
  wt add feature-branch -d ~/Git     # Specify target directory
  wt add feature-branch --no-hook    # Skip post-add hook
  wt add -b feature -r repo1 -r repo2 -d ~/Git    # By repo name
  wt add -b feature -l backend -d ~/Git           # By label
  wt add -b feature -l backend -r extra -d ~/Git  # Mixed`
}

func (c *AddCmd) Run(ctx *Context) error {
	return runAdd(c, ctx.Config)
}

// PruneCmd removes merged and clean worktrees.
type PruneCmd struct {
	ID           []int    `short:"i" name:"id" help:"worktree(s) to remove (by ID, repeatable)"`
	Dir          string   `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	DryRun       bool     `short:"n" name:"dry-run" negatable:"" help:"preview without removing"`
	Force        bool     `short:"f" name:"force" help:"force remove even if not merged or has uncommitted changes"`
	IncludeClean bool     `short:"c" name:"include-clean" help:"also remove worktrees with 0 commits ahead and clean working directory"`
	Global       bool     `short:"g" name:"global" help:"prune all worktrees (not just current repo)"`
	Refresh      bool     `short:"r" name:"refresh" help:"fetch origin and refresh PR status before pruning"`
	ResetCache   bool     `name:"reset-cache" help:"clear all cached data (PR info, worktree history) and reset IDs from 1"`
	Hook         string   `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook       bool     `name:"no-hook" help:"skip post-removal hooks" xor:"hook-ctrl"`
	Env          []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

func (c *PruneCmd) Help() string {
	return `Without --id, removes all worktrees where the branch is merged AND
working directory is clean. With --id, removes only that specific worktree.

When run inside a git repository, only prunes worktrees for that repo.
Use --global to prune worktrees from all repos in the directory.

Uses cached merge status and PR info. Use --refresh to fetch from origin and
update PR status from GitHub/GitLab.

Hooks with on=["prune"] run after each worktree removal. Hooks run with
working directory set to the main repo (since worktree path is deleted).

Merge detection uses git merge-base locally, which may miss squash-merged
or rebased PRs. For accurate detection, use GitHub/GitLab where PR status
shows if the branch was merged.

Examples:
  wt prune -r                   # Fetch origin + PR status, then prune
  wt prune                      # Remove merged worktrees (uses cached PR info)
  wt prune --global              # Prune all repos (not just current)
  wt prune -n                   # Dry-run: preview without removing
  wt prune -d ~/Git/worktrees   # Scan specific directory
  wt prune -c                   # Also remove clean (0-commit) worktrees
  wt prune -i 1                 # Remove specific worktree by ID
  wt prune -i 1 -i 2 -i 3       # Remove multiple worktrees by ID
  wt prune -i 1 -f              # Force remove even if not merged/dirty
  wt prune --no-hook            # Skip post-removal hooks
  wt prune --hook=cleanup       # Run 'cleanup' hook instead of default
  wt prune --reset-cache        # Clear PR cache and reset IDs from 1`
}

func (c *PruneCmd) Run(ctx *Context) error {
	return runPrune(c, ctx.Config)
}

// ListCmd lists worktrees in a directory.
type ListCmd struct {
	Dir     string `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	JSON    bool   `name:"json" help:"output as JSON"`
	Global  bool   `short:"g" name:"global" help:"show all worktrees (not just current repo)"`
	Sort    string `short:"s" name:"sort" default:"id" enum:"id,repo,branch" help:"sort by: id, repo, branch"`
	Refresh bool   `short:"r" name:"refresh" help:"fetch origin and refresh PR status before listing"`
}

func (c *ListCmd) Help() string {
	return `IDs are stable across runs - use them with 'wt exec -i'.
Inside a repo: shows only that repo's worktrees. Use --global for all.

Examples:
  wt list                      # List worktrees for current repo
  wt list -r                   # Refresh PR status before listing
  wt list --global              # List all worktrees (all repos)
  wt list -d ~/Git/worktrees   # List from specific directory
  wt list --json               # Output as JSON for scripting
  wt list --sort=repo          # Sort by repository name`
}

func (c *ListCmd) Run(ctx *Context) error {
	return runList(c, ctx.Config)
}

// ShowCmd shows detailed status for a single worktree.
type ShowCmd struct {
	Dir     string `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"directory to scan for worktrees"`
	ID      int    `short:"i" name:"id" help:"worktree ID (optional in worktree)"`
	Refresh bool   `short:"r" name:"refresh" help:"refresh PR status from API"`
	JSON    bool   `name:"json" help:"output as JSON"`
}

func (c *ShowCmd) Help() string {
	return `Inside a worktree: --id is optional (defaults to current).
Outside: specify a worktree ID.

Examples:
  wt show              # Inside worktree, show current
  wt show -i 3         # By worktree ID
  wt show --refresh    # Refresh PR status from API
  wt show --json       # Output as JSON`
}

func (c *ShowCmd) Run(ctx *Context) error {
	return runShow(c, ctx.Config)
}

// ExecCmd runs a command in one or more worktrees by ID.
type ExecCmd struct {
	ID      []int    `short:"i" name:"id" required:"" help:"worktree ID(s) (repeatable)"`
	Command []string `arg:"" optional:"" passthrough:"" placeholder:"COMMAND" help:"command to run (after --)"`
	Dir     string   `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
}

func (c *ExecCmd) Help() string {
	return `Use 'wt list' to see worktree IDs. Supports multiple -i flags.

Examples:
  wt exec -i 1 -- gh pr view         # By worktree ID
  wt exec -i 1 -i 2 -- git status    # Multiple worktrees
  wt exec -i 1 -- code .             # Open worktree in VS Code`
}

func (c *ExecCmd) Run(ctx *Context) error {
	return runExec(c)
}

// CdCmd prints the path of a worktree for shell scripting.
type CdCmd struct {
	ID      int    `short:"i" name:"id" required:"" help:"worktree ID"`
	Dir     string `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"directory to scan for worktrees"`
	Project bool   `short:"p" name:"project" help:"print main repository path instead of worktree path"`
}

func (c *CdCmd) Help() string {
	return `Use with shell command substitution: cd $(wt cd -i 1)
Use -p to get the main repository path instead.

Examples:
  cd $(wt cd -i 1)
  cd $(wt cd -p -i 1)  # cd to main repo`
}

func (c *CdCmd) Run(ctx *Context) error {
	return runCd(c)
}

// NoteSetCmd sets a note on a branch.
type NoteSetCmd struct {
	Text string `arg:"" required:"" placeholder:"TEXT" help:"note text"`
	ID   int    `short:"i" name:"id" help:"worktree ID (optional in worktree)"`
	Dir  string `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"worktree directory for ID lookup"`
}

func (c *NoteSetCmd) Help() string {
	return `When run inside a worktree, --id is optional (defaults to current branch).
When run outside, specify a worktree ID.

The note is stored in git config (branch.<name>.description) and displayed
in 'wt list' and 'wt prune' output.

Examples:
  wt note set "Working on login flow"      # Inside worktree, current branch
  wt note set "Working on login" -i 1      # By worktree ID`
}

func (c *NoteSetCmd) Run(ctx *Context) error {
	return runNoteSet(c)
}

// NoteGetCmd gets a note from a branch.
type NoteGetCmd struct {
	ID  int    `short:"i" name:"id" help:"worktree ID (optional in worktree)"`
	Dir string `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"worktree directory for ID lookup"`
}

func (c *NoteGetCmd) Help() string {
	return `When run inside a worktree, --id is optional (defaults to current branch).
When run outside, specify a worktree ID.

Prints the note if set, or nothing if no note exists.

Examples:
  wt note get        # Inside worktree, current branch
  wt note get -i 1   # By worktree ID`
}

func (c *NoteGetCmd) Run(ctx *Context) error {
	return runNoteGet(c)
}

// NoteClearCmd clears a note from a branch.
type NoteClearCmd struct {
	ID  int    `short:"i" name:"id" help:"worktree ID (optional in worktree)"`
	Dir string `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"worktree directory for ID lookup"`
}

func (c *NoteClearCmd) Help() string {
	return `When run inside a worktree, --id is optional (defaults to current branch).
When run outside, specify a worktree ID.

Removes the note from the branch. Safe to run even if no note exists.

Examples:
  wt note clear        # Inside worktree, current branch
  wt note clear -i 1   # By worktree ID`
}

func (c *NoteClearCmd) Run(ctx *Context) error {
	return runNoteClear(c)
}

// NoteCmd manages branch notes. Get is default subcommand.
type NoteCmd struct {
	Set   NoteSetCmd   `cmd:"" help:"Set a note on a branch"`
	Get   NoteGetCmd   `cmd:"" default:"withargs" help:"Get the note for a branch (default)"`
	Clear NoteClearCmd `cmd:"" help:"Clear the note from a branch"`
}

func (c *NoteCmd) Help() string {
	return `Notes are stored in git config and displayed in list/tidy output.
Inside a worktree, operates on current branch. Outside, specify ID.

Examples:
  wt note set "Working on login"      # Inside worktree
  wt note set "Working on login" -i 1 # By worktree ID
  wt note get                         # Inside worktree
  wt note get -i 1                    # By worktree ID
  wt note clear -i 1                  # By worktree ID`
}

// LabelAddCmd adds a label to a repository.
type LabelAddCmd struct {
	Label string `arg:"" required:"" placeholder:"LABEL" help:"label to add"`
	Dir   string `short:"d" name:"dir" placeholder:"DIR" help:"repository directory (optional inside repo)"`
}

func (c *LabelAddCmd) Help() string {
	return `Add a label to the current repository (or --dir).
Labels are stored in git config (wt.labels) and can be used with 'wt add -l'.

Examples:
  wt label add backend      # Add label to current repo
  wt label add -d ~/code/api backend`
}

func (c *LabelAddCmd) Run(ctx *Context) error {
	return runLabelAdd(c)
}

// LabelRemoveCmd removes a label from a repository.
type LabelRemoveCmd struct {
	Label string `arg:"" required:"" placeholder:"LABEL" help:"label to remove"`
	Dir   string `short:"d" name:"dir" placeholder:"DIR" help:"repository directory (optional inside repo)"`
}

func (c *LabelRemoveCmd) Help() string {
	return `Remove a label from the current repository (or --dir).

Examples:
  wt label remove backend      # Remove from current repo
  wt label remove -d ~/code/api backend`
}

func (c *LabelRemoveCmd) Run(ctx *Context) error {
	return runLabelRemove(c)
}

// LabelListCmd lists labels for a repository.
type LabelListCmd struct {
	Dir string `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"repository directory (or scan directory with --all)"`
	All bool   `short:"a" name:"all" help:"list all labels from repos in directory"`
}

func (c *LabelListCmd) Help() string {
	return `List labels for the current repository, or all repos with --all.

Examples:
  wt label list             # Labels for current repo
  wt label list -a          # All labels from repos in current dir
  wt label list -a -d ~/code`
}

func (c *LabelListCmd) Run(ctx *Context) error {
	return runLabelList(c, ctx.Config)
}

// LabelClearCmd clears all labels from a repository.
type LabelClearCmd struct {
	Dir string `short:"d" name:"dir" placeholder:"DIR" help:"repository directory (optional inside repo)"`
}

func (c *LabelClearCmd) Help() string {
	return `Clear all labels from the current repository (or --dir).

Examples:
  wt label clear            # Clear labels from current repo
  wt label clear -d ~/code/api`
}

func (c *LabelClearCmd) Run(ctx *Context) error {
	return runLabelClear(c)
}

// LabelCmd manages repository labels.
type LabelCmd struct {
	Add    LabelAddCmd    `cmd:"" help:"Add a label to a repository"`
	Remove LabelRemoveCmd `cmd:"" help:"Remove a label from a repository"`
	List   LabelListCmd   `cmd:"" default:"withargs" help:"List labels (default)"`
	Clear  LabelClearCmd  `cmd:"" help:"Clear all labels from a repository"`
}

func (c *LabelCmd) Help() string {
	return `Labels are stored in git config (wt.labels) and can target repos with 'wt add -l'.

Examples:
  wt label add backend        # Add label to current repo
  wt label remove backend     # Remove label
  wt label list               # List labels for current repo
  wt label list -a            # List all labels across repos
  wt label clear              # Clear all labels

Use with wt add:
  wt add -b feature -l backend   # Create worktree in all repos with 'backend' label`
}

// CompletionCmd generates shell completion scripts.
type CompletionCmd struct {
	Shell string `arg:"" required:"" placeholder:"SHELL" help:"shell type (fish, bash, zsh)" enum:"fish,bash,zsh"`
}

func (c *CompletionCmd) Help() string {
	return `Redirect output to the appropriate file for your shell.

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
	return `Config file: ~/.config/wt/config.toml

Examples:
  wt config init           # Create default config
  wt config show           # Show effective config
  wt config hooks          # List available hooks`
}

// HookCmd runs one or more hooks by name for a worktree.
type HookCmd struct {
	Hooks []string `arg:"" required:"" placeholder:"HOOK" help:"hook name(s) to run"`
	ID    []int    `short:"i" name:"id" help:"worktree ID(s) (optional in worktree, repeatable)"`
	Dir   string   `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"worktree directory for target lookup"`
	Env   []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

func (c *HookCmd) Help() string {
	return `When run inside a worktree, --id is optional (defaults to current worktree).
When run outside, specify a worktree ID. Multiple IDs can be specified.

Examples:
  wt hook kitty              # Single hook (current worktree)
  wt hook kitty idea         # Multiple hooks
  wt hook kitty -i 1         # By worktree ID
  wt hook kitty -i 1 -i 2    # Multiple worktrees
  wt hook kitty idea -i 1 -d ~/Git   # Multiple hooks with ID`
}

func (c *HookCmd) Run(ctx *Context) error {
	return runHookRun(c, ctx.Config)
}

// MvCmd moves worktrees to a different directory with optional renaming.
type MvCmd struct {
	Dir    string `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"destination directory (flag > WT_DEFAULT_PATH > config)"`
	Format string `name:"format" placeholder:"FORMAT" help:"worktree naming format"`
	DryRun bool   `short:"n" name:"dry-run" negatable:"" help:"show what would be moved"`
	Force  bool   `short:"f" name:"force" negatable:"" help:"force move dirty worktrees"`
}

func (c *MvCmd) Help() string {
	return `Scans current directory for worktrees and moves them to destination.
Use --format to rename during move.

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
	Number int      `arg:"" required:"" placeholder:"NUMBER" help:"PR number"`
	Repo   string   `arg:"" optional:"" placeholder:"REPO" help:"repository name to find locally"`
	Dir    string   `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Hook   string   `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook bool     `name:"no-hook" help:"skip post-create hook" xor:"hook-ctrl"`
	Env    []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

func (c *PrOpenCmd) Help() string {
	return `Only works with repos that already exist locally.
Use 'wt pr clone' to clone new repos first.

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
	Number int      `arg:"" required:"" placeholder:"NUMBER" help:"PR number"`
	Repo   string   `arg:"" required:"" placeholder:"REPO" help:"repository (org/repo or repo if [clone] org configured)"`
	Dir    string   `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Forge  string   `name:"forge" env:"WT_FORGE" placeholder:"FORGE" help:"forge: github or gitlab (flag > env > clone rules > config)"`
	Note   string   `name:"note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook   string   `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook bool     `name:"no-hook" help:"skip post-create hook" xor:"hook-ctrl"`
	Env    []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

func (c *PrCloneCmd) Help() string {
	return `Clones the repository if not present locally.
Use 'wt pr open' for repos you already have.
If [clone] org is configured, you can omit the org/ prefix.

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

// PrMergeCmd merges the PR for the current branch.
type PrMergeCmd struct {
	ID       int      `short:"i" name:"id" help:"worktree ID (optional in worktree)"`
	Dir      string   `short:"d" name:"dir" env:"WT_WORKTREE_DIR" placeholder:"DIR" help:"worktree directory for target lookup"`
	Strategy string   `short:"s" name:"strategy" env:"WT_MERGE_STRATEGY" placeholder:"STRATEGY" help:"merge strategy: squash, rebase, or merge"`
	Keep     bool     `short:"k" name:"keep" help:"keep worktree and branch after merge"`
	Hook     string   `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook   bool     `name:"no-hook" help:"skip post-merge hook" xor:"hook-ctrl"`
	Env      []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

func (c *PrMergeCmd) Help() string {
	return `Inside a worktree: --id is optional (defaults to current branch).
Outside: specify a worktree ID.

Merges the PR, removes the worktree, and deletes the local branch.
Use --keep to preserve the worktree after merge.
Merge strategy: flag > WT_MERGE_STRATEGY env > config. GitLab doesn't support rebase.

Examples:
  wt pr merge                    # Inside worktree: merge current branch's PR
  wt pr merge -i 1 -d ~/Git      # By worktree ID
  wt pr merge --keep             # Merge PR only, keep worktree and branch
  wt pr merge -s rebase          # Use rebase merge strategy`
}

func (c *PrMergeCmd) Run(ctx *Context) error {
	return runPrMerge(c, ctx.Config)
}

// PrCmd works with PRs.
type PrCmd struct {
	Open  PrOpenCmd  `cmd:"" help:"Checkout PR from existing local repo"`
	Clone PrCloneCmd `cmd:"" help:"Clone repo and checkout PR"`
	Merge PrMergeCmd `cmd:"" help:"Merge PR and clean up worktree"`
}

func (c *PrCmd) Help() string {
	return `Examples:
  wt pr open 123             # PR from current repo
  wt pr open 123 myrepo      # PR from existing local repo
  wt pr clone 123 org/repo   # Clone repo and checkout PR
  wt pr merge                # Merge PR and clean up worktree`
}

// VersionFlag is used to show version info.
type VersionFlag bool

// CLI is the root command.
type CLI struct {
	// Core commands (ungrouped - shown first)
	Add   AddCmd   `cmd:"" help:"Add worktree for branch"`
	List  ListCmd  `cmd:"" help:"List worktrees"`
	Show  ShowCmd  `cmd:"" help:"Show worktree details"`
	Prune PruneCmd `cmd:"" help:"Prune merged worktrees"`

	// PR commands
	Pr PrCmd `cmd:"" help:"Work with PRs" group:"pr"`

	// Utility commands
	Exec ExecCmd `cmd:"" help:"Run command in worktree by ID" group:"util"`
	Cd   CdCmd   `cmd:"" help:"Print worktree path" group:"util"`
	Mv   MvCmd   `cmd:"" help:"Move worktrees to another directory" group:"util"`
	Note  NoteCmd  `cmd:"" help:"Manage branch notes" group:"util"`
	Label LabelCmd `cmd:"" help:"Manage repository labels" group:"util"`
	Hook  HookCmd  `cmd:"" help:"Manage hooks" group:"util"`

	// Configuration commands
	Config     ConfigCmd     `cmd:"" help:"Manage configuration" group:"config"`
	Completion CompletionCmd `cmd:"" help:"Generate completion script" group:"config"`

	Version VersionFlag `short:"v" name:"version" help:"Show version"`
}

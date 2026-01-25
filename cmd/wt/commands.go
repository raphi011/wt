package main

import (
	"context"
	"io"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
)

// Context is passed to all command Run() methods.
type Context struct {
	Config  *config.Config
	WorkDir string          // Injected working directory (for testability)
	Stdout  io.Writer       // Injected stdout (for testability)
	Ctx     context.Context // Context for cancellation and logging
}

// Logger returns the logger from the context.
func (c *Context) Logger() *log.Logger {
	return log.FromContext(c.Ctx)
}

// BeforeApply runs before any command and verifies git is available.
func (c *Context) BeforeApply() error {
	return git.CheckGit()
}

// AddCmd adds a worktree for an existing or new branch.
type AddCmd struct {
	Branch     string   `arg:"" optional:"" placeholder:"BRANCH" help:"branch name"`
	Repository []string `short:"r" name:"repository" sep:"," help:"repository name(s) to create worktree in (repeatable, comma-separated)"`
	Label      []string `short:"l" name:"label" sep:"," help:"target repos by label (repeatable, comma-separated)"`
	NewBranch  bool     `short:"b" name:"new-branch" help:"create a new branch"`
	Base       string   `name:"base" placeholder:"BRANCH" help:"base branch to create from (default: main/master)"`
	Fetch      bool     `short:"f" name:"fetch" help:"fetch base branch from origin before creating"`
	Note       string   `name:"note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook       string   `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook     bool     `name:"no-hook" help:"skip post-add hook" xor:"hook-ctrl"`
	Env        []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

func (c *AddCmd) Help() string {
	return `Use -b to create a new branch, or omit for an existing branch.
Use -r to target repos by name, -l to target repos by label.

New branches are created from the default branch (main/master) by default.
Use --base to specify a different base branch.
Use -f/--fetch to fetch the base branch before creating (ensures up-to-date).

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt add feature-branch              # Existing branch in current repo
  wt add -b feature-branch           # Create new branch from origin/main
  wt add -b feature-branch -f        # Fetch main first, then create branch
  wt add -b feature-branch --base develop  # Create from origin/develop
  wt add feature-branch --no-hook    # Skip post-add hook
  wt add -b feature -r repo1 -r repo2      # By repo name
  wt add -b feature -l backend             # By label
  wt add -b feature -l backend -r extra    # Mixed`
}

func (c *AddCmd) Run(ctx *Context) error {
	return runAdd(c, ctx.Config, ctx.WorkDir)
}

// PruneCmd removes merged and clean worktrees.
type PruneCmd struct {
	ID           []int    `short:"i" name:"id" help:"worktree(s) to remove (by ID, repeatable)"`
	DryRun       bool     `short:"n" name:"dry-run" negatable:"" help:"preview without removing"`
	Force        bool     `short:"f" name:"force" help:"force remove targeted worktree (-i) even if not merged or dirty"`
	IncludeClean bool     `short:"c" name:"include-clean" help:"also remove worktrees with 0 commits ahead and clean working directory"`
	Verbose      bool     `short:"v" name:"verbose" help:"also show non-prunable worktrees with reasons"`
	Global       bool     `short:"g" name:"global" help:"prune all worktrees (not just current repo)"`
	Refresh      bool     `short:"R" name:"refresh" help:"fetch origin and refresh PR status before pruning"`
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

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt prune -R                   # Fetch origin + PR status, then prune
  wt prune                      # Remove merged worktrees (uses cached PR info)
  wt prune --global             # Prune all repos (not just current)
  wt prune -n                   # Dry-run: preview without removing
  wt prune -n --verbose         # Dry-run with skip reasons shown
  wt prune -c                   # Also remove clean (0-commit) worktrees
  wt prune -i 1                 # Remove specific worktree by ID
  wt prune -i 1 -i 2 -i 3       # Remove multiple worktrees by ID
  wt prune -i 1 -f              # Force remove even if not merged/dirty
  wt prune --no-hook            # Skip post-removal hooks
  wt prune --hook=cleanup       # Run 'cleanup' hook instead of default
  wt prune --reset-cache        # Clear PR cache and reset IDs from 1`
}

func (c *PruneCmd) Run(ctx *Context) error {
	return runPrune(c, ctx.Config, ctx.WorkDir)
}

// ListCmd lists worktrees in a directory.
type ListCmd struct {
	JSON       bool     `name:"json" help:"output as JSON"`
	Global     bool     `short:"g" name:"global" help:"show all worktrees (not just current repo)"`
	Sort       string   `short:"s" name:"sort" default:"" enum:",id,repo,branch,commit" help:"sort by: id, repo, branch, commit (default from config or 'id')"`
	Refresh    bool     `short:"R" name:"refresh" help:"fetch origin and refresh PR status before listing"`
	Repository []string `short:"r" name:"repository" sep:"," help:"filter by repository name(s) (repeatable, comma-separated)"`
	Label      []string `short:"l" name:"label" sep:"," help:"filter by label(s) (repeatable, comma-separated)"`
}

func (c *ListCmd) Help() string {
	return `IDs are stable across runs - use them with 'wt exec -i'.
Inside a repo: shows only that repo's worktrees. Use --global for all.
Use -r to filter by repo name(s), -l to filter by label(s).

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt list                      # List worktrees for current repo
  wt list -R                   # Refresh PR status before listing
  wt list --global             # List all worktrees (all repos)
  wt list -r myrepo            # Filter by repository name
  wt list -l backend           # Filter by label
  wt list -r repo1 -l team     # Combined filters
  wt list --json               # Output as JSON for scripting
  wt list --sort=commit        # Sort by most recent commit`
}

func (c *ListCmd) Run(ctx *Context) error {
	return runList(c, ctx.Config, ctx.WorkDir, ctx.Stdout)
}

// ShowCmd shows detailed status for a single worktree.
type ShowCmd struct {
	ID         int    `short:"i" name:"id" xor:"target" help:"worktree ID (optional in worktree/repo)"`
	Repository string `short:"r" name:"repository" xor:"target" help:"repository name"`
	Refresh    bool   `short:"R" name:"refresh" help:"refresh PR status from API"`
	JSON       bool   `name:"json" help:"output as JSON"`
}

func (c *ShowCmd) Help() string {
	return `Inside a worktree or repo: --id is optional (defaults to current branch).
Outside: specify a worktree ID or repo name.

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt show              # Inside worktree/repo, show current branch
  wt show -i 3         # By worktree ID
  wt show -r myrepo    # By repository name
  wt show --refresh    # Refresh PR status from API
  wt show --json       # Output as JSON`
}

func (c *ShowCmd) Run(ctx *Context) error {
	return runShow(c, ctx.Config, ctx.WorkDir)
}

// ExecCmd runs a command in one or more worktrees by ID, or in repos by name/label.
type ExecCmd struct {
	ID         []int    `short:"i" name:"id" xor:"target" help:"worktree ID(s) (repeatable)"`
	Repository []string `short:"r" name:"repository" xor:"target" sep:"," help:"repository name(s) (repeatable, comma-separated)"`
	Label      []string `short:"l" name:"label" xor:"target" sep:"," help:"target repos by label (repeatable, comma-separated)"`
	Command    []string `arg:"" optional:"" passthrough:"" placeholder:"COMMAND" help:"command to run (after --)"`
}

func (c *ExecCmd) Help() string {
	return `Use 'wt list' to see worktree IDs. Supports multiple -i flags.
Use -r to target repos by name, -l to target repos by label.
When using -r or -l, the command runs in the main repo directory (not worktrees).

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt exec -i 1 -- gh pr view           # By worktree ID
  wt exec -i 1 -i 2 -- git status      # Multiple worktrees
  wt exec -i 1 -- code .               # Open worktree in VS Code
  wt exec -r wt -- git status          # By repo name
  wt exec -l backend -- make           # By label`
}

func (c *ExecCmd) Run(ctx *Context) error {
	return runExec(c, ctx.Config, ctx.WorkDir)
}

// CdCmd prints the path of a worktree or repo for shell scripting.
type CdCmd struct {
	ID         int      `short:"i" name:"id" xor:"target" help:"worktree ID"`
	Repository string   `short:"r" name:"repository" xor:"target" help:"repository name"`
	Label      string   `short:"l" name:"label" xor:"target" help:"repository label (must match exactly one repo)"`
	Project    bool     `short:"p" name:"project" help:"print main repository path instead of worktree path"`
	Hook       string   `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook     bool     `name:"no-hook" help:"skip hooks" xor:"hook-ctrl"`
	Env        []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

func (c *CdCmd) Help() string {
	return `Use with shell command substitution: cd $(wt cd -i 1)
Use -p to get the main repository path instead.
Use -r to get the path of a repository by name.
Use -l to get the path of a repo by label (must match exactly one repo).

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  cd $(wt cd -i 1)         # By worktree ID
  cd $(wt cd -p -i 1)      # cd to main repo of worktree
  cd $(wt cd -r wt)        # By repo name
  cd $(wt cd -l backend)   # By label (must match exactly one repo)`
}

func (c *CdCmd) Run(ctx *Context) error {
	return runCd(c, ctx.Config, ctx.WorkDir, ctx.Stdout)
}

// NoteSetCmd sets a note on a branch.
type NoteSetCmd struct {
	Text       string `arg:"" required:"" placeholder:"TEXT" help:"note text"`
	ID         int    `short:"i" name:"id" xor:"target" help:"worktree ID (optional in worktree/repo)"`
	Repository string `short:"r" name:"repository" xor:"target" help:"repository name"`
}

func (c *NoteSetCmd) Help() string {
	return `When run inside a worktree or repo, --id is optional (defaults to current branch).
When run outside, specify a worktree ID or repo name.

The note is stored in git config (branch.<name>.description) and displayed
in 'wt list' and 'wt prune' output.

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt note set "Working on login flow"      # Inside worktree/repo, current branch
  wt note set "Working on login" -i 1      # By worktree ID
  wt note set "Working on login" -r myrepo # By repository name`
}

func (c *NoteSetCmd) Run(ctx *Context) error {
	return runNoteSet(c, ctx.Config, ctx.WorkDir, ctx.Stdout)
}

// NoteGetCmd gets a note from a branch.
type NoteGetCmd struct {
	ID         int    `short:"i" name:"id" xor:"target" help:"worktree ID (optional in worktree/repo)"`
	Repository string `short:"r" name:"repository" xor:"target" help:"repository name"`
}

func (c *NoteGetCmd) Help() string {
	return `When run inside a worktree or repo, --id is optional (defaults to current branch).
When run outside, specify a worktree ID or repo name.

Prints the note if set, or nothing if no note exists.

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt note get           # Inside worktree/repo, current branch
  wt note get -i 1      # By worktree ID
  wt note get -r myrepo # By repository name`
}

func (c *NoteGetCmd) Run(ctx *Context) error {
	return runNoteGet(c, ctx.Config, ctx.WorkDir, ctx.Stdout)
}

// NoteClearCmd clears a note from a branch.
type NoteClearCmd struct {
	ID         int    `short:"i" name:"id" xor:"target" help:"worktree ID (optional in worktree/repo)"`
	Repository string `short:"r" name:"repository" xor:"target" help:"repository name"`
}

func (c *NoteClearCmd) Help() string {
	return `When run inside a worktree or repo, --id is optional (defaults to current branch).
When run outside, specify a worktree ID or repo name.

Removes the note from the branch. Safe to run even if no note exists.

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt note clear           # Inside worktree/repo, current branch
  wt note clear -i 1      # By worktree ID
  wt note clear -r myrepo # By repository name`
}

func (c *NoteClearCmd) Run(ctx *Context) error {
	return runNoteClear(c, ctx.Config, ctx.WorkDir, ctx.Stdout)
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
	Label      string   `arg:"" required:"" placeholder:"LABEL" help:"label to add"`
	Repository []string `short:"r" name:"repository" sep:"," help:"repository name(s) (repeatable, comma-separated)"`
}

func (c *LabelAddCmd) Help() string {
	return `Add a label to the current repository (or specify repos with -r).
Labels are stored in git config (wt.labels) and can be used with 'wt add -l'.

Target directory is set via WT_REPO_DIR env var or repo_dir/worktree_dir config.

Examples:
  wt label add backend           # Add label to current repo
  wt label add backend -r api    # Add label to 'api' repo
  wt label add backend -r api,web  # Add label to multiple repos`
}

func (c *LabelAddCmd) Run(ctx *Context) error {
	return runLabelAdd(c, ctx.Config, ctx.WorkDir)
}

// LabelRemoveCmd removes a label from a repository.
type LabelRemoveCmd struct {
	Label      string   `arg:"" required:"" placeholder:"LABEL" help:"label to remove"`
	Repository []string `short:"r" name:"repository" sep:"," help:"repository name(s) (repeatable, comma-separated)"`
}

func (c *LabelRemoveCmd) Help() string {
	return `Remove a label from the current repository (or specify repos with -r).

Target directory is set via WT_REPO_DIR env var or repo_dir/worktree_dir config.

Examples:
  wt label remove backend           # Remove from current repo
  wt label remove backend -r api    # Remove from 'api' repo
  wt label remove backend -r api,web  # Remove from multiple repos`
}

func (c *LabelRemoveCmd) Run(ctx *Context) error {
	return runLabelRemove(c, ctx.Config, ctx.WorkDir)
}

// LabelListCmd lists labels for a repository.
type LabelListCmd struct {
	Repository []string `short:"r" name:"repository" sep:"," help:"repository name(s) (repeatable, comma-separated)"`
	Global     bool     `short:"g" name:"global" help:"list all labels from repos in directory"`
}

func (c *LabelListCmd) Help() string {
	return `List labels for the current repository, specific repos with -r, or all repos with -g.

Target directory is set via WT_REPO_DIR env var or repo_dir/worktree_dir config.

Examples:
  wt label list              # Labels for current repo
  wt label list -r api       # Labels for 'api' repo
  wt label list -r api,web   # Labels for multiple repos
  wt label list -g           # All labels from repos in directory`
}

func (c *LabelListCmd) Run(ctx *Context) error {
	return runLabelList(c, ctx.Config, ctx.WorkDir)
}

// LabelClearCmd clears all labels from a repository.
type LabelClearCmd struct {
	Repository []string `short:"r" name:"repository" sep:"," help:"repository name(s) (repeatable, comma-separated)"`
}

func (c *LabelClearCmd) Help() string {
	return `Clear all labels from the current repository (or specify repos with -r).

Target directory is set via WT_REPO_DIR env var or repo_dir/worktree_dir config.

Examples:
  wt label clear             # Clear labels from current repo
  wt label clear -r api      # Clear labels from 'api' repo
  wt label clear -r api,web  # Clear labels from multiple repos`
}

func (c *LabelClearCmd) Run(ctx *Context) error {
	return runLabelClear(c, ctx.Config, ctx.WorkDir)
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
  wt label add backend           # Add label to current repo
  wt label add backend -r api    # Add label to 'api' repo
  wt label remove backend        # Remove label from current repo
  wt label list                  # List labels for current repo
  wt label list -r api,web       # List labels for specific repos
  wt label list -g               # List all labels across repos in directory
  wt label clear                 # Clear all labels from current repo

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
	WorktreeDir string `arg:"" help:"base directory for worktrees (absolute path or ~/...)"`
	Force       bool   `short:"f" name:"force" negatable:"" help:"overwrite existing config file"`
	Stdout      bool   `short:"s" name:"stdout" help:"print config to stdout instead of writing to file"`
}

func (c *ConfigInitCmd) Help() string {
	return `Create default config file at ~/.config/wt/config.toml
Examples:
  wt config init ~/Git/worktrees     # Create config with worktree dir
  wt config init ~/Git/worktrees -f  # Overwrite existing config
  wt config init ~/Git -s            # Print config to stdout`
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

// HookCmd runs one or more hooks by name for a worktree or repos.
type HookCmd struct {
	Hooks      []string `arg:"" required:"" placeholder:"HOOK" help:"hook name(s) to run"`
	ID         []int    `short:"i" name:"id" xor:"target" help:"worktree ID(s) (optional in worktree, repeatable)"`
	Repository []string `short:"r" name:"repository" xor:"target" sep:"," help:"repository name(s) (repeatable, comma-separated)"`
	Label      []string `short:"l" name:"label" xor:"target" sep:"," help:"target repos by label (repeatable, comma-separated)"`
	Env        []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
	DryRun     bool     `short:"n" name:"dry-run" help:"print substituted command without executing"`
}

func (c *HookCmd) Help() string {
	return `When run inside a worktree, --id is optional (defaults to current worktree).
When run outside, specify a worktree ID. Multiple IDs can be specified.
Use -r to target repos by name, -l to target repos by label.

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt hook kitty              # Single hook (current worktree)
  wt hook kitty idea         # Multiple hooks
  wt hook kitty -i 1         # By worktree ID
  wt hook kitty -i 1 -i 2    # Multiple worktrees
  wt hook kitty -r wt        # By repo name
  wt hook kitty -l backend   # By label
  wt hook kitty -n           # Dry-run: print command without executing`
}

func (c *HookCmd) Run(ctx *Context) error {
	return runHookRun(c, ctx.Config, ctx.WorkDir)
}

// MvCmd moves worktrees to a different directory with optional renaming.
type MvCmd struct {
	Repository []string `short:"r" name:"repository" sep:"," help:"filter by repository name(s) (repeatable, comma-separated)"`
	Format     string   `name:"format" placeholder:"FORMAT" help:"worktree naming format"`
	DryRun     bool     `short:"n" name:"dry-run" negatable:"" help:"show what would be moved"`
	Force      bool     `short:"f" name:"force" negatable:"" help:"force move locked worktrees"`
}

func (c *MvCmd) Help() string {
	return `Scans current directory for worktrees and repositories, moving them to configured destinations.

Worktrees are moved to worktree_dir.
Repositories are moved to repo_dir (if set) or worktree_dir.

Use -r to filter by repository name(s). Use --format to rename worktrees during move.

Destinations are set via config or environment variables:
  worktree_dir / WT_WORKTREE_DIR  - where worktrees are moved
  repo_dir / WT_REPO_DIR          - where repositories are moved (optional)

Examples:
  wt mv                              # Move all worktrees and repos
  wt mv -r myrepo                    # Move only myrepo's worktrees and repo
  wt mv -r repo1,repo2               # Move worktrees and repos for multiple repos
  wt mv --format={branch}            # Move and rename worktrees
  wt mv --dry-run                    # Preview what would be moved
  wt mv -f                           # Force move locked worktrees`
}

func (c *MvCmd) Run(ctx *Context) error {
	return runMv(c, ctx.Config, ctx.WorkDir)
}

// PrCheckoutCmd creates a worktree for a PR, cloning the repo if needed.
type PrCheckoutCmd struct {
	Number     int      `arg:"" required:"" placeholder:"NUMBER" help:"PR number"`
	Repo       string   `arg:"" optional:"" placeholder:"ORG/REPO" help:"clone repo (org/repo format)"`
	Repository string   `short:"r" name:"repository" help:"local repo name"`
	Forge      string   `name:"forge" env:"WT_FORGE" placeholder:"FORGE" help:"forge for cloning: github or gitlab"`
	Note       string   `name:"note" placeholder:"TEXT" help:"set a note on the branch"`
	Hook       string   `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook     bool     `name:"no-hook" help:"skip post-create hook" xor:"hook-ctrl"`
	Env        []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

func (c *PrCheckoutCmd) Help() string {
	return `Checkout a PR, cloning the repo if it doesn't exist locally.

Two modes:
  Clone mode (positional org/repo): Clones repo first, then creates worktree
  Local mode (-r flag or no args): Uses existing local repo

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt pr checkout 123                    # PR from current directory
  wt pr checkout 123 -r myrepo          # PR from local repo by name
  wt pr checkout 123 org/repo           # Clone repo and checkout PR
  wt pr checkout 123 org/repo --forge=gitlab  # Use GitLab instead of GitHub
  wt pr checkout 123 org/repo --note "WIP"    # Set note on branch
  wt pr checkout 123 --no-hook          # Skip post-create hook`
}

func (c *PrCheckoutCmd) Run(ctx *Context) error {
	return runPrCheckout(c, ctx.Config, ctx.WorkDir)
}

// PrMergeCmd merges the PR for the current branch.
type PrMergeCmd struct {
	ID         int      `short:"i" name:"id" xor:"target" help:"worktree ID (optional in worktree/repo)"`
	Repository string   `short:"r" name:"repository" xor:"target" help:"repository name"`
	Strategy   string   `short:"s" name:"strategy" env:"WT_MERGE_STRATEGY" placeholder:"STRATEGY" help:"merge strategy: squash, rebase, or merge"`
	Keep       bool     `short:"k" name:"keep" help:"keep worktree and branch after merge"`
	Hook       string   `name:"hook" help:"run named hook instead of default" xor:"hook-ctrl"`
	NoHook     bool     `name:"no-hook" help:"skip post-merge hook" xor:"hook-ctrl"`
	Env        []string `short:"a" name:"arg" help:"set hook variable KEY=VALUE (use KEY=- to read from stdin)"`
}

// PrCreateCmd creates a PR for a worktree.
type PrCreateCmd struct {
	ID         int    `short:"i" name:"id" xor:"target" help:"worktree ID (optional in worktree/repo)"`
	Repository string `short:"r" name:"repository" xor:"target" help:"repository name"`
	Title      string `short:"t" name:"title" required:"" placeholder:"TITLE" help:"PR title"`
	Body       string `short:"b" name:"body" placeholder:"BODY" help:"PR body (use - to read from stdin)"`
	BodyFile   string `name:"body-file" placeholder:"FILE" help:"read body from file"`
	Base       string `name:"base" placeholder:"BRANCH" help:"base branch (default: repo's default branch)"`
	Draft      bool   `name:"draft" help:"create as draft PR"`
	Web        bool   `short:"w" name:"web" help:"open PR in browser after creation"`
}

func (c *PrCreateCmd) Help() string {
	return `Inside a worktree or repo: --id is optional (defaults to current branch).
Outside: specify a worktree ID or repo name.

The body can be provided via --body, --body-file, or piped from stdin with --body -.
Use --draft to create a draft PR. Use --web to open the PR in browser after creation.

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt pr create --title "Add feature"                    # Inside worktree/repo
  wt pr create --title "Add feature" --body "Details"   # With body
  wt pr create --title "Add feature" --body-file=pr.md  # Body from file
  echo "body" | wt pr create --title "Add feature" --body -  # Body from stdin
  wt pr create --title "Add feature" --draft            # Create as draft
  wt pr create --title "Add feature" -w                 # Open in browser after
  wt pr create --title "Add feature" -i 1               # By worktree ID
  wt pr create --title "Add feature" -r myrepo          # By repository name`
}

func (c *PrCreateCmd) Run(ctx *Context) error {
	return runPrCreate(c, ctx.Config, ctx.WorkDir)
}

func (c *PrMergeCmd) Help() string {
	return `Inside a worktree or repo: --id is optional (defaults to current branch).
Outside: specify a worktree ID or repo name.

Merges the PR, removes the worktree (if applicable), and deletes the local branch.
Use --keep to preserve the worktree/branch after merge.
Merge strategy: flag > WT_MERGE_STRATEGY env > config. GitLab doesn't support rebase.

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt pr merge                    # Inside worktree/repo: merge current branch's PR
  wt pr merge -i 1               # By worktree ID
  wt pr merge -r myrepo          # By repository name
  wt pr merge --keep             # Merge PR only, keep worktree and branch
  wt pr merge -s rebase          # Use rebase merge strategy`
}

func (c *PrMergeCmd) Run(ctx *Context) error {
	return runPrMerge(c, ctx.Config, ctx.WorkDir)
}

// PrViewCmd shows PR details or opens in browser.
type PrViewCmd struct {
	ID         int    `short:"i" name:"id" xor:"target" help:"worktree ID (optional in worktree/repo)"`
	Repository string `short:"r" name:"repository" xor:"target" help:"repository name"`
	Web        bool   `short:"w" name:"web" help:"open PR in browser"`
}

func (c *PrViewCmd) Help() string {
	return `Inside a worktree or repo: --id is optional (defaults to current branch).
Outside: specify a worktree ID or repo name.

Shows PR details for the branch. Use --web to open in browser instead.

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt pr view              # Inside worktree/repo: show PR details
  wt pr view -w           # Inside worktree/repo: open PR in browser
  wt pr view -i 1         # By worktree ID
  wt pr view -r myrepo    # By repository name
  wt pr view -i 1 -w      # Open PR in browser by ID`
}

func (c *PrViewCmd) Run(ctx *Context) error {
	return runPrView(c, ctx.Config, ctx.WorkDir)
}

// PrCmd works with PRs.
type PrCmd struct {
	Checkout PrCheckoutCmd `cmd:"" help:"Checkout PR (clones if needed)"`
	Create   PrCreateCmd   `cmd:"" help:"Create PR for worktree"`
	Merge    PrMergeCmd    `cmd:"" help:"Merge PR and clean up worktree"`
	View     PrViewCmd     `cmd:"" help:"View PR details or open in browser"`
}

func (c *PrCmd) Help() string {
	return `Examples:
  wt pr checkout 123                    # PR from current directory
  wt pr checkout 123 -r myrepo          # PR from existing local repo
  wt pr checkout 123 org/repo           # Clone repo and checkout PR
  wt pr create --title "Add feature"    # Create PR
  wt pr merge                           # Merge PR and clean up worktree
  wt pr view                            # View PR details
  wt pr view -w                         # Open PR in browser`
}

// ReposCmd lists repositories in a directory.
type ReposCmd struct {
	Label string `short:"l" name:"label" placeholder:"LABEL" help:"filter by label"`
	Sort  string `short:"s" name:"sort" default:"name" enum:"name,branch,worktrees,label" help:"sort by: name (default), branch, worktrees, label"`
	JSON  bool   `name:"json" help:"output as JSON"`
}

func (c *ReposCmd) Help() string {
	return `Scans a directory for git repositories and shows information about each.
Use --label to filter by repository label.
Use --sort to change ordering: name (default), branch, worktrees, label.

Target directory is set via WT_REPO_DIR env var or repo_dir/worktree_dir config.

Examples:
  wt repos                     # List repos in current/configured directory
  wt repos -l backend          # Filter repos by label
  wt repos -s worktrees        # Sort by worktree count (descending)
  wt repos -s label            # Sort by label (alphabetically)
  wt repos --json              # Output as JSON`
}

func (c *ReposCmd) Run(ctx *Context) error {
	return runRepos(c, ctx.Config)
}

// DoctorCmd diagnoses and repairs cache issues.
type DoctorCmd struct {
	Fix   bool `name:"fix" help:"auto-fix recoverable issues"`
	Reset bool `name:"reset" help:"rebuild cache from scratch (loses IDs)"`
}

func (c *DoctorCmd) Help() string {
	return `Diagnose and repair worktree issues.

Cache checks:
- Stale entries (worktree no longer exists)
- Orphaned keys (old cache format)
- Missing metadata (repo_path, branch)
- Duplicate IDs

Git link checks:
- Broken bidirectional links (repairable via 'git worktree repair')
- Stale git references (prunable via 'git worktree prune')
- Missing .git file (unrepairable)

Orphan checks:
- Untracked worktrees (folder exists but not in cache)
- Ghost entries (in cache but git doesn't recognize)

Use --fix to automatically repair issues.
Use --reset to rebuild the cache from scratch (all worktrees get new IDs).

Target directory is set via WT_WORKTREE_DIR env var or worktree_dir config.

Examples:
  wt doctor              # Check for issues
  wt doctor --fix        # Auto-fix recoverable issues
  wt doctor --reset      # Rebuild cache from scratch`
}

func (c *DoctorCmd) Run(ctx *Context) error {
	return runDoctor(c, ctx.Config)
}

// VersionFlag is used to show version info.
type VersionFlag bool

// CLI is the root command.
type CLI struct {
	// Core commands (ungrouped - shown first)
	Add   AddCmd   `cmd:"" aliases:"a" help:"Add worktree for branch"`
	List  ListCmd  `cmd:"" aliases:"ls" help:"List worktrees"`
	Show  ShowCmd  `cmd:"" aliases:"s" help:"Show worktree details"`
	Prune PruneCmd `cmd:"" aliases:"p" help:"Prune merged worktrees"`
	Repos ReposCmd `cmd:"" aliases:"r" help:"List repositories"`

	// PR commands
	Pr PrCmd `cmd:"" help:"Work with PRs" group:"pr"`

	// Utility commands
	Exec  ExecCmd  `cmd:"" aliases:"x" help:"Run command in worktree(s) or repo(s)" group:"util"`
	Cd    CdCmd    `cmd:"" help:"Print worktree path" group:"util"`
	Mv    MvCmd    `cmd:"" help:"Move worktrees to another directory" group:"util"`
	Note  NoteCmd  `cmd:"" help:"Manage branch notes" group:"util"`
	Label LabelCmd `cmd:"" help:"Manage repository labels" group:"util"`
	Hook  HookCmd  `cmd:"" help:"Run configured hook" group:"util"`

	// Configuration commands
	Config     ConfigCmd     `cmd:"" help:"Manage configuration" group:"config"`
	Completion CompletionCmd `cmd:"" help:"Generate completion script" group:"config"`
	Doctor     DoctorCmd     `cmd:"" help:"Diagnose and repair cache" group:"config"`

	Verbose bool        `short:"v" name:"verbose" help:"Show external commands being executed"`
	Version VersionFlag `name:"version" help:"Show version"`
}

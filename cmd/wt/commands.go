package main

// CreateCmd creates a new worktree for a new or existing remote branch.
type CreateCmd struct {
	Branch string `arg:"positional,required" placeholder:"BRANCH" help:"branch name"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
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
	Branch string `arg:"positional,required" placeholder:"BRANCH" help:"existing local branch name"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Hook   string `arg:"--hook" help:"run named hook instead of default"`
	NoHook bool   `arg:"--no-hook" help:"skip post-create hook"`
}

func (OpenCmd) Description() string {
	return `Open a worktree for an existing local branch

Unlike 'create', this command requires the branch to already exist locally.
Use this when you want to work on a branch that was created elsewhere.

Examples:
  wt open feature-branch              # Uses default path resolution
  wt open feature-branch -d ~/Git     # Specify target directory
  wt open feature-branch --no-hook    # Skip post-create hook`
}

// TidyCmd removes merged and clean worktrees.
type TidyCmd struct {
	Dir          string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	DryRun       bool   `arg:"-n,--dry-run" help:"preview without removing"`
	RefreshPR    bool   `arg:"--refresh-pr" help:"force refresh PR cache"`
	IncludeClean bool   `arg:"-c,--include-clean" help:"also remove worktrees with 0 commits ahead and clean working directory"`
	Hook         string `arg:"--hook" help:"run named hook instead of default"`
	NoHook       bool   `arg:"--no-hook" help:"skip post-removal hooks"`
}

func (TidyCmd) Description() string {
	return `Tidy up merged git worktrees with PR status display

Removes worktrees where the branch is merged AND working directory is clean.
Shows a table with PR status (requires gh/glab CLI) before removal.

Hooks with on=["tidy"] run after each worktree removal. Hooks run with
working directory set to the main repo (since worktree path is deleted).

Merge detection uses git merge-base locally, which may miss squash-merged
or rebased PRs. For accurate detection, use GitHub/GitLab where PR status
shows if the branch was merged.

Examples:
  wt tidy                      # Remove merged worktrees
  wt tidy -n                   # Dry-run: preview without removing
  wt tidy -d ~/Git/worktrees   # Scan specific directory
  wt tidy -c                   # Also remove clean (0-commit) worktrees
  wt tidy --refresh-pr         # Force refresh PR status from GitHub
  wt tidy --no-hook            # Skip post-removal hooks
  wt tidy --hook=cleanup       # Run 'cleanup' hook instead of default`
}

// ListCmd lists worktrees in a directory.
type ListCmd struct {
	Dir  string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	JSON bool   `arg:"--json" help:"output as JSON"`
}

func (ListCmd) Description() string {
	return `List all git worktrees with their status

When run inside a git repository, only shows worktrees for that repo.
Output format matches 'git worktree list'.

Examples:
  wt list                      # List worktrees for current repo
  wt list -d ~/Git/worktrees   # List from specific directory
  wt list --json               # Output as JSON for scripting`
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
	Hooks *ConfigHooksCmd `arg:"subcommand:hooks" help:"list available hooks"`
}

func (ConfigCmd) Description() string {
	return `Manage wt configuration
Examples:
  wt config init           # Create default config
  wt config hooks          # List available hooks`
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

// PrCmd works with PRs.
type PrCmd struct {
	Open  *PrOpenCmd  `arg:"subcommand:open" help:"checkout PR from existing local repo"`
	Clone *PrCloneCmd `arg:"subcommand:clone" help:"clone repo and checkout PR"`
}

func (PrCmd) Description() string {
	return `Work with PRs
Examples:
  wt pr open 123             # PR from current repo
  wt pr open 123 myrepo      # PR from existing local repo
  wt pr clone 123 org/repo   # Clone repo and checkout PR`
}

// Args is the root command.
type Args struct {
	Create     *CreateCmd     `arg:"subcommand:create" help:"create a new worktree"`
	Open       *OpenCmd       `arg:"subcommand:open" help:"open worktree for existing branch"`
	Tidy       *TidyCmd       `arg:"subcommand:tidy" help:"tidy up merged worktrees"`
	List       *ListCmd       `arg:"subcommand:list" help:"list worktrees"`
	Mv         *MvCmd         `arg:"subcommand:mv" help:"move worktrees to another directory"`
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

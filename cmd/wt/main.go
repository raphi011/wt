package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/alexflint/go-arg"

	"github.com/raphaelgruber/wt/internal/config"
	"github.com/raphaelgruber/wt/internal/format"
	"github.com/raphaelgruber/wt/internal/git"
	"github.com/raphaelgruber/wt/internal/github"
	"github.com/raphaelgruber/wt/internal/hooks"
	"github.com/raphaelgruber/wt/internal/ui"
)

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

type CleanCmd struct {
	Dir       string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	DryRun    bool   `arg:"-n,--dry-run" help:"preview without removing"`
	RefreshPR bool   `arg:"--refresh-pr" help:"force refresh PR cache"`
	Empty     bool   `arg:"-e,--empty" help:"also remove worktrees with 0 commits ahead and clean working directory"`
}

func (CleanCmd) Description() string {
	return `Cleanup merged git worktrees with PR status display

Removes worktrees where the branch is merged AND working directory is clean.
Shows a table with PR status (requires gh CLI) before removal.

Examples:
  wt clean                      # Remove merged worktrees
  wt clean -n                   # Dry-run: preview without removing
  wt clean -d ~/Git/worktrees   # Scan specific directory
  wt clean -e                   # Also remove 0-commit worktrees
  wt clean --refresh-pr         # Force refresh PR status from GitHub`
}

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

type ConfigInitCmd struct {
	Force bool `arg:"-f,--force" help:"overwrite existing config file"`
}

func (ConfigInitCmd) Description() string {
	return `Create default config file at ~/.config/wt/config.toml
Examples:
  wt config init           # Create config if missing
  wt config init -f        # Overwrite existing config`
}

type ConfigHooksCmd struct {
	JSON bool `arg:"--json" help:"output as JSON"`
}

func (ConfigHooksCmd) Description() string {
	return `List available hooks
Examples:
  wt config hooks          # Show hooks in text format
  wt config hooks --json   # Output as JSON`
}

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

type PrOpenCmd struct {
	Number int    `arg:"positional,required" placeholder:"NUMBER" help:"PR number"`
	Repo   string `arg:"positional" placeholder:"REPO" help:"repository (org/repo or name)"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"target directory (flag > WT_DEFAULT_PATH > config > cwd)"`
	Hook   string `arg:"--hook" help:"run named hook instead of default"`
	NoHook bool   `arg:"--no-hook" help:"skip post-create hook"`
}

func (PrOpenCmd) Description() string {
	return `Create a worktree for a GitHub PR

Fetches PR metadata and creates a worktree for the PR's branch.
If the repository isn't found locally, use org/repo format to clone it.

Examples:
  wt pr open 123                  # PR from current repo
  wt pr open 123 myrepo           # Find "myrepo" in target directory
  wt pr open 123 org/repo         # Clone if not found locally
  wt pr open 123 -d ~/Git         # Specify target directory
  wt pr open 123 --no-hook        # Skip post-create hook`
}

type PrCmd struct {
	Open *PrOpenCmd `arg:"subcommand:open" help:"checkout PR as new worktree"`
}

func (PrCmd) Description() string {
	return `Work with GitHub pull requests
Examples:
  wt pr open 123           # Checkout PR #123 from current repo
  wt pr open 123 myrepo    # Checkout PR #123 from myrepo`
}

type Args struct {
	Create     *CreateCmd     `arg:"subcommand:create" help:"create a new worktree"`
	Open       *OpenCmd       `arg:"subcommand:open" help:"open worktree for existing branch"`
	Clean      *CleanCmd      `arg:"subcommand:clean" help:"cleanup merged worktrees"`
	List       *ListCmd       `arg:"subcommand:list" help:"list worktrees"`
	Pr         *PrCmd         `arg:"subcommand:pr" help:"work with GitHub PRs"`
	Config     *ConfigCmd     `arg:"subcommand:config" help:"manage configuration"`
	Completion *CompletionCmd `arg:"subcommand:completion" help:"generate completion script"`
}

func (Args) Description() string {
	return `Git worktree manager with GitHub PR integration

Worktrees are created as <repo>-<branch> in the specified directory.
Set WT_DEFAULT_PATH or configure default_path in ~/.config/wt/config.toml.

Examples:
  wt create feature-x              # Create worktree for new branch
  wt open existing-branch          # Create worktree for existing local branch
  wt pr open 123                   # Checkout GitHub PR as worktree
  wt list                          # List worktrees in current directory
  wt clean                         # Remove merged worktrees
  wt clean -n                      # Dry-run: preview what would be removed
  wt config init                   # Create default config file

Exit codes:
  0  Success
  1  Error (invalid arguments, git/gh failures, etc.)`
}

func main() {
	var args Args
	p, err := arg.NewParser(arg.Config{StrictSubcommands: true}, &args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := p.Parse(os.Args[1:]); err != nil {
		switch err {
		case arg.ErrHelp:
			writeHelp(os.Stdout, p, &args)
			os.Exit(0)
		case arg.ErrVersion:
			fmt.Println(args.Version())
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			p.WriteUsage(os.Stderr)
			os.Exit(1)
		}
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	// Apply config defaults
	if args.Create != nil && args.Create.Dir == "" {
		args.Create.Dir = cfg.DefaultPath
	}
	if args.Open != nil && args.Open.Dir == "" {
		args.Open.Dir = cfg.DefaultPath
	}
	if args.Clean != nil && args.Clean.Dir == "" {
		args.Clean.Dir = cfg.DefaultPath
	}
	if args.List != nil && args.List.Dir == "" {
		args.List.Dir = cfg.DefaultPath
	}
	if args.Pr != nil && args.Pr.Open != nil && args.Pr.Open.Dir == "" {
		args.Pr.Open.Dir = cfg.DefaultPath
	}

	switch {
	case args.Create != nil:
		if err := runCreate(args.Create, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Open != nil:
		if err := runOpen(args.Open, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Clean != nil:
		if err := runClean(args.Clean); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.List != nil:
		if err := runList(args.List); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Completion != nil:
		if err := runCompletion(args.Completion); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Pr != nil:
		if err := runPr(args.Pr, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Config != nil:
		if err := runConfig(args.Config, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		p.WriteHelp(os.Stdout)
		os.Exit(1)
	}
}

func runCreate(cmd *CreateCmd, cfg config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	basePath, err := expandPath(dir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fmt.Printf("Creating worktree for branch %s in %s\n", cmd.Branch, absPath)

	result, err := git.CreateWorktree(basePath, cmd.Branch, cfg.WorktreeFormat)
	if err != nil {
		return err
	}

	if result.AlreadyExists {
		fmt.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		fmt.Printf("✓ Worktree created at: %s\n", result.Path)
	}

	// Run post-create hook
	hook, hookName, err := hooks.SelectHook(cfg.Hooks, cmd.Hook, cmd.NoHook, result.AlreadyExists)
	if err != nil {
		return err
	}

	if hook != nil {
		// Get context for placeholder substitution
		repoName, _ := git.GetRepoName()
		folderName, _ := git.GetRepoFolderName()
		mainRepo, err := git.GetMainRepoPath(result.Path)
		if err != nil || mainRepo == "" {
			// Fallback for newly created worktrees
			mainRepo, err = filepath.Abs(".")
			if err != nil {
				return fmt.Errorf("failed to determine main repo path: %w", err)
			}
		}

		ctx := hooks.Context{
			Path:     result.Path,
			Branch:   cmd.Branch,
			Repo:     repoName,
			Folder:   folderName,
			MainRepo: mainRepo,
		}

		fmt.Printf("Running hook '%s'...\n", hookName)
		if err := hooks.Run(hook, ctx); err != nil {
			return fmt.Errorf("hook %q failed: %w", hookName, err)
		}
		if hook.Description != "" {
			fmt.Printf("  ✓ %s\n", hook.Description)
		}
	}

	return nil
}

func runOpen(cmd *OpenCmd, cfg config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	basePath, err := expandPath(dir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fmt.Printf("Opening worktree for branch %s in %s\n", cmd.Branch, absPath)

	result, err := git.OpenWorktree(basePath, cmd.Branch, cfg.WorktreeFormat)
	if err != nil {
		return err
	}

	if result.AlreadyExists {
		fmt.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		fmt.Printf("✓ Worktree created at: %s\n", result.Path)
	}

	// Run post-create hook (always run for open, ignore run_on_exists config)
	hook, hookName, err := hooks.SelectHook(cfg.Hooks, cmd.Hook, cmd.NoHook, false)
	if err != nil {
		return err
	}

	if hook != nil {
		// Get context for placeholder substitution
		repoName, _ := git.GetRepoName()
		folderName, _ := git.GetRepoFolderName()
		mainRepo, err := git.GetMainRepoPath(result.Path)
		if err != nil || mainRepo == "" {
			// Fallback for newly created worktrees
			mainRepo, err = filepath.Abs(".")
			if err != nil {
				return fmt.Errorf("failed to determine main repo path: %w", err)
			}
		}

		ctx := hooks.Context{
			Path:     result.Path,
			Branch:   cmd.Branch,
			Repo:     repoName,
			Folder:   folderName,
			MainRepo: mainRepo,
		}

		fmt.Printf("Running hook '%s'...\n", hookName)
		if err := hooks.Run(hook, ctx); err != nil {
			return fmt.Errorf("hook %q failed: %w", hookName, err)
		}
		if hook.Description != "" {
			fmt.Printf("  ✓ %s\n", hook.Description)
		}
	}

	return nil
}

// maxConcurrentPRFetches limits parallel gh API calls to avoid rate limiting
const maxConcurrentPRFetches = 5

func runClean(cmd *CleanCmd) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Check gh (optional - PR status is a nice-to-have)
	ghAvailable := github.CheckGH() == nil

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	// Expand path
	scanPath, err := expandPath(dir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
	scanPath, err = filepath.Abs(scanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	fmt.Printf("Cleaning worktrees in %s\n", scanPath)

	// Start spinner
	sp := ui.NewSpinner("Scanning worktrees...")
	sp.Start()

	// List worktrees
	worktrees, err := git.ListWorktrees(scanPath)
	if err != nil {
		sp.Stop()
		return err
	}

	if len(worktrees) == 0 {
		sp.Stop()
		fmt.Println("No worktrees found")
		return nil
	}

	// Group by repo for fetching
	grouped := git.GroupWorktreesByRepo(worktrees)

	// Fetch default branch for each repo
	for repoName, wts := range grouped {
		if len(wts) == 0 {
			continue
		}
		defaultBranch := git.GetDefaultBranch(wts[0].MainRepo)
		sp.UpdateMessage(fmt.Sprintf("Fetching origin/%s for %s...", defaultBranch, repoName))
		if err := git.FetchDefaultBranch(wts[0].MainRepo); err != nil {
			// Non-fatal: log warning but continue
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	// PR status handling (requires gh CLI)
	prCache := make(github.PRCache)
	prMap := make(map[string]*github.PRInfo)

	if ghAvailable {
		// Load PR cache
		var err error
		prCache, err = github.LoadPRCache(scanPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load PR cache: %v\n", err)
			prCache = make(github.PRCache)
		}

		// Clean cache (remove deleted origins/branches)
		prCache = github.CleanPRCache(prCache, worktrees)

		// Determine which branches need fetching (uses cache expiration)
		toFetch := github.NeedsFetch(prCache, worktrees, cmd.RefreshPR)

		// Fetch uncached PRs in parallel with rate limiting
		if len(toFetch) > 0 {
			sp.UpdateMessage(fmt.Sprintf("Fetching PR status (%d branches)...", len(toFetch)))
			var prMutex sync.Mutex
			var prWg sync.WaitGroup
			semaphore := make(chan struct{}, maxConcurrentPRFetches)
			var fetchErrors []string
			var errMutex sync.Mutex

			for _, wt := range toFetch {
				prWg.Add(1)
				go func(wt git.Worktree) {
					defer prWg.Done()
					semaphore <- struct{}{}        // Acquire
					defer func() { <-semaphore }() // Release

					originURL, err := github.GetOriginURL(wt.MainRepo)
					if err != nil {
						errMutex.Lock()
						fetchErrors = append(fetchErrors, fmt.Sprintf("%s: %v", wt.Branch, err))
						errMutex.Unlock()
						return
					}
					if originURL == "" {
						return
					}

					// Use upstream branch name for PR lookup (may differ from local)
					upstreamBranch := git.GetUpstreamBranch(wt.MainRepo, wt.Branch)
					if upstreamBranch == "" {
						return // No upstream = never pushed = no PR
					}

					pr, err := github.GetPRForBranch(originURL, upstreamBranch)
					if err != nil {
						errMutex.Lock()
						fetchErrors = append(fetchErrors, fmt.Sprintf("%s: %v", wt.Branch, err))
						errMutex.Unlock()
						return
					}

					prMutex.Lock()
					if prCache[originURL] == nil {
						prCache[originURL] = make(map[string]*github.PRInfo)
					}
					prCache[originURL][wt.Branch] = pr // nil is valid (no PR)
					prMutex.Unlock()
				}(wt)
			}

			prWg.Wait()

			// Report errors (non-fatal)
			if len(fetchErrors) > 0 {
				sp.Stop()
				for _, e := range fetchErrors {
					fmt.Fprintf(os.Stderr, "Warning: PR fetch failed for %s\n", e)
				}
				sp = ui.NewSpinner("Continuing...")
				sp.Start()
			}

			// Save updated cache
			if err := github.SavePRCache(scanPath, prCache); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save PR cache: %v\n", err)
			}
		}

		// Build prMap from cache for display
		for _, wt := range worktrees {
			originURL, _ := github.GetOriginURL(wt.MainRepo)
			if originCache, ok := prCache[originURL]; ok {
				if pr, ok := originCache[wt.Branch]; ok {
					prMap[wt.Branch] = pr
				}
			}
		}

		// Update merge status for worktrees based on PR state
		for i := range worktrees {
			if pr, ok := prMap[worktrees[i].Branch]; ok && pr != nil {
				if pr.State == "MERGED" {
					worktrees[i].IsMerged = true
				}
			}
		}
	}

	// Stop spinner and clear line
	sp.Stop()

	// Sort worktrees by repo name
	sort.Slice(worktrees, func(i, j int) bool {
		return worktrees[i].RepoName < worktrees[j].RepoName
	})

	// Determine which to remove
	var toRemove []git.Worktree
	toRemoveMap := make(map[string]bool)
	var skipped int

	for _, wt := range worktrees {
		shouldRemove := false
		if wt.IsMerged && !wt.IsDirty {
			shouldRemove = true
		} else if cmd.Empty && wt.CommitCount == 0 && !wt.IsDirty {
			shouldRemove = true
		}

		if shouldRemove {
			toRemove = append(toRemove, wt)
			toRemoveMap[wt.Path] = true
		} else {
			skipped++
		}
	}

	// Display table with cleaned worktrees marked
	fmt.Print(ui.FormatWorktreesTable(worktrees, prMap, toRemoveMap, cmd.DryRun))

	// Remove worktrees
	if !cmd.DryRun && len(toRemove) > 0 {
		for _, wt := range toRemove {
			if err := git.RemoveWorktree(wt, true); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", wt.Path, err)
			}
		}

		// Prune stale references
		processedRepos := make(map[string]bool)
		for _, wt := range toRemove {
			if !processedRepos[wt.MainRepo] {
				git.PruneWorktrees(wt.MainRepo)
				processedRepos[wt.MainRepo] = true
			}
		}
	}

	// Print summary
	fmt.Print(ui.FormatSummary(len(toRemove), skipped, cmd.DryRun))

	return nil
}

// writeHelp prints help with subcommand-specific descriptions
func writeHelp(w *os.File, p *arg.Parser, args *Args) {
	// Determine active subcommand and get its description
	var desc string
	switch {
	case args.Create != nil:
		desc = args.Create.Description()
	case args.Open != nil:
		desc = args.Open.Description()
	case args.Clean != nil:
		desc = args.Clean.Description()
	case args.List != nil:
		desc = args.List.Description()
	case args.Pr != nil:
		if args.Pr.Open != nil {
			desc = args.Pr.Open.Description()
		} else {
			desc = args.Pr.Description()
		}
	case args.Config != nil:
		if args.Config.Init != nil {
			desc = args.Config.Init.Description()
		} else if args.Config.Hooks != nil {
			desc = args.Config.Hooks.Description()
		} else {
			desc = args.Config.Description()
		}
	case args.Completion != nil:
		desc = args.Completion.Description()
	default:
		// No subcommand - use default help
		p.WriteHelp(w)
		return
	}

	// Print subcommand description, then full help (which includes usage + flags)
	// Capture WriteHelp output and replace the parent description with subcommand's
	fmt.Fprintln(w, desc)
	fmt.Fprintln(w)

	// WriteHelp outputs: description, version line, usage, options
	// We need usage + options, so capture and skip first lines
	var buf strings.Builder
	p.WriteHelp(&buf)
	lines := strings.Split(buf.String(), "\n")

	// Find "Usage:" line and print from there
	for i, line := range lines {
		if strings.HasPrefix(line, "Usage:") {
			fmt.Fprintln(w, strings.Join(lines[i:], "\n"))
			break
		}
	}
}

// expandPath expands ~ to home directory
func expandPath(path string) (string, error) {
	if len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return home, nil
	}
	return path, nil
}

func runList(cmd *ListCmd) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	// Expand path
	scanPath, err := expandPath(dir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
	scanPath, err = filepath.Abs(scanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// List worktrees in directory
	worktrees, err := git.ListWorktrees(scanPath)
	if err != nil {
		return err
	}

	// If in a git repo, filter to only show worktrees from that repo
	if currentRepo := git.GetCurrentRepoMainPath(); currentRepo != "" {
		var filtered []git.Worktree
		for _, wt := range worktrees {
			if wt.MainRepo == currentRepo {
				filtered = append(filtered, wt)
			}
		}
		worktrees = filtered
	}

	if cmd.JSON {
		data, err := json.MarshalIndent(worktrees, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(worktrees) == 0 {
		return nil
	}

	// Find max path length for alignment
	maxPathLen := 0
	for _, wt := range worktrees {
		if len(wt.Path) > maxPathLen {
			maxPathLen = len(wt.Path)
		}
	}

	// Output in git worktree list format: /path  commit [branch]
	for _, wt := range worktrees {
		hash, _ := git.GetShortCommitHash(wt.Path)
		if hash == "" {
			hash = "???????"
		}
		fmt.Printf("%-*s  %s [%s]\n", maxPathLen, wt.Path, hash, wt.Branch)
	}

	return nil
}

func runPr(cmd *PrCmd, cfg config.Config) error {
	switch {
	case cmd.Open != nil:
		return runPrOpen(cmd.Open, cfg)
	default:
		return fmt.Errorf("no subcommand specified (try: wt pr open)")
	}
}

func runPrOpen(cmd *PrOpenCmd, cfg config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}
	if err := github.CheckGH(); err != nil {
		return err
	}

	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	basePath, err := expandPath(dir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// Determine which repo to use
	var repoPath string
	if cmd.Repo == "" {
		// No repo arg: use current directory
		repoPath = "."
	} else {
		// Repo arg provided: find or clone
		org, name := git.ParseRepoArg(cmd.Repo)

		// Try to find repo locally
		foundPath, err := git.FindRepoByName(basePath, name)
		if err == nil {
			repoPath = foundPath
			fmt.Printf("Using repo at %s\n", repoPath)
		} else if org != "" {
			// Not found but org/repo provided: clone it
			fmt.Printf("Cloning %s to %s...\n", cmd.Repo, basePath)
			clonedPath, err := github.CloneRepo(cmd.Repo, basePath)
			if err != nil {
				return fmt.Errorf("failed to clone repo: %w", err)
			}
			repoPath = clonedPath
			fmt.Printf("✓ Cloned to %s\n", repoPath)
		} else {
			// Not found and no org: error with suggestions
			similar := git.FindSimilarRepos(basePath, name)
			if len(similar) > 0 {
				return fmt.Errorf("repository %q not found in %s\nDid you mean: %s", name, basePath, strings.Join(similar, ", "))
			}
			return fmt.Errorf("repository %q not found in %s\nUse org/repo format to clone from GitHub", name, basePath)
		}
	}

	// Get origin URL from repo
	originURL, err := github.GetOriginURL(repoPath)
	if err != nil {
		return fmt.Errorf("failed to get origin URL: %w", err)
	}

	// Fetch PR branch name
	fmt.Printf("Fetching PR #%d...\n", cmd.Number)
	branch, err := github.GetPRBranch(originURL, cmd.Number)
	if err != nil {
		return fmt.Errorf("failed to get PR branch: %w", err)
	}

	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fmt.Printf("Creating worktree for branch %s in %s\n", branch, absPath)

	result, err := git.CreateWorktreeFrom(repoPath, basePath, branch, cfg.WorktreeFormat)
	if err != nil {
		return err
	}

	if result.AlreadyExists {
		fmt.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		fmt.Printf("✓ Worktree created at: %s\n", result.Path)
	}

	// Run post-create hook
	hook, hookName, err := hooks.SelectHook(cfg.Hooks, cmd.Hook, cmd.NoHook, result.AlreadyExists)
	if err != nil {
		return err
	}

	if hook != nil {
		// Get context for placeholder substitution
		repoName, _ := git.GetRepoNameFrom(repoPath)
		folderName := filepath.Base(repoPath)
		mainRepo, err := git.GetMainRepoPath(result.Path)
		if err != nil || mainRepo == "" {
			// Fallback for newly created worktrees
			mainRepo, err = filepath.Abs(repoPath)
			if err != nil {
				return fmt.Errorf("failed to determine main repo path: %w", err)
			}
		}

		ctx := hooks.Context{
			Path:     result.Path,
			Branch:   branch,
			Repo:     repoName,
			Folder:   folderName,
			MainRepo: mainRepo,
		}

		fmt.Printf("Running hook '%s'...\n", hookName)
		if err := hooks.Run(hook, ctx); err != nil {
			return fmt.Errorf("hook %q failed: %w", hookName, err)
		}
		if hook.Description != "" {
			fmt.Printf("  ✓ %s\n", hook.Description)
		}
	}

	return nil
}

func runCompletion(cmd *CompletionCmd) error {
	switch cmd.Shell {
	case "fish":
		fmt.Print(fishCompletions)
		return nil
	case "bash":
		fmt.Print(bashCompletions)
		return nil
	case "zsh":
		fmt.Print(zshCompletions)
		return nil
	default:
		return fmt.Errorf("unsupported shell: %s (supported: fish, bash, zsh)", cmd.Shell)
	}
}

func runConfig(cmd *ConfigCmd, cfg config.Config) error {
	switch {
	case cmd.Init != nil:
		path, err := config.Init(cmd.Init.Force)
		if err != nil {
			return err
		}
		fmt.Printf("Created config file: %s\n", path)
		return nil
	case cmd.Hooks != nil:
		return runConfigHooks(cmd.Hooks, cfg)
	default:
		return fmt.Errorf("no subcommand specified (try: wt config init, wt config hooks)")
	}
}

func runConfigHooks(cmd *ConfigHooksCmd, cfg config.Config) error {
	hooksConfig := cfg.Hooks

	if cmd.JSON {
		type hookJSON struct {
			Name        string `json:"name"`
			Command     string `json:"command"`
			Description string `json:"description,omitempty"`
			RunOnExists bool   `json:"run_on_exists"`
			IsDefault   bool   `json:"is_default"`
		}

		var result []hookJSON
		for name, hook := range hooksConfig.Hooks {
			result = append(result, hookJSON{
				Name:        name,
				Command:     hook.Command,
				Description: hook.Description,
				RunOnExists: hook.RunOnExists,
				IsDefault:   name == hooksConfig.Default,
			})
		}

		// Sort by name for consistent output
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Text output
	if len(hooksConfig.Hooks) == 0 {
		fmt.Println("No hooks configured.")
		fmt.Println("Add hooks to ~/.config/wt/config.toml (see: wt config init)")
		return nil
	}

	fmt.Println("Hooks:")

	// Sort hook names for consistent output
	var names []string
	for name := range hooksConfig.Hooks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		hook := hooksConfig.Hooks[name]
		suffix := ""
		if name == hooksConfig.Default {
			suffix = " (default)"
		}
		desc := hook.Description
		if desc == "" {
			desc = hook.Command
		}
		fmt.Printf("  %-10s %s%s\n", name, desc, suffix)
	}

	return nil
}

const bashCompletions = `# wt bash completions
# Install: wt completion bash > ~/.local/share/bash-completion/completions/wt
# Or: wt completion bash >> ~/.bashrc

_wt_completions() {
    local cur prev words cword
    if type _init_completion &>/dev/null; then
        _init_completion || return
    else
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    fi

    local commands="create open clean list pr config completion"

    # Handle subcommand-specific completions
    case "${words[1]}" in
        create)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                --hook)
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete branch names
                local branches=$(git branch --all --format='%(refname:short)' 2>/dev/null | sed 's|origin/||' | sort -u)
                COMPREPLY=($(compgen -W "$branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-d --dir --hook --no-hook" -- "$cur"))
            fi
            ;;
        open)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
                --hook)
                    return
                    ;;
            esac
            if [[ $cword -eq 2 ]]; then
                # Complete local branch names only
                local branches=$(git branch --format='%(refname:short)' 2>/dev/null)
                COMPREPLY=($(compgen -W "$branches" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "-d --dir --hook --no-hook" -- "$cur"))
            fi
            ;;
        clean)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-d --dir -n --dry-run --refresh-pr -e --empty" -- "$cur"))
            ;;
        list)
            case "$prev" in
                -d|--dir)
                    COMPREPLY=($(compgen -d -- "$cur"))
                    return
                    ;;
            esac
            COMPREPLY=($(compgen -W "-d --dir --json" -- "$cur"))
            ;;
        pr)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "open" -- "$cur"))
            elif [[ "${words[2]}" == "open" ]]; then
                case "$prev" in
                    -d|--dir)
                        COMPREPLY=($(compgen -d -- "$cur"))
                        return
                        ;;
                    --hook)
                        return
                        ;;
                esac
                COMPREPLY=($(compgen -W "-d --dir --hook --no-hook" -- "$cur"))
            fi
            ;;
        config)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "init hooks" -- "$cur"))
            elif [[ "${words[2]}" == "init" ]]; then
                COMPREPLY=($(compgen -W "-f --force" -- "$cur"))
            elif [[ "${words[2]}" == "hooks" ]]; then
                COMPREPLY=($(compgen -W "--json" -- "$cur"))
            fi
            ;;
        completion)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "fish bash zsh" -- "$cur"))
            fi
            ;;
        *)
            if [[ $cword -eq 1 ]]; then
                COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            fi
            ;;
    esac
}

complete -F _wt_completions wt
`

const zshCompletions = `#compdef wt
# wt zsh completions
# Install: wt completion zsh > ~/.zfunc/_wt
# Then add to ~/.zshrc: fpath=(~/.zfunc $fpath) && autoload -Uz compinit && compinit

_wt() {
    local curcontext="$curcontext" state line
    typeset -A opt_args

    _arguments -C \
        '1: :->command' \
        '*:: :->args'

    case $state in
        command)
            local commands=(
                'create:Create new worktree for a branch'
                'open:Open worktree for existing local branch'
                'clean:Cleanup merged worktrees'
                'list:List worktrees'
                'pr:Work with GitHub PRs'
                'config:Manage configuration'
                'completion:Generate completion script'
            )
            _describe 'command' commands
            ;;
        args)
            case $words[1] in
                create)
                    _arguments \
                        '1:branch:__wt_all_branches' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-create hook]'
                    ;;
                open)
                    _arguments \
                        '1:branch:__wt_local_branches' \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--hook[run named hook]:hook:' \
                        '--no-hook[skip post-create hook]'
                    ;;
                clean)
                    _arguments \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '-n[preview without removing]' \
                        '--dry-run[preview without removing]' \
                        '--refresh-pr[force refresh PR cache]' \
                        '-e[also remove 0-commit worktrees]' \
                        '--empty[also remove 0-commit worktrees]'
                    ;;
                list)
                    _arguments \
                        '-d[target directory]:directory:_files -/' \
                        '--dir[target directory]:directory:_files -/' \
                        '--json[output as JSON]'
                    ;;
                pr)
                    _arguments -C \
                        '1: :->subcmd' \
                        '*:: :->args'
                    case $state in
                        subcmd)
                            local subcommands=(
                                'open:Checkout PR as new worktree'
                            )
                            _describe 'subcommand' subcommands
                            ;;
                        args)
                            case $words[1] in
                                open)
                                    _arguments \
                                        '1:PR number:' \
                                        '2:repository:' \
                                        '-d[target directory]:directory:_files -/' \
                                        '--dir[target directory]:directory:_files -/' \
                                        '--hook[run named hook]:hook:' \
                                        '--no-hook[skip post-create hook]'
                                    ;;
                            esac
                            ;;
                    esac
                    ;;
                config)
                    _arguments -C \
                        '1: :->subcmd' \
                        '*:: :->args'
                    case $state in
                        subcmd)
                            local subcommands=(
                                'init:Create default config file'
                                'hooks:List available hooks'
                            )
                            _describe 'subcommand' subcommands
                            ;;
                        args)
                            case $words[1] in
                                init)
                                    _arguments \
                                        '-f[overwrite existing config]' \
                                        '--force[overwrite existing config]'
                                    ;;
                                hooks)
                                    _arguments \
                                        '--json[output as JSON]'
                                    ;;
                            esac
                            ;;
                    esac
                    ;;
                completion)
                    _arguments \
                        '1:shell:(fish bash zsh)'
                    ;;
            esac
            ;;
    esac
}

# Helper: complete all branches (local + remote)
__wt_all_branches() {
    local branches
    branches=(${(f)"$(git branch --all --format='%(refname:short)' 2>/dev/null | sed 's|origin/||' | sort -u)"})
    _describe 'branch' branches
}

# Helper: complete local branches only
__wt_local_branches() {
    local branches
    branches=(${(f)"$(git branch --format='%(refname:short)' 2>/dev/null)"})
    _describe 'branch' branches
}

_wt "$@"
`

const fishCompletions = `# wt completions - supports fish autosuggestions and tab completion
complete -c wt -f

# Subcommands (shown in completions and autosuggestions)
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "create" -d "Create new worktree"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "open" -d "Open worktree for existing branch"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "clean" -d "Cleanup merged worktrees"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "list" -d "List worktrees"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "pr" -d "Work with GitHub PRs"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "config" -d "Manage configuration"
complete -c wt -n "not __fish_seen_subcommand_from create open clean list pr config completion" -a "completion" -d "Generate completion script"

# create: branch name (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from create; and not __fish_seen_argument" -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Branch name"
complete -c wt -n "__fish_seen_subcommand_from create" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from create" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from create" -l no-hook -d "Skip post-create hook"

# open: local branch name (positional), then flags
complete -c wt -n "__fish_seen_subcommand_from open; and not __fish_seen_argument" -a "(git branch --format='%(refname:short)' 2>/dev/null)" -d "Local branch"
complete -c wt -n "__fish_seen_subcommand_from open" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from open" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from open" -l no-hook -d "Skip post-create hook"

# clean: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from clean" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from clean" -s n -l dry-run -d "Preview without removing"
complete -c wt -n "__fish_seen_subcommand_from clean" -l refresh-pr -d "Force refresh PR cache"
complete -c wt -n "__fish_seen_subcommand_from clean" -s e -l empty -d "Also remove worktrees with 0 commits ahead"

# list: flags only (no positional args)
complete -c wt -n "__fish_seen_subcommand_from list" -s d -l dir -r -a "(__fish_complete_directories)" -d "Directory to scan"
complete -c wt -n "__fish_seen_subcommand_from list" -l json -d "Output as JSON"

# pr: subcommands
complete -c wt -n "__fish_seen_subcommand_from pr; and not __fish_seen_subcommand_from open" -a "open" -d "Checkout PR as new worktree"
# pr open: PR number (first positional), then repo (second positional), then flags
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -a "(gh pr list --json number,title --jq '.[] | \"\\(.number)\t\\(.title)\"' 2>/dev/null)" -d "PR number"
# Repo names from default_path (second positional after PR number)
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -a "(__wt_list_repos)" -d "Repository"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -s d -l dir -r -a "(__fish_complete_directories)" -d "Base directory"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -l hook -d "Run named hook instead of default"
complete -c wt -n "__fish_seen_subcommand_from pr; and __fish_seen_subcommand_from open" -l no-hook -d "Skip post-create hook"

# Helper function to list repos in default_path
function __wt_list_repos
    set -l dir "$WT_DEFAULT_PATH"
    if test -z "$dir"
        set -l config_file ~/.config/wt/config.toml
        if test -f "$config_file"
            set dir (grep '^default_path' "$config_file" 2>/dev/null | sed 's/.*= *"\?\([^"]*\)"\?/\1/' | string replace '~' "$HOME")
        end
    end
    if test -d "$dir"
        for d in $dir/*/
            if test -d "$d/.git" -o -f "$d/.git"
                basename $d
            end
        end
    end
end

# config: subcommands
complete -c wt -n "__fish_seen_subcommand_from config; and not __fish_seen_subcommand_from init hooks" -a "init" -d "Create default config file"
complete -c wt -n "__fish_seen_subcommand_from config; and not __fish_seen_subcommand_from init hooks" -a "hooks" -d "List available hooks"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from init" -s f -l force -d "Overwrite existing config file"
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from hooks" -l json -d "Output as JSON"

# completion
complete -c wt -n "__fish_seen_subcommand_from completion" -a "fish" -d "Fish shell"
complete -c wt -n "__fish_seen_subcommand_from completion" -a "bash" -d "Bash shell"
complete -c wt -n "__fish_seen_subcommand_from completion" -a "zsh" -d "Zsh shell"

# Global help
complete -c wt -s h -l help -d "Show help message"
`

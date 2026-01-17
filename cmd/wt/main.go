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
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"base directory (from config or -d flag)"`
	Hook   string `arg:"--hook" help:"run named hook instead of default"`
	NoHook bool   `arg:"--no-hook" help:"skip post-create hook"`
}

func (CreateCmd) Description() string {
	return `Create a new git worktree at <dir>/<repo>-<branch>
Examples:
  wt create feature-branch              # Use default directory from config
  wt create branch -d ~/Git/worktrees   # Create in specific directory`
}

type OpenCmd struct {
	Branch string `arg:"positional,required" placeholder:"BRANCH" help:"existing local branch name"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"base directory (from config or -d flag)"`
	Hook   string `arg:"--hook" help:"run named hook instead of default"`
	NoHook bool   `arg:"--no-hook" help:"skip post-create hook"`
}

func (OpenCmd) Description() string {
	return `Open a worktree for an existing local branch
Examples:
  wt open feature-branch              # Use default directory from config
  wt open feature-branch -d ~/Git     # Create in specific directory`
}

type CleanCmd struct {
	Path      string `arg:"positional,env:WT_DEFAULT_PATH" placeholder:"DIRECTORY" help:"directory to scan (from config or positional arg)"`
	DryRun    bool   `arg:"-n,--dry-run" help:"preview without removing"`
	RefreshPR bool   `arg:"--refresh-pr" help:"force refresh PR cache"`
	Empty     bool   `arg:"-e,--empty" help:"also remove worktrees with 0 commits ahead and clean working directory"`
}

func (CleanCmd) Description() string {
	return `Cleanup merged git worktrees with PR status display.
Removes worktrees where the branch is merged AND working directory is clean.
With --empty, also removes worktrees with 0 commits ahead and clean working directory.`
}

type ListCmd struct {
	Path string `arg:"positional,env:WT_DEFAULT_PATH" placeholder:"DIRECTORY" help:"directory to scan (from config or positional arg)"`
	JSON bool   `arg:"--json" help:"output as JSON"`
}

func (ListCmd) Description() string {
	return "List all git worktrees with their status"
}

type CompletionCmd struct {
	Shell string `arg:"positional,required" placeholder:"SHELL" help:"shell type (fish)"`
}

func (CompletionCmd) Description() string {
	return "Generate shell completion script"
}

type ConfigInitCmd struct {
	Force bool `arg:"-f,--force" help:"overwrite existing config file"`
}

func (ConfigInitCmd) Description() string {
	return "Create default config file at ~/.config/wt/config.toml"
}

type ConfigHooksCmd struct {
	JSON bool `arg:"--json" help:"output as JSON"`
}

func (ConfigHooksCmd) Description() string {
	return "List available hooks"
}

type ConfigCmd struct {
	Init  *ConfigInitCmd  `arg:"subcommand:init" help:"create default config file"`
	Hooks *ConfigHooksCmd `arg:"subcommand:hooks" help:"list available hooks"`
}

func (ConfigCmd) Description() string {
	return "Manage wt configuration"
}

type PrOpenCmd struct {
	Number int    `arg:"positional,required" placeholder:"NUMBER" help:"PR number"`
	Repo   string `arg:"positional" placeholder:"REPO" help:"repository (org/repo or name)"`
	Dir    string `arg:"-d,--dir,env:WT_DEFAULT_PATH" placeholder:"DIR" help:"base directory (from config or -d flag)"`
	Hook   string `arg:"--hook" help:"run named hook instead of default"`
	NoHook bool   `arg:"--no-hook" help:"skip post-create hook"`
}

func (PrOpenCmd) Description() string {
	return `Create a worktree for a GitHub PR
Examples:
  wt pr open 123                  # Use current repo
  wt pr open 123 myrepo           # Find "myrepo" in default_path
  wt pr open 123 org/repo         # Find locally or clone to default_path
  wt pr open 123 -d ~/Git         # Create in specific directory`
}

type PrCmd struct {
	Open *PrOpenCmd `arg:"subcommand:open" help:"checkout PR as new worktree"`
}

func (PrCmd) Description() string {
	return "Work with GitHub pull requests"
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
	return "Git worktree manager - create, clean, and list worktrees"
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
			p.WriteHelp(os.Stdout)
			os.Exit(0)
		case arg.ErrVersion:
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
		fmt.Fprintf(os.Stderr, "Warning: config error: %v\n", err)
	}

	// Apply config defaults
	if args.Create != nil && args.Create.Dir == "" {
		args.Create.Dir = cfg.DefaultPath
	}
	if args.Open != nil && args.Open.Dir == "" {
		args.Open.Dir = cfg.DefaultPath
	}
	if args.Clean != nil && args.Clean.Path == "" {
		args.Clean.Path = cfg.DefaultPath
	}
	if args.List != nil && args.List.Path == "" {
		args.List.Path = cfg.DefaultPath
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
	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	if cmd.Dir == "" {
		return fmt.Errorf("no directory specified: use -d flag or set default_path in config")
	}

	basePath, err := expandPath(cmd.Dir)
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
	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	if cmd.Dir == "" {
		return fmt.Errorf("no directory specified: use -d flag or set default_path in config")
	}

	basePath, err := expandPath(cmd.Dir)
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

// maxConcurrentPRFetches limits parallel gh API calls to avoid rate limiting
const maxConcurrentPRFetches = 5

func runClean(cmd *CleanCmd) error {
	if cmd.Path == "" {
		return fmt.Errorf("no directory specified: provide a path or set default_path in config")
	}

	// Expand path
	scanPath, err := expandPath(cmd.Path)
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

	// Load PR cache
	prCache, err := github.LoadPRCache(scanPath)
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

				pr, err := github.GetPRForBranch(originURL, wt.Branch)
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
	prMap := make(map[string]*github.PRInfo)
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
	if cmd.Path == "" {
		return fmt.Errorf("no directory specified: provide a path or set default_path in config")
	}

	// Expand path
	scanPath, err := expandPath(cmd.Path)
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
	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	if cmd.Dir == "" {
		return fmt.Errorf("no directory specified: use -d flag or set default_path in config")
	}

	basePath, err := expandPath(cmd.Dir)
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
	default:
		return fmt.Errorf("unsupported shell: %s (supported: fish)", cmd.Shell)
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

# clean: suggest common paths + directories (absolute paths only)
complete -c wt -n "__fish_seen_subcommand_from clean; and not __fish_seen_argument -s n -l dry-run -l refresh-pr -s e -l empty" -a "~/Git/worktrees" -d "Default worktrees directory"
complete -c wt -n "__fish_seen_subcommand_from clean" -a "(__fish_complete_directories)" -d "Directory"
complete -c wt -n "__fish_seen_subcommand_from clean" -s n -l dry-run -d "Preview without removing"
complete -c wt -n "__fish_seen_subcommand_from clean" -l refresh-pr -d "Force refresh PR cache"
complete -c wt -n "__fish_seen_subcommand_from clean" -s e -l empty -d "Also remove worktrees with 0 commits ahead"

# list: suggest directories (absolute paths only)
complete -c wt -n "__fish_seen_subcommand_from list; and not __fish_seen_argument -l json" -a "~/Git/worktrees" -d "Worktrees directory"
complete -c wt -n "__fish_seen_subcommand_from list" -a "(__fish_complete_directories)" -d "Directory"
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
        set dir (wt config show 2>/dev/null | grep default_path | awk -F'=' '{print $2}' | string trim)
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
complete -c wt -n "__fish_seen_subcommand_from config; and __fish_seen_subcommand_from hooks" -l json -d "Output as JSON"

# completion
complete -c wt -n "__fish_seen_subcommand_from completion" -a "fish" -d "Fish shell"

# Global help
complete -c wt -s h -l help -d "Show help message"
`

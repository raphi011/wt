package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/alexflint/go-arg"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/ui"
)

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
	if args.Tidy != nil && args.Tidy.Dir == "" {
		args.Tidy.Dir = cfg.DefaultPath
	}
	if args.List != nil && args.List.Dir == "" {
		args.List.Dir = cfg.DefaultPath
	}
	if args.Exec != nil && args.Exec.Dir == "" {
		args.Exec.Dir = cfg.DefaultPath
	}
	if args.Pr != nil && args.Pr.Open != nil && args.Pr.Open.Dir == "" {
		args.Pr.Open.Dir = cfg.DefaultPath
	}
	if args.Pr != nil && args.Pr.Clone != nil && args.Pr.Clone.Dir == "" {
		args.Pr.Clone.Dir = cfg.DefaultPath
	}
	if args.Pr != nil && args.Pr.List != nil && args.Pr.List.Dir == "" {
		args.Pr.List.Dir = cfg.DefaultPath
	}
	if args.Mv != nil {
		if args.Mv.Dir == "" {
			args.Mv.Dir = cfg.DefaultPath
		}
		if args.Mv.Format == "" {
			args.Mv.Format = cfg.WorktreeFormat
		}
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
	case args.Tidy != nil:
		if err := runTidy(args.Tidy, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.List != nil:
		if err := runList(args.List); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Exec != nil:
		if err := runExec(args.Exec); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Mv != nil:
		if err := runMv(args.Mv, cfg); err != nil {
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

	// Run post-create hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, result.AlreadyExists, hooks.CommandCreate)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		// Get context for placeholder substitution
		repoName, err := git.GetRepoName()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get repo name for hook context: %v\n", err)
		}
		folderName, err := git.GetRepoFolderName()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get folder name for hook context: %v\n", err)
		}
		mainRepo, mainRepoErr := git.GetMainRepoPath(result.Path)
		if mainRepoErr != nil || mainRepo == "" {
			// Fallback to current directory (should be the main repo when creating worktrees)
			if mainRepoErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get main repo path: %v (using current directory)\n", mainRepoErr)
			}
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
			Trigger:  string(hooks.CommandCreate),
		}

		for _, match := range hookMatches {
			fmt.Printf("Running hook '%s'...\n", match.Name)
			if err := hooks.Run(match.Hook, ctx); err != nil {
				return fmt.Errorf("hook %q failed: %w", match.Name, err)
			}
			if match.Hook.Description != "" {
				fmt.Printf("  ✓ %s\n", match.Hook.Description)
			}
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

	// Run post-create hooks (always run for open, ignore run_on_exists config)
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, false, hooks.CommandOpen)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		// Get context for placeholder substitution
		repoName, err := git.GetRepoName()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get repo name for hook context: %v\n", err)
		}
		folderName, err := git.GetRepoFolderName()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get folder name for hook context: %v\n", err)
		}
		mainRepo, mainRepoErr := git.GetMainRepoPath(result.Path)
		if mainRepoErr != nil || mainRepo == "" {
			// Fallback to current directory (should be the main repo when opening worktrees)
			if mainRepoErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get main repo path: %v (using current directory)\n", mainRepoErr)
			}
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
			Trigger:  string(hooks.CommandOpen),
		}

		for _, match := range hookMatches {
			fmt.Printf("Running hook '%s'...\n", match.Name)
			if err := hooks.Run(match.Hook, ctx); err != nil {
				return fmt.Errorf("hook %q failed: %w", match.Name, err)
			}
			if match.Hook.Description != "" {
				fmt.Printf("  ✓ %s\n", match.Hook.Description)
			}
		}
	}

	return nil
}

// maxConcurrentPRFetches limits parallel gh API calls to avoid rate limiting
const maxConcurrentPRFetches = 5

func runTidy(cmd *TidyCmd, cfg config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Detect forge per-worktree (optional - PR status is a nice-to-have)
	// We'll check availability when we actually need it

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

	if cmd.DryRun {
		fmt.Printf("Cleaning worktrees in %s (dry run)\n", scanPath)
	} else {
		fmt.Printf("Cleaning worktrees in %s\n", scanPath)
	}

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

	// PR status handling - load from cache only (run 'wt pr list' to fetch)
	prCache := make(forge.PRCache)
	prMap := make(map[string]*forge.PRInfo)
	prUnknown := make(map[string]bool)

	// Load PR cache
	prCache, err = forge.LoadPRCache(scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load PR cache: %v\n", err)
		prCache = make(forge.PRCache)
	}

	// Build PR map from cache for display, track unknown branches
	for _, wt := range worktrees {
		if originCache, ok := prCache[wt.OriginURL]; ok {
			if pr, ok := originCache[wt.Branch]; ok && pr != nil && pr.Fetched {
				prMap[wt.Branch] = pr
				continue
			}
		}
		// Not in cache or not fetched - mark as unknown
		prUnknown[wt.Branch] = true
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
		} else if cmd.IncludeClean && wt.CommitCount == 0 && !wt.IsDirty {
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
	fmt.Print(ui.FormatWorktreesTable(worktrees, prMap, prUnknown, toRemoveMap, cmd.DryRun))

	// Select hooks for tidy (before removing, so we can report errors early)
	// alreadyExists=false since tidy hooks always run for removed worktrees
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, false, hooks.CommandTidy)
	if err != nil {
		return err
	}

	// Remove worktrees
	if !cmd.DryRun && len(toRemove) > 0 {
		for _, wt := range toRemove {
			if err := git.RemoveWorktree(wt, true); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", wt.Path, err)
				continue // Skip hooks for failed removals
			}

			// Run tidy hooks for this worktree
			if len(hookMatches) > 0 {
				ctx := hooks.Context{
					Path:     wt.Path,
					Branch:   wt.Branch,
					Repo:     wt.RepoName,
					Folder:   filepath.Base(wt.MainRepo),
					MainRepo: wt.MainRepo,
					Trigger:  string(hooks.CommandTidy),
				}

				for _, match := range hookMatches {
					fmt.Printf("Running hook '%s' for %s...\n", match.Name, wt.Branch)
					// Use RunWithDir with main repo since worktree path is deleted
					if err := hooks.RunWithDir(match.Hook, ctx, wt.MainRepo); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: hook %q failed for %s: %v\n", match.Name, wt.Branch, err)
					} else if match.Hook.Description != "" {
						fmt.Printf("  ✓ %s\n", match.Hook.Description)
					}
				}
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
	case args.Tidy != nil:
		desc = args.Tidy.Description()
	case args.List != nil:
		desc = args.List.Description()
	case args.Exec != nil:
		desc = args.Exec.Description()
	case args.Mv != nil:
		desc = args.Mv.Description()
	case args.Pr != nil:
		if args.Pr.Open != nil {
			desc = args.Pr.Open.Description()
		} else if args.Pr.Clone != nil {
			desc = args.Pr.Clone.Description()
		} else if args.Pr.List != nil {
			desc = args.Pr.List.Description()
		} else if args.Pr.Merge != nil {
			desc = args.Pr.Merge.Description()
		} else {
			desc = args.Pr.Description()
		}
	case args.Config != nil:
		if args.Config.Init != nil {
			desc = args.Config.Init.Description()
		} else if args.Config.Show != nil {
			desc = args.Config.Show.Description()
		} else if args.Config.Hooks != nil {
			desc = args.Config.Hooks.Description()
		} else {
			desc = args.Config.Description()
		}
	case args.Completion != nil:
		desc = args.Completion.Description()
	default:
		// No subcommand - use custom root help
		writeRootHelp(w)
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

// writeRootHelp prints custom help for the root command
func writeRootHelp(w *os.File) {
	fmt.Fprintln(w, "wt - Git worktree manager with GitHub/GitLab integration")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: wt <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  create      Create worktree for new branch")
	fmt.Fprintln(w, "  open        Open worktree for existing branch")
	fmt.Fprintln(w, "  tidy        Remove merged worktrees")
	fmt.Fprintln(w, "  list        List worktrees with stable IDs")
	fmt.Fprintln(w, "  exec        Run command in worktree by ID")
	fmt.Fprintln(w, "  mv          Move worktrees")
	fmt.Fprintln(w, "  pr          Work with PRs")
	fmt.Fprintln(w, "  config      Manage configuration")
	fmt.Fprintln(w, "  completion  Generate shell completions")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -h, --help     Show help")
	fmt.Fprintln(w, "      --version  Show version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  wt create feature-x       Create worktree for new branch")
	fmt.Fprintln(w, "  wt pr list && wt tidy -n  Fetch PR status, preview tidy")
	fmt.Fprintln(w, "  wt pr open 123            Checkout PR #123 as worktree")
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

	// Acquire lock on cache
	lock := cache.NewFileLock(forge.LockPath(scanPath))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Unlock()

	// Load cache
	wtCache, err := forge.LoadCache(scanPath)
	if err != nil {
		return fmt.Errorf("failed to load cache: %w", err)
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

	// Convert to WorktreeInfo for cache sync
	wtInfos := make([]forge.WorktreeInfo, len(worktrees))
	for i, wt := range worktrees {
		wtInfos[i] = forge.WorktreeInfo{
			Path:      wt.Path,
			Branch:    wt.Branch,
			OriginURL: wt.OriginURL,
		}
	}

	// Sync cache and get ID mapping
	pathToID := wtCache.SyncWorktrees(wtInfos)

	// Save updated cache
	if err := forge.SaveCache(scanPath, wtCache); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
	}

	if cmd.JSON {
		// Build JSON output with IDs
		type worktreeJSON struct {
			ID           int    `json:"id"`
			Path         string `json:"path"`
			Branch       string `json:"branch"`
			MainRepo     string `json:"main_repo"`
			RepoName     string `json:"repo_name"`
			OriginURL    string `json:"origin_url"`
			IsMerged     bool   `json:"is_merged"`
			CommitCount  int    `json:"commit_count"`
			Additions    int    `json:"additions"`
			Deletions    int    `json:"deletions"`
			IsDirty      bool   `json:"is_dirty"`
			HasUntracked bool   `json:"has_untracked"`
			LastCommit   string `json:"last_commit"`
		}
		result := make([]worktreeJSON, len(worktrees))
		for i, wt := range worktrees {
			result[i] = worktreeJSON{
				ID:           pathToID[wt.Path],
				Path:         wt.Path,
				Branch:       wt.Branch,
				MainRepo:     wt.MainRepo,
				RepoName:     wt.RepoName,
				OriginURL:    wt.OriginURL,
				IsMerged:     wt.IsMerged,
				CommitCount:  wt.CommitCount,
				Additions:    wt.Additions,
				Deletions:    wt.Deletions,
				IsDirty:      wt.IsDirty,
				HasUntracked: wt.HasUntracked,
				LastCommit:   wt.LastCommit,
			}
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(worktrees) == 0 {
		return nil
	}

	// Find max ID width and path length for alignment
	maxID := 0
	maxPathLen := 0
	for _, wt := range worktrees {
		if id := pathToID[wt.Path]; id > maxID {
			maxID = id
		}
		if len(wt.Path) > maxPathLen {
			maxPathLen = len(wt.Path)
		}
	}
	idWidth := len(fmt.Sprintf("%d", maxID))

	// Output with ID: ID  /path  commit [branch]
	for _, wt := range worktrees {
		hash, _ := git.GetShortCommitHash(wt.Path)
		if hash == "" {
			hash = "???????"
		}
		id := pathToID[wt.Path]
		fmt.Printf("%*d  %-*s  %s [%s]\n", idWidth, id, maxPathLen, wt.Path, hash, wt.Branch)
	}

	return nil
}

func runExec(cmd *ExecCmd) error {
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

	// Validate command is provided
	if len(cmd.Command) == 0 {
		return fmt.Errorf("no command specified (use: wt exec <id> -- <command>)")
	}

	// Acquire lock on cache (read-only, but ensures consistency)
	lock := cache.NewFileLock(forge.LockPath(scanPath))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Unlock()

	// Load cache
	wtCache, err := forge.LoadCache(scanPath)
	if err != nil {
		return fmt.Errorf("failed to load cache: %w", err)
	}

	// Look up worktree by ID
	path, found := wtCache.GetByID(cmd.ID)
	if !found {
		return fmt.Errorf("worktree ID %d not found (run 'wt list' to see available IDs)", cmd.ID)
	}

	// Validate path still exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("worktree path no longer exists: %s (run 'wt list' to refresh)", path)
	}

	// Release lock before executing command (command may take a while)
	lock.Unlock()

	// Execute command in worktree directory
	c := exec.Command(cmd.Command[0], cmd.Command[1:]...)
	c.Dir = path
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin

	return c.Run()
}

func runMv(cmd *MvCmd, cfg config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Validate destination directory is provided
	if cmd.Dir == "" {
		return fmt.Errorf("destination directory required: use -d flag, set WT_DEFAULT_PATH, or configure default_path in config")
	}

	// Validate worktree format
	if err := format.ValidateFormat(cmd.Format); err != nil {
		return fmt.Errorf("invalid format: %w", err)
	}

	// Expand and validate destination path
	destPath, err := expandPath(cmd.Dir)
	if err != nil {
		return fmt.Errorf("failed to expand destination path: %w", err)
	}
	destPath, err = filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if destination directory exists
	if info, err := os.Stat(destPath); os.IsNotExist(err) {
		return fmt.Errorf("destination directory does not exist: %s", destPath)
	} else if err != nil {
		return fmt.Errorf("failed to check destination directory: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("destination is not a directory: %s", destPath)
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Scan for worktrees in current directory
	worktrees, err := git.ListWorktrees(cwd)
	if err != nil {
		return err
	}

	if len(worktrees) == 0 {
		fmt.Println("No worktrees found in current directory")
		return nil
	}

	// Track results
	var moved, skipped, failed int

	for _, wt := range worktrees {
		// Check if dirty and force not set
		if wt.IsDirty && !cmd.Force {
			fmt.Printf("⚠ Skipping %s: dirty working directory (use -f to force)\n", filepath.Base(wt.Path))
			skipped++
			continue
		}

		// Get repo info for formatting
		gitOrigin := wt.RepoName
		folderName := filepath.Base(wt.MainRepo)

		// Format new worktree name
		newName := format.FormatWorktreeName(cmd.Format, format.FormatParams{
			GitOrigin:  gitOrigin,
			BranchName: wt.Branch,
			FolderName: folderName,
		})

		newPath := filepath.Join(destPath, newName)

		// Check if already at destination with same name
		if wt.Path == newPath {
			fmt.Printf("→ Skipping %s: already at destination\n", filepath.Base(wt.Path))
			skipped++
			continue
		}

		// Check if target path already exists
		if _, err := os.Stat(newPath); err == nil {
			fmt.Printf("⚠ Skipping %s: target path already exists: %s\n", filepath.Base(wt.Path), newPath)
			skipped++
			continue
		}

		if cmd.DryRun {
			fmt.Printf("Would move: %s → %s\n", wt.Path, newPath)
			moved++
			continue
		}

		// Move the worktree
		if err := git.MoveWorktree(wt, newPath, cmd.Force); err != nil {
			fmt.Printf("✗ Failed to move %s: %v\n", filepath.Base(wt.Path), err)
			failed++
			continue
		}

		fmt.Printf("✓ Moved: %s → %s\n", wt.Path, newPath)
		moved++
	}

	// Print summary
	fmt.Println()
	if cmd.DryRun {
		fmt.Printf("Dry run: %d would be moved, %d skipped\n", moved, skipped)
	} else {
		fmt.Printf("Moved: %d, Skipped: %d, Failed: %d\n", moved, skipped, failed)
	}

	return nil
}

func runPr(cmd *PrCmd, cfg config.Config) error {
	switch {
	case cmd.Open != nil:
		return runPrOpen(cmd.Open, cfg)
	case cmd.Clone != nil:
		return runPrClone(cmd.Clone, cfg)
	case cmd.List != nil:
		return runPrList(cmd.List, cfg)
	case cmd.Merge != nil:
		return runPrMerge(cmd.Merge, cfg)
	default:
		return fmt.Errorf("no subcommand specified (try: wt pr open, wt pr clone, wt pr list, wt pr merge)")
	}
}

func runPrOpen(cmd *PrOpenCmd, cfg config.Config) error {
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

	// Determine which repo to use (local only - never clones)
	var repoPath string
	if cmd.Repo == "" {
		// No repo arg: use current directory
		repoPath = "."
	} else {
		// Repo arg provided: find locally only
		_, name := git.ParseRepoArg(cmd.Repo)

		// Try to find repo locally
		foundPath, err := git.FindRepoByName(basePath, name)
		if err == nil {
			repoPath = foundPath
			fmt.Printf("Using repo at %s\n", repoPath)
		} else {
			// Not found: error with suggestions and hint to use pr clone
			similar := git.FindSimilarRepos(basePath, name)
			if len(similar) > 0 {
				return fmt.Errorf("repository %q not found in %s\nDid you mean: %s\nTo clone a new repo, use: wt pr clone %d %s", name, basePath, strings.Join(similar, ", "), cmd.Number, cmd.Repo)
			}
			return fmt.Errorf("repository %q not found in %s\nTo clone a new repo, use: wt pr clone %d %s", name, basePath, cmd.Number, cmd.Repo)
		}
	}

	// Get origin URL from repo
	originURL, err := git.GetOriginURL(repoPath)
	if err != nil {
		return fmt.Errorf("failed to get origin URL: %w", err)
	}

	// Detect forge and check CLI availability
	f := forge.Detect(originURL, cfg.Hosts)
	if err := f.Check(); err != nil {
		return err
	}

	// Fetch PR branch name
	fmt.Printf("Fetching PR #%d...\n", cmd.Number)
	branch, err := f.GetPRBranch(originURL, cmd.Number)
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

	// Run post-create hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, result.AlreadyExists, hooks.CommandPR)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		// Get context for placeholder substitution
		repoName, err := git.GetRepoNameFrom(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get repo name for hook context: %v\n", err)
		}
		folderName := filepath.Base(repoPath)
		mainRepo, mainRepoErr := git.GetMainRepoPath(result.Path)
		if mainRepoErr != nil || mainRepo == "" {
			// Fallback to the repo path used for creating the worktree
			if mainRepoErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get main repo path: %v (using %s)\n", mainRepoErr, repoPath)
			}
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
			Trigger:  string(hooks.CommandPR),
		}

		for _, match := range hookMatches {
			fmt.Printf("Running hook '%s'...\n", match.Name)
			if err := hooks.Run(match.Hook, ctx); err != nil {
				return fmt.Errorf("hook %q failed: %w", match.Name, err)
			}
			if match.Hook.Description != "" {
				fmt.Printf("  ✓ %s\n", match.Hook.Description)
			}
		}
	}

	return nil
}

func runPrClone(cmd *PrCloneCmd, cfg config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	// Parse org/repo - use clone.org if org not specified
	org, name := git.ParseRepoArg(cmd.Repo)
	if org == "" {
		if cfg.Clone.Org == "" {
			return fmt.Errorf("repository must be in org/repo format, or configure [clone] org in config")
		}
		org = cfg.Clone.Org
	}
	repoSpec := org + "/" + name

	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	basePath, err := expandPath(dir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// Check if repo already exists locally
	if existingPath, err := git.FindRepoByName(basePath, name); err == nil {
		return fmt.Errorf("repository %q already exists at %s\nUse 'wt pr open %d %s' instead", name, existingPath, cmd.Number, name)
	}

	// Determine forge: cmd.Forge > cfg.Clone rules > cfg.Clone.Forge
	forgeName := cmd.Forge
	if forgeName == "" {
		forgeName = cfg.Clone.GetForgeForRepo(repoSpec)
	}

	f := forge.ByName(forgeName)
	if err := f.Check(); err != nil {
		return err
	}

	// Clone the repo
	fmt.Printf("Cloning %s to %s (using %s)...\n", repoSpec, basePath, forgeName)
	repoPath, err := f.CloneRepo(repoSpec, basePath)
	if err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}
	fmt.Printf("✓ Cloned to %s\n", repoPath)

	// Get origin URL from cloned repo
	originURL, err := git.GetOriginURL(repoPath)
	if err != nil {
		return fmt.Errorf("failed to get origin URL: %w", err)
	}

	// Fetch PR branch name
	fmt.Printf("Fetching PR #%d...\n", cmd.Number)
	branch, err := f.GetPRBranch(originURL, cmd.Number)
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

	// Run post-create hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, result.AlreadyExists, hooks.CommandPR)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		// Get context for placeholder substitution
		repoName, err := git.GetRepoNameFrom(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get repo name for hook context: %v\n", err)
		}
		folderName := filepath.Base(repoPath)
		mainRepo, mainRepoErr := git.GetMainRepoPath(result.Path)
		if mainRepoErr != nil || mainRepo == "" {
			// Fallback to the repo path used for creating the worktree
			if mainRepoErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get main repo path: %v (using %s)\n", mainRepoErr, repoPath)
			}
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
			Trigger:  string(hooks.CommandPR),
		}

		for _, match := range hookMatches {
			fmt.Printf("Running hook '%s'...\n", match.Name)
			if err := hooks.Run(match.Hook, ctx); err != nil {
				return fmt.Errorf("hook %q failed: %w", match.Name, err)
			}
			if match.Hook.Description != "" {
				fmt.Printf("  ✓ %s\n", match.Hook.Description)
			}
		}
	}

	return nil
}

func runPrList(cmd *PrListCmd, cfg config.Config) error {
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

	fmt.Printf("Fetching PR status for worktrees in %s\n", scanPath)

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

	// Load existing cache
	prCache, err := forge.LoadPRCache(scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load PR cache: %v\n", err)
		prCache = make(forge.PRCache)
	}

	// Filter to worktrees with upstream branches
	var toFetch []git.Worktree
	for _, wt := range worktrees {
		if wt.OriginURL == "" {
			continue
		}
		if git.GetUpstreamBranch(wt.MainRepo, wt.Branch) == "" {
			continue
		}
		toFetch = append(toFetch, wt)
	}

	if len(toFetch) == 0 {
		sp.Stop()
		fmt.Println("No worktrees with upstream branches found")
		return nil
	}

	sp.UpdateMessage(fmt.Sprintf("Fetching PR status (%d branches)...", len(toFetch)))

	// Fetch PRs in parallel with rate limiting
	var prMutex sync.Mutex
	var prWg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentPRFetches)
	var fetchedCount, failedCount int
	var countMutex sync.Mutex

	for _, wt := range toFetch {
		prWg.Add(1)
		go func(wt git.Worktree) {
			defer prWg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			// Detect forge for this repo
			f := forge.Detect(wt.OriginURL, cfg.Hosts)

			// Check if forge CLI is available
			if err := f.Check(); err != nil {
				countMutex.Lock()
				failedCount++
				countMutex.Unlock()
				return
			}

			// Use upstream branch name for PR lookup (may differ from local)
			upstreamBranch := git.GetUpstreamBranch(wt.MainRepo, wt.Branch)
			if upstreamBranch == "" {
				return
			}

			pr, err := f.GetPRForBranch(wt.OriginURL, upstreamBranch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: PR fetch failed for %s: %v\n", wt.Branch, err)
				countMutex.Lock()
				failedCount++
				countMutex.Unlock()
				return
			}

			prMutex.Lock()
			if prCache[wt.OriginURL] == nil {
				prCache[wt.OriginURL] = make(map[string]*forge.PRInfo)
			}
			prCache[wt.OriginURL][wt.Branch] = pr
			prMutex.Unlock()

			countMutex.Lock()
			fetchedCount++
			countMutex.Unlock()
		}(wt)
	}

	prWg.Wait()
	sp.Stop()

	// Save updated cache
	if err := forge.SavePRCache(scanPath, prCache); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save PR cache: %v\n", err)
	}

	// Print summary
	if failedCount > 0 {
		fmt.Printf("Fetched: %d, Failed: %d\n", fetchedCount, failedCount)
	} else {
		fmt.Printf("Fetched: %d\n", fetchedCount)
	}

	return nil
}

func runPrMerge(cmd *PrMergeCmd, cfg config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Must be in a worktree (not main repo)
	if !git.IsWorktree(cwd) {
		return fmt.Errorf("must run from inside a worktree (not the main repo)")
	}

	// Get branch and main repo
	branch, err := git.GetCurrentBranch(cwd)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	mainRepo, err := git.GetMainRepoPath(cwd)
	if err != nil {
		return fmt.Errorf("failed to get main repo path: %w", err)
	}

	// Get origin URL and detect forge
	originURL, err := git.GetOriginURL(mainRepo)
	if err != nil {
		return fmt.Errorf("failed to get origin URL: %w", err)
	}
	f := forge.Detect(originURL, cfg.Hosts)
	if err := f.Check(); err != nil {
		return err
	}

	// Get PR for current branch
	fmt.Printf("Checking PR status for branch %s...\n", branch)
	pr, err := f.GetPRForBranch(originURL, branch)
	if err != nil {
		return fmt.Errorf("failed to get PR info: %w", err)
	}
	if pr == nil || pr.Number == 0 {
		return fmt.Errorf("no PR found for branch %q", branch)
	}
	if pr.State == "MERGED" {
		return fmt.Errorf("PR #%d is already merged (use 'wt tidy' to clean up)", pr.Number)
	}
	if pr.State == "CLOSED" {
		return fmt.Errorf("PR #%d is closed", pr.Number)
	}

	// Resolve merge strategy: flag > config > default
	strategy := cmd.Strategy
	if strategy == "" {
		strategy = cfg.Merge.Strategy
	}
	if strategy == "" {
		strategy = "squash"
	}

	// Validate strategy
	if strategy != "squash" && strategy != "rebase" && strategy != "merge" {
		return fmt.Errorf("invalid merge strategy %q: must be squash, rebase, or merge", strategy)
	}

	// Merge the PR
	fmt.Printf("Merging PR #%d (%s)...\n", pr.Number, strategy)
	if err := f.MergePR(originURL, pr.Number, strategy); err != nil {
		return err
	}
	fmt.Printf("✓ PR #%d merged\n", pr.Number)

	// Cleanup (unless --keep)
	if !cmd.Keep {
		// Build worktree struct for removal
		wt := git.Worktree{
			Path:     cwd,
			Branch:   branch,
			MainRepo: mainRepo,
		}

		fmt.Printf("Removing worktree...\n")
		if err := git.RemoveWorktree(wt, true); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		fmt.Printf("Deleting local branch %s...\n", branch)
		if err := git.DeleteLocalBranch(mainRepo, branch, true); err != nil {
			// Don't fail - branch might already be gone or worktree removal handled it
			fmt.Fprintf(os.Stderr, "Warning: failed to delete local branch: %v\n", err)
		}

		git.PruneWorktrees(mainRepo)
		fmt.Printf("✓ Cleanup complete\n")
	}

	// Run hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, false, hooks.CommandMerge)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		// Get repo info for hook context
		repoName, _ := git.GetRepoNameFrom(mainRepo)
		folderName := filepath.Base(mainRepo)

		ctx := hooks.Context{
			Path:     cwd,
			Branch:   branch,
			Repo:     repoName,
			Folder:   folderName,
			MainRepo: mainRepo,
			Trigger:  string(hooks.CommandMerge),
		}

		// If worktree was removed, run hooks from main repo
		workDir := cwd
		if !cmd.Keep {
			workDir = mainRepo
		}

		for _, match := range hookMatches {
			fmt.Printf("Running hook '%s'...\n", match.Name)
			if err := hooks.RunWithDir(match.Hook, ctx, workDir); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: hook %q failed: %v\n", match.Name, err)
			} else if match.Hook.Description != "" {
				fmt.Printf("  ✓ %s\n", match.Hook.Description)
			}
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
	case cmd.Show != nil:
		return runConfigShow(cmd.Show, cfg)
	case cmd.Hooks != nil:
		return runConfigHooks(cmd.Hooks, cfg)
	default:
		return fmt.Errorf("no subcommand specified (try: wt config init, wt config show, wt config hooks)")
	}
}

func runConfigShow(cmd *ConfigShowCmd, cfg config.Config) error {
	if cmd.JSON {
		// Build JSON output structure
		type cloneRuleJSON struct {
			Pattern string `json:"pattern"`
			Forge   string `json:"forge"`
		}
		type cloneJSON struct {
			Forge string          `json:"forge"`
			Org   string          `json:"org,omitempty"`
			Rules []cloneRuleJSON `json:"rules,omitempty"`
		}
		type mergeJSON struct {
			Strategy string `json:"strategy,omitempty"`
		}
		type configJSON struct {
			DefaultPath    string            `json:"default_path,omitempty"`
			WorktreeFormat string            `json:"worktree_format"`
			Clone          cloneJSON         `json:"clone"`
			Merge          mergeJSON         `json:"merge,omitempty"`
			Hosts          map[string]string `json:"hosts,omitempty"`
		}

		var rules []cloneRuleJSON
		for _, r := range cfg.Clone.Rules {
			rules = append(rules, cloneRuleJSON{
				Pattern: r.Pattern,
				Forge:   r.Forge,
			})
		}

		result := configJSON{
			DefaultPath:    cfg.DefaultPath,
			WorktreeFormat: cfg.WorktreeFormat,
			Clone: cloneJSON{
				Forge: cfg.Clone.Forge,
				Org:   cfg.Clone.Org,
				Rules: rules,
			},
			Merge: mergeJSON{
				Strategy: cfg.Merge.Strategy,
			},
			Hosts: cfg.Hosts,
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Text output
	fmt.Printf("worktree_format = %q\n", cfg.WorktreeFormat)
	if cfg.DefaultPath != "" {
		fmt.Printf("default_path = %q\n", cfg.DefaultPath)
	}

	// Clone section
	fmt.Printf("\n[clone]\n")
	fmt.Printf("forge = %q\n", cfg.Clone.Forge)
	if cfg.Clone.Org != "" {
		fmt.Printf("org = %q\n", cfg.Clone.Org)
	}
	for _, rule := range cfg.Clone.Rules {
		fmt.Printf("\n[[clone.rules]]\n")
		fmt.Printf("pattern = %q\n", rule.Pattern)
		fmt.Printf("forge = %q\n", rule.Forge)
	}

	// Merge section
	if cfg.Merge.Strategy != "" {
		fmt.Printf("\n[merge]\n")
		fmt.Printf("strategy = %q\n", cfg.Merge.Strategy)
	}

	// Hosts section
	if len(cfg.Hosts) > 0 {
		fmt.Printf("\n[hosts]\n")
		for host, forgeType := range cfg.Hosts {
			fmt.Printf("%q = %q\n", host, forgeType)
		}
	}

	return nil
}

func runConfigHooks(cmd *ConfigHooksCmd, cfg config.Config) error {
	hooksConfig := cfg.Hooks

	if cmd.JSON {
		type hookJSON struct {
			Name        string   `json:"name"`
			Command     string   `json:"command"`
			Description string   `json:"description,omitempty"`
			RunOnExists bool     `json:"run_on_exists"`
			On          []string `json:"on,omitempty"`
		}

		var result []hookJSON
		for name, hook := range hooksConfig.Hooks {
			result = append(result, hookJSON{
				Name:        name,
				Command:     hook.Command,
				Description: hook.Description,
				RunOnExists: hook.RunOnExists,
				On:          hook.On,
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
		if len(hook.On) > 0 {
			suffix = fmt.Sprintf(" (on: %v)", hook.On)
		}
		if hook.Description != "" {
			fmt.Printf("  %-10s %s%s\n", name, hook.Description, suffix)
			fmt.Printf("             %s\n", hook.Command)
		} else {
			fmt.Printf("  %-10s %s%s\n", name, hook.Command, suffix)
		}
	}

	return nil
}

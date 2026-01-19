package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/resolve"
	"github.com/raphi011/wt/internal/ui"
)

// maxConcurrentPRFetches limits parallel gh API calls to avoid rate limiting
const maxConcurrentPRFetches = 5

func runPr(cmd *PrCmd, cfg config.Config) error {
	switch {
	case cmd.Open != nil:
		return runPrOpen(cmd.Open, cfg)
	case cmd.Clone != nil:
		return runPrClone(cmd.Clone, cfg)
	case cmd.Refresh != nil:
		return runPrRefresh(cmd.Refresh, cfg)
	case cmd.Merge != nil:
		return runPrMerge(cmd.Merge, cfg)
	default:
		return fmt.Errorf("no subcommand specified (try: wt pr open, wt pr clone, wt pr refresh, wt pr merge)")
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

	// Set note if provided
	if cmd.Note != "" {
		absRepoPath, err := filepath.Abs(repoPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute repo path: %w", err)
		}
		if err := git.SetBranchNote(absRepoPath, branch, cmd.Note); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set note: %v\n", err)
		}
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

func runPrRefresh(cmd *PrRefreshCmd, cfg config.Config) error {
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

	fmt.Printf("Refreshing PR cache for worktrees in %s\n", scanPath)

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

	var branch, mainRepo, wtPath string
	var err error

	// Resolve target: if Target is empty and in worktree, use current branch
	// Otherwise use the resolver
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if cmd.Target == "" && git.IsWorktree(cwd) {
		// Inside worktree, no target specified - use current branch
		branch, err = git.GetCurrentBranch(cwd)
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		mainRepo, err = git.GetMainRepoPath(cwd)
		if err != nil {
			return fmt.Errorf("failed to get main repo path: %w", err)
		}
		wtPath = cwd
	} else if cmd.Target == "" {
		return fmt.Errorf("target required when not inside a worktree (use ID or branch name)")
	} else {
		// Resolve target by ID or branch name
		scanDir := cmd.Dir
		if scanDir == "" {
			scanDir = "."
		}
		scanDir, err = expandPath(scanDir)
		if err != nil {
			return fmt.Errorf("failed to expand path: %w", err)
		}
		scanDir, err = filepath.Abs(scanDir)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path: %w", err)
		}

		target, err := resolve.ByIDOrBranch(cmd.Target, scanDir)
		if err != nil {
			return err
		}
		branch = target.Branch
		mainRepo = target.MainRepo
		wtPath = target.Path
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
			Path:     wtPath,
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
			Path:     wtPath,
			Branch:   branch,
			Repo:     repoName,
			Folder:   folderName,
			MainRepo: mainRepo,
			Trigger:  string(hooks.CommandMerge),
		}

		// If worktree was removed, run hooks from main repo
		workDir := wtPath
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

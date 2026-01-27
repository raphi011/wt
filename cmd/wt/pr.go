package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/resolve"
	"github.com/raphi011/wt/internal/ui"
)

func (c *PrCheckoutCmd) runPrCheckout(ctx context.Context) error {
	l := log.FromContext(ctx)
	cfg := c.Config
	workDir := c.WorkDir

	// Handle interactive mode
	if c.Interactive {
		// Error if clone mode is used with interactive
		if c.Repo != "" {
			return fmt.Errorf("cannot use clone mode (org/repo) with interactive mode (-i)")
		}

		opts, err := c.runInteractive(ctx)
		if err != nil {
			return err
		}
		if opts.Cancelled {
			return nil
		}

		// Apply gathered options
		c.Number = opts.SelectedPR
		if opts.SelectedRepo != "" && c.Repository == "" {
			// Find repo name from path
			c.Repository = git.GetRepoDisplayName(opts.SelectedRepo)
		}

		// Apply hook selection from wizard
		if opts.NoHook {
			c.NoHook = true
		} else if len(opts.SelectedHooks) > 0 {
			c.Hook = opts.SelectedHooks
		}
	}

	// Validate PR number is provided (required unless cancelled in interactive mode)
	if c.Number == 0 {
		return fmt.Errorf("PR number required (use -i for interactive mode)")
	}

	// Validate mutual exclusion: positional org/repo and -r flag can't both be used
	if c.Repo != "" && c.Repository != "" {
		return fmt.Errorf("cannot use both positional org/repo argument and -r/--repository flag\nUse 'wt pr checkout %d %s' (clone mode) OR 'wt pr checkout %d -r %s' (local mode)", c.Number, c.Repo, c.Number, c.Repository)
	}

	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	// Worktree target dir from config
	dir, absPath, err := resolveWorktreeTargetDir(cfg.WorktreeDir, workDir)
	if err != nil {
		return err
	}

	// Repo dir (from config, fallback to dir)
	repoDir := cfg.RepoScanDir()
	if repoDir == "" {
		repoDir = dir
	}

	var repoPath string
	var f forge.Forge

	// Clone mode: positional Repo arg with org/repo format
	if c.Repo != "" {
		// Validate org/repo format (must contain /)
		if !strings.Contains(c.Repo, "/") {
			return fmt.Errorf("clone mode requires org/repo format (e.g., 'org/repo'), got %q\nTo use a local repo by name, use: wt pr checkout %d -r %s", c.Repo, c.Number, c.Repo)
		}

		// Parse org/repo - use forge.default_org if org not specified
		org, name := git.ParseRepoArg(c.Repo)
		if org == "" {
			if cfg.Forge.DefaultOrg == "" {
				return fmt.Errorf("repository must be in org/repo format, or configure [forge] default_org in config")
			}
			org = cfg.Forge.DefaultOrg
		}
		repoSpec := org + "/" + name

		// Check if repo already exists locally
		if existingPath, err := git.FindRepoByName(repoDir, name); err == nil {
			return fmt.Errorf("repository %q already exists at %s\nUse 'wt pr checkout %d -r %s' instead", name, existingPath, c.Number, name)
		}

		// Determine forge: c.Forge > cfg.Forge rules > cfg.Forge.Default
		forgeName := c.Forge
		if forgeName == "" {
			forgeName = cfg.Forge.GetForgeTypeForRepo(repoSpec)
		}

		f = forge.ByNameWithConfig(forgeName, &cfg.Forge)
		if err := f.Check(ctx); err != nil {
			return err
		}

		// Clone the repo to repoDir (repo_dir if set, else worktree_dir)
		l.Printf("Cloning %s to %s (using %s)...\n", repoSpec, repoDir, forgeName)
		repoPath, err = f.CloneRepo(ctx, repoSpec, repoDir)
		if err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}
		l.Printf("✓ Cloned to %s\n", repoPath)
	} else {
		// Local mode: -r flag or current directory
		if c.Repository != "" {
			// -r flag: find repo locally by name
			foundPath, err := git.FindRepoByName(repoDir, c.Repository)
			if err != nil {
				similar := git.FindSimilarRepos(repoDir, c.Repository)
				if len(similar) > 0 {
					return fmt.Errorf("repository %q not found in %s\nDid you mean: %s", c.Repository, repoDir, strings.Join(similar, ", "))
				}
				return fmt.Errorf("repository %q not found in %s", c.Repository, repoDir)
			}
			repoPath = foundPath
			l.Printf("Using repo at %s\n", repoPath)
		} else {
			// No args: use current directory
			repoPath = "."
		}

		// Get origin URL and detect forge
		originURL, err := git.GetOriginURL(ctx, repoPath)
		if err != nil {
			return fmt.Errorf("failed to get origin URL: %w", err)
		}

		f = forge.Detect(originURL, cfg.Hosts, &cfg.Forge)
		if err := f.Check(ctx); err != nil {
			return err
		}
	}

	// Get origin URL from repo (may have just been cloned)
	originURL, err := git.GetOriginURL(ctx, repoPath)
	if err != nil {
		return fmt.Errorf("failed to get origin URL: %w", err)
	}

	// Try cache first to avoid network request
	var branch string
	wtCache, err := cache.Load(dir)
	if err == nil {
		branch = wtCache.GetBranchByPRNumber(originURL, c.Number)
	}

	if branch == "" {
		// Cache miss - fetch from network
		l.Printf("Fetching PR #%d...\n", c.Number)
		branch, err = f.GetPRBranch(ctx, originURL, c.Number)
		if err != nil {
			return fmt.Errorf("failed to get PR branch: %w", err)
		}
	}

	l.Printf("Creating worktree for branch %s in %s\n", branch, absPath)

	result, err := git.CreateWorktreeFrom(ctx, repoPath, dir, branch, cfg.WorktreeFormat, "")
	if err != nil {
		return err
	}

	// Assign worktree ID and save to cache
	var id int
	mainRepo, _ := git.GetMainRepoPath(result.Path)
	if mainRepo == "" {
		mainRepo, _ = filepath.Abs(repoPath)
	}
	wtCache, unlock, err := cache.LoadWithLock(dir)
	if err == nil {
		defer unlock()
		id = wtCache.GetOrAssignID(cache.WorktreeInfo{
			Path:     result.Path,
			RepoPath: mainRepo,
			Branch:   branch,
		})
		if saveErr := cache.Save(dir, wtCache); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", saveErr)
		}
	}

	// Print result (to stderr via logger)
	if result.AlreadyExists {
		l.Printf("→ Worktree already exists at: %s (nr: %d)\n", result.Path, id)
	} else {
		l.Printf("✓ Worktree created at: %s (nr: %d)\n", result.Path, id)
	}

	// Print path to stdout for piping
	out := output.FromContext(ctx)
	out.Println(result.Path)

	// Set note if provided
	if c.Note != "" {
		absRepoPath, err := filepath.Abs(repoPath)
		if err != nil {
			return fmt.Errorf("failed to get absolute repo path: %w", err)
		}
		if err := git.SetBranchNote(ctx, absRepoPath, branch, c.Note); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set note: %v\n", err)
		}
	}

	// Run post-add hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, c.Hook, c.NoHook, hooks.CommandPR)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(c.Env)
	if err != nil {
		return err
	}

	hookCtx := hooks.Context{
		Path:    result.Path,
		Branch:  branch,
		Folder:  filepath.Base(repoPath),
		Trigger: string(hooks.CommandPR),
		Env:     env,
	}
	hookCtx.Repo, _ = git.GetRepoNameFrom(ctx, repoPath)
	hookCtx.MainRepo, _ = git.GetMainRepoPath(result.Path)
	if hookCtx.MainRepo == "" {
		hookCtx.MainRepo, _ = filepath.Abs(repoPath)
	}

	return hooks.RunAll(hookMatches, hookCtx)
}

func (c *PrMergeCmd) runPrMerge(ctx context.Context) error {
	l := log.FromContext(ctx)
	cfg := c.Config
	worktreeDir, _ := cfg.GetAbsWorktreeDir()
	target, err := resolve.ByIDOrRepoOrPath(ctx, c.ID, c.Repository, worktreeDir, cfg.RepoScanDir(), c.WorkDir)
	if err != nil {
		return err
	}

	// Check if we're operating from main repo (not a worktree)
	isMainRepo := target.Path == target.MainRepo

	f, originURL, err := forge.DetectAndCheck(ctx, target.MainRepo, cfg.Hosts, &cfg.Forge)
	if err != nil {
		return err
	}

	// Get PR for current branch
	l.Printf("Checking PR status for branch %s...\n", target.Branch)
	pr, err := f.GetPRForBranch(ctx, originURL, target.Branch)
	if err != nil {
		return fmt.Errorf("failed to get PR info: %w", err)
	}
	if pr == nil || pr.Number == 0 {
		return fmt.Errorf("no PR found for branch %q", target.Branch)
	}
	if pr.State == "MERGED" {
		return fmt.Errorf("PR #%d is already merged (use 'wt prune' to clean up)", pr.Number)
	}
	if pr.State == "CLOSED" {
		return fmt.Errorf("PR #%d is closed", pr.Number)
	}

	// Resolve merge strategy: flag > config > default
	strategy := c.Strategy
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
	l.Printf("Merging PR #%d (%s)...\n", pr.Number, strategy)
	if err := f.MergePR(ctx, originURL, pr.Number, strategy); err != nil {
		return err
	}
	l.Printf("✓ PR #%d merged\n", pr.Number)

	// Cleanup (unless --keep)
	if !c.Keep {
		// Only remove worktree if we're operating on one (not main repo)
		if !isMainRepo {
			// Build worktree struct for removal
			wt := git.Worktree{
				Path:     target.Path,
				Branch:   target.Branch,
				MainRepo: target.MainRepo,
			}

			l.Printf("Removing worktree...\n")
			if err := git.RemoveWorktree(ctx, wt, true); err != nil {
				return fmt.Errorf("failed to remove worktree: %w", err)
			}
			git.PruneWorktrees(ctx, target.MainRepo)
		}

		l.Printf("Deleting local branch %s...\n", target.Branch)
		if err := git.DeleteLocalBranch(ctx, target.MainRepo, target.Branch, true); err != nil {
			// Don't fail - branch might already be gone or worktree removal handled it
			fmt.Fprintf(os.Stderr, "Warning: failed to delete local branch: %v\n", err)
		}

		l.Printf("✓ Cleanup complete\n")
	}

	// Run hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, c.Hook, c.NoHook, hooks.CommandMerge)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(c.Env)
	if err != nil {
		return err
	}

	hookCtx := hooks.Context{
		Path:     target.Path,
		Branch:   target.Branch,
		MainRepo: target.MainRepo,
		Folder:   filepath.Base(target.MainRepo),
		Trigger:  string(hooks.CommandMerge),
		Env:      env,
	}
	hookCtx.Repo, _ = git.GetRepoNameFrom(ctx, target.MainRepo)

	// If worktree was removed or was main repo, run hooks from main repo
	hookWorkDir := target.Path
	if !c.Keep || isMainRepo {
		hookWorkDir = target.MainRepo
	}

	hooks.RunAllNonFatal(hookMatches, hookCtx, hookWorkDir)
	return nil
}

func (c *PrCreateCmd) runPrCreate(ctx context.Context) error {
	l := log.FromContext(ctx)
	cfg := c.Config
	worktreeDir, _ := cfg.GetAbsWorktreeDir()
	target, err := resolve.ByIDOrRepoOrPath(ctx, c.ID, c.Repository, worktreeDir, cfg.RepoScanDir(), c.WorkDir)
	if err != nil {
		return err
	}

	f, originURL, err := forge.DetectAndCheck(ctx, target.MainRepo, cfg.Hosts, &cfg.Forge)
	if err != nil {
		return err
	}

	// Get body content
	body := c.Body
	if c.BodyFile != "" {
		// Read body from file
		content, err := os.ReadFile(c.BodyFile)
		if err != nil {
			return fmt.Errorf("failed to read body file: %w", err)
		}
		body = string(content)
	} else if c.Body == "-" {
		// Read body from stdin
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read body from stdin: %w", err)
		}
		body = string(content)
	}

	// Create PR params
	params := forge.CreatePRParams{
		Title: c.Title,
		Body:  body,
		Base:  c.Base,
		Head:  target.Branch,
		Draft: c.Draft,
	}

	// Create the PR
	l.Printf("Creating PR for branch %s...\n", target.Branch)
	result, err := f.CreatePR(ctx, originURL, params)
	if err != nil {
		return err
	}

	l.Printf("✓ PR #%d created: %s\n", result.Number, result.URL)

	// Update cache with PR info
	wtCache, err := cache.Load(worktreeDir)
	if err == nil {
		state := "OPEN"
		if c.Draft {
			state = "DRAFT"
		}
		prInfo := &cache.PRInfo{
			Number:   result.Number,
			State:    state,
			IsDraft:  c.Draft,
			URL:      result.URL,
			CachedAt: time.Now(),
			Fetched:  true,
		}
		folderName := filepath.Base(target.Path)
		wtCache.SetPRForBranch(folderName, prInfo)
		if err := cache.Save(worktreeDir, wtCache); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update cache: %v\n", err)
		}
	}

	// Open in browser if requested
	if c.Web && result.URL != "" {
		if err := openBrowser(result.URL); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open browser: %v\n", err)
		}
	}

	return nil
}

func (c *PrViewCmd) runPrView(ctx context.Context) error {
	cfg := c.Config
	worktreeDir, _ := cfg.GetAbsWorktreeDir()
	target, err := resolve.ByIDOrRepoOrPath(ctx, c.ID, c.Repository, worktreeDir, cfg.RepoScanDir(), c.WorkDir)
	if err != nil {
		return err
	}

	f, originURL, err := forge.DetectAndCheck(ctx, target.MainRepo, cfg.Hosts, &cfg.Forge)
	if err != nil {
		return err
	}

	// Get PR for current branch
	pr, err := f.GetPRForBranch(ctx, originURL, target.Branch)
	if err != nil {
		return fmt.Errorf("failed to get PR info: %w", err)
	}
	if pr == nil || pr.Number == 0 {
		return fmt.Errorf("no PR found for branch %q", target.Branch)
	}

	// View the PR
	return f.ViewPR(ctx, originURL, pr.Number, c.Web)
}

// runInteractive runs the interactive PR checkout wizard.
func (c *PrCheckoutCmd) runInteractive(ctx context.Context) (ui.PrCheckoutOptions, error) {
	cfg := c.Config
	workDir := c.WorkDir
	l := log.FromContext(ctx)

	insideRepo := git.IsInsideRepoPath(ctx, workDir)

	// Get repo scan directory
	repoDir := cfg.RepoScanDir()
	if repoDir == "" {
		repoDir = cfg.WorktreeDir
	}
	if repoDir == "" && insideRepo {
		repoDir = workDir
	}

	// Build wizard params
	params := ui.PrCheckoutWizardParams{
		PreSelectedRepo: -1,
	}

	// If -r flag is provided, use that specific repo
	if c.Repository != "" {
		if repoDir == "" {
			return ui.PrCheckoutOptions{}, fmt.Errorf("directory required when using -r (set WT_WORKTREE_DIR or worktree_dir in config)")
		}

		repoPath, err := git.FindRepoByName(repoDir, c.Repository)
		if err != nil {
			return ui.PrCheckoutOptions{}, fmt.Errorf("repository %q not found: %w", c.Repository, err)
		}

		params.AvailableRepos = []string{repoPath}
		params.RepoNames = []string{c.Repository}
		params.PreSelectedRepo = 0
	} else if insideRepo {
		// Inside a repo - use current repo
		currentRepoPath, _ := git.GetMainRepoPath(workDir)
		if currentRepoPath == "" {
			currentRepoPath = workDir
		}
		params.AvailableRepos = []string{currentRepoPath}
		params.RepoNames = []string{git.GetRepoDisplayName(currentRepoPath)}
		params.PreSelectedRepo = 0
	} else {
		// Outside repo - scan for all repos
		if repoDir == "" {
			return ui.PrCheckoutOptions{}, fmt.Errorf("directory required for interactive mode (set WT_WORKTREE_DIR or worktree_dir in config)")
		}

		allRepos, err := git.FindAllRepos(repoDir)
		if err != nil {
			return ui.PrCheckoutOptions{}, fmt.Errorf("failed to find repositories: %w", err)
		}
		if len(allRepos) == 0 {
			return ui.PrCheckoutOptions{}, fmt.Errorf("no repositories found in %s", repoDir)
		}

		for _, repoPath := range allRepos {
			params.AvailableRepos = append(params.AvailableRepos, repoPath)
			params.RepoNames = append(params.RepoNames, git.GetRepoDisplayName(repoPath))
		}
	}

	// PR fetcher function
	params.FetchPRs = func(repoPath string) ([]forge.OpenPR, error) {
		l.Printf("Fetching open PRs for %s...\n", git.GetRepoDisplayName(repoPath))

		// Get origin URL and detect forge
		originURL, err := git.GetOriginURL(ctx, repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get origin URL: %w", err)
		}

		f := forge.Detect(originURL, cfg.Hosts, &cfg.Forge)
		if err := f.Check(ctx); err != nil {
			return nil, err
		}

		return f.ListOpenPRs(ctx, originURL)
	}

	// Build available hooks info (skip if --hook or --no-hook was passed)
	params.HooksFromCLI = len(c.Hook) > 0 || c.NoHook
	if !params.HooksFromCLI {
		for name, hook := range cfg.Hooks.Hooks {
			isDefault := false
			for _, on := range hook.On {
				if on == "pr" || on == "all" {
					isDefault = true
					break
				}
			}
			params.AvailableHooks = append(params.AvailableHooks, ui.HookInfo{
				Name:        name,
				Description: hook.Description,
				IsDefault:   isDefault,
			})
		}
	}

	return ui.PrCheckoutInteractive(params)
}

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

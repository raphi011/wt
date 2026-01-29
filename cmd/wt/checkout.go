package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/ui"
)

// successResult holds the result of a successful worktree creation
type successResult struct {
	Path     string
	Branch   string
	RepoName string
	MainRepo string
	Folder   string
	Existed  bool
}

// createWorktreeParams holds parameters for worktree creation
type createWorktreeParams struct {
	repoPath  string // repo path (empty = use current dir with AddWorktree)
	targetDir string
	branch    string
	format    string
	newBranch bool
	baseRef   string // already resolved (only used if newBranch=true)
	note      string
	workDir   string // working directory for current repo mode
}

// createWorktree creates a worktree and returns the result.
// Uses repoPath if specified, otherwise uses workDir as the repo path.
// Operates using CreateWorktreeFrom/OpenWorktreeFrom for path-based operations.
func createWorktree(ctx context.Context, p createWorktreeParams) (*successResult, error) {
	l := log.FromContext(ctx)
	var result *git.CreateWorktreeResult
	var err error
	var repoName, folder, mainRepo string

	// Use workDir as repoPath when not explicitly specified
	effectiveRepoPath := p.repoPath
	if effectiveRepoPath == "" {
		effectiveRepoPath = p.workDir
	}

	// Use path-based functions for all operations
	if p.newBranch {
		result, err = git.CreateWorktreeFrom(ctx, effectiveRepoPath, p.targetDir, p.branch, p.format, p.baseRef)
	} else {
		result, err = git.OpenWorktreeFrom(ctx, effectiveRepoPath, p.targetDir, p.branch, p.format)
	}
	if err != nil {
		return nil, err
	}

	// Get repo metadata using path-based functions
	repoName, _ = git.GetRepoNameFrom(ctx, effectiveRepoPath)
	if repoName == "" {
		repoName = git.GetRepoDisplayName(effectiveRepoPath)
	}
	folder = filepath.Base(effectiveRepoPath)
	mainRepo, _ = git.GetMainRepoPath(result.Path)
	if mainRepo == "" {
		mainRepo, _ = filepath.Abs(effectiveRepoPath)
	}

	// Assign worktree ID and save to cache
	var id int
	wtCache, unlock, err := cache.LoadWithLock(p.targetDir)
	if err == nil {
		defer unlock()
		id = wtCache.GetOrAssignID(cache.WorktreeInfo{
			Path:     result.Path,
			RepoPath: mainRepo,
			Branch:   p.branch,
		})
		if saveErr := cache.Save(p.targetDir, wtCache); saveErr != nil {
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
	if p.note != "" {
		if err := git.SetBranchNote(ctx, effectiveRepoPath, p.branch, p.note); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set note: %v\n", err)
		}
	}

	return &successResult{
		Path:     result.Path,
		Branch:   p.branch,
		RepoName: repoName,
		MainRepo: mainRepo,
		Folder:   folder,
		Existed:  result.AlreadyExists,
	}, nil
}

// resolveBaseRef determines the base ref for creating a new branch.
// If fetch is true (or autoFetch config is true), fetches the base branch from origin first.
// Returns the full ref (e.g., "origin/main" or "main") based on config.
func resolveBaseRef(ctx context.Context, repoPath, baseBranch string, fetch, autoFetch bool, baseRefConfig string) (string, error) {
	l := log.FromContext(ctx)

	// Determine base branch (default: auto-detected main/master)
	if baseBranch == "" {
		baseBranch = git.GetDefaultBranch(ctx, repoPath)
	}

	// Check if remote exists
	hasRemote := git.HasRemote(ctx, repoPath, "origin")

	// Fetch if requested via flag or auto_fetch config
	shouldFetch := fetch || autoFetch
	if shouldFetch {
		if !hasRemote {
			l.Printf("Warning: no remote 'origin' found, skipping fetch\n")
			shouldFetch = false
		} else {
			l.Printf("Fetching origin/%s...\n", baseBranch)
			if err := git.FetchBranch(ctx, repoPath, baseBranch); err != nil {
				return "", err
			}
		}
	}

	// Determine whether to use remote or local ref
	// --fetch/auto_fetch always implies remote ref, otherwise use config
	// But fall back to local if no remote exists
	useRemote := (shouldFetch || baseRefConfig != "local") && hasRemote
	if useRemote {
		l.Printf("Branching from origin/%s\n", baseBranch)
		return "origin/" + baseBranch, nil
	}
	l.Printf("Branching from %s (local)\n", baseBranch)
	return baseBranch, nil
}

// fetchBranchesWithWorktreeInfo returns all branches with their worktree status
func fetchBranchesWithWorktreeInfo(ctx context.Context, repoPath string) ui.BranchFetchResult {
	branches, err := git.ListRemoteBranches(ctx, repoPath)
	if err != nil {
		branches, _ = git.ListLocalBranches(ctx, repoPath)
	}
	worktreeBranches := git.GetWorktreeBranches(ctx, repoPath)

	result := ui.BranchFetchResult{
		Branches: make([]ui.BranchInfo, len(branches)),
	}
	for i, b := range branches {
		result.Branches[i] = ui.BranchInfo{
			Name:       b,
			InWorktree: worktreeBranches[b],
		}
	}
	return result
}

func (c *CheckoutCmd) runCheckout(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir

	// Read stdin early if needed (before wizard takes control)
	if hooks.NeedsStdin(c.Env) {
		var err error
		c.stdinContent, err = hooks.ReadStdinIfPiped()
		if err != nil {
			return err
		}
		if c.stdinContent == "" {
			return fmt.Errorf("stdin not piped: KEY=- requires piped input")
		}
	}

	// Handle interactive mode
	if c.Interactive {
		insideRepo := git.IsInsideRepoPath(ctx, workDir)

		// Get repo scan directory
		repoDir := cfg.RepoScanDir()
		if repoDir == "" {
			repoDir = cfg.WorktreeDir
		}
		if repoDir == "" && insideRepo {
			repoDir = workDir
		}
		if repoDir == "" {
			return fmt.Errorf("directory required for interactive mode (set WT_WORKTREE_DIR or worktree_dir in config)")
		}

		// Build wizard params - always load all repos for selection
		params := ui.CheckoutWizardParams{}

		// Load all available repos
		allRepos, err := git.FindAllRepos(repoDir)
		if err != nil {
			return fmt.Errorf("failed to find repositories: %w", err)
		}
		if len(allRepos) == 0 && !insideRepo {
			return fmt.Errorf("no repositories found in %s", repoDir)
		}

		for _, repoPath := range allRepos {
			params.AvailableRepos = append(params.AvailableRepos, repoPath)
			params.RepoNames = append(params.RepoNames, git.GetRepoDisplayName(repoPath))
		}

		// If inside a repo, add it to the list if not already there and pre-select it
		if insideRepo {
			// Get the main repo path for the current directory
			currentRepoPath, _ := git.GetMainRepoPath(workDir)
			if currentRepoPath == "" {
				currentRepoPath = workDir
			}

			// Find or add current repo to the list
			currentIdx := -1
			for i, repoPath := range params.AvailableRepos {
				if repoPath == currentRepoPath {
					currentIdx = i
					break
				}
			}
			if currentIdx == -1 {
				// Current repo not in list, add it
				currentIdx = len(params.AvailableRepos)
				params.AvailableRepos = append(params.AvailableRepos, currentRepoPath)
				params.RepoNames = append(params.RepoNames, git.GetRepoDisplayName(currentRepoPath))
			}
			params.PreSelectedRepos = []int{currentIdx}

			// Get branches from current repo (with worktree info)
			branchResult := fetchBranchesWithWorktreeInfo(ctx, currentRepoPath)
			params.Branches = branchResult.Branches
		}

		// Handle -r or -l flags: pre-select those repos
		if len(c.Repository) > 0 || len(c.Label) > 0 {
			repoPaths, errs := collectRepoPaths(ctx, c.Repository, c.Label, repoDir)
			if len(errs) > 0 {
				return fmt.Errorf("failed to resolve repositories: %v", errs[0])
			}
			if len(repoPaths) == 0 {
				return fmt.Errorf("no matching repositories found")
			}

			// Find indices of specified repos and pre-select them
			params.PreSelectedRepos = nil
			for repoPath := range repoPaths {
				for i, availRepo := range params.AvailableRepos {
					if availRepo == repoPath {
						params.PreSelectedRepos = append(params.PreSelectedRepos, i)
						break
					}
				}
			}

			// Get branches from first pre-selected repo (with worktree info)
			if len(params.PreSelectedRepos) > 0 {
				firstRepo := params.AvailableRepos[params.PreSelectedRepos[0]]
				branchResult := fetchBranchesWithWorktreeInfo(ctx, firstRepo)
				params.Branches = branchResult.Branches
			}
		}

		// Provide callback to fetch branches when repo selection changes
		params.FetchBranches = func(repoPath string) ui.BranchFetchResult {
			return fetchBranchesWithWorktreeInfo(ctx, repoPath)
		}

		// Build available hooks info (skip if --hook or --no-hook was passed)
		params.HooksFromCLI = len(c.Hook) > 0 || c.NoHook
		if !params.HooksFromCLI {
			for name, hook := range cfg.Hooks.Hooks {
				isDefault := false
				for _, on := range hook.On {
					if on == "checkout" || on == "all" {
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

		opts, err := ui.CheckoutInteractive(params)
		if err != nil {
			return fmt.Errorf("interactive mode error: %w", err)
		}
		if opts.Cancelled {
			return nil
		}

		// Apply gathered options
		c.Branch = opts.Branch
		c.NewBranch = opts.NewBranch
		c.Fetch = opts.Fetch

		// If repos were selected via wizard, set them as target repos
		if len(opts.SelectedRepos) > 0 {
			// Convert paths to repo names for the -r flag
			var repoNames []string
			for _, repoPath := range opts.SelectedRepos {
				repoNames = append(repoNames, git.GetRepoDisplayName(repoPath))
			}
			c.Repository = repoNames
		}

		// Apply hook selection from wizard (only if not set via CLI)
		if !params.HooksFromCLI {
			if opts.NoHook {
				c.NoHook = true
			} else {
				c.Hook = opts.SelectedHooks
			}
		}
	}

	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	insideRepo := git.IsInsideRepoPath(ctx, workDir)

	// If -r or -l specified, use multi-repo mode
	if len(c.Repository) > 0 || len(c.Label) > 0 {
		return c.runCheckoutMultiRepo(ctx, insideRepo)
	}

	// No -r or -l specified
	if insideRepo {
		if c.Branch == "" {
			return fmt.Errorf("branch name required inside git repo")
		}
		return c.runCheckoutInRepo(ctx)
	}

	// Outside repo without -r or -l
	return fmt.Errorf("--repository (-r) or --label (-l) required when outside git repo")
}

// runCheckoutInRepo handles wt checkout when inside a git repository (single repo mode)
func (c *CheckoutCmd) runCheckoutInRepo(ctx context.Context) error {
	l := log.FromContext(ctx)
	cfg := c.Config
	workDir := c.WorkDir
	targetDir, absPath, err := resolveWorktreeTargetDir(cfg.WorktreeDir, workDir)
	if err != nil {
		return err
	}

	// Handle autostash: stash uncommitted changes before creating worktree
	var didStash bool
	if c.AutoStash && git.IsDirty(ctx, workDir) {
		l.Printf("Stashing uncommitted changes...\n")
		if err := git.Stash(ctx, workDir); err != nil {
			return err
		}
		didStash = true
	}

	// Resolve base ref if creating new branch
	var baseRef string
	if c.NewBranch {
		baseRef, err = resolveBaseRef(ctx, workDir, c.Base, c.Fetch, cfg.AutoFetch, cfg.BaseRef)
		if err != nil {
			return err
		}
		l.Printf("Creating worktree for new branch %s in %s\n", c.Branch, absPath)
	} else {
		l.Printf("Adding worktree for branch %s in %s\n", c.Branch, absPath)
	}

	result, err := createWorktree(ctx, createWorktreeParams{
		repoPath:  "",
		targetDir: targetDir,
		branch:    c.Branch,
		format:    cfg.WorktreeFormat,
		newBranch: c.NewBranch,
		baseRef:   baseRef,
		note:      c.Note,
		workDir:   workDir,
	})
	if err != nil {
		if didStash {
			l.Printf("Warning: worktree creation failed but changes are stashed. Run 'git stash pop' to restore.\n")
		}
		return err
	}

	// Apply stashed changes to the new worktree
	if didStash {
		l.Printf("Applying stashed changes to new worktree...\n")
		if err := git.StashPop(ctx, result.Path); err != nil {
			l.Printf("Warning: failed to apply stash to new worktree: %v\n", err)
			l.Printf("Your changes are still in the stash. Run 'git stash pop' in %s to apply manually.\n", workDir)
		}
	}

	// Run post-add hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, c.Hook, c.NoHook, hooks.CommandCheckout)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithCachedStdin(c.Env, c.stdinContent)
	if err != nil {
		return err
	}

	hookCtx := hooks.Context{
		WorktreeDir: result.Path,
		RepoDir:     result.MainRepo,
		Branch:      result.Branch,
		Repo:        result.Folder,
		Origin:      result.RepoName,
		Trigger:     string(hooks.CommandCheckout),
		Env:         env,
	}

	return hooks.RunAll(hookMatches, hookCtx)
}

// runCheckoutMultiRepo handles wt checkout with -r or -l flags for multiple repositories
func (c *CheckoutCmd) runCheckoutMultiRepo(ctx context.Context, insideRepo bool) error {
	cfg := c.Config
	if c.Branch == "" {
		return fmt.Errorf("branch name required with --repository or --label")
	}

	// Worktree target dir from config
	wtDir := cfg.WorktreeDir
	if wtDir == "" {
		if !insideRepo {
			return fmt.Errorf("directory required when outside git repo (set WT_WORKTREE_DIR or worktree_dir in config)")
		}
		wtDir = "."
	}

	var err error
	wtDir, err = filepath.Abs(wtDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Repo dir (from config, fallback to wtDir)
	repoDir := cfg.RepoScanDir()
	if repoDir == "" {
		repoDir = wtDir
	}

	var results []successResult
	var errs []error

	// Collect repo paths: from -r (by name) and -l (by label)
	repoPaths, collectErrs := collectRepoPaths(ctx, c.Repository, c.Label, repoDir)
	errs = append(errs, collectErrs...)

	// Process each unique repository
	for repoPath := range repoPaths {
		result, err := c.createWorktreeForRepoOrCurrent(ctx, repoPath, wtDir)
		repoName := git.GetRepoDisplayName(repoPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
		} else {
			results = append(results, *result)
		}
	}

	// Run hooks for each successful creation
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, c.Hook, c.NoHook, hooks.CommandCheckout)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithCachedStdin(c.Env, c.stdinContent)
	if err != nil {
		return err
	}

	for _, r := range results {
		hookCtx := hooks.Context{
			WorktreeDir: r.Path,
			RepoDir:     r.MainRepo,
			Branch:      r.Branch,
			Repo:        r.Folder,
			Origin:      r.RepoName,
			Trigger:     string(hooks.CommandCheckout),
			Env:         env,
		}
		hooks.RunForEach(hookMatches, hookCtx, r.Path)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add worktrees:\n%w", errors.Join(errs...))
	}
	return nil
}

// createWorktreeForRepoOrCurrent creates a worktree for the specified repository.
// If repoPath is empty, uses the current working directory (current repo mode).
func (c *CheckoutCmd) createWorktreeForRepoOrCurrent(ctx context.Context, repoPath, targetDir string) (*successResult, error) {
	l := log.FromContext(ctx)
	cfg := c.Config

	// Determine effective repo path and display name
	effectiveRepoPath := repoPath
	if effectiveRepoPath == "" {
		effectiveRepoPath = c.WorkDir
	}

	repoLabel := "current repo"
	if repoPath != "" {
		repoLabel = git.GetRepoDisplayName(repoPath)
	}

	// Resolve base ref if creating new branch
	var baseRef string
	if c.NewBranch {
		var err error
		baseRef, err = resolveBaseRef(ctx, effectiveRepoPath, c.Base, c.Fetch, cfg.AutoFetch, cfg.BaseRef)
		if err != nil {
			return nil, err
		}
		l.Printf("Creating worktree for new branch %s in %s (%s)\n", c.Branch, targetDir, repoLabel)
	} else {
		l.Printf("Adding worktree for branch %s in %s (%s)\n", c.Branch, targetDir, repoLabel)
	}

	return createWorktree(ctx, createWorktreeParams{
		repoPath:  repoPath,
		targetDir: targetDir,
		branch:    c.Branch,
		format:    cfg.WorktreeFormat,
		newBranch: c.NewBranch,
		baseRef:   baseRef,
		note:      c.Note,
		workDir:   c.WorkDir,
	})
}

// resolveWorktreeTargetDir returns both the target dir and its absolute path.
// If worktreeDir is empty, uses workDir as the target.
func resolveWorktreeTargetDir(worktreeDir, workDir string) (targetDir string, absPath string, err error) {
	targetDir = worktreeDir
	if targetDir == "" {
		targetDir = workDir
	}
	if !filepath.IsAbs(targetDir) {
		absPath = filepath.Join(workDir, targetDir)
	} else {
		absPath = targetDir
	}
	// Clean the path to normalize it
	absPath = filepath.Clean(absPath)
	return targetDir, absPath, nil
}

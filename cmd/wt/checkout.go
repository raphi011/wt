package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
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

	// Print result
	if result.AlreadyExists {
		l.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		l.Printf("✓ Worktree created at: %s\n", result.Path)
	}

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
// If fetch is true, fetches the base branch from origin first.
// Returns the full ref (e.g., "origin/main" or "main") based on config.
func resolveBaseRef(ctx context.Context, repoPath, baseBranch string, fetch bool, baseRefConfig string) (string, error) {
	l := log.FromContext(ctx)

	// Determine base branch (default: auto-detected main/master)
	if baseBranch == "" {
		baseBranch = git.GetDefaultBranch(ctx, repoPath)
	}

	// Fetch if requested
	if fetch {
		l.Printf("Fetching origin/%s...\n", baseBranch)
		if err := git.FetchBranch(ctx, repoPath, baseBranch); err != nil {
			return "", err
		}
	}

	// Determine whether to use remote or local ref
	// --fetch always implies remote ref, otherwise use config
	if fetch || baseRefConfig != "local" {
		return "origin/" + baseBranch, nil
	}
	return baseBranch, nil
}

func (c *CheckoutCmd) runCheckout(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir

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

	// Resolve base ref if creating new branch
	var baseRef string
	if c.NewBranch {
		baseRef, err = resolveBaseRef(ctx, workDir, c.Base, c.Fetch, cfg.BaseRef)
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
		return err
	}

	// Run post-add hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, c.Hook, c.NoHook, hooks.CommandCheckout)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(c.Env)
	if err != nil {
		return err
	}

	hookCtx := hooks.Context{
		Path:     result.Path,
		Branch:   result.Branch,
		Repo:     result.RepoName,
		Folder:   result.Folder,
		MainRepo: result.MainRepo,
		Trigger:  string(hooks.CommandCheckout),
		Env:      env,
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

	// If inside repo with -r flag, include current repo first (original behavior)
	// With -l only, we don't auto-include current repo (only labeled repos)
	if insideRepo && len(c.Repository) > 0 {
		result, err := c.createWorktreeInCurrentRepo(ctx, wtDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("(current repo): %w", err))
		} else {
			results = append(results, *result)
		}
	}

	// Collect repo paths: from -r (by name) and -l (by label)
	repoPaths := make(map[string]bool) // dedupe by path

	// Process -r flags (repository names)
	for _, repoName := range c.Repository {
		repoPath, err := git.FindRepoByName(repoDir, repoName)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
			continue
		}
		repoPaths[repoPath] = true
	}

	// Process -l flags (labels)
	for _, label := range c.Label {
		paths, err := git.FindReposByLabel(ctx, repoDir, label)
		if err != nil {
			errs = append(errs, fmt.Errorf("label %q: %w", label, err))
			continue
		}
		if len(paths) == 0 {
			errs = append(errs, fmt.Errorf("label %q: no repos found with this label", label))
			continue
		}
		for _, p := range paths {
			repoPaths[p] = true
		}
	}

	// Process each unique repository
	for repoPath := range repoPaths {
		result, err := c.createWorktreeForRepo(ctx, repoPath, wtDir)
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

	env, err := hooks.ParseEnvWithStdin(c.Env)
	if err != nil {
		return err
	}

	for _, r := range results {
		hookCtx := hooks.Context{
			Path:     r.Path,
			Branch:   r.Branch,
			Repo:     r.RepoName,
			Folder:   r.Folder,
			MainRepo: r.MainRepo,
			Trigger:  string(hooks.CommandCheckout),
			Env:      env,
		}
		hooks.RunForEach(hookMatches, hookCtx, r.Path)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add worktrees:\n%w", errors.Join(errs...))
	}
	return nil
}

// createWorktreeInCurrentRepo creates a worktree in the current git repository
func (c *CheckoutCmd) createWorktreeInCurrentRepo(ctx context.Context, targetDir string) (*successResult, error) {
	l := log.FromContext(ctx)
	cfg := c.Config
	workDir := c.WorkDir
	// Resolve base ref if creating new branch
	var baseRef string
	if c.NewBranch {
		var err error
		baseRef, err = resolveBaseRef(ctx, workDir, c.Base, c.Fetch, cfg.BaseRef)
		if err != nil {
			return nil, err
		}
		l.Printf("Creating worktree for new branch %s (current repo) in %s\n", c.Branch, targetDir)
	} else {
		l.Printf("Adding worktree for branch %s (current repo) in %s\n", c.Branch, targetDir)
	}

	return createWorktree(ctx, createWorktreeParams{
		repoPath:  "",
		targetDir: targetDir,
		branch:    c.Branch,
		format:    cfg.WorktreeFormat,
		newBranch: c.NewBranch,
		baseRef:   baseRef,
		note:      c.Note,
		workDir:   workDir,
	})
}

// createWorktreeForRepo creates a worktree for a specified repository
func (c *CheckoutCmd) createWorktreeForRepo(ctx context.Context, repoPath string, targetDir string) (*successResult, error) {
	l := log.FromContext(ctx)
	cfg := c.Config
	repoName := git.GetRepoDisplayName(repoPath)

	// Resolve base ref if creating new branch
	var baseRef string
	if c.NewBranch {
		var err error
		baseRef, err = resolveBaseRef(ctx, repoPath, c.Base, c.Fetch, cfg.BaseRef)
		if err != nil {
			return nil, err
		}
		l.Printf("Creating worktree for new branch %s in %s (%s)\n", c.Branch, targetDir, repoName)
	} else {
		l.Printf("Adding worktree for branch %s in %s (%s)\n", c.Branch, targetDir, repoName)
	}

	return createWorktree(ctx, createWorktreeParams{
		repoPath:  repoPath,
		targetDir: targetDir,
		branch:    c.Branch,
		format:    cfg.WorktreeFormat,
		newBranch: c.NewBranch,
		baseRef:   baseRef,
		note:      c.Note,
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

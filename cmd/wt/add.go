package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
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

// resolveBaseRef determines the base ref for creating a new branch.
// If fetch is true, fetches the base branch from origin first.
// Returns the full ref (e.g., "origin/main" or "main") based on config.
func resolveBaseRef(repoPath, baseBranch string, fetch bool, baseRefConfig string) (string, error) {
	// Determine base branch (default: auto-detected main/master)
	if baseBranch == "" {
		baseBranch = git.GetDefaultBranch(repoPath)
	}

	// Fetch if requested
	if fetch {
		fmt.Printf("Fetching origin/%s...\n", baseBranch)
		if err := git.FetchBranch(repoPath, baseBranch); err != nil {
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

func runAdd(cmd *AddCmd, cfg *config.Config) error {
	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	insideRepo := git.IsInsideRepo()

	// If -r or -l specified, use multi-repo mode
	if len(cmd.Repository) > 0 || len(cmd.Label) > 0 {
		return runAddMultiRepo(cmd, cfg, insideRepo)
	}

	// No -r or -l specified
	if insideRepo {
		if cmd.Branch == "" {
			return fmt.Errorf("branch name required inside git repo")
		}
		return runAddInRepo(cmd, cfg)
	}

	// Outside repo without -r or -l
	return fmt.Errorf("--repository (-r) or --label (-l) required when outside git repo")
}

// runAddInRepo handles wt add when inside a git repository (single repo mode)
func runAddInRepo(cmd *AddCmd, cfg *config.Config) error {
	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	var result *git.CreateWorktreeResult

	if cmd.NewBranch {
		// Resolve base branch and ref for new branches
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		baseRef, err := resolveBaseRef(cwd, cmd.Base, cmd.Fetch, cfg.BaseRef)
		if err != nil {
			return err
		}

		fmt.Printf("Creating worktree for new branch %s in %s\n", cmd.Branch, absPath)
		result, err = git.AddWorktree(dir, cmd.Branch, cfg.WorktreeFormat, true, baseRef)
	} else {
		fmt.Printf("Adding worktree for branch %s in %s\n", cmd.Branch, absPath)
		result, err = git.AddWorktree(dir, cmd.Branch, cfg.WorktreeFormat, false, "")
	}

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
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		if err := git.SetBranchNote(cwd, cmd.Branch, cmd.Note); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set note: %v\n", err)
		}
	}

	// Run post-add hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandAdd)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(cmd.Env)
	if err != nil {
		return err
	}

	ctx := hooks.Context{
		Path:    result.Path,
		Branch:  cmd.Branch,
		Trigger: string(hooks.CommandAdd),
		Env:     env,
	}
	ctx.Repo, _ = git.GetRepoName()
	ctx.Folder, _ = git.GetRepoFolderName()
	ctx.MainRepo, _ = git.GetMainRepoPath(result.Path)
	if ctx.MainRepo == "" {
		ctx.MainRepo, _ = filepath.Abs(".")
	}

	return hooks.RunAll(hookMatches, ctx)
}

// runAddMultiRepo handles wt add with -r or -l flags for multiple repositories
func runAddMultiRepo(cmd *AddCmd, cfg *config.Config, insideRepo bool) error {
	if cmd.Branch == "" {
		return fmt.Errorf("branch name required with --repository or --label")
	}

	// Worktree target dir (from -d flag / config)
	wtDir := cmd.Dir
	if wtDir == "" {
		if !insideRepo {
			return fmt.Errorf("directory required when outside git repo (-d flag or WT_WORKTREE_DIR)")
		}
		wtDir = "."
	}

	var err error
	wtDir, err = filepath.Abs(wtDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Repo scan dir (from config, fallback to wtDir)
	repoScanDir := cfg.RepoScanDir()
	if repoScanDir == "" {
		repoScanDir = wtDir
	}

	var results []successResult
	var errs []error

	// If inside repo with -r flag, include current repo first (original behavior)
	// With -l only, we don't auto-include current repo (only labeled repos)
	if insideRepo && len(cmd.Repository) > 0 {
		result, err := createWorktreeInCurrentRepo(cmd, cfg, wtDir)
		if err != nil {
			errs = append(errs, fmt.Errorf("(current repo): %w", err))
		} else {
			results = append(results, *result)
		}
	}

	// Collect repo paths: from -r (by name) and -l (by label)
	repoPaths := make(map[string]bool) // dedupe by path

	// Process -r flags (repository names)
	for _, repoName := range cmd.Repository {
		repoPath, err := git.FindRepoByName(repoScanDir, repoName)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
			continue
		}
		repoPaths[repoPath] = true
	}

	// Process -l flags (labels)
	for _, label := range cmd.Label {
		paths, err := git.FindReposByLabel(repoScanDir, label)
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
		result, err := createWorktreeForRepo(repoPath, cmd, cfg, wtDir)
		repoName, _ := git.GetRepoNameFrom(repoPath)
		if repoName == "" {
			repoName = filepath.Base(repoPath)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
		} else {
			results = append(results, *result)
		}
	}

	// Run hooks for each successful creation
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandAdd)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(cmd.Env)
	if err != nil {
		return err
	}

	for _, r := range results {
		ctx := hooks.Context{
			Path:     r.Path,
			Branch:   r.Branch,
			Repo:     r.RepoName,
			Folder:   r.Folder,
			MainRepo: r.MainRepo,
			Trigger:  string(hooks.CommandAdd),
			Env:      env,
		}
		hooks.RunForEach(hookMatches, ctx, r.Path)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add worktrees:\n%w", joinErrors(errs))
	}
	return nil
}

// createWorktreeInCurrentRepo creates a worktree in the current git repository
func createWorktreeInCurrentRepo(cmd *AddCmd, cfg *config.Config, targetDir string) (*successResult, error) {
	var result *git.CreateWorktreeResult
	var err error

	if cmd.NewBranch {
		// Resolve base branch and ref for new branches
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
		baseRef, err := resolveBaseRef(cwd, cmd.Base, cmd.Fetch, cfg.BaseRef)
		if err != nil {
			return nil, err
		}

		fmt.Printf("Creating worktree for new branch %s (current repo) in %s\n", cmd.Branch, targetDir)
		result, err = git.AddWorktree(targetDir, cmd.Branch, cfg.WorktreeFormat, true, baseRef)
	} else {
		fmt.Printf("Adding worktree for branch %s (current repo) in %s\n", cmd.Branch, targetDir)
		result, err = git.AddWorktree(targetDir, cmd.Branch, cfg.WorktreeFormat, false, "")
	}

	if err != nil {
		return nil, err
	}

	if result.AlreadyExists {
		fmt.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		fmt.Printf("✓ Worktree created at: %s\n", result.Path)
	}

	// Set note if provided
	if cmd.Note != "" {
		cwd, err := os.Getwd()
		if err == nil {
			if err := git.SetBranchNote(cwd, cmd.Branch, cmd.Note); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to set note for current repo: %v\n", err)
			}
		}
	}

	repoName, _ := git.GetRepoName()
	folder, _ := git.GetRepoFolderName()
	mainRepo, _ := git.GetMainRepoPath(result.Path)
	if mainRepo == "" {
		mainRepo, _ = filepath.Abs(".")
	}

	return &successResult{
		Path:     result.Path,
		Branch:   cmd.Branch,
		RepoName: repoName,
		MainRepo: mainRepo,
		Folder:   folder,
		Existed:  result.AlreadyExists,
	}, nil
}

// createWorktreeForRepo creates a worktree for a specified repository
func createWorktreeForRepo(repoPath string, cmd *AddCmd, cfg *config.Config, targetDir string) (*successResult, error) {
	var result *git.CreateWorktreeResult
	var err error

	repoName, _ := git.GetRepoNameFrom(repoPath)
	folder := filepath.Base(repoPath)

	if cmd.NewBranch {
		// Resolve base branch and ref for new branches
		baseRef, err := resolveBaseRef(repoPath, cmd.Base, cmd.Fetch, cfg.BaseRef)
		if err != nil {
			return nil, err
		}

		fmt.Printf("Creating worktree for new branch %s in %s (%s)\n", cmd.Branch, targetDir, repoName)
		result, err = git.CreateWorktreeFrom(repoPath, targetDir, cmd.Branch, cfg.WorktreeFormat, baseRef)
	} else {
		fmt.Printf("Adding worktree for branch %s in %s (%s)\n", cmd.Branch, targetDir, repoName)
		result, err = git.OpenWorktreeFrom(repoPath, targetDir, cmd.Branch, cfg.WorktreeFormat)
	}

	if err != nil {
		return nil, err
	}

	if result.AlreadyExists {
		fmt.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		fmt.Printf("✓ Worktree created at: %s\n", result.Path)
	}

	// Set note if provided
	if cmd.Note != "" {
		if err := git.SetBranchNote(repoPath, cmd.Branch, cmd.Note); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set note for %s: %v\n", repoName, err)
		}
	}

	return &successResult{
		Path:     result.Path,
		Branch:   cmd.Branch,
		RepoName: repoName,
		MainRepo: repoPath,
		Folder:   folder,
		Existed:  result.AlreadyExists,
	}, nil
}

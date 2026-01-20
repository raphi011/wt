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

func runAdd(cmd *AddCmd, cfg *config.Config) error {
	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	insideRepo := git.IsInsideRepo()

	// If -r specified, use multi-repo mode
	if len(cmd.Repository) > 0 {
		return runAddMultiRepo(cmd, cfg, insideRepo)
	}

	// No -r specified
	if insideRepo {
		if cmd.Branch == "" {
			return fmt.Errorf("branch name required inside git repo")
		}
		return runAddInRepo(cmd, cfg)
	}

	// Outside repo without -r
	return fmt.Errorf("--repository (-r) required when outside git repo")
}

// runAddInRepo handles wt add when inside a git repository (single repo mode)
func runAddInRepo(cmd *AddCmd, cfg *config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

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
		fmt.Printf("Creating worktree for new branch %s in %s\n", cmd.Branch, absPath)
		result, err = git.AddWorktree(dir, cmd.Branch, cfg.WorktreeFormat, true)
	} else {
		fmt.Printf("Adding worktree for branch %s in %s\n", cmd.Branch, absPath)
		result, err = git.AddWorktree(dir, cmd.Branch, cfg.WorktreeFormat, false)
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

// runAddMultiRepo handles wt add with -r flag for multiple repositories
func runAddMultiRepo(cmd *AddCmd, cfg *config.Config, insideRepo bool) error {
	if cmd.Branch == "" {
		return fmt.Errorf("branch name required with --repository")
	}

	// Determine scan directory
	scanDir := cmd.Dir
	if scanDir == "" {
		if !insideRepo {
			return fmt.Errorf("directory required when outside git repo (-d flag or WT_DEFAULT_PATH)")
		}
		scanDir = "."
	}

	var err error
	scanDir, err = filepath.Abs(scanDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	var results []successResult
	var errs []error

	// If inside repo, include current repo first
	if insideRepo {
		if err := git.CheckGit(); err != nil {
			errs = append(errs, fmt.Errorf("(current repo): %w", err))
		} else {
			result, err := createWorktreeInCurrentRepo(cmd, cfg, scanDir)
			if err != nil {
				errs = append(errs, fmt.Errorf("(current repo): %w", err))
			} else {
				results = append(results, *result)
			}
		}
	}

	// Process each specified repository
	for _, repoName := range cmd.Repository {
		repoPath, err := git.FindRepoByName(scanDir, repoName)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
			continue
		}

		result, err := createWorktreeForRepo(repoPath, cmd, cfg, scanDir)
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
		fmt.Printf("Creating worktree for new branch %s (current repo) in %s\n", cmd.Branch, targetDir)
		result, err = git.AddWorktree(targetDir, cmd.Branch, cfg.WorktreeFormat, true)
	} else {
		fmt.Printf("Adding worktree for branch %s (current repo) in %s\n", cmd.Branch, targetDir)
		result, err = git.AddWorktree(targetDir, cmd.Branch, cfg.WorktreeFormat, false)
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
		fmt.Printf("Creating worktree for new branch %s in %s (%s)\n", cmd.Branch, targetDir, repoName)
		result, err = git.CreateWorktreeFrom(repoPath, targetDir, cmd.Branch, cfg.WorktreeFormat)
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


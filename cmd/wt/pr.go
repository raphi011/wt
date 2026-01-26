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
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/resolve"
)

func (c *PrCheckoutCmd) runPrCheckout(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	// Validate mutual exclusion: positional org/repo and -r flag can't both be used
	if c.Repo != "" && c.Repository != "" {
		return fmt.Errorf("cannot use both positional org/repo argument and -r/--repository flag\nUse 'wt pr checkout %d %s' (clone mode) OR 'wt pr checkout %d -r %s' (local mode)", c.Number, c.Repo, c.Number, c.Repository)
	}

	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	// Worktree target dir from config
	dir, absPath, err := resolveWorktreeTargetDir(cfg, workDir)
	if err != nil {
		return err
	}

	// Repo scan dir (from config, fallback to dir)
	repoScanDir := cfg.RepoScanDir()
	if repoScanDir == "" {
		repoScanDir = dir
	}

	var repoPath string
	var f forge.Forge

	// Clone mode: positional Repo arg with org/repo format
	if c.Repo != "" {
		// Validate org/repo format (must contain /)
		if !strings.Contains(c.Repo, "/") {
			return fmt.Errorf("clone mode requires org/repo format (e.g., 'org/repo'), got %q\nTo use a local repo by name, use: wt pr checkout %d -r %s", c.Repo, c.Number, c.Repo)
		}

		// Parse org/repo - use clone.org if org not specified
		org, name := git.ParseRepoArg(c.Repo)
		if org == "" {
			if cfg.Clone.Org == "" {
				return fmt.Errorf("repository must be in org/repo format, or configure [clone] org in config")
			}
			org = cfg.Clone.Org
		}
		repoSpec := org + "/" + name

		// Check if repo already exists locally
		if existingPath, err := git.FindRepoByName(repoScanDir, name); err == nil {
			return fmt.Errorf("repository %q already exists at %s\nUse 'wt pr checkout %d -r %s' instead", name, existingPath, c.Number, name)
		}

		// Determine forge: c.Forge > cfg.Clone rules > cfg.Clone.Forge
		forgeName := c.Forge
		if forgeName == "" {
			forgeName = cfg.Clone.GetForgeForRepo(repoSpec)
		}

		f = forge.ByName(forgeName)
		if err := f.Check(ctx); err != nil {
			return err
		}

		// Clone the repo to repoScanDir (repo_dir if set, else worktree_dir)
		fmt.Printf("Cloning %s to %s (using %s)...\n", repoSpec, repoScanDir, forgeName)
		repoPath, err = f.CloneRepo(ctx, repoSpec, repoScanDir)
		if err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}
		fmt.Printf("✓ Cloned to %s\n", repoPath)
	} else {
		// Local mode: -r flag or current directory
		if c.Repository != "" {
			// -r flag: find repo locally by name
			foundPath, err := git.FindRepoByName(repoScanDir, c.Repository)
			if err != nil {
				similar := git.FindSimilarRepos(repoScanDir, c.Repository)
				if len(similar) > 0 {
					return fmt.Errorf("repository %q not found in %s\nDid you mean: %s", c.Repository, repoScanDir, strings.Join(similar, ", "))
				}
				return fmt.Errorf("repository %q not found in %s", c.Repository, repoScanDir)
			}
			repoPath = foundPath
			fmt.Printf("Using repo at %s\n", repoPath)
		} else {
			// No args: use current directory
			repoPath = "."
		}

		// Get origin URL and detect forge
		originURL, err := git.GetOriginURL(ctx, repoPath)
		if err != nil {
			return fmt.Errorf("failed to get origin URL: %w", err)
		}

		f = forge.Detect(originURL, cfg.Hosts)
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
		fmt.Printf("Fetching PR #%d...\n", c.Number)
		branch, err = f.GetPRBranch(ctx, originURL, c.Number)
		if err != nil {
			return fmt.Errorf("failed to get PR branch: %w", err)
		}
	}

	fmt.Printf("Creating worktree for branch %s in %s\n", branch, absPath)

	result, err := git.CreateWorktreeFrom(ctx, repoPath, dir, branch, cfg.WorktreeFormat, "")
	if err != nil {
		return err
	}

	if result.AlreadyExists {
		fmt.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		fmt.Printf("✓ Worktree created at: %s\n", result.Path)
	}

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
	cfg := c.Config
	workDir := c.WorkDir
	target, err := resolvePrTarget(ctx, c.ID, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	// Check if we're operating from main repo (not a worktree)
	isMainRepo := target.Path == target.MainRepo

	// Get origin URL and detect forge
	originURL, err := git.GetOriginURL(ctx, target.MainRepo)
	if err != nil {
		return fmt.Errorf("failed to get origin URL: %w", err)
	}
	f := forge.Detect(originURL, cfg.Hosts)
	if err := f.Check(ctx); err != nil {
		return err
	}

	// Get PR for current branch
	fmt.Printf("Checking PR status for branch %s...\n", target.Branch)
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
	fmt.Printf("Merging PR #%d (%s)...\n", pr.Number, strategy)
	if err := f.MergePR(ctx, originURL, pr.Number, strategy); err != nil {
		return err
	}
	fmt.Printf("✓ PR #%d merged\n", pr.Number)

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

			fmt.Printf("Removing worktree...\n")
			if err := git.RemoveWorktree(ctx, wt, true); err != nil {
				return fmt.Errorf("failed to remove worktree: %w", err)
			}
			git.PruneWorktrees(ctx, target.MainRepo)
		}

		fmt.Printf("Deleting local branch %s...\n", target.Branch)
		if err := git.DeleteLocalBranch(ctx, target.MainRepo, target.Branch, true); err != nil {
			// Don't fail - branch might already be gone or worktree removal handled it
			fmt.Fprintf(os.Stderr, "Warning: failed to delete local branch: %v\n", err)
		}

		fmt.Printf("✓ Cleanup complete\n")
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
	cfg := c.Config
	workDir := c.WorkDir
	target, err := resolvePrTarget(ctx, c.ID, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	// Get origin URL and detect forge
	originURL, err := git.GetOriginURL(ctx, target.MainRepo)
	if err != nil {
		return fmt.Errorf("failed to get origin URL: %w", err)
	}
	f := forge.Detect(originURL, cfg.Hosts)
	if err := f.Check(ctx); err != nil {
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
	fmt.Printf("Creating PR for branch %s...\n", target.Branch)
	result, err := f.CreatePR(ctx, originURL, params)
	if err != nil {
		return err
	}

	fmt.Printf("✓ PR #%d created: %s\n", result.Number, result.URL)

	// Update cache with PR info
	scanDir := cfg.WorktreeDir
	if scanDir == "" {
		scanDir = filepath.Dir(target.Path)
	}
	scanDir, _ = filepath.Abs(scanDir)

	wtCache, err := cache.Load(scanDir)
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
		if err := cache.Save(scanDir, wtCache); err != nil {
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
	workDir := c.WorkDir
	target, err := resolvePrTarget(ctx, c.ID, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	// Get origin URL and detect forge
	originURL, err := git.GetOriginURL(ctx, target.MainRepo)
	if err != nil {
		return fmt.Errorf("failed to get origin URL: %w", err)
	}
	f := forge.Detect(originURL, cfg.Hosts)
	if err := f.Check(ctx); err != nil {
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

// resolvePrTarget resolves target for pr commands with 3 modes:
// 1. --id: by worktree ID
// 2. -r: by repository name
// 3. neither: use workDir (worktree or main repo)
func resolvePrTarget(ctx context.Context, id int, repository string, cfg *config.Config, workDir string) (*resolve.Target, error) {
	if id != 0 {
		worktreeDir, err := cfg.GetAbsWorktreeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		return resolve.ByID(id, worktreeDir)
	}

	if repository != "" {
		repoScanDir := cfg.RepoScanDir()
		if repoScanDir == "" {
			var err error
			repoScanDir, err = cfg.GetAbsWorktreeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
			}
		}
		return resolve.ByRepoName(ctx, repository, repoScanDir)
	}

	return resolve.FromWorktreeOrRepoPath(ctx, workDir)
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

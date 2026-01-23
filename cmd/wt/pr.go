package main

import (
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

func runPrOpen(cmd *PrOpenCmd, cfg *config.Config) error {
	// Validate worktree format
	if err := format.ValidateFormat(cfg.WorktreeFormat); err != nil {
		return fmt.Errorf("invalid worktree_format in config: %w", err)
	}

	// Worktree target dir (from -d flag / config)
	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	// Repo scan dir (from config, fallback to dir)
	repoScanDir := cfg.RepoScanDir()
	if repoScanDir == "" {
		repoScanDir = dir
	}

	// Determine which repo to use (local only - never clones)
	var repoPath string
	if cmd.Repo == "" {
		// No repo arg: use current directory
		repoPath = "."
	} else {
		// Repo arg provided: find locally only
		_, name := git.ParseRepoArg(cmd.Repo)

		// Try to find repo locally (search in repoScanDir)
		foundPath, err := git.FindRepoByName(repoScanDir, name)
		if err == nil {
			repoPath = foundPath
			fmt.Printf("Using repo at %s\n", repoPath)
		} else {
			// Not found: error with suggestions and hint to use pr clone
			similar := git.FindSimilarRepos(repoScanDir, name)
			if len(similar) > 0 {
				return fmt.Errorf("repository %q not found in %s\nDid you mean: %s\nTo clone a new repo, use: wt pr clone %d %s", name, repoScanDir, strings.Join(similar, ", "), cmd.Number, cmd.Repo)
			}
			return fmt.Errorf("repository %q not found in %s\nTo clone a new repo, use: wt pr clone %d %s", name, repoScanDir, cmd.Number, cmd.Repo)
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

	// Try cache first to avoid network request
	var branch string
	wtCache, err := cache.Load(dir)
	if err == nil {
		branch = wtCache.GetBranchByPRNumber(originURL, cmd.Number)
	}

	if branch == "" {
		// Cache miss - fetch from network
		fmt.Printf("Fetching PR #%d...\n", cmd.Number)
		branch, err = f.GetPRBranch(originURL, cmd.Number)
		if err != nil {
			return fmt.Errorf("failed to get PR branch: %w", err)
		}
	}

	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fmt.Printf("Creating worktree for branch %s in %s\n", branch, absPath)

	result, err := git.CreateWorktreeFrom(repoPath, dir, branch, cfg.WorktreeFormat, "")
	if err != nil {
		return err
	}

	if result.AlreadyExists {
		fmt.Printf("→ Worktree already exists at: %s\n", result.Path)
	} else {
		fmt.Printf("✓ Worktree created at: %s\n", result.Path)
	}

	// Run post-add hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandPR)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(cmd.Env)
	if err != nil {
		return err
	}

	ctx := hooks.Context{
		Path:    result.Path,
		Branch:  branch,
		Folder:  filepath.Base(repoPath),
		Trigger: string(hooks.CommandPR),
		Env:     env,
	}
	ctx.Repo, _ = git.GetRepoNameFrom(repoPath)
	ctx.MainRepo, _ = git.GetMainRepoPath(result.Path)
	if ctx.MainRepo == "" {
		ctx.MainRepo, _ = filepath.Abs(repoPath)
	}

	return hooks.RunAll(hookMatches, ctx)
}

func runPrClone(cmd *PrCloneCmd, cfg *config.Config) error {
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

	// Worktree target dir (from -d flag / config)
	dir := cmd.Dir
	if dir == "" {
		dir = "."
	}

	// Repo scan dir (from config, fallback to dir)
	repoScanDir := cfg.RepoScanDir()
	if repoScanDir == "" {
		repoScanDir = dir
	}

	// Check if repo already exists locally (search in repoScanDir)
	if existingPath, err := git.FindRepoByName(repoScanDir, name); err == nil {
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
	fmt.Printf("Cloning %s to %s (using %s)...\n", repoSpec, dir, forgeName)
	repoPath, err := f.CloneRepo(repoSpec, dir)
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

	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fmt.Printf("Creating worktree for branch %s in %s\n", branch, absPath)

	result, err := git.CreateWorktreeFrom(repoPath, dir, branch, cfg.WorktreeFormat, "")
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

	// Run post-add hooks
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandPR)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(cmd.Env)
	if err != nil {
		return err
	}

	ctx := hooks.Context{
		Path:    result.Path,
		Branch:  branch,
		Folder:  filepath.Base(repoPath),
		Trigger: string(hooks.CommandPR),
		Env:     env,
	}
	ctx.Repo, _ = git.GetRepoNameFrom(repoPath)
	ctx.MainRepo, _ = git.GetMainRepoPath(result.Path)
	if ctx.MainRepo == "" {
		ctx.MainRepo, _ = filepath.Abs(repoPath)
	}

	return hooks.RunAll(hookMatches, ctx)
}

func runPrMerge(cmd *PrMergeCmd, cfg *config.Config) error {
	var branch, mainRepo, wtPath string
	var err error

	// Resolve target: if ID is 0 and in worktree, use current branch
	// Otherwise use the resolver
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if cmd.ID == 0 && git.IsWorktree(cwd) {
		// Inside worktree, no ID specified - use current branch
		branch, err = git.GetCurrentBranch(cwd)
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		mainRepo, err = git.GetMainRepoPath(cwd)
		if err != nil {
			return fmt.Errorf("failed to get main repo path: %w", err)
		}
		wtPath = cwd
	} else if cmd.ID == 0 {
		return fmt.Errorf("--id required when not inside a worktree (run 'wt list' to see IDs)")
	} else {
		// Resolve target by ID
		scanDir := cmd.Dir
		if scanDir == "" {
			scanDir = "."
		}
		scanDir, err = filepath.Abs(scanDir)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path: %w", err)
		}

		target, err := resolve.ByID(cmd.ID, scanDir)
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
		return fmt.Errorf("PR #%d is already merged (use 'wt prune' to clean up)", pr.Number)
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
	hookMatches, err := hooks.SelectHooks(cfg.Hooks, cmd.Hook, cmd.NoHook, hooks.CommandMerge)
	if err != nil {
		return err
	}

	env, err := hooks.ParseEnvWithStdin(cmd.Env)
	if err != nil {
		return err
	}

	ctx := hooks.Context{
		Path:     wtPath,
		Branch:   branch,
		MainRepo: mainRepo,
		Folder:   filepath.Base(mainRepo),
		Trigger:  string(hooks.CommandMerge),
		Env:      env,
	}
	ctx.Repo, _ = git.GetRepoNameFrom(mainRepo)

	// If worktree was removed, run hooks from main repo
	workDir := wtPath
	if !cmd.Keep {
		workDir = mainRepo
	}

	hooks.RunAllNonFatal(hookMatches, ctx, workDir)
	return nil
}

func runPrCreate(cmd *PrCreateCmd, cfg *config.Config) error {
	var branch, mainRepo, wtPath string
	var err error

	// Resolve target: if ID is 0 and in worktree, use current branch
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	if cmd.ID == 0 && git.IsWorktree(cwd) {
		// Inside worktree, no ID specified - use current branch
		branch, err = git.GetCurrentBranch(cwd)
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		mainRepo, err = git.GetMainRepoPath(cwd)
		if err != nil {
			return fmt.Errorf("failed to get main repo path: %w", err)
		}
		wtPath = cwd
	} else if cmd.ID == 0 {
		return fmt.Errorf("--id required when not inside a worktree (run 'wt list' to see IDs)")
	} else {
		// Resolve target by ID
		scanDir := cmd.Dir
		if scanDir == "" {
			scanDir = "."
		}
		scanDir, err = filepath.Abs(scanDir)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path: %w", err)
		}

		target, err := resolve.ByID(cmd.ID, scanDir)
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

	// Get body content
	body := cmd.Body
	if cmd.BodyFile != "" {
		// Read body from file
		content, err := os.ReadFile(cmd.BodyFile)
		if err != nil {
			return fmt.Errorf("failed to read body file: %w", err)
		}
		body = string(content)
	} else if cmd.Body == "-" {
		// Read body from stdin
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read body from stdin: %w", err)
		}
		body = string(content)
	}

	// Create PR params
	params := forge.CreatePRParams{
		Title: cmd.Title,
		Body:  body,
		Base:  cmd.Base,
		Head:  branch,
		Draft: cmd.Draft,
	}

	// Create the PR
	fmt.Printf("Creating PR for branch %s...\n", branch)
	result, err := f.CreatePR(originURL, params)
	if err != nil {
		return err
	}

	fmt.Printf("✓ PR #%d created: %s\n", result.Number, result.URL)

	// Update cache with PR info
	scanDir := cmd.Dir
	if scanDir == "" {
		scanDir = filepath.Dir(wtPath)
	}
	scanDir, _ = filepath.Abs(scanDir)

	wtCache, err := cache.Load(scanDir)
	if err == nil {
		state := "OPEN"
		if cmd.Draft {
			state = "DRAFT"
		}
		prInfo := &cache.PRInfo{
			Number:   result.Number,
			State:    state,
			IsDraft:  cmd.Draft,
			URL:      result.URL,
			CachedAt: time.Now(),
			Fetched:  true,
		}
		wtCache.SetPRForBranch(originURL, branch, prInfo)
		if err := cache.Save(scanDir, wtCache); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update cache: %v\n", err)
		}
	}

	// Open in browser if requested
	if cmd.Web && result.URL != "" {
		if err := openBrowser(result.URL); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open browser: %v\n", err)
		}
	}

	return nil
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

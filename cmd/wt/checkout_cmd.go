package main

import (
	"context"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/preserve"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/wizard/flows"
	"github.com/raphi011/wt/internal/worktree"
)

func newCheckoutCmd() *cobra.Command {
	var (
		newBranch   bool
		base        string
		fetch       bool
		autoStash   bool
		note        string
		hf          hookFlags
		noPreserve  bool
		interactive bool
	)

	cmd := &cobra.Command{
		Use:     "checkout [[scope:]branch]",
		Short:   "Create or open worktree for branch",
		Aliases: []string{"co"},
		GroupID: GroupCore,
		Long: `Create a worktree for a branch, or open it if one already exists.

Use -b to create a new branch, or omit for an existing branch.
Use -i for interactive mode to be prompted for options.

Target uses [scope:]branch format where scope can be a repo name or label:
  - Without scope: uses current repo (or searches all repos for existing branch)
  - With repo scope: targets that specific repo
  - With label scope (requires -b): targets all repos with that label`,
		Example: `  wt checkout feature-branch              # Existing branch in current repo
  wt checkout myrepo:feature              # Existing branch in myrepo
  wt checkout -b feature-branch           # Create new branch in current repo
  wt checkout -b myrepo:feature           # Create new branch in myrepo
  wt checkout -b backend:feature          # Create new branch in backend label repos
  wt checkout -i                          # Interactive mode`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			fetchExplicit := cmd.Flags().Changed("fetch")
			baseExplicit := cmd.Flags().Changed("base")

			var target string
			if len(args) > 0 {
				target = args[0]
			}

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Interactive mode
			if interactive {
				result, err := runCheckoutInteractive(ctx, reg, hf, baseExplicit)
				if err != nil {
					return err
				}
				if result.Cancelled {
					return nil
				}
				target = result.Target
				newBranch = result.NewBranch
				hf = result.HookFlags
				if result.Base != "" {
					base = result.Base
				}
			}

			// Parse target
			parsed, err := parseScopedTarget(reg, target)
			if err != nil {
				return err
			}

			// Resolve fetch for repo routing (global config); per-repo config is applied in checkoutInRepo
			fetchResolved := fetch
			if !fetchExplicit {
				fetchResolved = cfg.Checkout.AutoFetch
			}

			// Determine repos to operate on
			repos, err := resolveCheckoutRepos(ctx, l, reg, parsed, newBranch, fetchResolved, hf)
			if err != nil {
				return err
			}

			l.Debug("checkout", "branch", parsed.Branch, "repos", len(repos), "new", newBranch)

			if autoStash && len(repos) > 1 {
				return fmt.Errorf("--autostash cannot be used with label targets (affects multiple repos)")
			}

			coOpts := checkoutOpts{
				NewBranch:     newBranch,
				Base:          base,
				Fetch:         fetch,
				FetchExplicit: fetchExplicit,
				AutoStash:     autoStash,
				NoPreserve:    noPreserve,
				Note:          note,
				Hooks:         hf,
			}
			for _, repo := range repos {
				if err := checkoutInRepo(ctx, repo, parsed.Branch, coOpts); err != nil {
					return fmt.Errorf("%s: %w", repo.Name, err)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&newBranch, "new-branch", "b", false, "Create a new branch")
	cmd.Flags().StringVar(&base, "base", "", "Base branch to create from")
	cmd.Flags().BoolVarP(&fetch, "fetch", "f", false, "Fetch from origin before checkout")
	cmd.Flags().BoolVarP(&autoStash, "autostash", "s", false, "Stash changes and apply to new worktree")
	cmd.Flags().StringVar(&note, "note", "", "Set a note on the branch")
	registerHookFlags(cmd, &hf)
	cmd.Flags().BoolVar(&noPreserve, "no-preserve", false, "Skip file preservation")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")

	// Completions
	cmd.RegisterFlagCompletionFunc("note", cobra.NoFileCompletions)
	registerCheckoutCompletions(cmd)

	return cmd
}

// checkoutOpts holds all options for creating or opening a worktree checkout.
type checkoutOpts struct {
	NewBranch     bool
	Base          string
	Fetch         bool
	FetchExplicit bool // true when --fetch was explicitly passed on CLI
	AutoStash     bool
	NoPreserve    bool
	Note          string
	Hooks         hookFlags
}

func checkoutInRepo(ctx context.Context, repo registry.Repo, branch string, opts checkoutOpts) error {
	l := log.FromContext(ctx)

	cfg := resolveEffectiveConfig(ctx, repo.Path)

	// Override fetch with per-repo config if not explicitly set by CLI flag
	fetch := opts.Fetch
	if !opts.FetchExplicit {
		fetch = cfg.Checkout.AutoFetch
	}

	format := repo.GetEffectiveWorktreeFormat(cfg.Checkout.WorktreeFormat)
	wtPath := worktree.ResolvePath(repo.Path, repo.Name, branch, format)

	l.Debug("creating worktree", "path", wtPath, "branch", branch)

	repoType, err := git.DetectRepoType(repo.Path)
	if err != nil {
		return err
	}

	gitDir := git.GetGitDir(repo.Path, repoType)
	repoHasCommits := git.RefExists(ctx, gitDir, "HEAD")

	var stashed bool
	if opts.AutoStash {
		stashed, err = autoStashChanges(ctx, repo, repoHasCommits)
		if err != nil {
			return err
		}
	}

	fetchForCheckout(ctx, gitDir, cfg, branch, opts, fetch, repoHasCommits)

	if err := createWorktreeForBranch(ctx, gitDir, wtPath, branch, opts, repoHasCommits, cfg.Checkout.BaseRef); err != nil {
		return err
	}

	setUpstreamTracking(ctx, gitDir, branch, opts.NewBranch, repoHasCommits, cfg)

	fmt.Printf("Created worktree: %s (%s)\n", wtPath, branch)

	if stashed {
		if err := git.StashPop(ctx, wtPath); err != nil {
			l.Printf("Warning: failed to apply stashed changes: %v\n", err)
		}
	}

	if opts.Note != "" {
		if err := git.SetBranchNote(ctx, gitDir, branch, opts.Note); err != nil {
			l.Printf("Warning: failed to set note: %v\n", err)
		}
	}

	preserveWorktreeFiles(ctx, repo.Path, wtPath, opts.NoPreserve, cfg.Preserve)

	action := hooks.ActionOpen
	if opts.NewBranch {
		action = hooks.ActionCreate
	}

	hp, err := buildHookParams(cfg, repo, wtPath, branch, hooks.CommandCheckout, action, opts.Hooks)
	if err != nil {
		return err
	}

	return withHooks(ctx, hp, func() error {
		recordHistory(ctx, cfg, wtPath, repo.Name, branch)
		return nil
	})
}

// autoStashChanges stashes uncommitted changes in the current worktree if autostash
// is enabled (via checkoutOpts.AutoStash). Returns true if changes were stashed.
func autoStashChanges(ctx context.Context, repo registry.Repo, repoHasCommits bool) (bool, error) {
	l := log.FromContext(ctx)

	workDir := config.WorkDirFromContext(ctx)
	mainPath := git.GetCurrentRepoMainPathFrom(ctx, workDir)
	if mainPath == "" {
		return false, fmt.Errorf("--autostash: cannot determine repo from working directory %s (are you in a git repository?)", workDir)
	}
	// Both paths are already canonical (symlinks resolved by
	// GetCurrentRepoMainPathFrom and the registry).
	if mainPath != repo.Path {
		return false, fmt.Errorf("--autostash requires running from a worktree of %s", repo.Name)
	}
	if !repoHasCommits {
		return false, nil
	}

	n, err := git.Stash(ctx, workDir)
	if err != nil {
		l.Printf("Warning: stash failed: %v\n", err)
		return false, nil
	}
	if n > 0 {
		l.Printf("Stashed %d file(s)\n", n)
		return true, nil
	}
	return false, nil
}

// fetchForCheckout fetches the relevant branch from the remote before checkout.
// Warnings are logged but errors don't abort checkout.
func fetchForCheckout(ctx context.Context, gitDir string, cfg *config.Config, branch string, opts checkoutOpts, fetch, repoHasCommits bool) {
	if !fetch || !repoHasCommits {
		return
	}
	l := log.FromContext(ctx)

	var fetchRemote, fetchBranch string
	if opts.Base != "" {
		remote, branchPart, isRemote := git.ParseRemoteRef(ctx, gitDir, opts.Base)
		if isRemote {
			fetchRemote = remote
			fetchBranch = branchPart
		} else if cfg.Checkout.BaseRef == "local" {
			l.Printf("Warning: --fetch has no effect with local base ref %q\n", opts.Base)
			return
		} else {
			fetchRemote = "origin"
			fetchBranch = opts.Base
		}
	} else if opts.NewBranch {
		fetchRemote = "origin"
		fetchBranch = git.GetDefaultBranch(ctx, gitDir)
	} else {
		fetchRemote = "origin"
		fetchBranch = branch
	}

	if fetchBranch != "" {
		if err := git.FetchBranchFromRemote(ctx, gitDir, fetchRemote, fetchBranch); err != nil {
			l.Printf("Warning: fetch failed for %s/%s: %v (continuing with local refs)\n", fetchRemote, fetchBranch, err)
		}
	}
}

// createWorktreeForBranch creates the git worktree, handling new branch, existing branch,
// orphan (empty repo), and remote base ref resolution.
func createWorktreeForBranch(ctx context.Context, gitDir, wtPath, branch string, opts checkoutOpts, repoHasCommits bool, baseRefMode string) error {
	if !opts.NewBranch {
		return git.CreateWorktree(ctx, gitDir, wtPath, branch)
	}

	baseRef := opts.Base
	if baseRef == "" {
		baseRef = git.GetDefaultBranch(ctx, gitDir)
	}

	// Use remote ref by default, unless already explicit or config says local
	_, _, isRemote := git.ParseRemoteRef(ctx, gitDir, baseRef)
	if !isRemote && baseRefMode != "local" {
		remoteRef := "origin/" + baseRef
		if git.RefExists(ctx, gitDir, remoteRef) {
			baseRef = remoteRef
		} else {
			l := log.FromContext(ctx)
			l.Printf("Warning: %s not found, using local ref %s\n", remoteRef, baseRef)
		}
	}

	if !git.RefExists(ctx, gitDir, baseRef) {
		if repoHasCommits {
			return git.CreateWorktreeNewBranch(ctx, gitDir, wtPath, branch, baseRef)
		}
		return git.CreateWorktreeOrphan(ctx, gitDir, wtPath, branch)
	}
	return git.CreateWorktreeNewBranch(ctx, gitDir, wtPath, branch, baseRef)
}

// setUpstreamTracking sets up remote tracking for the branch if configured.
func setUpstreamTracking(ctx context.Context, gitDir, branch string, newBranch, repoHasCommits bool, cfg *config.Config) {
	if !repoHasCommits || !cfg.Checkout.ShouldSetUpstream() || !git.HasRemote(ctx, gitDir, "origin") {
		return
	}
	l := log.FromContext(ctx)

	if newBranch {
		if err := git.PushBranch(ctx, gitDir, branch); err != nil {
			l.Printf("Warning: failed to push branch: %v\n", err)
		} else if err := git.SetUpstreamBranch(ctx, gitDir, branch, branch); err != nil {
			l.Debug("failed to set upstream", "error", err)
		}
	} else if git.RemoteBranchExists(ctx, gitDir, branch) {
		if err := git.SetUpstreamBranch(ctx, gitDir, branch, branch); err != nil {
			l.Debug("failed to set upstream", "error", err)
		}
	}
}

// preserveWorktreeFiles symlinks preserved files from the repo root into the new worktree.
func preserveWorktreeFiles(ctx context.Context, repoPath, wtPath string, noPreserve bool, preserveCfg config.PreserveConfig) {
	if noPreserve || len(preserveCfg.Paths) == 0 {
		return
	}
	l := log.FromContext(ctx)

	linked, err := preserve.PreserveFiles(ctx, preserveCfg, repoPath, wtPath)
	if err != nil {
		l.Printf("Warning: preserve files failed: %v\n", err)
	} else if len(linked) > 0 {
		l.Printf("Preserved %d file(s)\n", len(linked))
		for _, f := range linked {
			l.Debug("  preserved", "file", f)
		}
	}
}

// openExistingWorktree handles the case where a worktree for the branch already exists.
// It prints the worktree path to stdout, records history, and runs hooks with action="open",
// skipping worktree creation.
func openExistingWorktree(ctx context.Context, repo registry.Repo, branch, wtPath string, hf hookFlags) error {
	cfg := resolveEffectiveConfig(ctx, repo.Path)

	hp, err := buildHookParams(cfg, repo, wtPath, branch, hooks.CommandCheckout, hooks.ActionOpen, hf)
	if err != nil {
		return err
	}

	return withHooks(ctx, hp, func() error {
		fmt.Printf("Opened worktree: %s (%s)\n", wtPath, branch)
		recordHistory(ctx, cfg, wtPath, repo.Name, branch)
		return nil
	})
}

// findWorktreeForBranch checks if the given branch already has a worktree in the repo.
// Returns the worktree path and true if found, or ("", false) otherwise.
func findWorktreeForBranch(ctx context.Context, repoPath, branch string) (string, bool) {
	wts, err := git.ListWorktreesFromRepo(ctx, repoPath)
	if err != nil {
		l := log.FromContext(ctx)
		l.Debug("failed to list worktrees", "repo", repoPath, "error", err)
		return "", false
	}
	for _, wt := range wts {
		if wt.Branch == branch {
			return wt.Path, true
		}
	}
	return "", false
}

// resolveCheckoutRepos determines which repos need a new worktree created.
// For repos that already have a worktree for the branch, it opens them directly.
func resolveCheckoutRepos(
	ctx context.Context,
	l *log.Logger,
	reg *registry.Registry,
	parsed ScopedTargetResult,
	newBranch, fetch bool,
	hf hookFlags,
) ([]registry.Repo, error) {
	if len(parsed.Repos) > 0 {
		if newBranch {
			return parsed.Repos, nil
		}
		return resolveScopedExisting(ctx, parsed.Repos, parsed.Branch, hf)
	}

	if newBranch {
		repo, err := findOrRegisterCurrentRepoFromContext(ctx, reg)
		if err != nil {
			return nil, fmt.Errorf("not in a repo, use scope:branch to specify target: %w", err)
		}
		return []registry.Repo{repo}, nil
	}

	// Existing branch without scope
	repo, err := findOrRegisterCurrentRepoFromContext(ctx, reg)
	if err == nil {
		return resolveUnscopedInRepo(ctx, repo, parsed.Branch, fetch, hf)
	}
	return resolveUnscopedAcrossRepos(ctx, l, reg, parsed.Branch, hf)
}

// resolveScopedExisting handles scoped targets for existing branches.
// Opens worktrees that already exist and returns repos that still need creation.
func resolveScopedExisting(
	ctx context.Context,
	repos []registry.Repo,
	branch string,
	hf hookFlags,
) ([]registry.Repo, error) {
	var remaining []registry.Repo
	for _, repo := range repos {
		wtPath, found := findWorktreeForBranch(ctx, repo.Path, branch)
		if !found {
			remaining = append(remaining, repo)
			continue
		}
		if err := openExistingWorktree(ctx, repo, branch, wtPath, hf); err != nil {
			return nil, err
		}
	}
	return remaining, nil
}

// resolveUnscopedInRepo resolves an existing branch checkout within the current repo.
func resolveUnscopedInRepo(
	ctx context.Context,
	repo registry.Repo,
	branch string,
	fetch bool,
	hf hookFlags,
) ([]registry.Repo, error) {
	if wtPath, found := findWorktreeForBranch(ctx, repo.Path, branch); found {
		return nil, openExistingWorktree(ctx, repo, branch, wtPath, hf)
	}

	branches, err := git.ListLocalBranches(ctx, repo.Path)
	if err != nil {
		l := log.FromContext(ctx)
		l.Debug("failed to list branches", "repo", repo.Name, "error", err)
	} else if slices.Contains(branches, branch) {
		return []registry.Repo{repo}, nil
	}

	if fetch {
		return []registry.Repo{repo}, nil
	}
	return nil, fmt.Errorf("branch %q not found in repo %s", branch, repo.Name)
}

// resolveUnscopedAcrossRepos searches all registered repos for an existing branch.
func resolveUnscopedAcrossRepos(
	ctx context.Context,
	l *log.Logger,
	reg *registry.Registry,
	branch string,
	hf hookFlags,
) ([]registry.Repo, error) {
	var repos []registry.Repo
	var opened bool
	for _, repo := range filterOrphanedRepos(l, reg.Repos) {
		if wtPath, found := findWorktreeForBranch(ctx, repo.Path, branch); found {
			if err := openExistingWorktree(ctx, repo, branch, wtPath, hf); err != nil {
				return nil, err
			}
			opened = true
			continue
		}
		branches, err := git.ListLocalBranches(ctx, repo.Path)
		if err != nil {
			l.Debug("failed to list branches", "repo", repo.Name, "error", err)
			continue
		}
		if slices.Contains(branches, branch) {
			repos = append(repos, repo)
		}
	}

	if len(repos) == 0 {
		if opened {
			return nil, nil
		}
		return nil, fmt.Errorf("branch %q not found in any repo", branch)
	}
	if len(repos) > 1 {
		var names []string
		for _, r := range repos {
			names = append(names, r.Name+":"+branch)
		}
		return nil, fmt.Errorf("branch %q exists in multiple repos: %v\nUse scope:branch to specify", branch, names)
	}
	return repos, nil
}

// completeHooks provides completion for hook flags
func completeHooks(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	hooksMap := getEffectiveHooksForCompletion(cmd.Context())
	var hooks []string
	for name := range hooksMap {
		hooks = append(hooks, name)
	}
	return hooks, cobra.ShellCompDirectiveNoFileComp
}

// getEffectiveHooksForCompletion returns the effective hooks map for completions,
// using the resolver to include local hooks if available in a repo context.
func getEffectiveHooksForCompletion(ctx context.Context) map[string]config.Hook {
	resolver := config.ResolverFromContext(ctx)
	if resolver == nil {
		cfg := config.FromContext(ctx)
		return cfg.Hooks.Hooks
	}

	// Try to resolve for the current repo
	workDir := config.WorkDirFromContext(ctx)
	repoPath := git.GetCurrentRepoMainPathFrom(ctx, workDir)
	if repoPath != "" {
		effCfg, err := resolver.ConfigForRepo(repoPath)
		if err == nil {
			return effCfg.Hooks.Hooks
		}
	}

	return resolver.Global().Hooks.Hooks
}

type checkoutInteractiveResult struct {
	Target    string
	NewBranch bool
	Base      string
	HookFlags hookFlags
	Cancelled bool
}

// runCheckoutInteractive runs the checkout wizard and applies the selections to
// produce a resolved target, newBranch flag, base branch, and updated hook flags.
func runCheckoutInteractive(ctx context.Context, reg *registry.Registry, hf hookFlags, baseFromCLI bool) (checkoutInteractiveResult, error) {
	wizOpts, err := runCheckoutWizard(ctx, reg, hf.HookNames, hf.NoHook, baseFromCLI)
	if err != nil {
		return checkoutInteractiveResult{}, err
	}
	if wizOpts.Cancelled {
		return checkoutInteractiveResult{Cancelled: true}, nil
	}

	target := wizOpts.Branch
	hf.HookNames = wizOpts.SelectedHooks
	hf.NoHook = wizOpts.NoHook

	if len(wizOpts.SelectedRepos) > 0 {
		repo, err := reg.FindByPath(wizOpts.SelectedRepos[0])
		if err != nil {
			return checkoutInteractiveResult{}, fmt.Errorf("selected repo no longer registered: %s", wizOpts.SelectedRepos[0])
		}
		target = repo.Name + ":" + wizOpts.Branch
	}

	return checkoutInteractiveResult{
		Target:    target,
		NewBranch: wizOpts.NewBranch,
		Base:      wizOpts.Base,
		HookFlags: hf,
	}, nil
}

// runCheckoutWizard runs the interactive checkout wizard
func runCheckoutWizard(ctx context.Context, reg *registry.Registry, cliHooks []string, cliNoHook bool, baseFromCLI bool) (flows.CheckoutOptions, error) {
	l := log.FromContext(ctx)

	// Use global config for wizard — hooks from all repos are shown
	cfg := config.FromContext(ctx)

	// Build available repos list
	var repoPaths, repoNames []string
	var preSelectedRepos []int

	// Get current repo path if inside one
	currentRepoPath := git.GetCurrentRepoMainPathFrom(ctx, config.WorkDirFromContext(ctx))

	for i, repo := range reg.Repos {
		repoPaths = append(repoPaths, repo.Path)
		repoNames = append(repoNames, repo.Name)
		if repo.Path == currentRepoPath {
			preSelectedRepos = append(preSelectedRepos, i)
		}
	}

	// Build branch fetcher
	fetchBranches := func(repoPath string) flows.BranchFetchResult {
		// Get worktree branches to mark them
		wtBranches := git.GetWorktreeBranches(ctx, repoPath)

		// Get all local branches
		branches, err := git.ListLocalBranches(ctx, repoPath)
		if err != nil {
			l.Debug("failed to list branches for wizard", "repo", repoPath, "error", err)
			return flows.BranchFetchResult{}
		}

		var result []flows.BranchInfo
		for _, b := range branches {
			result = append(result, flows.BranchInfo{
				Name:       b,
				InWorktree: wtBranches[b],
			})
		}
		return flows.BranchFetchResult{
			Branches:      result,
			DefaultBranch: git.GetDefaultBranch(ctx, repoPath),
		}
	}

	// Build initial branches and default branch from first repo (or current repo)
	var initialBranches []flows.BranchInfo
	var defaultBranch string
	if len(preSelectedRepos) > 0 {
		result := fetchBranches(repoPaths[preSelectedRepos[0]])
		initialBranches = result.Branches
		defaultBranch = result.DefaultBranch
	} else if len(repoPaths) > 0 {
		result := fetchBranches(repoPaths[0])
		initialBranches = result.Branches
		defaultBranch = result.DefaultBranch
	}

	// Build available hooks
	var availableHooks []flows.HookInfo
	for name, hook := range cfg.Hooks.Hooks {
		isDefault := slices.Contains(hook.On, "checkout")
		availableHooks = append(availableHooks, flows.HookInfo{
			Name:        name,
			Description: hook.Description,
			IsDefault:   isDefault,
		})
	}

	params := flows.CheckoutWizardParams{
		Branches:         initialBranches,
		AvailableRepos:   repoPaths,
		RepoNames:        repoNames,
		PreSelectedRepos: preSelectedRepos,
		FetchBranches:    fetchBranches,
		AvailableHooks:   availableHooks,
		HooksFromCLI:     len(cliHooks) > 0 || cliNoHook,
		DefaultBranch:    defaultBranch,
		BaseFromCLI:      baseFromCLI,
	}

	return flows.CheckoutInteractive(params)
}

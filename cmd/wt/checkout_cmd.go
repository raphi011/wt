package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

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
		global      bool
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
  - Without scope: uses current repo; outside a repo, or with -g, searches
    all repos for an existing branch and errors if it is in several
  - With repo scope: targets that specific repo
  - With label scope (requires -b): targets all repos with that label`,
		Example: `  wt checkout feature-branch              # Existing branch in current repo
  wt checkout -g feature-branch           # Existing branch in whichever repo has it
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

			// Parse hook args once: stdin (KEY=-) can only be read once,
			// but hooks run per repo
			if err := hf.parseArgs(); err != nil {
				return err
			}

			// Parse target
			parsed, err := parseScopedTarget(reg, target)
			if err != nil {
				return err
			}

			// Reject before the first repo is touched
			if autoStash && len(parsed.Repos) > 1 {
				return fmt.Errorf("--autostash cannot be used with label targets (affects multiple repos)")
			}

			// Resolve fetch for repo routing (global config); per-repo config is applied in ensureWorktree
			fetchResolved := fetch
			if !fetchExplicit {
				fetchResolved = cfg.Checkout.AutoFetch
			}

			// Determine repos to operate on
			repos, err := resolveCheckoutRepos(ctx, l, reg, parsed, newBranch, fetchResolved, global)
			if err != nil {
				return err
			}

			l.Debug("checkout", "branch", parsed.Branch, "repos", len(repos), "new", newBranch)

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
			// A label target can fail in some repos and still continue with the rest
			var errs []error
			for _, repo := range repos {
				if _, err := ensureWorktree(ctx, repo, parsed.Branch, coOpts); err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", repo.Name, err))
				}
			}

			return errors.Join(errs...)
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
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Search all repos for an unscoped existing branch")
	cmd.MarkFlagsMutuallyExclusive("global", "new-branch")

	// Completions
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("note", cobra.NoFileCompletions))
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
	PR            *prIntent // set by pr checkout: the branch is the source branch of a PR
}

// prIntent identifies the PR a worktree is checked out for. Hooks run with
// action "pr" and get the PR number and repo.
type prIntent struct {
	Number int
	Repo   string // org/repo on the forge, empty if unknown
}

// worktreeResult reports what ensureWorktree did.
type worktreeResult struct {
	Path    string
	Created bool // false: the branch already had a worktree, which was opened
}

// ensureWorktree makes sure the branch has a worktree in the repo: an existing
// worktree is opened, otherwise one is created. Both set the note, run hooks
// and record history. Before-hooks run first and can cancel the creation.
// With opts.NewBranch the branch is created with the worktree.
func ensureWorktree(ctx context.Context, repo registry.Repo, branch string, opts checkoutOpts) (worktreeResult, error) {
	l := log.FromContext(ctx)

	cfg := resolveEffectiveConfig(ctx, repo.Path)

	var res worktreeResult
	if !opts.NewBranch {
		wtPath, found, err := findWorktreeForBranch(ctx, repo.Path, branch)
		if err != nil {
			return worktreeResult{}, err
		}
		if found {
			res.Path = wtPath
		}
	}

	repoType, err := git.DetectRepoType(ctx, repo.Path)
	if err != nil {
		return worktreeResult{}, err
	}
	gitDir := git.GetGitDir(ctx, repo.Path, repoType)

	if res.Path == "" {
		format := repo.GetEffectiveWorktreeFormat(cfg.Checkout.WorktreeFormat)
		res.Path = worktree.ResolvePath(repo.Path, repo.Name, branch, format)
		res.Created = true
	}

	action := hooks.ActionOpen
	if opts.NewBranch {
		action = hooks.ActionCreate
	}
	if opts.PR != nil {
		action = hooks.ActionPR
	}

	hp, err := hookOperation(cfg, repo, res.Path, branch, hooks.CommandCheckout, action, opts.Hooks)
	if err != nil {
		return worktreeResult{}, err
	}
	if opts.PR != nil {
		hp.PRNumber = new(opts.PR.Number)
		hp.PRRepo = opts.PR.Repo
	}

	if res.Created {
		// The worktree does not exist yet when before-hooks run
		hp.BeforeWorkDir = repo.Path
	}

	err = hp.Run(ctx, func() error {
		if res.Created {
			if err := createWorktree(ctx, repo, gitDir, res.Path, cfg, branch, opts); err != nil {
				return err
			}
		} else {
			l.Printf("Opened worktree: %s (%s)\n", res.Path, branch)
		}
		if opts.Note != "" {
			if err := git.SetBranchNote(ctx, gitDir, branch, opts.Note); err != nil {
				l.Printf("Warning: failed to set note: %v\n", err)
			}
		}
		recordHistory(ctx, cfg, res.Path, repo.Name, branch)
		return nil
	})
	if err != nil {
		return worktreeResult{}, err
	}
	return res, nil
}

// createWorktree creates the worktree of a branch that has none at wtPath.
// Order matters: fetch, create, upstream, autostash, preserve.
func createWorktree(ctx context.Context, repo registry.Repo, gitDir, wtPath string, cfg *config.Config, branch string, opts checkoutOpts) error {
	l := log.FromContext(ctx)

	// Override fetch with per-repo config if not explicitly set by CLI flag
	fetch := opts.Fetch
	if !opts.FetchExplicit {
		fetch = cfg.Checkout.AutoFetch
	}

	l.Debug("creating worktree", "path", wtPath, "branch", branch)

	repoHasCommits := git.RefExists(ctx, gitDir, "HEAD")

	if opts.AutoStash {
		if err := checkAutoStash(ctx, repo); err != nil {
			return err
		}
	}

	fetchForCheckout(ctx, gitDir, cfg, branch, opts, fetch, repoHasCommits)

	if err := createWorktreeForBranch(ctx, gitDir, wtPath, branch, opts, repoHasCommits, cfg.Checkout.BaseRef); err != nil {
		return err
	}

	setUpstreamTracking(ctx, gitDir, branch, opts.NewBranch, repoHasCommits, cfg)

	l.Printf("Created worktree: %s (%s)\n", wtPath, branch)

	// Stash only after the worktree exists, so a failed checkout leaves the
	// working tree untouched
	if opts.AutoStash && autoStashChanges(ctx, repoHasCommits) {
		if err := git.StashPop(ctx, wtPath); err != nil {
			l.Printf("Warning: failed to apply stashed changes in %s: %v\n", wtPath, err)
			l.Printf("Your changes are kept in the latest stash entry (stash@{0}); run 'git stash pop' to retry\n")
		}
	}

	preserveWorktreeFiles(ctx, repo.Path, wtPath, opts.NoPreserve, cfg.Preserve)

	return nil
}

// checkAutoStash verifies that --autostash is run from a worktree of the target repo.
func checkAutoStash(ctx context.Context, repo registry.Repo) error {
	workDir := config.WorkDirFromContext(ctx)
	mainPath := currentRepoPath(ctx)
	if mainPath == "" {
		return fmt.Errorf("--autostash: cannot determine repo from working directory %s (are you in a git repository?)", workDir)
	}
	// Both paths are already canonical (symlinks resolved by
	// GetCurrentRepoMainPathFrom and the registry).
	if mainPath != repo.Path {
		return fmt.Errorf("--autostash requires running from a worktree of %s", repo.Name)
	}
	return nil
}

// autoStashChanges stashes uncommitted changes in the current worktree.
// Returns true if changes were stashed.
func autoStashChanges(ctx context.Context, repoHasCommits bool) bool {
	l := log.FromContext(ctx)

	if !repoHasCommits {
		return false
	}

	n, err := git.Stash(ctx, config.WorkDirFromContext(ctx))
	if err != nil {
		l.Printf("Warning: stash failed: %v\n", err)
		return false
	}
	if n > 0 {
		l.Printf("Stashed %d file(s)\n", n)
		return true
	}
	return false
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

// findWorktreeForBranch checks if the given branch already has a worktree in the repo.
// Returns the worktree path and true if found, or ("", false) otherwise.
// Returns an error if the repo's worktrees cannot be listed.
func findWorktreeForBranch(ctx context.Context, repoPath, branch string) (string, bool, error) {
	wts, err := git.ListWorktreesFromRepo(ctx, repoPath)
	if err != nil {
		return "", false, err
	}
	for _, wt := range wts {
		if wt.Branch == branch {
			return wt.Path, true, nil
		}
	}
	return "", false, nil
}

// resolveCheckoutRepos determines the repos to check the branch out in.
func resolveCheckoutRepos(
	ctx context.Context,
	l *log.Logger,
	reg *registry.Registry,
	parsed ScopedTargetResult,
	newBranch, fetch, global bool,
) ([]registry.Repo, error) {
	if len(parsed.Repos) > 0 {
		return parsed.Repos, nil
	}

	if newBranch {
		repo, err := currentRepo(ctx, reg, autoRegister)
		if err != nil {
			return nil, fmt.Errorf("not in a repo, use scope:branch to specify target: %w", err)
		}
		return []registry.Repo{repo}, nil
	}

	// Existing branch without scope: current repo, or all repos with -g or outside a repo
	repos, current, err := unscopedRepos(ctx, reg, global)
	if err != nil {
		return nil, err
	}
	if current {
		return resolveUnscopedInRepo(ctx, repos[0], parsed.Branch, fetch)
	}
	return resolveUnscopedAcrossRepos(ctx, l, repos, parsed.Branch)
}

// resolveUnscopedInRepo resolves an existing branch checkout within the current repo.
func resolveUnscopedInRepo(
	ctx context.Context,
	repo registry.Repo,
	branch string,
	fetch bool,
) ([]registry.Repo, error) {
	_, found, err := findWorktreeForBranch(ctx, repo.Path, branch)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", repo.Name, err)
	}
	if found {
		return []registry.Repo{repo}, nil
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

// resolveUnscopedAcrossRepos searches repos for an existing branch.
// The branch must have a worktree or local branch in exactly one repo, which
// is returned.
func resolveUnscopedAcrossRepos(
	ctx context.Context,
	l *log.Logger,
	repos []registry.Repo,
	branch string,
) ([]registry.Repo, error) {
	var candidates []registry.Repo
	for _, repo := range repos {
		_, found, err := findWorktreeForBranch(ctx, repo.Path, branch)
		if err != nil {
			l.Printf("Warning: %s: %v\n", repo.Name, err)
			continue
		}
		if found {
			candidates = append(candidates, repo)
			continue
		}
		branches, err := git.ListLocalBranches(ctx, repo.Path)
		if err != nil {
			l.Debug("failed to list branches", "repo", repo.Name, "error", err)
			continue
		}
		if slices.Contains(branches, branch) {
			candidates = append(candidates, repo)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("branch %q not found in any repo", branch)
	}
	if len(candidates) > 1 {
		var names []string
		for _, c := range candidates {
			names = append(names, c.Name+":"+branch)
		}
		return nil, fmt.Errorf("branch %q exists in multiple repos: %s (use repo:branch to pick one)", branch, strings.Join(names, ", "))
	}

	return candidates, nil
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
	repoPath := currentRepoPath(ctx)
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
	currentPath := currentRepoPath(ctx)

	for i, repo := range reg.Repos {
		repoPaths = append(repoPaths, repo.Path)
		repoNames = append(repoNames, repo.Name)
		if repo.Path == currentPath {
			preSelectedRepos = append(preSelectedRepos, i)
		}
	}

	// Build branch fetcher
	fetchBranches := func(ctx context.Context, repoPath string) (flows.BranchFetchResult, error) {
		// Get worktree branches to mark them
		wtBranches := git.GetWorktreeBranches(ctx, repoPath)

		// Get all local branches
		branches, err := git.ListLocalBranches(ctx, repoPath)
		if err != nil {
			l.Debug("failed to list branches for wizard", "repo", repoPath, "error", err)
			return flows.BranchFetchResult{}, err
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
		}, nil
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
		Context:          ctx,
		AvailableRepos:   repoPaths,
		RepoNames:        repoNames,
		PreSelectedRepos: preSelectedRepos,
		FetchBranches:    fetchBranches,
		AvailableHooks:   availableHooks,
		HooksFromCLI:     len(cliHooks) > 0 || cliNoHook,
		BaseFromCLI:      baseFromCLI,
	}

	return flows.CheckoutInteractive(params)
}

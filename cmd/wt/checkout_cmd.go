package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/wizard/flows"
)

func newCheckoutCmd() *cobra.Command {
	var (
		repository  []string
		labels      []string
		newBranch   bool
		base        string
		fetch       bool
		autoStash   bool
		note        string
		hookNames   []string
		noHook      bool
		env         []string
		interactive bool
	)

	cmd := &cobra.Command{
		Use:     "checkout [branch]",
		Short:   "Create worktree for branch",
		Aliases: []string{"co"},
		GroupID: GroupCore,
		Long: `Create a worktree for an existing or new branch.

Use -b to create a new branch, or omit for an existing branch.
Use -r to target repos by name, -l to target repos by label.
Use -i for interactive mode to be prompted for options.`,
		Example: `  wt checkout feature-branch              # Existing branch in current repo
  wt checkout -b feature-branch           # Create new branch from origin/main
  wt checkout -b feature-branch -f        # Fetch main first, then create branch
  wt checkout -b feature-branch -r repo1  # In specific repo
  wt checkout -b feature -l backend       # In repos with label
  wt checkout -i                          # Interactive mode`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			// Apply config default if --fetch flag not explicitly set
			if !cmd.Flags().Changed("fetch") {
				fetch = cfg.Checkout.AutoFetch
			}

			var branch string
			if len(args) > 0 {
				branch = args[0]
			}

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Interactive mode
			if interactive {
				opts, err := runCheckoutWizard(ctx, reg, repository, labels, hookNames, noHook)
				if err != nil {
					return err
				}
				if opts.Cancelled {
					return nil
				}

				// Apply wizard selections
				branch = opts.Branch
				newBranch = opts.NewBranch
				fetch = opts.Fetch
				hookNames = opts.SelectedHooks
				noHook = opts.NoHook

				// If already in worktree, just run hooks
				if opts.IsWorktree {
					wtPath, _ := git.GetBranchWorktree(ctx, branch)
					if wtPath != "" {
						fmt.Printf("Branch %s already checked out at: %s\n", branch, wtPath)
						// Run hooks if any
						if len(hookNames) > 0 {
							repo, _ := findOrRegisterCurrentRepo(ctx, reg)
							hookEnv, _ := hooks.ParseEnvWithStdin(env)
							hookMatches, _ := hooks.SelectHooks(cfg.Hooks, hookNames, noHook, hooks.CommandCheckout)
							if len(hookMatches) > 0 {
								hookCtx := hooks.Context{
									WorktreeDir: wtPath,
									RepoDir:     repo.Path,
									Branch:      branch,
									Repo:        repo.Name,
									Origin:      git.GetRepoDisplayName(repo.Path),
									Trigger:     "checkout",
									Env:         hookEnv,
								}
								hooks.RunAllNonFatal(hookMatches, hookCtx, wtPath)
							}
						}
						return nil
					}
				}

				// Use wizard-selected repos
				if len(opts.SelectedRepos) > 0 {
					repository = nil // Clear CLI repos
					for _, path := range opts.SelectedRepos {
						repo, err := reg.FindByPath(path)
						if err == nil {
							repository = append(repository, repo.Name)
						}
					}
				}
			}

			// Resolve target repos
			repos, err := resolveTargetRepos(ctx, reg, repository, labels)
			if err != nil {
				return err
			}

			if len(repos) == 0 {
				return fmt.Errorf("no repos found")
			}

			l.Debug("checkout", "branch", branch, "repos", len(repos), "new", newBranch)

			for _, repo := range repos {
				if err := checkoutInRepo(ctx, repo, branch, newBranch, base, fetch, autoStash, note, hookNames, noHook, env); err != nil {
					return fmt.Errorf("%s: %w", repo.Name, err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Repository name(s)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Target repos by label")
	cmd.Flags().BoolVarP(&newBranch, "new-branch", "b", false, "Create a new branch")
	cmd.Flags().StringVar(&base, "base", "", "Base branch to create from")
	cmd.Flags().BoolVarP(&fetch, "fetch", "f", false, "Fetch base branch first")
	cmd.Flags().BoolVarP(&autoStash, "autostash", "s", false, "Stash changes and apply to new worktree")
	cmd.Flags().StringVar(&note, "note", "", "Set a note on the branch")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-checkout hook")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")

	// Completions
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("label", completeLabels)
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)
	registerCheckoutCompletions(cmd)

	return cmd
}

// resolveTargetRepos finds repos based on -r and -l flags, or current directory
func resolveTargetRepos(ctx context.Context, reg *registry.Registry, repoNames, labels []string) ([]*registry.Repo, error) {
	var repos []*registry.Repo

	// If specific repos requested
	if len(repoNames) > 0 {
		for _, name := range repoNames {
			repo, err := reg.FindByName(name)
			if err != nil {
				return nil, err
			}
			repos = append(repos, repo)
		}
	}

	// If labels requested
	if len(labels) > 0 {
		labeled := reg.FindByLabels(labels)
		repos = append(repos, labeled...)
	}

	// If no specific target, try current directory
	if len(repos) == 0 {
		repo, err := findOrRegisterCurrentRepo(ctx, reg)
		if err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []*registry.Repo
	for _, r := range repos {
		if !seen[r.Path] {
			seen[r.Path] = true
			unique = append(unique, r)
		}
	}

	return unique, nil
}

// findOrRegisterCurrentRepo finds the repo for cwd, auto-registering if needed
func findOrRegisterCurrentRepo(ctx context.Context, reg *registry.Registry) (*registry.Repo, error) {
	// Get main repo path from current directory
	repoPath := git.GetCurrentRepoMainPath(ctx)
	if repoPath == "" {
		return nil, fmt.Errorf("not in a git repository")
	}

	// Try to find in registry
	repo, err := reg.FindByPath(repoPath)
	if err == nil {
		return repo, nil
	}

	// Auto-register
	newRepo := registry.Repo{
		Path:   repoPath,
		Name:   git.GetRepoDisplayName(repoPath),
		Labels: cfg.DefaultLabels,
	}

	if err := reg.Add(newRepo); err != nil {
		return nil, err
	}

	if err := reg.Save(); err != nil {
		return nil, err
	}

	return reg.FindByPath(repoPath)
}

func checkoutInRepo(ctx context.Context, repo *registry.Repo, branch string, newBranch bool, base string, fetch, autoStash bool, note string, hookNames []string, noHook bool, env []string) error {
	l := log.FromContext(ctx)

	// Get effective worktree format
	format := repo.GetEffectiveWorktreeFormat(cfg.Checkout.WorktreeFormat)

	// Resolve worktree path
	wtPath := resolveWorktreePath(repo, branch, format)

	l.Debug("creating worktree", "path", wtPath, "branch", branch)

	// Detect repo type
	repoType, err := git.DetectRepoType(repo.Path)
	if err != nil {
		return err
	}

	gitDir := git.GetGitDir(repo.Path, repoType)

	// Fetch if requested
	if fetch && base != "" {
		if err := git.FetchBranch(ctx, gitDir, base); err != nil {
			l.Printf("Warning: fetch failed: %v\n", err)
		}
	} else if fetch {
		if err := git.FetchDefaultBranch(ctx, gitDir); err != nil {
			l.Printf("Warning: fetch failed: %v\n", err)
		}
	}

	// Create worktree
	if newBranch {
		baseRef := base
		if baseRef == "" {
			baseRef = git.GetDefaultBranch(ctx, gitDir)
		}
		// Use remote ref by default
		if cfg.Checkout.BaseRef != "local" {
			baseRef = "origin/" + baseRef
		}
		if err := git.CreateWorktreeNewBranch(ctx, gitDir, wtPath, branch, baseRef); err != nil {
			return err
		}
	} else {
		if err := git.CreateWorktree(ctx, gitDir, wtPath, branch); err != nil {
			return err
		}
	}

	fmt.Printf("Created worktree: %s (%s)\n", wtPath, branch)

	// Set note if provided
	if note != "" {
		if err := git.SetBranchNote(ctx, gitDir, branch, note); err != nil {
			l.Printf("Warning: failed to set note: %v\n", err)
		}
	}

	// Run hooks
	hookEnv, err := hooks.ParseEnvWithStdin(env)
	if err != nil {
		return err
	}

	hookMatches, err := hooks.SelectHooks(cfg.Hooks, hookNames, noHook, hooks.CommandCheckout)
	if err != nil {
		return err
	}

	if len(hookMatches) > 0 {
		hookCtx := hooks.Context{
			WorktreeDir: wtPath,
			RepoDir:     repo.Path,
			Branch:      branch,
			Repo:        repo.Name,
			Origin:      git.GetRepoDisplayName(repo.Path),
			Trigger:     "checkout",
			Env:         hookEnv,
		}
		hooks.RunAllNonFatal(hookMatches, hookCtx, wtPath)
	}

	return nil
}

// resolveWorktreePath computes the worktree path based on format
func resolveWorktreePath(repo *registry.Repo, branch, format string) string {
	return resolveWorktreePathWithConfig(repo.Path, repo.Name, branch, format)
}

// completeHooks provides completion for hook flags
func completeHooks(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var hooks []string
	for name := range cfg.Hooks.Hooks {
		hooks = append(hooks, name)
	}
	return hooks, cobra.ShellCompDirectiveNoFileComp
}

// runCheckoutWizard runs the interactive checkout wizard
func runCheckoutWizard(ctx context.Context, reg *registry.Registry, cliRepos, cliLabels, cliHooks []string, cliNoHook bool) (flows.CheckoutOptions, error) {
	// Build available repos list
	var repoPaths, repoNames []string
	var preSelectedRepos []int

	// Get current repo path if inside one
	currentRepoPath := git.GetCurrentRepoMainPath(ctx)

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
			return flows.BranchFetchResult{}
		}

		var result []flows.BranchInfo
		for _, b := range branches {
			result = append(result, flows.BranchInfo{
				Name:       b,
				InWorktree: wtBranches[b],
			})
		}
		return flows.BranchFetchResult{Branches: result}
	}

	// Build initial branches from first repo (or current repo)
	var initialBranches []flows.BranchInfo
	if len(preSelectedRepos) > 0 {
		result := fetchBranches(repoPaths[preSelectedRepos[0]])
		initialBranches = result.Branches
	} else if len(repoPaths) > 0 {
		result := fetchBranches(repoPaths[0])
		initialBranches = result.Branches
	}

	// Build available hooks
	var availableHooks []flows.HookInfo
	for name, hook := range cfg.Hooks.Hooks {
		isDefault := false
		for _, trigger := range hook.On {
			if trigger == "checkout" {
				isDefault = true
				break
			}
		}
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
	}

	return flows.CheckoutInteractive(params)
}

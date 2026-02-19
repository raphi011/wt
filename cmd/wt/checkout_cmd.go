package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/preserve"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/wizard/flows"
)

func newCheckoutCmd() *cobra.Command {
	var (
		newBranch   bool
		base        string
		fetch       bool
		autoStash   bool
		note        string
		hookNames   []string
		noHook      bool
		noPreserve  bool
		env         []string
		interactive bool
	)

	cmd := &cobra.Command{
		Use:     "checkout [[scope:]branch]",
		Short:   "Create worktree for branch",
		Aliases: []string{"co"},
		GroupID: GroupCore,
		Long: `Create a worktree for an existing or new branch.

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

			// Apply config default if --fetch flag not explicitly set
			if !cmd.Flags().Changed("fetch") {
				fetch = cfg.Checkout.AutoFetch
			}

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
				opts, err := runCheckoutWizard(ctx, reg, hookNames, noHook)
				if err != nil {
					return err
				}
				if opts.Cancelled {
					return nil
				}

				// Apply wizard selections
				target = opts.Branch
				newBranch = opts.NewBranch
				hookNames = opts.SelectedHooks
				noHook = opts.NoHook

				// Build scope:branch if repos selected
				if len(opts.SelectedRepos) > 0 {
					repo, err := reg.FindByPath(opts.SelectedRepos[0])
					if err != nil {
						return fmt.Errorf("selected repo no longer registered: %s", opts.SelectedRepos[0])
					}
					target = repo.Name + ":" + opts.Branch
				}
			}

			// Parse target
			parsed, err := parseScopedTarget(reg, target)
			if err != nil {
				return err
			}

			// Determine repos to operate on
			var repos []registry.Repo

			if len(parsed.Repos) > 0 {
				// Scoped target
				repos = parsed.Repos
			} else if newBranch {
				// New branch without scope - use current repo
				repo, err := findOrRegisterCurrentRepoFromContext(ctx, reg)
				if err != nil {
					return fmt.Errorf("not in a repo, use scope:branch to specify target")
				}
				repos = []registry.Repo{repo}
			} else {
				// Existing branch without scope — prefer current repo
				repo, currentRepoErr := findOrRegisterCurrentRepoFromContext(ctx, reg)
				if currentRepoErr == nil {
					// In a registered repo — check only this one
					wts, err := git.ListWorktreesFromRepo(ctx, repo.Path)
					if err == nil {
						for _, wt := range wts {
							if wt.Branch == parsed.Branch {
								return fmt.Errorf("branch %q already checked out at %s", parsed.Branch, wt.Path)
							}
						}
					}
					branches, err := git.ListLocalBranches(ctx, repo.Path)
					if err == nil {
						if slices.Contains(branches, parsed.Branch) {
							repos = append(repos, repo)
						}
					}
					if len(repos) == 0 {
						if fetch {
							// Branch not found locally but --fetch may pull it from remote
							repos = append(repos, repo)
						} else {
							return fmt.Errorf("branch %q not found in repo %s", parsed.Branch, repo.Name)
						}
					}
				} else {
					// Not in a repo — search all repos
					for _, repo := range filterOrphanedRepos(l, reg.Repos) {
						wts, err := git.ListWorktreesFromRepo(ctx, repo.Path)
						if err != nil {
							l.Debug("skipping repo", "repo", repo.Name, "error", err)
							continue
						}
						for _, wt := range wts {
							if wt.Branch == parsed.Branch {
								return fmt.Errorf("branch %q already checked out at %s", parsed.Branch, wt.Path)
							}
						}
						branches, err := git.ListLocalBranches(ctx, repo.Path)
						if err != nil {
							l.Debug("failed to list branches", "repo", repo.Name, "error", err)
							continue
						}
						if slices.Contains(branches, parsed.Branch) {
							repos = append(repos, repo)
						}
					}

					if len(repos) == 0 {
						return fmt.Errorf("branch %q not found in any repo", parsed.Branch)
					}

					if len(repos) > 1 {
						var names []string
						for _, r := range repos {
							names = append(names, r.Name+":"+parsed.Branch)
						}
						return fmt.Errorf("branch %q exists in multiple repos: %v\nUse scope:branch to specify", parsed.Branch, names)
					}
				}
			}

			l.Debug("checkout", "branch", parsed.Branch, "repos", len(repos), "new", newBranch)

			for _, repo := range repos {
				if err := checkoutInRepo(ctx, repo, parsed.Branch, newBranch, base, fetch, autoStash, noPreserve, note, hookNames, noHook, env); err != nil {
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
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-checkout hook")
	cmd.Flags().BoolVar(&noPreserve, "no-preserve", false, "Skip file preservation")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")

	// Completions
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)
	cmd.RegisterFlagCompletionFunc("note", cobra.NoFileCompletions)
	cmd.RegisterFlagCompletionFunc("arg", cobra.NoFileCompletions)
	registerCheckoutCompletions(cmd)

	return cmd
}

func checkoutInRepo(ctx context.Context, repo registry.Repo, branch string, newBranch bool, base string, fetch, autoStash, noPreserve bool, note string, hookNames []string, noHook bool, env []string) error {
	cfg := config.FromContext(ctx)
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

	// Check if repo has any commits (empty repo detection)
	repoHasCommits := git.RefExists(ctx, gitDir, "HEAD")

	// Stash changes if autostash is enabled (skip for empty repos — no commits to stash)
	var stashed bool
	if autoStash && repoHasCommits {
		n, err := git.Stash(ctx, repo.Path)
		if err != nil {
			l.Printf("Warning: stash failed: %v\n", err)
		} else if n > 0 {
			stashed = true
			l.Printf("Stashed %d file(s)\n", n)
		}
	}

	// Fetch if requested (skip for empty repos — nothing to fetch)
	if fetch && repoHasCommits {
		var fetchRemote, fetchBranch string
		if base != "" {
			// Check if base is an explicit remote ref (e.g., origin/develop, upstream/main)
			remote, branchPart, isRemote := git.ParseRemoteRef(ctx, gitDir, base)
			if isRemote {
				fetchRemote = remote
				fetchBranch = branchPart
			} else if cfg.Checkout.BaseRef == "local" {
				// Local base ref + --fetch: skip fetch and warn
				l.Printf("Warning: --fetch has no effect with local base ref %q\n", base)
				fetchBranch = "" // skip fetch
			} else {
				fetchRemote = "origin"
				fetchBranch = base
			}
		} else if newBranch {
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

	// Create worktree
	if newBranch {
		baseRef := base
		if baseRef == "" {
			baseRef = git.GetDefaultBranch(ctx, gitDir)
		}

		// Check if base is an explicit remote ref (e.g., origin/develop, upstream/main)
		_, _, isRemote := git.ParseRemoteRef(ctx, gitDir, baseRef)
		// Use remote ref by default, unless already explicit or config says local
		if !isRemote && cfg.Checkout.BaseRef != "local" {
			baseRef = "origin/" + baseRef
		}

		if !git.RefExists(ctx, gitDir, baseRef) {
			if repoHasCommits {
				// Repo has commits but baseRef is invalid — let git report the error
				if err := git.CreateWorktreeNewBranch(ctx, gitDir, wtPath, branch, baseRef); err != nil {
					return err
				}
			} else {
				// Repo is truly empty — create orphan worktree
				if err := git.CreateWorktreeOrphan(ctx, gitDir, wtPath, branch); err != nil {
					return err
				}
			}
		} else {
			if err := git.CreateWorktreeNewBranch(ctx, gitDir, wtPath, branch, baseRef); err != nil {
				return err
			}
		}
	} else {
		if err := git.CreateWorktree(ctx, gitDir, wtPath, branch); err != nil {
			return err
		}
	}

	// Set upstream tracking if enabled and origin exists (skip for empty repos — nothing to push)
	if repoHasCommits && cfg.Checkout.ShouldSetUpstream() && git.HasRemote(ctx, gitDir, "origin") {
		if newBranch {
			// New branches: push to origin first, then set upstream
			if err := git.PushBranch(ctx, gitDir, branch); err != nil {
				l.Printf("Warning: failed to push branch: %v\n", err)
			} else if err := git.SetUpstreamBranch(ctx, gitDir, branch, branch); err != nil {
				l.Debug("failed to set upstream", "error", err)
			}
		} else if git.RemoteBranchExists(ctx, gitDir, branch) {
			// Existing branches: only set upstream if remote branch exists
			if err := git.SetUpstreamBranch(ctx, gitDir, branch, branch); err != nil {
				l.Debug("failed to set upstream", "error", err)
			}
		}
	}

	fmt.Printf("Created worktree: %s (%s)\n", wtPath, branch)

	// Record to history for wt cd
	if err := history.RecordAccess(wtPath, repo.Name, branch, cfg.GetHistoryPath()); err != nil {
		l.Printf("Warning: failed to record history: %v\n", err)
	}

	// Apply stashed changes to new worktree
	if stashed {
		if err := git.StashPop(ctx, wtPath); err != nil {
			l.Printf("Warning: failed to apply stashed changes: %v\n", err)
		}
	}

	// Set note if provided
	if note != "" {
		if err := git.SetBranchNote(ctx, gitDir, branch, note); err != nil {
			l.Printf("Warning: failed to set note: %v\n", err)
		}
	}

	// Preserve git-ignored files from an existing worktree (prefers default branch worktree)
	if !noPreserve && len(cfg.Preserve.Patterns) > 0 {
		sourceWT, err := preserve.FindSourceWorktree(ctx, gitDir, wtPath)
		if err != nil {
			if errors.Is(err, preserve.ErrNoSourceWorktree) {
				l.Debug("preserve: no source worktree found")
			} else {
				l.Printf("Warning: preserve: %v\n", err)
			}
		} else {
			copied, err := preserve.PreserveFiles(ctx, cfg.Preserve, sourceWT, wtPath)
			if err != nil {
				l.Printf("Warning: preserve files failed: %v\n", err)
			} else if len(copied) > 0 {
				l.Printf("Preserved %d file(s) from %s\n", len(copied), filepath.Base(sourceWT))
				for _, f := range copied {
					l.Debug("  preserved", "file", f)
				}
			}
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
func resolveWorktreePath(repo registry.Repo, branch, format string) string {
	return resolveWorktreePathWithConfig(repo.Path, repo.Name, branch, format)
}

// completeHooks provides completion for hook flags
func completeHooks(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg := config.FromContext(cmd.Context())
	var hooks []string
	for name := range cfg.Hooks.Hooks {
		hooks = append(hooks, name)
	}
	return hooks, cobra.ShellCompDirectiveNoFileComp
}

// runCheckoutWizard runs the interactive checkout wizard
func runCheckoutWizard(ctx context.Context, reg *registry.Registry, cliHooks []string, cliNoHook bool) (flows.CheckoutOptions, error) {
	cfg := config.FromContext(ctx)
	l := log.FromContext(ctx)

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
	}

	return flows.CheckoutInteractive(params)
}

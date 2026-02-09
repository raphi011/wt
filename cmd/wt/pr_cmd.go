package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
)

func newPrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Short:   "Work with PRs",
		GroupID: GroupPR,
		Long:    `Work with pull requests.`,
		Example: `  wt pr checkout 123                # Checkout PR from current repo
  wt pr checkout myrepo 123         # Checkout PR from local repo
  wt pr checkout org/repo 123       # Clone repo and checkout PR
  wt pr create --title "Add feature"
  wt pr merge
  wt pr view`,
	}

	cmd.AddCommand(newPrCheckoutCmd())
	cmd.AddCommand(newPrCreateCmd())
	cmd.AddCommand(newPrMergeCmd())
	cmd.AddCommand(newPrViewCmd())

	return cmd
}

func newPrCheckoutCmd() *cobra.Command {
	var (
		forgeName string
		note      string
		hookNames []string
		noHook    bool
		env       []string
	)

	cmd := &cobra.Command{
		Use:     "checkout [repo] <number>",
		Short:   "Checkout PR (clones if needed)",
		Aliases: []string{"co"},
		Args:    cobra.RangeArgs(1, 2),
		Long: `Checkout a PR, cloning the repo as a bare repo if it doesn't exist locally.

If repo contains '/', it's treated as org/repo and cloned from GitHub/GitLab.
Otherwise, it's looked up in the local registry.`,
		Example: `  wt pr checkout 123               # PR from current repo
  wt pr checkout myrepo 123        # PR from local repo in registry
  wt pr checkout org/repo 123      # Clone repo and checkout PR`,
		ValidArgsFunction: completePrCheckoutArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)

			// Parse arguments: [repo] <number>
			var prNumber int
			var repoArg string
			if len(args) == 1 {
				// Just PR number
				num, err := strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("invalid PR number: %s", args[0])
				}
				prNumber = num
			} else {
				// repo + PR number
				repoArg = args[0]
				num, err := strconv.Atoi(args[1])
				if err != nil {
					return fmt.Errorf("invalid PR number: %s", args[1])
				}
				prNumber = num
			}

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine target repo
			var repo registry.Repo
			var repoPath string
			var f forge.Forge

			if repoArg != "" && strings.Contains(repoArg, "/") {
				// Clone mode: org/repo format
				orgRepo := repoArg

				// Check if already exists in registry
				parts := strings.Split(orgRepo, "/")
				repoName := parts[len(parts)-1]
				if existing, err := reg.FindByName(repoName); err == nil {
					return fmt.Errorf("repository %q already exists at %s\nUse 'wt pr checkout %s %d' instead", repoName, existing.Path, repoName, prNumber)
				}

				// Get forge
				if forgeName == "" {
					forgeName = cfg.Forge.GetForgeTypeForRepo(orgRepo)
				}
				f = forge.ByNameWithConfig(forgeName, &cfg.Forge)
				if err := f.Check(ctx); err != nil {
					return err
				}

				// Clone the repo as bare into .git directory
				cwd, _ := os.Getwd()
				l.Printf("Cloning %s (bare)...\n", orgRepo)
				repoPath, err = f.CloneBareRepo(ctx, orgRepo, cwd)
				if err != nil {
					return fmt.Errorf("failed to clone repo: %w", err)
				}

				// Register in registry
				repo = registry.Repo{
					Path:   repoPath,
					Name:   repoName,
					Labels: cfg.DefaultLabels,
				}
				if err := reg.Add(repo); err != nil {
					return fmt.Errorf("register repo: %w", err)
				}
				if err := reg.Save(cfg.RegistryPath); err != nil {
					return fmt.Errorf("save registry: %w", err)
				}

				l.Printf("✓ Cloned and registered: %s\n", repoPath)
			} else if repoArg != "" {
				// Local mode: look up in registry
				repo, err = reg.FindByName(repoArg)
				if err != nil {
					return fmt.Errorf("repository %q not found in registry", repoArg)
				}
				repoPath = repo.Path
			} else {
				// Current directory
				repo, err = findOrRegisterCurrentRepoFromContext(ctx, reg)
				if err != nil {
					return err
				}
				repoPath = repo.Path
			}

			// Get origin URL and detect forge
			originURL, err := git.GetOriginURL(ctx, repoPath)
			if err != nil {
				return fmt.Errorf("failed to get origin URL: %w", err)
			}

			if f == nil {
				f = forge.Detect(originURL, cfg.Hosts, &cfg.Forge)
				if err := f.Check(ctx); err != nil {
					return err
				}
			}

			// Get PR branch
			l.Printf("Fetching PR #%d...\n", prNumber)
			branch, err := f.GetPRBranch(ctx, originURL, prNumber)
			if err != nil {
				return fmt.Errorf("failed to get PR branch: %w", err)
			}

			l.Debug("pr checkout", "branch", branch, "repo", repoPath)

			// Get worktree format
			format := repo.GetEffectiveWorktreeFormat(cfg.Checkout.WorktreeFormat)
			wtPath := resolveWorktreePathWithConfig(repoPath, repo.Name, branch, format)

			// Detect repo type
			repoType, err := git.DetectRepoType(repoPath)
			if err != nil {
				return err
			}
			gitDir := git.GetGitDir(repoPath, repoType)

			// Fetch the branch
			if err := git.FetchBranch(ctx, gitDir, branch); err != nil {
				l.Printf("Warning: fetch failed: %v\n", err)
			}

			// Create worktree
			if err := git.CreateWorktree(ctx, gitDir, wtPath, branch); err != nil {
				return fmt.Errorf("create worktree: %w", err)
			}

			// Set upstream - branch was fetched so remote exists
			if cfg.Checkout.ShouldSetUpstream() {
				if err := git.SetUpstreamBranch(ctx, gitDir, branch, branch); err != nil {
					l.Debug("failed to set upstream", "error", err)
				}
			}

			// Cache PR info for the new worktree
			if cache, err := prcache.Load(); err == nil {
				prInfo, err := f.GetPRForBranch(ctx, originURL, branch)
				if err != nil {
					l.Debug("failed to fetch PR info", "branch", branch, "error", err)
				} else {
					cache.Set(filepath.Base(wtPath), prcache.FromForge(prInfo))
					if err := cache.Save(); err != nil {
						l.Printf("Warning: failed to save PR cache: %v\n", err)
					}
				}
			}

			fmt.Printf("Created worktree: %s (%s)\n", wtPath, branch)

			// Record to history for wt cd
			if err := history.RecordAccess(wtPath, repo.Name, branch, cfg.GetHistoryPath()); err != nil {
				l.Debug("failed to record history", "error", err)
			}

			// Set note if provided
			if note != "" {
				if err := git.SetBranchNote(ctx, gitDir, branch, note); err != nil {
					l.Printf("Warning: failed to set note: %v\n", err)
				}
			}

			// Run hooks
			hookMatches, err := hooks.SelectHooks(cfg.Hooks, hookNames, noHook, hooks.CommandCheckout)
			if err != nil {
				return err
			}

			hookEnv, err := hooks.ParseEnvWithStdin(env)
			if err != nil {
				return err
			}

			hookCtx := hooks.Context{
				WorktreeDir: wtPath,
				RepoDir:     repoPath,
				Branch:      branch,
				Repo:        repo.Name,
				Origin:      repo.Name,
				Trigger:     string(hooks.CommandCheckout),
				Env:         hookEnv,
			}
			hooks.RunForEach(hookMatches, hookCtx, repoPath)

			return nil
		},
	}

	cmd.Flags().StringVar(&forgeName, "forge", "", "Forge type: github or gitlab")
	cmd.Flags().StringVar(&note, "note", "", "Set a note on the branch")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-checkout hook")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)
	cmd.RegisterFlagCompletionFunc("forge", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"github", "gitlab"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func newPrCreateCmd() *cobra.Command {
	var (
		title    string
		body     string
		bodyFile string
		base     string
		draft    bool
		web      bool
	)

	cmd := &cobra.Command{
		Use:               "create [repo]",
		Short:             "Create PR for worktree",
		Aliases:           []string{"c", "new"},
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRepoNames,
		Long:              `Create a PR for the current branch.`,
		Example: `  wt pr create --title "Add feature"
  wt pr create myrepo --title "Add feature"  # Create for specific repo
  wt pr create --title "Add feature" --body "Details"
  wt pr create --title "Add feature" --draft
  wt pr create --title "Add feature" -w      # Open in browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine target repo
			var repo registry.Repo
			if len(args) > 0 {
				repo, err = reg.FindByName(args[0])
				if err != nil {
					return fmt.Errorf("repository %q not found", args[0])
				}
			} else {
				repo, err = findOrRegisterCurrentRepoFromContext(ctx, reg)
				if err != nil {
					return err
				}
			}

			// Get current branch
			cwd, _ := os.Getwd()
			branch, err := git.GetCurrentBranch(ctx, cwd)
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}

			// Get origin URL and detect forge
			originURL, err := git.GetOriginURL(ctx, repo.Path)
			if err != nil {
				return fmt.Errorf("failed to get origin URL: %w", err)
			}

			f := forge.Detect(originURL, cfg.Hosts, &cfg.Forge)
			if err := f.Check(ctx); err != nil {
				return err
			}

			// Read body from file if specified
			prBody := body
			if bodyFile != "" {
				content, err := os.ReadFile(bodyFile)
				if err != nil {
					return fmt.Errorf("failed to read body file: %w", err)
				}
				prBody = string(content)
			}

			l.Debug("pr create", "title", title, "branch", branch, "base", base)

			// Push branch first
			l.Printf("Pushing branch %s...\n", branch)
			if err := git.RunGitCommand(ctx, repo.Path, "push", "-u", "origin", branch); err != nil {
				return fmt.Errorf("push failed: %w", err)
			}

			// Create PR
			l.Printf("Creating PR...\n")
			result, err := f.CreatePR(ctx, originURL, forge.CreatePRParams{
				Title: title,
				Body:  prBody,
				Base:  base,
				Head:  branch,
				Draft: draft,
			})
			if err != nil {
				return fmt.Errorf("create PR failed: %w", err)
			}

			out.Printf("Created PR #%d: %s\n", result.Number, result.URL)

			// Open in browser if requested
			if web {
				openBrowser(result.URL)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "PR title")
	cmd.Flags().StringVarP(&body, "body", "b", "", "PR body")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read body from file")
	cmd.Flags().StringVar(&base, "base", "", "Base branch")
	cmd.Flags().BoolVar(&draft, "draft", false, "Create as draft PR")
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open in browser after creation")

	cmd.MarkFlagRequired("title")
	cmd.MarkFlagFilename("body-file") // Enable file completion for body-file flag
	cmd.MarkFlagsMutuallyExclusive("body", "body-file")
	cmd.RegisterFlagCompletionFunc("base", completeBranches)

	return cmd
}

func newPrMergeCmd() *cobra.Command {
	var (
		strategy  string
		keep      bool
		hookNames []string
		noHook    bool
		env       []string
	)

	cmd := &cobra.Command{
		Use:               "merge [repo]",
		Short:             "Merge PR and clean up worktree",
		Aliases:           []string{"m"},
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRepoNames,
		Long: `Merge the PR for the current branch.

Merges the PR, removes the worktree (if applicable), and deletes the local branch.`,
		Example: `  wt pr merge                  # Merge current branch's PR
  wt pr merge myrepo           # Merge for specific repo
  wt pr merge --keep           # Keep worktree after merge
  wt pr merge -s rebase        # Use rebase strategy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine target repo
			var repo registry.Repo
			if len(args) > 0 {
				repo, err = reg.FindByName(args[0])
				if err != nil {
					return fmt.Errorf("repository %q not found", args[0])
				}
			} else {
				repo, err = findOrRegisterCurrentRepoFromContext(ctx, reg)
				if err != nil {
					return err
				}
			}

			// Get current branch and worktree path
			cwd, _ := os.Getwd()
			branch, err := git.GetCurrentBranch(ctx, cwd)
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}

			// Get origin URL and detect forge
			originURL, err := git.GetOriginURL(ctx, repo.Path)
			if err != nil {
				return fmt.Errorf("failed to get origin URL: %w", err)
			}

			f := forge.Detect(originURL, cfg.Hosts, &cfg.Forge)
			if err := f.Check(ctx); err != nil {
				return err
			}

			l.Debug("pr merge", "branch", branch, "strategy", strategy)

			// Get PR for branch
			pr, err := f.GetPRForBranch(ctx, originURL, branch)
			if err != nil {
				return fmt.Errorf("no PR found for branch %s: %w", branch, err)
			}

			// Load PR cache for updates
			cache, cacheErr := prcache.Load()
			if cacheErr != nil {
				l.Printf("Warning: failed to load PR cache: %v\n", cacheErr)
			}
			cacheKey := filepath.Base(cwd)

			if pr.State == forge.PRStateMerged {
				out.Printf("PR #%d is already merged\n", pr.Number)
			} else if pr.State == forge.PRStateClosed {
				return fmt.Errorf("PR #%d is closed", pr.Number)
			} else {
				// Merge the PR
				l.Printf("Merging PR #%d...\n", pr.Number)
				if err := f.MergePR(ctx, originURL, pr.Number, strategy); err != nil {
					return fmt.Errorf("merge failed: %w", err)
				}
				out.Printf("Merged PR #%d\n", pr.Number)

				// Update cache with merged state
				if cache != nil {
					pr.State = forge.PRStateMerged
					cache.Set(cacheKey, prcache.FromForge(pr))
					if err := cache.Save(); err != nil {
						l.Printf("Warning: failed to save PR cache: %v\n", err)
					}
				}
			}

			// Run hooks
			hookMatches, err := hooks.SelectHooks(cfg.Hooks, hookNames, noHook, hooks.CommandPrune)
			if err != nil {
				return err
			}

			hookEnv, err := hooks.ParseEnvWithStdin(env)
			if err != nil {
				return err
			}

			hookCtx := hooks.Context{
				WorktreeDir: cwd,
				RepoDir:     repo.Path,
				Branch:      branch,
				Repo:        repo.Name,
				Origin:      repo.Name,
				Trigger:     "merge",
				Env:         hookEnv,
			}
			hooks.RunForEach(hookMatches, hookCtx, repo.Path)

			// Remove worktree unless --keep
			if !keep {
				wtInfo, err := git.GetWorktreeInfo(ctx, cwd)
				if err == nil && wtInfo != nil {
					l.Printf("Removing worktree...\n")
					if err := git.RemoveWorktree(ctx, *wtInfo, false); err != nil {
						l.Printf("Warning: failed to remove worktree: %v\n", err)
					} else {
						out.Printf("Removed worktree: %s\n", cwd)
						// Remove from cache since worktree no longer exists
						cache.Delete(cacheKey)
						_ = cache.Save()
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "Merge strategy: squash, rebase, merge")
	cmd.Flags().BoolVarP(&keep, "keep", "k", false, "Keep worktree after merge")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-merge hook")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)
	cmd.RegisterFlagCompletionFunc("strategy", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"squash", "rebase", "merge"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func newPrViewCmd() *cobra.Command {
	var web bool

	cmd := &cobra.Command{
		Use:               "view [repo]",
		Short:             "View PR details or open in browser",
		Aliases:           []string{"v"},
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRepoNames,
		Long:              `View PR details for the current branch.`,
		Example: `  wt pr view              # View PR details
  wt pr view myrepo       # View PR for specific repo
  wt pr view -w           # Open PR in browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine target repo
			var repo registry.Repo
			if len(args) > 0 {
				repo, err = reg.FindByName(args[0])
				if err != nil {
					return fmt.Errorf("repository %q not found", args[0])
				}
			} else {
				repo, err = findOrRegisterCurrentRepoFromContext(ctx, reg)
				if err != nil {
					return err
				}
			}

			// Get current branch
			cwd, _ := os.Getwd()
			branch, err := git.GetCurrentBranch(ctx, cwd)
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}

			// Get origin URL and detect forge
			originURL, err := git.GetOriginURL(ctx, repo.Path)
			if err != nil {
				return fmt.Errorf("failed to get origin URL: %w", err)
			}

			f := forge.Detect(originURL, cfg.Hosts, &cfg.Forge)
			if err := f.Check(ctx); err != nil {
				return err
			}

			l.Debug("pr view", "branch", branch)

			// Get PR for branch
			pr, err := f.GetPRForBranch(ctx, originURL, branch)
			if err != nil {
				return fmt.Errorf("no PR found for branch %s", branch)
			}

			if web {
				openBrowser(pr.URL)
				return nil
			}

			// Display PR info
			out.Printf("PR #%d\n", pr.Number)
			out.Printf("State: %s\n", pr.State)
			out.Printf("URL: %s\n", pr.URL)
			if pr.Author != "" {
				out.Printf("Author: %s\n", pr.Author)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open in browser")

	return cmd
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) {
	var cmd string
	var args []string

	switch {
	case isWSL():
		cmd = "wslview"
		args = []string{url}
	case isMac():
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	_ = runCommand(cmd, args...)
}

func isWSL() bool {
	_, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop")
	return err == nil
}

func isMac() bool {
	return runtime.GOOS == "darwin"
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

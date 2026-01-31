package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

func newPrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Short:   "Work with PRs",
		GroupID: GroupPR,
		Long:    `Work with pull requests.`,
		Example: `  wt pr checkout 123                # Checkout PR from current repo
  wt pr checkout 123 org/repo       # Clone repo and checkout PR
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
		repository string
		forgeName  string
		note       string
		hookNames  []string
		noHook     bool
		env        []string
	)

	cmd := &cobra.Command{
		Use:     "checkout <number> [org/repo]",
		Short:   "Checkout PR (clones if needed)",
		Aliases: []string{"co"},
		Args:    cobra.RangeArgs(1, 2),
		Long:    `Checkout a PR, cloning the repo as a bare repo if it doesn't exist locally.`,
		Example: `  wt pr checkout 123               # PR from current repo
  wt pr checkout 123 -r myrepo     # PR from local repo
  wt pr checkout 123 org/repo      # Clone repo and checkout PR`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			// Parse arguments

			var prNumber int
			var orgRepo string
			if len(args) >= 1 {
				num, err := strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("invalid PR number: %s", args[0])
				}
				prNumber = num
			}
			if len(args) >= 2 {
				orgRepo = args[1]
			}

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine target repo
			var repo *registry.Repo
			var repoPath string
			var f forge.Forge

			if orgRepo != "" {
				// Clone mode: org/repo format
				if !strings.Contains(orgRepo, "/") {
					return fmt.Errorf("clone mode requires org/repo format (e.g., 'org/repo'), got %q", orgRepo)
				}

				// Check if already exists in registry
				parts := strings.Split(orgRepo, "/")
				repoName := parts[len(parts)-1]
				if existing, err := reg.FindByName(repoName); err == nil {
					return fmt.Errorf("repository %q already exists at %s\nUse 'wt pr checkout %d -r %s' instead", repoName, existing.Path, prNumber, repoName)
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
				repo = &registry.Repo{
					Path:   repoPath,
					Name:   repoName,
					Labels: cfg.DefaultLabels,
				}
				if err := reg.Add(*repo); err != nil {
					return fmt.Errorf("register repo: %w", err)
				}
				if err := reg.Save(); err != nil {
					return fmt.Errorf("save registry: %w", err)
				}

				l.Printf("✓ Cloned and registered: %s\n", repoPath)
			} else if repository != "" {
				// Local mode with -r flag
				repo, err = reg.FindByName(repository)
				if err != nil {
					return fmt.Errorf("repository %q not found", repository)
				}
				repoPath = repo.Path
			} else {
				// Current directory
				repo, err = findOrRegisterCurrentRepo(ctx, reg)
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
			format := repo.GetEffectiveWorktreeFormat(cfg.WorktreeFormat)
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

			fmt.Printf("Created worktree: %s (%s)\n", wtPath, branch)

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

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Local repo name")
	cmd.Flags().StringVar(&forgeName, "forge", "", "Forge type: github or gitlab")
	cmd.Flags().StringVar(&note, "note", "", "Set a note on the branch")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-checkout hook")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)
	cmd.RegisterFlagCompletionFunc("forge", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"github", "gitlab"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func newPrCreateCmd() *cobra.Command {
	var (
		repository string
		title      string
		body       string
		bodyFile   string
		base       string
		draft      bool
		web        bool
	)

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create PR for worktree",
		Aliases: []string{"c", "new"},
		Args:    cobra.NoArgs,
		Long:    `Create a PR for the current branch.`,
		Example: `  wt pr create --title "Add feature"
  wt pr create --title "Add feature" --body "Details"
  wt pr create --title "Add feature" --draft
  wt pr create --title "Add feature" -w    # Open in browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine target repo
			var repo *registry.Repo
			if repository != "" {
				repo, err = reg.FindByName(repository)
				if err != nil {
					return fmt.Errorf("repository %q not found", repository)
				}
			} else {
				repo, err = findOrRegisterCurrentRepo(ctx, reg)
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

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.Flags().StringVarP(&title, "title", "t", "", "PR title")
	cmd.Flags().StringVarP(&body, "body", "b", "", "PR body")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read body from file")
	cmd.Flags().StringVar(&base, "base", "", "Base branch")
	cmd.Flags().BoolVar(&draft, "draft", false, "Create as draft PR")
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open in browser after creation")

	cmd.MarkFlagRequired("title")
	cmd.MarkFlagFilename("body-file") // Enable file completion for body-file flag
	cmd.MarkFlagsMutuallyExclusive("body", "body-file")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("base", completeBranches)

	return cmd
}

func newPrMergeCmd() *cobra.Command {
	var (
		repository string
		strategy   string
		keep       bool
		hookNames  []string
		noHook     bool
		env        []string
	)

	cmd := &cobra.Command{
		Use:     "merge",
		Short:   "Merge PR and clean up worktree",
		Aliases: []string{"m"},
		Args:    cobra.NoArgs,
		Long: `Merge the PR for the current branch.

Merges the PR, removes the worktree (if applicable), and deletes the local branch.`,
		Example: `  wt pr merge                  # Merge current branch's PR
  wt pr merge --keep           # Keep worktree after merge
  wt pr merge -s rebase        # Use rebase strategy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine target repo
			var repo *registry.Repo
			if repository != "" {
				repo, err = reg.FindByName(repository)
				if err != nil {
					return fmt.Errorf("repository %q not found", repository)
				}
			} else {
				repo, err = findOrRegisterCurrentRepo(ctx, reg)
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

			if pr.State == "MERGED" {
				out.Printf("PR #%d is already merged\n", pr.Number)
			} else if pr.State == "CLOSED" {
				return fmt.Errorf("PR #%d is closed", pr.Number)
			} else {
				// Merge the PR
				l.Printf("Merging PR #%d...\n", pr.Number)
				if err := f.MergePR(ctx, originURL, pr.Number, strategy); err != nil {
					return fmt.Errorf("merge failed: %w", err)
				}
				out.Printf("Merged PR #%d\n", pr.Number)
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
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "Merge strategy: squash, rebase, merge")
	cmd.Flags().BoolVarP(&keep, "keep", "k", false, "Keep worktree after merge")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-merge hook")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)
	cmd.RegisterFlagCompletionFunc("strategy", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"squash", "rebase", "merge"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func newPrViewCmd() *cobra.Command {
	var (
		repository string
		web        bool
	)

	cmd := &cobra.Command{
		Use:     "view",
		Short:   "View PR details or open in browser",
		Aliases: []string{"v"},
		Args:    cobra.NoArgs,
		Long:    `View PR details for the current branch.`,
		Example: `  wt pr view              # View PR details
  wt pr view -w           # Open PR in browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine target repo
			var repo *registry.Repo
			if repository != "" {
				repo, err = reg.FindByName(repository)
				if err != nil {
					return fmt.Errorf("repository %q not found", repository)
				}
			} else {
				repo, err = findOrRegisterCurrentRepo(ctx, reg)
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

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open in browser")

	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

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

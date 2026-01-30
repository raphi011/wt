package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newPrCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Work with PRs",
		Long: `Work with pull requests.

Examples:
  wt pr checkout 123                # Checkout PR from current repo
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
		repository  string
		forge       string
		note        string
		hooks       []string
		noHook      bool
		env         []string
		interactive bool
	)

	cmd := &cobra.Command{
		Use:   "checkout <number> [org/repo]",
		Short: "Checkout PR (clones if needed)",
		Long: `Checkout a PR, cloning the repo as a bare repo if it doesn't exist locally.

Examples:
  wt pr checkout 123                    # PR from current repo
  wt pr checkout 123 -r myrepo          # PR from local repo
  wt pr checkout 123 org/repo           # Clone repo and checkout PR
  wt pr checkout -i                     # Interactive mode`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			l.Debug("pr checkout", "args", args)

			// TODO: Implement PR checkout logic
			_ = reg
			_ = repository
			_ = forge
			_ = note
			_ = hooks
			_ = noHook
			_ = env
			_ = interactive

			fmt.Println("PR checkout not yet fully implemented")
			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Local repo name")
	cmd.Flags().StringVar(&forge, "forge", "", "Forge type: github or gitlab")
	cmd.Flags().StringVar(&note, "note", "", "Set a note on the branch")
	cmd.Flags().StringSliceVar(&hooks, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-checkout hook")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)

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
		Use:   "create",
		Short: "Create PR for worktree",
		Long: `Create a PR for the current branch.

Examples:
  wt pr create --title "Add feature"
  wt pr create --title "Add feature" --body "Details"
  wt pr create --title "Add feature" --draft
  wt pr create --title "Add feature" -w    # Open in browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			l.Debug("pr create", "title", title)

			// TODO: Implement PR create logic
			_ = reg
			_ = repository
			_ = body
			_ = bodyFile
			_ = base
			_ = draft
			_ = web

			fmt.Println("PR create not yet fully implemented")
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
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

func newPrMergeCmd() *cobra.Command {
	var (
		repository string
		strategy   string
		keep       bool
		hooks      []string
		noHook     bool
		env        []string
	)

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge PR and clean up worktree",
		Long: `Merge the PR for the current branch.

Merges the PR, removes the worktree (if applicable), and deletes the local branch.

Examples:
  wt pr merge                  # Merge current branch's PR
  wt pr merge --keep           # Keep worktree after merge
  wt pr merge -s rebase        # Use rebase strategy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			l.Debug("pr merge", "strategy", strategy)

			// TODO: Implement PR merge logic
			_ = reg
			_ = repository
			_ = keep
			_ = hooks
			_ = noHook
			_ = env

			fmt.Println("PR merge not yet fully implemented")
			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.Flags().StringVarP(&strategy, "strategy", "s", "", "Merge strategy: squash, rebase, merge")
	cmd.Flags().BoolVarP(&keep, "keep", "k", false, "Keep worktree after merge")
	cmd.Flags().StringSliceVar(&hooks, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-merge hook")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)

	return cmd
}

func newPrViewCmd() *cobra.Command {
	var (
		repository string
		web        bool
	)

	cmd := &cobra.Command{
		Use:   "view",
		Short: "View PR details or open in browser",
		Long: `View PR details for the current branch.

Examples:
  wt pr view              # View PR details
  wt pr view -w           # Open PR in browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			l.Debug("pr view", "web", web)

			// TODO: Implement PR view logic
			_ = reg
			_ = repository

			fmt.Println("PR view not yet fully implemented")
			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open in browser")

	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newPruneCmd() *cobra.Command {
	var (
		dryRun      bool
		force       bool
		verbose     bool
		global      bool
		refresh     bool
		resetCache  bool
		hooks       []string
		noHook      bool
		env         []string
		interactive bool
	)

	cmd := &cobra.Command{
		Use:     "prune",
		Short:   "Prune merged worktrees",
		Aliases: []string{"p"},
		Long: `Remove worktrees with merged PRs.

Without flags, removes all worktrees with merged PRs.
Use --interactive to select worktrees to prune.

Examples:
  wt prune                    # Remove worktrees with merged PRs
  wt prune --global           # Prune all repos
  wt prune -d                 # Dry-run: preview without removing
  wt prune -i                 # Interactive mode`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			l.Debug("pruning worktrees", "global", global, "dryRun", dryRun)

			// TODO: Implement prune logic
			_ = reg
			_ = force
			_ = verbose
			_ = refresh
			_ = resetCache
			_ = hooks
			_ = noHook
			_ = env
			_ = interactive

			fmt.Println("Prune command not yet implemented")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Preview without removing")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force remove")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show non-prunable worktrees")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Prune all repos")
	cmd.Flags().BoolVarP(&refresh, "refresh", "R", false, "Refresh PR status first")
	cmd.Flags().BoolVar(&resetCache, "reset-cache", false, "Clear all cached data")
	cmd.Flags().StringSliceVar(&hooks, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip post-removal hooks")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")

	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")

	// Completions
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)

	return cmd
}

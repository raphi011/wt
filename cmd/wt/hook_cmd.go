package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newHookCmd() *cobra.Command {
	var (
		repository []string
		labels     []string
		env        []string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "hook <name>...",
		Short: "Run configured hook",
		Args:  cobra.MinimumNArgs(1),
		Long: `Run one or more configured hooks.

Hooks are defined in config.toml and can use placeholders.

Examples:
  wt hook code                # Run 'code' hook in current repo
  wt hook code -r myrepo      # Run in specific repo
  wt hook code idea           # Run multiple hooks
  wt hook code -d             # Dry-run: print command without executing`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			hookNames := args

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Resolve target repos
			repos, err := resolveTargetRepos(ctx, reg, repository, labels)
			if err != nil {
				return err
			}

			l.Debug("running hooks", "hooks", hookNames, "repos", len(repos))

			// TODO: Implement hook execution
			_ = env
			_ = dryRun

			fmt.Println("Hook command not yet fully implemented")
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Repository name(s)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Target repos by label")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Print command without executing")

	// Completions
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("label", completeLabels)
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)

	return cmd
}

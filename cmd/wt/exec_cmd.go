package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newExecCmd() *cobra.Command {
	var (
		repository []string
		labels     []string
	)

	cmd := &cobra.Command{
		Use:     "exec [flags] -- <command>",
		Short:   "Run command in worktree(s) or repo(s)",
		Aliases: []string{"x"},
		GroupID: GroupUtility,
		Long: `Run a command in one or more worktrees or repos.

Use -r to target repos by name, -l to target repos by label.

Examples:
  wt exec -r myrepo -- git status        # In specific repo
  wt exec -l backend -- make test        # In repos with label
  wt exec -r repo1 -r repo2 -- npm install`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			if len(args) == 0 {
				return fmt.Errorf("no command specified (use -- before command)")
			}

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

			if len(repos) == 0 {
				return fmt.Errorf("no repos found")
			}

			l.Debug("exec", "command", args[0], "repos", len(repos))

			// Execute command in each repo
			for _, repo := range repos {
				fmt.Printf("=== %s ===\n", repo.Name)

				execCmd := exec.CommandContext(ctx, args[0], args[1:]...)
				execCmd.Dir = repo.Path
				execCmd.Stdout = os.Stdout
				execCmd.Stderr = os.Stderr
				execCmd.Stdin = os.Stdin

				if err := execCmd.Run(); err != nil {
					l.Printf("Error in %s: %v\n", repo.Name, err)
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Repository name(s)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Target repos by label")

	// Completions
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("label", completeLabels)

	return cmd
}

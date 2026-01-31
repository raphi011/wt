package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

func newLabelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "label",
		Short:   "Manage repository labels",
		GroupID: GroupUtility,
		Long: `Manage labels on repositories.

Labels are stored in the registry and can be used to target repos with -l flag.`,
		Example: `  wt label add backend           # Add label to current repo
  wt label add backend -r api    # Add label to specific repo
  wt label remove backend        # Remove label
  wt label list                  # List labels`,
	}

	cmd.AddCommand(newLabelAddCmd())
	cmd.AddCommand(newLabelRemoveCmd())
	cmd.AddCommand(newLabelListCmd())
	cmd.AddCommand(newLabelClearCmd())

	return cmd
}

func newLabelAddCmd() *cobra.Command {
	var repository []string

	cmd := &cobra.Command{
		Use:               "add <label>",
		Short:             "Add a label to a repository",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeLabels,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			label := args[0]

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get target repos
			repos, err := resolveTargetReposForLabel(ctx, reg, repository)
			if err != nil {
				return err
			}

			for _, repo := range repos {
				if err := reg.AddLabel(repo.Name, label); err != nil {
					return fmt.Errorf("%s: %w", repo.Name, err)
				}
				fmt.Printf("Added label %q to %s\n", label, repo.Name)
			}

			return reg.Save()
		},
	}

	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Repository name(s)")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

func newLabelRemoveCmd() *cobra.Command {
	var repository []string

	cmd := &cobra.Command{
		Use:               "remove <label>",
		Short:             "Remove a label from a repository",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeLabels,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			label := args[0]

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get target repos
			repos, err := resolveTargetReposForLabel(ctx, reg, repository)
			if err != nil {
				return err
			}

			for _, repo := range repos {
				if err := reg.RemoveLabel(repo.Name, label); err != nil {
					return fmt.Errorf("%s: %w", repo.Name, err)
				}
				fmt.Printf("Removed label %q from %s\n", label, repo.Name)
			}

			return reg.Save()
		},
	}

	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Repository name(s)")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

func newLabelListCmd() *cobra.Command {
	var (
		repository []string
		global     bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List labels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			if global {
				// List all labels
				labels := reg.AllLabels()
				if len(labels) == 0 {
					fmt.Println("No labels defined")
					return nil
				}
				for _, l := range labels {
					out.Println(l)
				}
				return nil
			}

			// Get target repos
			repos, err := resolveTargetReposForLabel(ctx, reg, repository)
			if err != nil {
				return err
			}

			for _, repo := range repos {
				if len(repos) > 1 {
					fmt.Printf("%s: ", repo.Name)
				}
				if len(repo.Labels) == 0 {
					fmt.Println("(no labels)")
				} else {
					out.Println(strings.Join(repo.Labels, ", "))
				}
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Repository name(s)")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "List all labels across repos")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

func newLabelClearCmd() *cobra.Command {
	var repository []string

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear all labels from a repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get target repos
			repos, err := resolveTargetReposForLabel(ctx, reg, repository)
			if err != nil {
				return err
			}

			for _, repo := range repos {
				if err := reg.ClearLabels(repo.Name); err != nil {
					return fmt.Errorf("%s: %w", repo.Name, err)
				}
				fmt.Printf("Cleared labels from %s\n", repo.Name)
			}

			return reg.Save()
		},
	}

	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Repository name(s)")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

// resolveTargetReposForLabel resolves repos for label commands
func resolveTargetReposForLabel(ctx context.Context, reg *registry.Registry, repoNames []string) ([]*registry.Repo, error) {
	if len(repoNames) > 0 {
		var repos []*registry.Repo
		for _, name := range repoNames {
			repo, err := reg.FindByName(name)
			if err != nil {
				return nil, err
			}
			repos = append(repos, repo)
		}
		return repos, nil
	}

	// Try current repo
	repoPath := git.GetCurrentRepoMainPath(ctx)
	if repoPath == "" {
		return nil, fmt.Errorf("not in a git repository (use -r to specify)")
	}

	repo, err := reg.FindByPath(repoPath)
	if err != nil {
		return nil, err
	}

	return []*registry.Repo{repo}, nil
}

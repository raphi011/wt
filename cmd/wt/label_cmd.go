package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

func newLabelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "label",
		Short:   "Manage repository labels",
		Aliases: []string{"lbl"},
		GroupID: GroupUtility,
		Long: `Manage labels on repositories.

Labels are stored in the registry and can be used to target repos.`,
		Example: `  wt label add backend           # Add label to current repo
  wt label add backend api       # Add label to specific repo
  wt label add backend mygroup   # Add label to repos with 'mygroup' label
  wt label remove backend        # Remove label from current repo
  wt label list                  # List labels for current repo
  wt label list -g               # List all labels`,
	}

	cmd.AddCommand(newLabelAddCmd())
	cmd.AddCommand(newLabelRemoveCmd())
	cmd.AddCommand(newLabelListCmd())
	cmd.AddCommand(newLabelClearCmd())

	return cmd
}

func newLabelAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <label> [scope...]",
		Short: "Add a label to repositories",
		Args:  cobra.MinimumNArgs(1),
		Long: `Add a label to one or more repositories.

If no scope is specified, adds to the current repository.
Scopes are resolved as repo name first, then label.`,
		Example: `  wt label add backend           # Add to current repo
  wt label add backend api       # Add to repo named 'api'
  wt label add backend frontend  # Add to repos with 'frontend' label`,
		ValidArgsFunction: completeLabelAddArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			label := args[0]
			scopes := args[1:]

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get target repos
			repos, err := resolveScopeArgsOrCurrent(ctx, reg, scopes)
			if err != nil {
				return err
			}

			for _, repo := range repos {
				if err := reg.AddLabel(repo.Name, label); err != nil {
					return fmt.Errorf("%s: %w", repo.Name, err)
				}
				fmt.Printf("Added label %q to %s\n", label, repo.Name)
			}

			return reg.Save(cfg.RegistryPath)
		},
	}

	return cmd
}

func newLabelRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <label> [scope...]",
		Short: "Remove a label from repositories",
		Args:  cobra.MinimumNArgs(1),
		Long: `Remove a label from one or more repositories.

If no scope is specified, removes from the current repository.
Scopes are resolved as repo name first, then label.`,
		Example: `  wt label remove backend           # Remove from current repo
  wt label remove backend api       # Remove from repo named 'api'
  wt label remove backend frontend  # Remove from repos with 'frontend' label`,
		ValidArgsFunction: completeLabelAddArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			label := args[0]
			scopes := args[1:]

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get target repos
			repos, err := resolveScopeArgsOrCurrent(ctx, reg, scopes)
			if err != nil {
				return err
			}

			for _, repo := range repos {
				if err := reg.RemoveLabel(repo.Name, label); err != nil {
					return fmt.Errorf("%s: %w", repo.Name, err)
				}
				fmt.Printf("Removed label %q from %s\n", label, repo.Name)
			}

			return reg.Save(cfg.RegistryPath)
		},
	}

	return cmd
}

func newLabelListCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "list [scope...]",
		Short: "List labels",
		Args:  cobra.ArbitraryArgs,
		Long: `List labels for repositories.

If no scope is specified, lists labels for the current repository.
Scopes are resolved as repo name first, then label.`,
		Example: `  wt label list           # List labels for current repo
  wt label list api       # List labels for repo named 'api'
  wt label list -g        # List all labels across repos`,
		ValidArgsFunction: completeScopeArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
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
			repos, err := resolveScopeArgsOrCurrent(ctx, reg, args)
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

	cmd.Flags().BoolVarP(&global, "global", "g", false, "List all labels across repos")

	return cmd
}

func newLabelClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear [scope...]",
		Short: "Clear all labels from repositories",
		Args:  cobra.ArbitraryArgs,
		Long: `Clear all labels from one or more repositories.

If no scope is specified, clears labels from the current repository.
Scopes are resolved as repo name first, then label.`,
		Example: `  wt label clear           # Clear labels from current repo
  wt label clear api       # Clear labels from repo named 'api'
  wt label clear frontend  # Clear labels from repos with 'frontend' label`,
		ValidArgsFunction: completeScopeArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get target repos
			repos, err := resolveScopeArgsOrCurrent(ctx, reg, args)
			if err != nil {
				return err
			}

			for _, repo := range repos {
				if err := reg.ClearLabels(repo.Name); err != nil {
					return fmt.Errorf("%s: %w", repo.Name, err)
				}
				fmt.Printf("Cleared labels from %s\n", repo.Name)
			}

			return reg.Save(cfg.RegistryPath)
		},
	}

	return cmd
}

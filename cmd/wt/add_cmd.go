package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newAddCmd() *cobra.Command {
	var (
		name           string
		worktreeFormat string
		labels         []string
	)

	cmd := &cobra.Command{
		Use:     "add <path>",
		Short:   "Register an existing repository",
		Aliases: []string{"a"},
		Args:    cobra.ExactArgs(1),
		Long: `Register an existing git repository with wt.

The repository will be added to the registry (~/.wt/repos.json) and can then
be managed with other wt commands.

Examples:
  wt add ~/work/my-project                    # Register with default name
  wt add ~/work/my-project -n myproj          # Custom display name
  wt add ~/work/my-project -l work -l api     # Add labels
  wt add ~/work/my-project -w "./{branch}"    # Custom worktree format`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			path := args[0]

			// Resolve to absolute path
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			// Verify it's a git repo
			repoType, err := git.DetectRepoType(absPath)
			if err != nil {
				return fmt.Errorf("not a git repository: %s", absPath)
			}

			// Use directory name as default name
			if name == "" {
				name = filepath.Base(absPath)
			}

			l.Debug("registering repo", "path", absPath, "name", name, "type", repoType)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Add repo
			repo := registry.Repo{
				Path:           absPath,
				Name:           name,
				WorktreeFormat: worktreeFormat,
				Labels:         labels,
			}

			if err := reg.Add(repo); err != nil {
				return err
			}

			// Save registry
			if err := reg.Save(); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			typeStr := "regular"
			if repoType == git.RepoTypeBare {
				typeStr = "bare"
			}
			fmt.Printf("Registered %s repo: %s (%s)\n", typeStr, name, absPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Display name (default: directory name)")
	cmd.Flags().StringVarP(&worktreeFormat, "worktree-format", "w", "", "Worktree format override")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Labels for grouping (repeatable)")

	// Completions
	cmd.RegisterFlagCompletionFunc("label", completeLabels)

	return cmd
}

// completeLabels provides completion for label flags
func completeLabels(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	reg, err := registry.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return reg.AllLabels(), cobra.ShellCompDirectiveNoFileComp
}

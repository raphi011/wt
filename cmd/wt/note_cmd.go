package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

func newNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Manage branch notes",
		Long: `Manage notes on branches.

Notes are stored in git config and displayed in list output.

Examples:
  wt note set "Working on login"    # Set note on current branch
  wt note get                       # Get note for current branch
  wt note clear                     # Clear note from current branch`,
	}

	cmd.AddCommand(newNoteSetCmd())
	cmd.AddCommand(newNoteGetCmd())
	cmd.AddCommand(newNoteClearCmd())

	return cmd
}

func newNoteSetCmd() *cobra.Command {
	var repository string

	cmd := &cobra.Command{
		Use:   "set <text>",
		Short: "Set a note on a branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			text := args[0]

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get repo and branch
			repo, branch, err := resolveRepoBranch(ctx, reg, repository)
			if err != nil {
				return err
			}

			// Set note
			if err := git.SetBranchNote(ctx, repo.Path, branch, text); err != nil {
				return err
			}

			fmt.Printf("Note set on %s/%s\n", repo.Name, branch)
			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

func newNoteGetCmd() *cobra.Command {
	var repository string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get the note for a branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get repo and branch
			repo, branch, err := resolveRepoBranch(ctx, reg, repository)
			if err != nil {
				return err
			}

			// Get note
			note, err := git.GetBranchNote(ctx, repo.Path, branch)
			if err != nil {
				return err
			}

			if note != "" {
				out.Println(note)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

func newNoteClearCmd() *cobra.Command {
	var repository string

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the note from a branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get repo and branch
			repo, branch, err := resolveRepoBranch(ctx, reg, repository)
			if err != nil {
				return err
			}

			// Clear note
			if err := git.ClearBranchNote(ctx, repo.Path, branch); err != nil {
				return err
			}

			fmt.Printf("Note cleared on %s/%s\n", repo.Name, branch)
			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

// resolveRepoBranch gets the repo and current branch
func resolveRepoBranch(ctx context.Context, reg *registry.Registry, repository string) (*registry.Repo, string, error) {
	var repo *registry.Repo
	var err error

	if repository != "" {
		repo, err = reg.FindByName(repository)
		if err != nil {
			return nil, "", err
		}
	} else {
		repoPath := git.GetCurrentRepoMainPath(ctx)
		if repoPath == "" {
			return nil, "", fmt.Errorf("not in a git repository")
		}
		repo, err = reg.FindByPath(repoPath)
		if err != nil {
			return nil, "", err
		}
	}

	branch, err := git.GetCurrentBranch(ctx, repo.Path)
	if err != nil {
		return nil, "", err
	}

	return repo, branch, nil
}

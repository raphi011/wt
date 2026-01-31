package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

func newMigrateCmd() *cobra.Command {
	var (
		name           string
		labels         []string
		worktreeFormat string
		dryRun         bool
	)

	cmd := &cobra.Command{
		Use:     "migrate [path]",
		Short:   "Migrate a regular repo to bare-in-.git structure",
		GroupID: GroupRegistry,
		Args:    cobra.MaximumNArgs(1),
		Long: `Migrate an existing normal git repository into the bare repo structure used by 'wt clone'.

Before migration:
  myrepo/
  ├── .git/           (regular git directory)
  ├── src/
  └── README.md

After migration:
  myrepo/
  ├── .git/           (bare repo)
  │   └── worktrees/
  └── main/           (original working tree moved here)
      ├── .git        (file pointing to ../.git/worktrees/main)
      ├── src/
      └── README.md

The migration:
- Preserves all uncommitted changes and untracked files
- Converts the .git directory to a bare repository
- Moves all working tree files into a subdirectory named after the current branch
- Updates any existing worktrees to work with the new structure
- Registers the repository in the wt registry`,
		Example: `  wt migrate                  # Migrate repo in current directory
  wt migrate ./myrepo         # Migrate repo at path
  wt migrate -n myapp         # Migrate with custom display name
  wt migrate -l backend       # Migrate with labels
  wt migrate --dry-run        # Preview migration without making changes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Determine path to migrate
			repoPath := "."
			if len(args) > 0 {
				repoPath = args[0]
			}

			// Resolve to absolute path
			absPath, err := filepath.Abs(repoPath)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			l.Debug("validating migration", "path", absPath)

			// Validate and get migration plan
			plan, err := git.ValidateMigration(ctx, absPath)
			if err != nil {
				return err
			}

			// Determine display name
			repoName := name
			if repoName == "" {
				repoName = filepath.Base(absPath)
			}

			// Check if already registered
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			if _, err := reg.FindByPath(absPath); err == nil {
				return fmt.Errorf("repo already registered: %s", absPath)
			}

			// Check for name conflicts
			if _, err := reg.FindByName(repoName); err == nil {
				return fmt.Errorf("repo name already exists: %s", repoName)
			}

			// Show migration plan
			out.Printf("Migration plan for: %s\n\n", absPath)
			out.Printf("  Current branch: %s\n", plan.CurrentBranch)
			out.Printf("  Main worktree will be at: %s/%s/\n", absPath, plan.CurrentBranch)

			if len(plan.WorktreesToFix) > 0 {
				out.Printf("\n  Existing worktrees:\n")
				for _, wt := range plan.WorktreesToFix {
					if wt.NeedsMove {
						out.Printf("    %s → %s\n", wt.OldName, wt.NewName)
					} else if wt.IsOutside {
						out.Printf("    %s (outside repo, links will be updated)\n", wt.OldName)
					} else {
						out.Printf("    %s (links will be updated)\n", wt.OldName)
					}
				}
			}

			out.Printf("\n  Registry name: %s\n", repoName)
			if len(labels) > 0 {
				out.Printf("  Labels: %v\n", labels)
			}

			if dryRun {
				out.Printf("\n  (dry run - no changes made)\n")
				return nil
			}

			out.Printf("\n")

			// Perform migration
			l.Debug("performing migration")
			result, err := git.MigrateToBare(ctx, plan)
			if err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}

			// Register the repo
			repo := registry.Repo{
				Path:           absPath,
				Name:           repoName,
				WorktreeFormat: worktreeFormat,
				Labels:         labels,
			}

			if err := reg.Add(repo); err != nil {
				return fmt.Errorf("register repo: %w", err)
			}

			if err := reg.Save(); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			out.Printf("Migration complete!\n")
			out.Printf("  Main worktree: %s\n", result.MainWorktreePath)
			out.Printf("  Registered as: %s\n", repoName)

			// Verify by listing worktrees
			worktrees, err := git.ListWorktreesFromRepo(ctx, absPath)
			if err == nil && len(worktrees) > 0 {
				out.Printf("\n  Worktrees:\n")
				for _, wt := range worktrees {
					out.Printf("    %s (%s)\n", wt.Path, wt.Branch)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Display name (default: directory name)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Labels for grouping (repeatable)")
	cmd.Flags().StringVarP(&worktreeFormat, "worktree-format", "w", "", "Worktree format override")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Preview migration without making changes")

	cmd.RegisterFlagCompletionFunc("label", completeLabels)

	// Path argument should complete directories only
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveFilterDirs
	}

	return cmd
}

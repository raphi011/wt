package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newMigrateCmd() *cobra.Command {
	var (
		worktreeDir string
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "migrate <repo-dir>",
		Short: "Import repos from old wt setup",
		Args:  cobra.ExactArgs(1),
		Long: `Import existing repos from an old wt directory structure.

Scans the given directory for git repos and registers them.

Examples:
  wt migrate ~/Git                      # Import repos from directory
  wt migrate ~/Git -d                   # Preview without importing
  wt migrate ~/Git --worktree-dir ~/Git/worktrees  # With separate worktree dir`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			repoDir := args[0]

			// Verify directory exists
			info, err := os.Stat(repoDir)
			if err != nil {
				return fmt.Errorf("directory not found: %s", repoDir)
			}
			if !info.IsDir() {
				return fmt.Errorf("not a directory: %s", repoDir)
			}

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Scan for repos
			entries, err := os.ReadDir(repoDir)
			if err != nil {
				return fmt.Errorf("read directory: %w", err)
			}

			var imported int
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				repoPath := filepath.Join(repoDir, entry.Name())

				// Check if it's a git repo
				repoType, err := git.DetectRepoType(repoPath)
				if err != nil {
					continue // Not a git repo
				}

				// Check if already registered
				if _, err := reg.FindByPath(repoPath); err == nil {
					l.Printf("Skipping %s (already registered)\n", entry.Name())
					continue
				}

				typeStr := "regular"
				if repoType == git.RepoTypeBare {
					typeStr = "bare"
				}

				if dryRun {
					fmt.Printf("Would import: %s (%s)\n", entry.Name(), typeStr)
				} else {
					repo := registry.Repo{
						Path: repoPath,
						Name: entry.Name(),
					}

					if err := reg.Add(repo); err != nil {
						l.Printf("Warning: %s: %v\n", entry.Name(), err)
						continue
					}

					fmt.Printf("Imported: %s (%s)\n", entry.Name(), typeStr)
					imported++
				}
			}

			if !dryRun && imported > 0 {
				if err := reg.Save(); err != nil {
					return fmt.Errorf("save registry: %w", err)
				}
				fmt.Printf("\nImported %d repos\n", imported)
			}

			_ = worktreeDir
			return nil
		},
	}

	cmd.Flags().StringVar(&worktreeDir, "worktree-dir", "", "Current worktree directory (optional)")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Preview without importing")

	return cmd
}

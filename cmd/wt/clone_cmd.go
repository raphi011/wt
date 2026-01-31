package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newCloneCmd() *cobra.Command {
	var (
		name           string
		labels         []string
		worktreeFormat string
		destination    string
		branch         string
	)

	cmd := &cobra.Command{
		Use:     "clone <url> [destination]",
		Short:   "Clone a repository as bare",
		Aliases: []string{"cl"},
		GroupID: GroupRegistry,
		Args:    cobra.RangeArgs(1, 2),
		Long: `Clone a git repository as bare and register it.

Clones directly into .git (no working tree):
  repo/
  └── .git/    # bare git repo contents (HEAD, objects/, refs/, etc.)

This allows worktrees to be created as siblings to .git.
Use -b to create an initial worktree for a branch.

If destination is not specified, clones into the current directory.`,
		Example: `  wt clone https://github.com/org/repo           # Clone to ./repo
  wt clone https://github.com/org/repo myrepo    # Clone to ./myrepo
  wt clone https://github.com/org/repo -b main   # Clone and create worktree for main
  wt clone git@github.com:org/repo.git -l work   # Clone with label`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			url := args[0]

			// Determine destination
			dest := destination
			if len(args) > 1 {
				dest = args[1]
			}
			if dest == "" {
				// Extract repo name from URL
				dest = extractRepoNameFromURL(url)
			}

			// Resolve to absolute path
			absPath, err := filepath.Abs(dest)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			// Check if directory already exists
			if _, err := os.Stat(absPath); err == nil {
				return fmt.Errorf("destination already exists: %s", absPath)
			}

			l.Debug("cloning repo", "url", url, "dest", absPath)

			// Clone the repository as bare
			if err := git.CloneBareWithWorktreeSupport(ctx, url, absPath); err != nil {
				return fmt.Errorf("clone failed: %w", err)
			}

			// Determine display name
			repoName := name
			if repoName == "" {
				repoName = filepath.Base(absPath)
			}

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Register the repo
			repo := registry.Repo{
				Path:           absPath,
				Name:           repoName,
				WorktreeFormat: worktreeFormat,
				Labels:         labels,
			}

			if err := reg.Add(repo); err != nil {
				// Clean up on failure
				os.RemoveAll(absPath)
				return fmt.Errorf("register repo: %w", err)
			}

			if err := reg.Save(); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			fmt.Printf("Cloned repo: %s (%s)\n", repoName, absPath)

			// Create an initial worktree if branch specified
			if branch != "" {
				format := worktreeFormat
				if format == "" {
					format = cfg.WorktreeFormat
				}
				wtPath := resolveWorktreePathWithConfig(absPath, repoName, branch, format)

				l.Debug("creating initial worktree", "path", wtPath, "branch", branch)

				gitDir := filepath.Join(absPath, ".git")
				if err := git.CreateWorktree(ctx, gitDir, wtPath, branch); err != nil {
					l.Printf("Warning: failed to create initial worktree: %v\n", err)
				} else {
					fmt.Printf("Created worktree: %s (%s)\n", wtPath, branch)
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Display name (default: directory name)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Labels for grouping (repeatable)")
	cmd.Flags().StringVarP(&worktreeFormat, "worktree-format", "w", "", "Worktree format override")
	cmd.Flags().StringVarP(&destination, "destination", "d", "", "Destination directory")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Create initial worktree for branch")

	cmd.RegisterFlagCompletionFunc("label", completeLabels)
	cmd.MarkFlagDirname("destination")

	return cmd
}

// extractRepoNameFromURL extracts the repository name from a git URL
func extractRepoNameFromURL(url string) string {
	// Remove trailing .git
	url = strings.TrimSuffix(url, ".git")

	// Handle SSH URLs (git@github.com:org/repo)
	if strings.Contains(url, ":") && !strings.Contains(url, "://") {
		parts := strings.Split(url, ":")
		if len(parts) == 2 {
			pathParts := strings.Split(parts[1], "/")
			return pathParts[len(pathParts)-1]
		}
	}

	// Handle HTTPS URLs
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/prompt"
	"github.com/raphi011/wt/internal/ui/static"
)

func newReposCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repos",
		Short:   "Manage registered repositories",
		Aliases: []string{"r"},
		GroupID: GroupRegistry,
		Long: `Manage registered repositories.

Use subcommands to list, add, clone, or remove repositories from the registry.`,
		Example: `  wt repos list                  # List all repos
  wt repos add ~/work/my-project # Register a repo
  wt repos clone <url>           # Clone and register a repo
  wt repos remove my-project     # Unregister a repo`,
	}

	// Add subcommands
	cmd.AddCommand(newReposListCmd())
	cmd.AddCommand(newReposAddCmd())
	cmd.AddCommand(newReposCloneCmd())
	cmd.AddCommand(newReposRemoveCmd())

	return cmd
}

func newReposListCmd() *cobra.Command {
	var (
		label      string
		sortBy     string
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List registered repositories",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Long: `List all registered repositories.

Shows name, path, type (bare/regular), worktree format, and labels.`,
		Example: `  wt repos list                  # List all repos
  wt repos list -l backend       # Filter by label
  wt repos list --json           # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := output.FromContext(cmd.Context())

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Filter by label if specified
			var repos []registry.Repo
			if label != "" {
				for _, repo := range reg.Repos {
					if repo.HasLabel(label) {
						repos = append(repos, repo)
					}
				}
			} else {
				repos = reg.Repos
			}

			// Sort repos
			switch sortBy {
			case "label":
				sort.Slice(repos, func(i, j int) bool {
					li := ""
					if len(repos[i].Labels) > 0 {
						li = repos[i].Labels[0]
					}
					lj := ""
					if len(repos[j].Labels) > 0 {
						lj = repos[j].Labels[0]
					}
					if li != lj {
						return li < lj
					}
					return repos[i].Name < repos[j].Name
				})
			default: // "name"
				sort.Slice(repos, func(i, j int) bool {
					return repos[i].Name < repos[j].Name
				})
			}

			// Output
			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(repos)
			}

			// Table output
			if len(repos) == 0 {
				fmt.Println("No repos registered. Use 'wt repos add <path>' to register a repo.")
				return nil
			}

			// Build table rows
			headers := []string{"NAME", "PATH", "LABELS"}
			var rows [][]string
			for _, repo := range repos {
				labels := "-"
				if len(repo.Labels) > 0 {
					labels = strings.Join(repo.Labels, ", ")
				}
				rows = append(rows, []string{repo.Name, repo.Path, labels})
			}

			out.Print(static.RenderTable(headers, rows))

			return nil
		},
	}

	cmd.Flags().StringVarP(&label, "label", "l", "", "Filter by label")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "name", "Sort by: name, label")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	// Completions
	cmd.RegisterFlagCompletionFunc("label", completeLabels)
	cmd.RegisterFlagCompletionFunc("sort", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"name", "label"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func newReposAddCmd() *cobra.Command {
	var (
		name           string
		worktreeFormat string
		labels         []string
	)

	cmd := &cobra.Command{
		Use:     "add <path>...",
		Short:   "Register existing repositories",
		Aliases: []string{"a"},
		Args:    cobra.MinimumNArgs(1),
		Long: `Register existing git repositories with wt.

Repositories will be added to the registry (~/.wt/repos.json) and can then
be managed with other wt commands. Non-git directories are silently skipped.`,
		Example: `  wt repos add ~/work/my-project                    # Register single repo
  wt repos add ~/work/*                             # Register all repos in directory
  wt repos add ~/work/my-project -n myproj          # Custom display name (single repo only)
  wt repos add ~/work/my-project -l work -l api     # Add labels
  wt repos add ~/work/my-project -w "./{branch}"    # Custom worktree format`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			// Custom name only works with single path
			if name != "" && len(args) > 1 {
				return fmt.Errorf("--name can only be used with a single path")
			}

			// Load registry once
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			var added int
			for _, path := range args {
				// Resolve to absolute path
				absPath, err := filepath.Abs(path)
				if err != nil {
					l.Printf("skipping %s: %v\n", path, err)
					continue
				}

				// Verify it's a git repo - skip if not
				repoType, err := git.DetectRepoType(absPath)
				if err != nil {
					l.Debug("skipping non-git directory", "path", absPath)
					continue
				}

				// Use directory name as default name
				repoName := name
				if repoName == "" {
					repoName = filepath.Base(absPath)
				}

				l.Debug("registering repo", "path", absPath, "name", repoName, "type", repoType)

				// Add repo
				repo := registry.Repo{
					Path:           absPath,
					Name:           repoName,
					WorktreeFormat: worktreeFormat,
					Labels:         labels,
				}

				if err := reg.Add(repo); err != nil {
					l.Printf("skipping %s: %v\n", absPath, err)
					continue
				}

				typeStr := "regular"
				if repoType == git.RepoTypeBare {
					typeStr = "bare"
				}
				fmt.Printf("Registered %s repo: %s (%s)\n", typeStr, repoName, absPath)
				added++
			}

			if added == 0 {
				return fmt.Errorf("no repositories added")
			}

			// Save registry
			if err := reg.Save(); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

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

func newReposRemoveCmd() *cobra.Command {
	var (
		deleteFiles bool
		force       bool
	)

	cmd := &cobra.Command{
		Use:               "remove <repo>",
		Short:             "Unregister a repository",
		Aliases:           []string{"rm"},
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeRepoNames,
		Long: `Unregister a repository from wt.

The repository will be removed from the registry (~/.wt/repos.json).
By default, files are kept on disk. Use --delete to also remove files.`,
		Example: `  wt repos remove my-project           # Unregister, keep files
  wt repos remove my-project --delete  # Unregister and delete from disk
  wt repos remove my-project -D -f     # Delete without confirmation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)

			nameOrPath := args[0]

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Find repo
			repo, err := reg.Find(nameOrPath)
			if err != nil {
				return err
			}

			l.Debug("removing repo", "name", repo.Name, "path", repo.Path)

			// Confirm deletion if --delete and not --force
			if deleteFiles && !force {
				result, err := prompt.Confirm(fmt.Sprintf("Delete %s and all its worktrees from disk?", repo.Path))
				if err != nil {
					return err
				}
				if result.Cancelled || !result.Confirmed {
					fmt.Println("Cancelled")
					return nil
				}
			}

			// Remove from registry
			if err := reg.Remove(nameOrPath); err != nil {
				return err
			}

			// Save registry
			if err := reg.Save(); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			// Delete files if requested
			if deleteFiles {
				// First remove the repo directory
				if err := os.RemoveAll(repo.Path); err != nil {
					return fmt.Errorf("delete repo: %w", err)
				}
				fmt.Printf("Deleted: %s\n", repo.Path)
			}

			fmt.Printf("Unregistered: %s (%s)\n", repo.Name, filepath.Base(repo.Path))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&deleteFiles, "delete", "D", false, "Also delete repo and worktrees from disk")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")

	// Completions
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

func newReposCloneCmd() *cobra.Command {
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
		Args:    cobra.RangeArgs(1, 2),
		Long: `Clone a git repository as bare and register it.

Clones directly into .git (no working tree):
  repo/
  └── .git/    # bare git repo contents (HEAD, objects/, refs/, etc.)

This allows worktrees to be created as siblings to .git.
Use -b to create an initial worktree for a branch.

If destination is not specified, clones into the current directory.`,
		Example: `  wt repos clone https://github.com/org/repo           # Clone to ./repo
  wt repos clone https://github.com/org/repo myrepo    # Clone to ./myrepo
  wt repos clone https://github.com/org/repo -b main   # Clone and create worktree for main
  wt repos clone git@github.com:org/repo.git -l work   # Clone with label`,
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

// completeLabels provides completion for label flags
func completeLabels(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	reg, err := registry.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return reg.AllLabels(), cobra.ShellCompDirectiveNoFileComp
}

// completeRepoNames provides completion for repo name arguments
func completeRepoNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	reg, err := registry.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return reg.AllRepoNames(), cobra.ShellCompDirectiveNoFileComp
}

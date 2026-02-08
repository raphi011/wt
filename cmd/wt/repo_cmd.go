package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/prompt"
	"github.com/raphi011/wt/internal/ui/static"
)

func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repo",
		Short:   "Manage registered repositories",
		Aliases: []string{"r"},
		GroupID: GroupRegistry,
		Long: `Manage registered repositories.

Use subcommands to list, add, clone, or remove repositories from the registry.`,
		Example: `  wt repo list                  # List all repos
  wt repo add ~/work/my-project # Register a repo
  wt repo clone <url|org/repo>  # Clone and register a repo
  wt repo remove my-project     # Unregister a repo
  wt repo make-bare ./myrepo    # Migrate to bare structure`,
	}

	// Add subcommands
	cmd.AddCommand(newRepoListCmd())
	cmd.AddCommand(newRepoAddCmd())
	cmd.AddCommand(newRepoCloneCmd())
	cmd.AddCommand(newRepoRemoveCmd())
	cmd.AddCommand(newRepoMakeBareCmd())

	return cmd
}

func newRepoListCmd() *cobra.Command {
	var (
		sortBy     string
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:     "list [label...]",
		Short:   "List registered repositories",
		Aliases: []string{"ls"},
		Args:    cobra.ArbitraryArgs,
		Long: `List all registered repositories.

Shows name, path, and labels.
Use positional args to filter by label(s).`,
		Example: `  wt repo list                  # List all repos
  wt repo list backend          # Filter by label
  wt repo list backend frontend # Filter by multiple labels
  wt repo list --json           # Output as JSON`,
		ValidArgsFunction: completeLabels,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.FromContext(cmd.Context())
			out := output.FromContext(cmd.Context())

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Filter by labels if specified
			var repos []registry.Repo
			if len(args) > 0 {
				// Collect repos matching any of the labels
				seen := make(map[string]bool)
				for _, label := range args {
					for _, repo := range reg.Repos {
						if repo.HasLabel(label) && !seen[repo.Path] {
							seen[repo.Path] = true
							repos = append(repos, repo)
						}
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
				if len(args) > 0 {
					return fmt.Errorf("no repos found with label(s): %s", strings.Join(args, ", "))
				}
				fmt.Println("No repos registered. Use 'wt repo add <path>' to register a repo.")
				return nil
			}

			// Build table rows
			headers := []string{"NAME", "PATH", "LABELS"}
			var rows [][]string
			for _, repo := range repos {
				labels := ""
				if len(repo.Labels) > 0 {
					labels = strings.Join(repo.Labels, ", ")
				}
				rows = append(rows, []string{repo.Name, repo.Path, labels})
			}

			out.Print(static.RenderTable(headers, rows))

			return nil
		},
	}

	cmd.Flags().StringVarP(&sortBy, "sort", "s", "name", "Sort by: name, label")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	// Completions
	cmd.RegisterFlagCompletionFunc("sort", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"name", "label"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func newRepoAddCmd() *cobra.Command {
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
		Example: `  wt repo add ~/work/my-project                    # Register single repo
  wt repo add ~/work/*                             # Register all repos in directory
  wt repo add ~/work/my-project -n myproj          # Custom display name (single repo only)
  wt repo add ~/work/my-project -l work -l api     # Add labels
  wt repo add ~/work/my-project -w "./{branch}"    # Custom worktree format`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)

			// Custom name only works with single path
			if name != "" && len(args) > 1 {
				return fmt.Errorf("--name can only be used with a single path")
			}

			// Load registry once
			reg, err := registry.Load(cfg.RegistryPath)
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
			if err := reg.Save(cfg.RegistryPath); err != nil {
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

func newRepoRemoveCmd() *cobra.Command {
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
		Example: `  wt repo remove my-project           # Unregister, keep files
  wt repo remove my-project --delete  # Unregister and delete from disk
  wt repo remove my-project -D -f     # Delete without confirmation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)

			nameOrPath := args[0]

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
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
					fmt.Fprintln(cmd.OutOrStdout(), "Cancelled")
					return nil
				}
			}

			// Remove from registry
			if err := reg.Remove(nameOrPath); err != nil {
				return err
			}

			// Save registry
			if err := reg.Save(cfg.RegistryPath); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			// Delete files if requested
			if deleteFiles {
				// First remove the repo directory
				if err := os.RemoveAll(repo.Path); err != nil {
					return fmt.Errorf("delete repo: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Deleted: %s\n", repo.Path)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Unregistered: %s (%s)\n", repo.Name, filepath.Base(repo.Path))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&deleteFiles, "delete", "D", false, "Also delete repo and worktrees from disk")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")

	// Completions
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)

	return cmd
}

func newRepoCloneCmd() *cobra.Command {
	var (
		name           string
		labels         []string
		worktreeFormat string
		destination    string
		branch         string
	)

	cmd := &cobra.Command{
		Use:     "clone <url|org/repo> [destination]",
		Short:   "Clone a repository as bare",
		Aliases: []string{"cl"},
		Args:    cobra.RangeArgs(1, 2),
		Long: `Clone a git repository as bare and register it.

Clones directly into .git (no working tree):
  repo/
  └── .git/    # bare git repo contents (HEAD, objects/, refs/, etc.)

This allows worktrees to be created as siblings to .git.
Use -b to create an initial worktree for a branch.

Supports both full URLs and short-form org/repo format:
  - Full URLs use git clone directly
  - org/repo format uses gh/glab CLI (determined by forge config)
  - repo-only format uses default_org from config

If destination is not specified, clones into the current directory.`,
		Example: `  wt repo clone https://github.com/org/repo           # Clone via git URL
  wt repo clone git@github.com:org/repo.git           # Clone via SSH URL
  wt repo clone org/repo                              # Clone via gh/glab (uses forge config)
  wt repo clone myrepo                                # Clone with default_org
  wt repo clone org/repo -b main                      # Clone and create worktree for main
  wt repo clone org/repo -l work                      # Clone with label`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			workDir := config.WorkDirFromContext(ctx)

			input := args[0]

			// Determine destination name
			dest := destination
			if len(args) > 1 {
				dest = args[1]
			}
			if dest == "" {
				if isGitURL(input) {
					dest = extractRepoNameFromURL(input)
				} else {
					// org/repo or just repo → extract last part
					parts := strings.Split(input, "/")
					dest = parts[len(parts)-1]
				}
			}

			// Resolve to absolute path relative to working directory
			var absPath string
			if filepath.IsAbs(dest) {
				absPath = dest
			} else {
				absPath = filepath.Join(workDir, dest)
			}

			// Check if directory already exists
			if _, err := os.Stat(absPath); err == nil {
				return fmt.Errorf("destination already exists: %s", absPath)
			}

			// Clone based on input type
			if isGitURL(input) {
				// Full URL: use git clone directly
				l.Debug("cloning repo via git", "url", input, "dest", absPath)
				if err := git.CloneBareWithWorktreeSupport(ctx, input, absPath); err != nil {
					return fmt.Errorf("clone failed: %w", err)
				}
			} else {
				// Short-form: org/repo or just repo - use forge CLI
				orgRepo := input
				if !strings.Contains(orgRepo, "/") {
					if cfg.Forge.DefaultOrg == "" {
						return fmt.Errorf("no organization specified and forge.default_org not configured")
					}
					orgRepo = cfg.Forge.DefaultOrg + "/" + orgRepo
				}

				// Determine forge type from config rules
				forgeName := cfg.Forge.GetForgeTypeForRepo(orgRepo)
				f := forge.ByNameWithConfig(forgeName, &cfg.Forge)

				// Check forge CLI is available
				if err := f.Check(ctx); err != nil {
					return err
				}

				l.Debug("cloning repo via forge", "spec", orgRepo, "forge", forgeName, "dest", absPath)

				// CloneBareRepo creates destPath/repoName, so pass parent dir
				clonedPath, err := f.CloneBareRepo(ctx, orgRepo, filepath.Dir(absPath))
				if err != nil {
					return fmt.Errorf("clone failed: %w", err)
				}
				absPath = clonedPath // Update to actual path created
			}

			// Determine display name
			repoName := name
			if repoName == "" {
				repoName = filepath.Base(absPath)
			}

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
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

			if err := reg.Save(cfg.RegistryPath); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			fmt.Printf("Cloned repo: %s (%s)\n", repoName, absPath)

			// Create an initial worktree if branch specified
			if branch != "" {
				format := worktreeFormat
				if format == "" {
					format = cfg.Checkout.WorktreeFormat
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

// isGitURL returns true if input looks like a full git URL
// (has protocol prefix or SSH format with @)
func isGitURL(input string) bool {
	return strings.Contains(input, "://") || // https://, git://, ssh://
		strings.HasPrefix(input, "git@") || // git@github.com:org/repo
		strings.HasPrefix(input, "file://") // file:///path
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

func newRepoMakeBareCmd() *cobra.Command {
	var (
		name           string
		labels         []string
		worktreeFormat string
		dryRun         bool
	)

	cmd := &cobra.Command{
		Use:   "make-bare [path]",
		Short: "Convert a regular repo to bare-in-.git structure",
		Args:  cobra.MaximumNArgs(1),
		Long: `Convert an existing normal git repository into the bare repo structure used by 'wt repo clone'.

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
- Registers the repository in the wt registry (if not already registered)`,
		Example: `  wt repo make-bare                  # Migrate repo in current directory
  wt repo make-bare ./myrepo         # Migrate repo at path
  wt repo make-bare -n myapp         # Migrate with custom display name
  wt repo make-bare -l backend       # Migrate with labels
  wt repo make-bare --dry-run        # Preview migration without making changes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
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

			// Determine display name
			repoName := name
			if repoName == "" {
				repoName = filepath.Base(absPath)
			}

			// Check if already registered
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			existingRepo, findErr := reg.FindByPath(absPath)
			alreadyRegistered := findErr == nil

			// Check for name conflicts (only if not already registered)
			if !alreadyRegistered {
				if _, err := reg.FindByName(repoName); err == nil {
					return fmt.Errorf("repo name already exists: %s", repoName)
				}
			}

			// Determine effective worktree format
			// Priority: flag → existing repo config → default "{branch}" (nested)
			effectiveFormat := worktreeFormat
			if effectiveFormat == "" && alreadyRegistered {
				effectiveFormat = existingRepo.WorktreeFormat
			}
			if effectiveFormat == "" {
				effectiveFormat = "{branch}" // make-bare default: nested within repo
			}

			l.Debug("validating migration", "path", absPath, "format", effectiveFormat)

			// Validate and get migration plan
			opts := git.MigrationOptions{
				WorktreeFormat: effectiveFormat,
				RepoName:       repoName,
			}
			plan, err := git.ValidateMigration(ctx, absPath, opts)
			if err != nil {
				return err
			}

			// Show migration plan
			out.Printf("Migration plan for: %s\n\n", absPath)
			out.Printf("  Current branch: %s\n", plan.CurrentBranch)
			out.Printf("  Main worktree will be at: %s\n", plan.MainWorktreePath)
			out.Printf("  Worktree format: %s\n", effectiveFormat)

			if len(plan.WorktreesToFix) > 0 {
				out.Printf("\n  Existing worktrees:\n")
				for _, wt := range plan.WorktreesToFix {
					if wt.NeedsMove {
						out.Printf("    %s → %s\n", wt.OldPath, wt.NewPath)
					} else {
						out.Printf("    %s (links will be updated)\n", wt.OldPath)
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

			// Register the repo (skip if already registered)
			if !alreadyRegistered {
				repo := registry.Repo{
					Path:           absPath,
					Name:           repoName,
					WorktreeFormat: worktreeFormat,
					Labels:         labels,
				}

				if err := reg.Add(repo); err != nil {
					return fmt.Errorf("register repo: %w", err)
				}

				if err := reg.Save(cfg.RegistryPath); err != nil {
					return fmt.Errorf("save registry: %w", err)
				}
			}

			out.Printf("Migration complete!\n")
			out.Printf("  Main worktree: %s\n", result.MainWorktreePath)
			if alreadyRegistered {
				out.Printf("  Already registered as: %s\n", repoName)
			} else {
				out.Printf("  Registered as: %s\n", repoName)
			}

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

// completeLabels provides completion for label flags
func completeLabels(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg := config.FromContext(cmd.Context())
	reg, err := registry.Load(cfg.RegistryPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return reg.AllLabels(), cobra.ShellCompDirectiveNoFileComp
}

// completeRepoNames provides completion for repo name arguments
func completeRepoNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg := config.FromContext(cmd.Context())
	reg, err := registry.Load(cfg.RegistryPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return reg.AllRepoNames(), cobra.ShellCompDirectiveNoFileComp
}

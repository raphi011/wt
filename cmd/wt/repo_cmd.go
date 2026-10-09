package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

Use subcommands to list, add, clone, remove, or convert registered repositories.`,
		Example: `  wt repo list                  # List all repos
  wt repo add ~/work/my-project # Register a repo
  wt repo clone <url|org/repo>  # Clone and register a repo
  wt repo remove my-project     # Unregister a repo
  wt repo convert --clone-mode bare  # Convert to bare structure
  wt repo convert --clone-mode regular # Convert bare to regular`,
	}

	// Add subcommands
	cmd.AddCommand(newRepoListCmd())
	cmd.AddCommand(newRepoAddCmd())
	cmd.AddCommand(newRepoCloneCmd())
	cmd.AddCommand(newRepoRemoveCmd())
	cmd.AddCommand(newRepoConvertCmd())

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
			repos := []registry.Repo{}
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
				if len(repos) == 0 {
					return fmt.Errorf("no repos found with label(s): %s", strings.Join(args, ", "))
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
				out.Println("No repos registered. Use 'wt repo add <path>' to register a repo.")
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

			out.Print(static.RenderTableAtWidth(headers, rows, out.TerminalWidth()))

			return nil
		},
	}

	cmd.Flags().StringVarP(&sortBy, "sort", "s", "name", "Sort by: name, label")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	// Completions
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("sort", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"name", "label"}, cobra.ShellCompDirectiveNoFileComp
	}))

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
  wt repo add ~/work/my-project --worktree-format "./{branch}"  # Custom worktree format`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)

			// Custom name only works with single path
			if name != "" && len(args) > 1 {
				return fmt.Errorf("--name can only be used with a single path")
			}

			// Validate paths before taking the registry lock
			type candidate struct {
				repo     registry.Repo
				repoType git.RepoType
			}
			var candidates []candidate
			for _, path := range args {
				// Resolve to absolute path
				absPath, err := filepath.Abs(path)
				if err != nil {
					l.Printf("skipping %s: %v\n", path, err)
					continue
				}

				// Verify it's a git repo - skip if not
				repoType, err := git.DetectRepoType(ctx, absPath)
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

				candidates = append(candidates, candidate{repo: repo, repoType: repoType})
			}

			var added []candidate
			var skipped []string
			_, err := registry.Update(cfg.RegistryPath, func(r *registry.Registry) error {
				for _, c := range candidates {
					if err := r.Add(c.repo); err != nil {
						skipped = append(skipped, fmt.Sprintf("skipping %s: %v\n", c.repo.Path, err))
						continue
					}
					added = append(added, c)
				}

				if len(added) == 0 {
					return fmt.Errorf("no repositories added")
				}
				return nil
			})

			for _, msg := range skipped {
				l.Printf("%s", msg)
			}
			if err != nil {
				return err
			}

			for _, c := range added {
				typeStr := "regular"
				if c.repoType == git.RepoTypeBare {
					typeStr = "bare"
				}
				l.Printf("Registered %s repo: %s (%s)\n", typeStr, c.repo.Name, c.repo.Path)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Display name (default: directory name)")
	cmd.Flags().StringVar(&worktreeFormat, "worktree-format", "", "Worktree format override")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Labels for grouping (repeatable)")

	// Completions
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("label", completeLabels))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("name", cobra.NoFileCompletions))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("worktree-format", cobra.NoFileCompletions))

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
					_, err = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled")
					return err
				}
			}

			// Remove from registry
			if _, err := registry.Update(cfg.RegistryPath, func(r *registry.Registry) error {
				return r.Remove(nameOrPath)
			}); err != nil {
				return err
			}

			// Delete files if requested
			if deleteFiles {
				// First remove the repo directory
				if err := os.RemoveAll(repo.Path); err != nil {
					return fmt.Errorf("delete repo: %w", err)
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted: %s\n", repo.Path); err != nil {
					return err
				}
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Unregistered: %s (%s)\n", repo.Name, filepath.Base(repo.Path))
			return err
		},
	}

	cmd.Flags().BoolVarP(&deleteFiles, "delete", "D", false, "Also delete repo and worktrees from disk")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force deletion without confirmation")

	return cmd
}

func newRepoCloneCmd() *cobra.Command {
	var (
		name           string
		labels         []string
		worktreeFormat string
		branch         string
		cloneMode      string
	)

	cmd := &cobra.Command{
		Use:     "clone <url|org/repo> [destination]",
		Short:   "Clone a repository and register it",
		Aliases: []string{"cl"},
		Args:    cobra.RangeArgs(1, 2),
		Long: `Clone a git repository and register it.

By default, clones as a regular repo with a working tree at root:
  repo/
  ├── .git/    # git directory
  └── ...      # working tree files

Use --clone-mode bare (or clone.mode in config) for a bare clone (git data in .git, no working tree at root).

When cloning bare, creates a worktree for the default branch (main/master).
Use --branch to specify a different branch instead.

Supports both full URLs and short-form org/repo format:
  - Full URLs use git clone directly
  - org/repo format uses gh/glab CLI (determined by forge config)
  - repo-only format uses default_org from config

If destination is not specified, clones into <repo-name> in the current directory.`,
		Example: `  wt repo clone https://github.com/org/repo           # Clone via git URL
  wt repo clone git@github.com:org/repo.git           # Clone via SSH URL
  wt repo clone org/repo                              # Clone via gh/glab (uses forge config)
  wt repo clone myrepo                                # Clone with default_org
  wt repo clone org/repo --clone-mode bare            # Bare clone with default branch worktree
  wt repo clone org/repo --clone-mode bare --branch develop # Bare clone with worktree for develop
  wt repo clone org/repo -l work                      # Clone with label`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			workDir := config.WorkDirFromContext(ctx)

			input := args[0]

			// Determine destination name
			var dest string
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

			// Resolve effective clone mode
			bareMode, err := resolveCloneBare(ctx, cloneMode)
			if err != nil {
				return err
			}

			if !bareMode && branch != "" {
				return fmt.Errorf("--branch is only supported in bare clone mode; remove --branch or use --clone-mode bare")
			}

			// Clone based on input type
			if isGitURL(input) {
				// Full URL: use git clone directly
				l.Debug("cloning repo via git", "url", input, "dest", absPath, "bare", bareMode)
				if bareMode {
					if err := git.CloneBareWithWorktreeSupport(ctx, input, absPath); err != nil {
						return fmt.Errorf("clone failed: %w", err)
					}
				} else {
					if err := git.CloneRegular(ctx, input, absPath); err != nil {
						return fmt.Errorf("clone failed: %w", err)
					}
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
				f := forge.ResolverFromContext(ctx)(orgRepo, forgeName, cfg.Hosts, &cfg.Forge)

				// Check forge CLI is available
				if err := f.Check(ctx); err != nil {
					return err
				}

				l.Debug("cloning repo via forge", "spec", orgRepo, "forge", forgeName, "dest", absPath, "bare", bareMode)

				var clonedPath string
				var cloneErr error
				if bareMode {
					clonedPath, cloneErr = f.CloneBareRepo(ctx, orgRepo, filepath.Dir(absPath))
				} else {
					clonedPath, cloneErr = f.CloneRepo(ctx, orgRepo, filepath.Dir(absPath))
				}
				if cloneErr != nil {
					return fmt.Errorf("clone failed: %w", cloneErr)
				}
				absPath = clonedPath // Update to actual path created
			}

			// Determine display name
			repoName := name
			if repoName == "" {
				repoName = filepath.Base(absPath)
			}

			// Register the repo
			repo := registry.Repo{
				Path:           absPath,
				Name:           repoName,
				WorktreeFormat: worktreeFormat,
				Labels:         labels,
			}

			if err := registerClone(cfg, &repo); err != nil {
				return err
			}

			l.Printf("Cloned repo: %s (%s)\n", repoName, absPath)

			// Create initial worktree only in bare mode
			// Regular clones already have a working tree at root
			if bareMode {
				// Determine which branch to create worktree for
				worktreeBranch := branch
				gitDir := filepath.Join(absPath, ".git")
				if branch == "" {
					// Auto-detect default branch if no explicit branch specified
					if git.RefExists(ctx, gitDir, "HEAD") {
						worktreeBranch = git.GetDefaultBranch(ctx, gitDir)
						// Verify the detected branch actually exists before attempting worktree creation
						// Use LocalBranchExists since bare clones have refs/heads/* but not refs/remotes/origin/*
						if !git.LocalBranchExists(ctx, gitDir, worktreeBranch) {
							l.Printf("Warning: default branch %q not found, skipping worktree creation\n", worktreeBranch)
							worktreeBranch = ""
						} else {
							l.Debug("auto-detected default branch", "branch", worktreeBranch)
						}
					} else {
						l.Debug("skipping worktree creation: repo has no commits")
					}
				}

				// Create worktree if we have a branch
				if worktreeBranch != "" {
					// No fetch right after the clone, and no checkout hooks
					if _, err := ensureWorktree(ctx, repo, worktreeBranch, checkoutOpts{
						Fetch: new(false),
						Hooks: hookFlags{NoHook: true},
					}); err != nil {
						l.Printf("Warning: failed to create initial worktree: %v\n", err)
					}
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Display name (default: directory name)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Labels for grouping (repeatable)")
	cmd.Flags().StringVar(&worktreeFormat, "worktree-format", "", "Worktree format override")
	cmd.Flags().StringVar(&branch, "branch", "", "Create initial worktree for branch (bare mode only)")
	cmd.Flags().StringVar(&cloneMode, "clone-mode", "", "Clone mode: bare or regular (default: config)")

	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("clone-mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"bare", "regular"}, cobra.ShellCompDirectiveNoFileComp
	}))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("label", completeLabels))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("name", cobra.NoFileCompletions))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("worktree-format", cobra.NoFileCompletions))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("branch", cobra.NoFileCompletions))

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

func newRepoConvertCmd() *cobra.Command {
	var (
		name           string
		labels         []string
		worktreeFormat string
		dryRun         bool
		cloneMode      string
	)

	cmd := &cobra.Command{
		Use:   "convert [path]",
		Short: "Convert a repo between regular and bare-in-.git structure",
		Args:  cobra.MaximumNArgs(1),
		Long: `Convert a repository between regular and bare-in-.git structure.

Use --clone-mode to specify the target structure:

  --clone-mode bare     Convert regular → bare-in-.git
  --clone-mode regular  Convert bare-in-.git → regular

Regular → bare (--clone-mode bare):
  Before:                    After:
  myrepo/                    myrepo/
  ├── .git/  (regular)       ├── .git/  (bare repo)
  ├── src/                   │   └── worktrees/
  └── README.md              └── main/  (working tree)

Bare → regular (--clone-mode regular):
  Before:                    After:
  myrepo/                    myrepo/
  ├── .git/  (bare repo)     ├── .git/  (regular)
  │   └── worktrees/         ├── src/
  └── main/  (working tree)  └── README.md

The conversion:
- Preserves all uncommitted changes and untracked files
- Updates any existing worktrees to work with the new structure
- Registers the repository in the wt registry (if not already registered)`,
		Example: `  wt repo convert --clone-mode bare               # Convert to bare in current dir
  wt repo convert --clone-mode bare ./myrepo      # Convert repo at path
  wt repo convert --clone-mode regular            # Convert bare to regular
  wt repo convert --clone-mode bare -n myapp      # Convert with custom name
  wt repo convert --clone-mode bare --dry-run     # Preview without changes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Validate --clone-mode
			if err := config.ValidateCloneMode(cloneMode); err != nil {
				return err
			}

			// Determine path to convert
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
				effectiveFormat = "{branch}"
			}

			l.Debug("validating conversion", "path", absPath, "format", effectiveFormat, "target", cloneMode)

			p := convertParams{
				ctx:               ctx,
				out:               out,
				l:                 l,
				cfg:               cfg,
				absPath:           absPath,
				repoName:          repoName,
				effectiveFormat:   effectiveFormat,
				worktreeFormat:    worktreeFormat,
				labels:            labels,
				dryRun:            dryRun,
				alreadyRegistered: alreadyRegistered,
			}

			// Route based on clone-mode
			switch cloneMode {
			case "bare":
				return convertToBare(p)
			case "regular":
				return convertToRegular(p)
			default:
				return fmt.Errorf("invalid clone-mode: %s", cloneMode)
			}
		},
	}

	cmd.Flags().StringVar(&cloneMode, "clone-mode", "", "Target mode: bare or regular (required)")
	cobra.CheckErr(cmd.MarkFlagRequired("clone-mode"))
	cmd.Flags().StringVarP(&name, "name", "n", "", "Display name (default: directory name)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Labels for grouping (repeatable)")
	cmd.Flags().StringVar(&worktreeFormat, "worktree-format", "", "Worktree format override")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Preview conversion without making changes")

	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("clone-mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"bare", "regular"}, cobra.ShellCompDirectiveNoFileComp
	}))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("label", completeLabels))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("name", cobra.NoFileCompletions))
	cobra.CheckErr(cmd.RegisterFlagCompletionFunc("worktree-format", cobra.NoFileCompletions))

	// Path argument should complete directories only
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveFilterDirs
	}

	return cmd
}

// convertParams holds the common parameters for convertToBare and convertToRegular.
type convertParams struct {
	ctx               context.Context
	out               *output.Printer
	l                 *log.Logger
	cfg               *config.Config
	absPath           string
	repoName          string
	effectiveFormat   string
	worktreeFormat    string
	labels            []string
	dryRun            bool
	alreadyRegistered bool
}

// registerRepo adds the repo to the registry.
func registerRepo(cfg *config.Config, repo registry.Repo) error {
	_, err := registry.Update(cfg.RegistryPath, func(r *registry.Registry) error {
		if err := r.Add(repo); err != nil {
			return fmt.Errorf("register repo: %w", err)
		}
		return nil
	})
	return err
}

// registerClone adds a fresh clone to the registry, with default_labels in
// front of the repo's own labels. A clone the registry rejects (duplicate name
// or path) is deleted; it is kept when the registry can't be updated.
func registerClone(cfg *config.Config, repo *registry.Repo) error {
	labels := slices.Clone(cfg.DefaultLabels)
	for _, label := range repo.Labels {
		if !slices.Contains(labels, label) {
			labels = append(labels, label)
		}
	}
	repo.Labels = labels

	var addErr error
	if _, err := registry.Update(cfg.RegistryPath, func(r *registry.Registry) error {
		addErr = r.Add(*repo)
		return addErr
	}); err != nil {
		if addErr != nil {
			return errors.Join(fmt.Errorf("register repo: %w", err), os.RemoveAll(repo.Path))
		}
		return err
	}
	return nil
}

// registerAndVerify registers the repo (if not already registered) and lists worktrees for verification.
func registerAndVerify(p convertParams, warnings []string) error {
	for _, w := range warnings {
		p.l.Printf("Warning: %s\n", w)
	}

	if !p.alreadyRegistered {
		repo := registry.Repo{
			Path:           p.absPath,
			Name:           p.repoName,
			WorktreeFormat: p.worktreeFormat,
			Labels:         p.labels,
		}

		if err := registerRepo(p.cfg, repo); err != nil {
			return err
		}
	}

	if p.alreadyRegistered {
		p.l.Printf("  Already registered as: %s\n", p.repoName)
	} else {
		p.l.Printf("  Registered as: %s\n", p.repoName)
	}

	worktrees, err := git.ListWorktreesFromRepo(p.ctx, p.absPath)
	if err != nil {
		p.l.Printf("Warning: could not list worktrees after conversion: %v\n", err)
		p.l.Printf("Run 'git worktree list' manually to verify the conversion\n")
	} else if len(worktrees) > 0 {
		p.l.Printf("\n  Worktrees:\n")
		for _, wt := range worktrees {
			p.l.Printf("    %s (%s)\n", wt.Path, wt.Branch)
		}
	}

	return nil
}

func convertToBare(p convertParams) error {
	opts := git.MigrationOptions{
		WorktreeFormat: p.effectiveFormat,
		RepoName:       p.repoName,
	}
	plan, err := git.ValidateMigration(p.ctx, p.absPath, opts)
	if err != nil {
		return err
	}

	// Show migration plan
	p.out.Printf("Conversion plan for: %s (→ bare)\n\n", p.absPath)
	p.out.Printf("  Current branch: %s\n", plan.CurrentBranch)
	p.out.Printf("  Main worktree will be at: %s\n", plan.MainWorktreePath)
	p.out.Printf("  Worktree format: %s\n", p.effectiveFormat)

	if len(plan.WorktreesToFix) > 0 {
		p.out.Printf("\n  Existing worktrees:\n")
		for _, wt := range plan.WorktreesToFix {
			if wt.NeedsMove {
				p.out.Printf("    %s → %s\n", wt.OldPath, wt.NewPath)
			} else {
				p.out.Printf("    %s (links will be updated)\n", wt.OldPath)
			}
		}
	}

	p.out.Printf("\n  Registry name: %s\n", p.repoName)
	if len(p.labels) > 0 {
		p.out.Printf("  Labels: %v\n", p.labels)
	}

	if p.dryRun {
		p.out.Printf("\n  (dry run - no changes made)\n")
		return nil
	}

	p.out.Printf("\n")

	p.l.Debug("performing conversion to bare")
	result, err := git.MigrateToBare(p.ctx, plan)
	if err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	p.l.Printf("Conversion complete!\n")
	p.l.Printf("  Main worktree: %s\n", result.MainWorktreePath)

	return registerAndVerify(p, result.Warnings)
}

func convertToRegular(p convertParams) error {
	opts := git.MigrationOptions{
		WorktreeFormat: p.effectiveFormat,
		RepoName:       p.repoName,
	}
	plan, err := git.ValidateMigrationToRegular(p.ctx, p.absPath, opts)
	if err != nil {
		return err
	}

	// Show conversion plan
	p.out.Printf("Conversion plan for: %s (→ regular)\n\n", p.absPath)
	p.out.Printf("  Default branch: %s\n", plan.DefaultBranch)
	p.out.Printf("  Working tree from: %s\n", plan.DefaultBranchWT)
	p.out.Printf("  Worktree format: %s\n", p.effectiveFormat)

	if len(plan.WorktreesToFix) > 0 {
		p.out.Printf("\n  Worktrees to reformat:\n")
		for _, wt := range plan.WorktreesToFix {
			if wt.NeedsMove {
				p.out.Printf("    %s → %s\n", wt.OldPath, wt.NewPath)
			} else {
				p.out.Printf("    %s (links will be updated)\n", wt.OldPath)
			}
		}
	}

	if len(plan.DetachedWorktrees) > 0 {
		p.l.Printf("Warning: %d detached HEAD worktree(s) will be skipped (may need manual repair after conversion):\n", len(plan.DetachedWorktrees))
		for _, path := range plan.DetachedWorktrees {
			p.l.Printf("  %s\n", path)
		}
	}

	p.out.Printf("\n  Registry name: %s\n", p.repoName)
	if len(p.labels) > 0 {
		p.out.Printf("  Labels: %v\n", p.labels)
	}

	if p.dryRun {
		p.out.Printf("\n  (dry run - no changes made)\n")
		return nil
	}

	p.out.Printf("\n")

	p.l.Debug("performing conversion to regular")
	result, err := git.MigrateToRegular(p.ctx, plan)
	if err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	p.l.Printf("Conversion complete!\n")
	p.l.Printf("  Repo root: %s\n", p.absPath)

	return registerAndVerify(p, result.Warnings)
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

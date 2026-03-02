package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/static"
)

func newListCmd() *cobra.Command {
	var (
		jsonOutput bool
		global     bool
		sortBy     string
		refresh    bool
	)

	cmd := &cobra.Command{
		Use:     "list [scope...]",
		Short:   "List worktrees",
		Aliases: []string{"ls"},
		GroupID: GroupCore,
		Args:    cobra.ArbitraryArgs,
		Long: `List worktrees for registered repos.

Inside a repo: shows only that repo's worktrees. Use --global for all.
Use positional args to filter by repo name(s) or label(s).
Resolution order: repo name → label.

Worktrees are sorted by commit date (most recent first) by default.
Use --refresh-pr/-R to fetch PR status from GitHub/GitLab.`,
		Example: `  wt list                      # List worktrees for current repo
  wt list --global             # List all worktrees (all repos)
  wt list myrepo               # Filter by repository name
  wt list backend              # Filter by label (if no repo named 'backend')
  wt list myrepo backend       # Filter by multiple scopes
  wt list -R                   # Refresh PR status before listing
  wt list --json               # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine which repos to list
			var repos []registry.Repo
			if global {
				repos = reg.Repos
			} else if len(args) > 0 {
				repos, err = resolveScopeArgs(reg, args)
				if err != nil {
					return err
				}
			} else {
				// Try current repo
				repo, err := findOrRegisterCurrentRepoFromContext(ctx, reg)
				if err != nil {
					// Not in a repo, show all
					repos = reg.Repos
				} else {
					repos = []registry.Repo{repo}
				}
			}

			repos = filterOrphanedRepos(l, repos)

			l.Debug("listing worktrees", "repos", len(repos))

			allWorktrees, warnings := git.LoadWorktreesForRepos(ctx, reposToRefs(repos))
			for _, w := range warnings {
				l.Printf("Warning: %s: %v\n", w.RepoName, w.Err)
			}

			// Load PR cache
			prCache := prcache.Load()

			// Refresh PR status if requested
			if refresh {
				if failed := refreshPRs(ctx, allWorktrees, prCache, cfg.Hosts, &cfg.Forge); len(failed) > 0 {
					l.Printf("Warning: failed to fetch PR status for: %v\n", failed)
				}
			}

			populatePRFields(allWorktrees, prCache)

			// Save PR cache if modified
			if err := prCache.SaveIfDirty(); err != nil {
				l.Printf("Warning: failed to save PR cache: %v\n", err)
			}

			// Apply config default when --sort not explicitly set
			if !cmd.Flags().Changed("sort") && cfg.DefaultSort != "" {
				sortBy = cfg.DefaultSort
			}
			if sortBy == "" {
				sortBy = "date"
			}

			switch sortBy {
			case "date":
				sort.Slice(allWorktrees, func(i, j int) bool {
					return allWorktrees[i].CommitDate.After(allWorktrees[j].CommitDate)
				})
			case "repo":
				sort.Slice(allWorktrees, func(i, j int) bool {
					if allWorktrees[i].RepoName != allWorktrees[j].RepoName {
						return allWorktrees[i].RepoName < allWorktrees[j].RepoName
					}
					return allWorktrees[i].Branch < allWorktrees[j].Branch
				})
			case "branch":
				sort.Slice(allWorktrees, func(i, j int) bool {
					return allWorktrees[i].Branch < allWorktrees[j].Branch
				})
			default:
				return fmt.Errorf("invalid sort value %q (valid: date, repo, branch)", sortBy)
			}

			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(allWorktrees)
			}

			// Table output
			if len(allWorktrees) == 0 {
				out.Println("No worktrees found")
				return nil
			}

			// Build table rows
			var rows [][]string
			for _, wt := range allWorktrees {
				rows = append(rows, static.WorktreeTableRow(wt, cfg.Prune.StaleDays))
			}

			out.Print(static.RenderTable(static.WorktreeTableHeaders, rows))

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Show all worktrees (not just current repo)")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "", "Sort by: date, repo, branch")
	cmd.Flags().BoolVarP(&refresh, "refresh-pr", "R", false, "Refresh PR status before listing")

	// Completions
	cmd.ValidArgsFunction = completeScopeArgs
	cmd.RegisterFlagCompletionFunc("sort", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"date", "repo", "branch"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

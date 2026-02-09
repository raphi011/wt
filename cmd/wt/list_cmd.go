package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/static"
	"github.com/raphi011/wt/internal/ui/styles"
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

Worktrees are sorted by creation date (most recent first) by default.
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
			prCache, err := prcache.Load()
			if err != nil {
				l.Printf("Warning: failed to load PR cache: %v\n", err)
				prCache = prcache.New()
			}

			// Refresh PR status if requested
			if refresh {
				if f := refreshPRs(ctx, allWorktrees, prCache, cfg.Hosts, &cfg.Forge); f > 0 {
					l.Printf("Warning: failed to fetch PR status for %d branch(es)\n", f)
				}
			}

			populatePRFields(allWorktrees, prCache)

			// Save PR cache if modified
			if err := prCache.SaveIfDirty(); err != nil {
				l.Printf("Warning: failed to save PR cache: %v\n", err)
			}

			switch sortBy {
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
			default: // "created" or empty
				sort.Slice(allWorktrees, func(i, j int) bool {
					return allWorktrees[i].CreatedAt.After(allWorktrees[j].CreatedAt)
				})
			}

			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(allWorktrees)
			}

			// Table output
			if len(allWorktrees) == 0 {
				fmt.Println("No worktrees found")
				return nil
			}

			// Build table rows
			headers := []string{"REPO", "BRANCH", "PR", "COMMIT", "CREATED", "NOTE"}
			var rows [][]string
			for _, wt := range allWorktrees {
				created := format.RelativeTime(wt.CreatedAt)
				commit := wt.CommitHash
				if len(commit) > 7 {
					commit = commit[:7]
				}
				pr := styles.FormatPRRef(wt.PRNumber, wt.PRState, wt.PRDraft, wt.PRURL)
				rows = append(rows, []string{wt.RepoName, wt.Branch, pr, commit, created, wt.Note})
			}

			out.Print(static.RenderTable(headers, rows))

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Show all worktrees (not just current repo)")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "", "Sort by: created, repo, branch")
	cmd.Flags().BoolVarP(&refresh, "refresh-pr", "R", false, "Refresh PR status before listing")

	// Completions
	cmd.ValidArgsFunction = completeScopeArgs
	cmd.RegisterFlagCompletionFunc("sort", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"created", "repo", "branch"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

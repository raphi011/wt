package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/prcache"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/static"
	"github.com/raphi011/wt/internal/ui/styles"
)

// WorktreeDisplay holds worktree info for display
type WorktreeDisplay struct {
	RepoName   string    `json:"repo"`
	Branch     string    `json:"branch"`
	Path       string    `json:"path"`
	CommitHash string    `json:"commit"`
	CreatedAt  time.Time `json:"created_at"`
	Note       string    `json:"note,omitempty"`
	PRNumber   int       `json:"pr_number,omitempty"`
	PRState    string    `json:"pr_state,omitempty"`
	PRURL      string    `json:"pr_url,omitempty"`
	PRDraft    bool      `json:"pr_draft,omitempty"`
}

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
Use --refresh/-R to fetch PR status from GitHub/GitLab.`,
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

			var allWorktrees []WorktreeDisplay
			for _, repo := range repos {
				worktrees, err := listWorktreesForRepo(ctx, repo)
				if err != nil {
					l.Printf("Warning: %s: %v\n", repo.Name, err)
					continue
				}
				allWorktrees = append(allWorktrees, worktrees...)
			}

			// Load PR cache
			prCache, err := prcache.Load()
			if err != nil {
				l.Printf("Warning: failed to load PR cache: %v\n", err)
				prCache = prcache.New()
			}

			// Refresh PR status if requested
			if refresh {
				refreshPRStatusForList(ctx, allWorktrees, prCache, cfg.Hosts, &cfg.Forge)
				if err := prCache.Save(); err != nil {
					l.Printf("Warning: failed to save PR cache: %v\n", err)
				}
			}

			// Populate PR fields from cache
			for i := range allWorktrees {
				folderName := filepath.Base(allWorktrees[i].Path)
				if pr := prCache.Get(folderName); pr != nil && pr.Fetched && pr.Number > 0 {
					allWorktrees[i].PRNumber = pr.Number
					allWorktrees[i].PRState = pr.State
					allWorktrees[i].PRURL = pr.URL
					allWorktrees[i].PRDraft = pr.IsDraft
				}
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
	cmd.Flags().BoolVarP(&refresh, "refresh", "R", false, "Refresh PR status before listing")

	// Completions
	cmd.ValidArgsFunction = completeScopeArgs
	cmd.RegisterFlagCompletionFunc("sort", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"created", "repo", "branch"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func listWorktreesForRepo(ctx context.Context, repo registry.Repo) ([]WorktreeDisplay, error) {
	worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
	if err != nil {
		return nil, err
	}

	// Get branch notes in batch
	notes, _ := git.GetAllBranchConfig(ctx, repo.Path)

	var display []WorktreeDisplay
	for _, wt := range worktrees {
		// Get worktree creation time (use mtime of directory)
		var createdAt time.Time
		if info, err := os.Stat(wt.Path); err == nil {
			createdAt = info.ModTime()
		}

		display = append(display, WorktreeDisplay{
			RepoName:   repo.Name,
			Branch:     wt.Branch,
			Path:       wt.Path,
			CommitHash: wt.CommitHash,
			CreatedAt:  createdAt,
			Note:       notes[wt.Branch],
		})
	}

	return display, nil
}

// refreshPRStatusForList fetches PR status for worktrees in parallel.
func refreshPRStatusForList(ctx context.Context, worktrees []WorktreeDisplay, prCache *prcache.Cache, hosts map[string]string, forgeConfig *config.ForgeConfig) {
	l := log.FromContext(ctx)

	// Build a map of repo name -> origin URL (deduplicate per repo)
	type repoInfo struct {
		path      string
		originURL string
	}
	repoOrigins := make(map[string]repoInfo)

	for i := range worktrees {
		wt := &worktrees[i]
		if _, ok := repoOrigins[wt.RepoName]; ok {
			continue
		}
		originURL, err := git.GetOriginURL(ctx, wt.Path)
		if err != nil {
			l.Debug("failed to get origin URL", "path", wt.Path, "err", err)
			continue
		}
		repoOrigins[wt.RepoName] = repoInfo{path: wt.Path, originURL: originURL}
	}

	// Build fetch items
	var items []prFetchItem
	for i := range worktrees {
		wt := &worktrees[i]
		ri, ok := repoOrigins[wt.RepoName]
		if !ok || ri.originURL == "" {
			continue
		}
		folderName := filepath.Base(wt.Path)
		// Skip already-merged entries (stable state)
		if pr := prCache.Get(folderName); pr != nil && pr.Fetched && pr.State == forge.PRStateMerged {
			continue
		}
		items = append(items, prFetchItem{
			originURL: ri.originURL,
			repoPath:  ri.path,
			branch:    wt.Branch,
			cacheKey:  folderName,
		})
	}

	_, failed := refreshPRStatuses(ctx, items, prCache, hosts, forgeConfig)
	if failed > 0 {
		l.Printf("Warning: failed to fetch PR status for %d branch(es)\n", failed)
	}
}

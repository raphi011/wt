package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/format"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui"
)

// WorktreeDisplay holds worktree info for display
type WorktreeDisplay struct {
	RepoName   string    `json:"repo"`
	Branch     string    `json:"branch"`
	Path       string    `json:"path"`
	CommitHash string    `json:"commit"`
	CreatedAt  time.Time `json:"created_at"`
	IsDirty    bool      `json:"is_dirty"`
	Note       string    `json:"note,omitempty"`
}

func newListCmd() *cobra.Command {
	var (
		jsonOutput bool
		global     bool
		sortBy     string
		refresh    bool
		repository []string
		labels     []string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List worktrees",
		Aliases: []string{"ls"},
		GroupID: GroupCore,
		Args:    cobra.NoArgs,
		Long: `List worktrees for registered repos.

Inside a repo: shows only that repo's worktrees. Use --global for all.
Use -r to filter by repo name(s), -l to filter by label(s).

Worktrees are sorted by creation date (most recent first) by default.`,
		Example: `  wt list                      # List worktrees for current repo
  wt list --global             # List all worktrees (all repos)
  wt list -r myrepo            # Filter by repository name
  wt list -l backend           # Filter by label
  wt list --json               # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			l := log.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Determine which repos to list
			var repos []*registry.Repo
			if global {
				for i := range reg.Repos {
					repos = append(repos, &reg.Repos[i])
				}
			} else if len(repository) > 0 || len(labels) > 0 {
				repos, err = resolveTargetRepos(ctx, reg, repository, labels)
				if err != nil {
					return err
				}
			} else {
				// Try current repo
				repo, err := findOrRegisterCurrentRepo(ctx, reg)
				if err != nil {
					// Not in a repo, show all
					for i := range reg.Repos {
						repos = append(repos, &reg.Repos[i])
					}
				} else {
					repos = []*registry.Repo{repo}
				}
			}

			l.Debug("listing worktrees", "repos", len(repos))

			// Collect all worktrees
			var allWorktrees []WorktreeDisplay
			for _, repo := range repos {
				worktrees, err := listWorktreesForRepo(ctx, repo)
				if err != nil {
					l.Printf("Warning: %s: %v\n", repo.Name, err)
					continue
				}
				allWorktrees = append(allWorktrees, worktrees...)
			}

			// Sort by creation date (most recent first)
			sort.Slice(allWorktrees, func(i, j int) bool {
				return allWorktrees[i].CreatedAt.After(allWorktrees[j].CreatedAt)
			})

			// Output
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
			headers := []string{"REPO", "BRANCH", "COMMIT", "DIRTY", "CREATED"}
			var rows [][]string
			for _, wt := range allWorktrees {
				dirty := ""
				if wt.IsDirty {
					dirty = "*"
				}
				created := format.RelativeTime(wt.CreatedAt)
				commit := wt.CommitHash
				if len(commit) > 7 {
					commit = commit[:7]
				}
				rows = append(rows, []string{wt.RepoName, wt.Branch, commit, dirty, created})
			}

			out.Print(ui.RenderTable(headers, rows))

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Show all worktrees (not just current repo)")
	cmd.Flags().StringVarP(&sortBy, "sort", "s", "", "Sort by: created, repo, branch")
	cmd.Flags().BoolVarP(&refresh, "refresh", "R", false, "Refresh PR status before listing")
	cmd.Flags().StringSliceVarP(&repository, "repository", "r", nil, "Filter by repository name(s)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Filter by label(s)")

	// Completions
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("label", completeLabels)

	return cmd
}

func listWorktreesForRepo(ctx context.Context, repo *registry.Repo) ([]WorktreeDisplay, error) {
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
			IsDirty:    git.IsDirty(ctx, wt.Path),
			Note:       notes[wt.Branch],
		})
	}

	return display, nil
}

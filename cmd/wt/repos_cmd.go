package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui"
)

func newReposCmd() *cobra.Command {
	var (
		label      string
		sortBy     string
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:     "repos",
		Short:   "List registered repositories",
		Aliases: []string{"r"},
		GroupID: GroupCore,
		Args:    cobra.NoArgs,
		Long: `List all registered repositories.

Shows name, path, type (bare/regular), worktree format, and labels.`,
		Example: `  wt repos                     # List all repos
  wt repos -l backend          # Filter by label
  wt repos --json              # Output as JSON`,
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

			// Output
			if jsonOutput {
				enc := json.NewEncoder(out.Writer())
				enc.SetIndent("", "  ")
				return enc.Encode(repos)
			}

			// Table output
			if len(repos) == 0 {
				fmt.Println("No repos registered. Use 'wt add <path>' to register a repo.")
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

			out.Print(ui.RenderTable(headers, rows))

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

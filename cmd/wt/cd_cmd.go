package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui/wizard/flows"
)

func newCdCmd() *cobra.Command {
	var interactive bool
	var copyToClipboard bool

	cmd := &cobra.Command{
		Use:     "cd [repo:]branch",
		Short:   "Print worktree path for shell scripting",
		GroupID: GroupUtility,
		Args:    cobra.MaximumNArgs(1),
		Long: `Print the path of a worktree for shell scripting.

Use with shell command substitution: cd $(wt cd feature-x)

The argument can be:
  - branch name: searches all repos, errors if ambiguous
  - repo:branch: finds exact worktree in specified repo

With no arguments, returns the most recently accessed worktree.`,
		Example: `  cd $(wt cd)              # cd to most recently accessed worktree
  cd $(wt cd feature-x)    # cd to feature-x worktree (error if ambiguous)
  cd $(wt cd wt:feature-x) # cd to feature-x worktree in wt repo
  cd $(wt cd -i)           # interactive fuzzy search for worktree
  wt cd --copy feature-x   # copy worktree path to clipboard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			var targetPath, repoName, branchName string

			if interactive {
				// Interactive mode: show fuzzy list of all worktrees
				l := log.FromContext(ctx)

				// Load history for recency ranking
				hist, err := history.Load(cfg.GetHistoryPath())
				if err != nil {
					l.Printf("Warning: failed to load history: %v\n", err)
					hist = &history.History{}
				}

				allRepos := filterOrphanedRepos(l, reg.Repos)

				loaded, warnings := git.LoadWorktreesForRepos(ctx, reposToRefs(allRepos))
				for _, w := range warnings {
					l.Debug("skipping repo", "repo", w.RepoName, "error", w.Err)
				}

				var allWorktrees []flows.CdWorktreeInfo
				for _, wt := range loaded {
					info := flows.CdWorktreeInfo{
						RepoName: wt.RepoName,
						Branch:   wt.Branch,
						Path:     wt.Path,
					}
					if entry := hist.FindByPath(wt.Path); entry != nil {
						info.LastAccess = entry.LastAccess
					}
					allWorktrees = append(allWorktrees, info)
				}

				if len(allWorktrees) == 0 {
					return fmt.Errorf("no worktrees found")
				}

				// Opportunistically clean stale history entries
				if removed := hist.RemoveStale(); removed > 0 {
					if err := hist.Save(cfg.GetHistoryPath()); err != nil {
						l.Printf("Warning: failed to save history after cleanup: %v\n", err)
					}
				}

				// Sort: worktrees with history first (by LastAccess desc),
				// then worktrees without history (alphabetical by repo:branch)
				sort.Slice(allWorktrees, func(i, j int) bool {
					iHasHistory := !allWorktrees[i].LastAccess.IsZero()
					jHasHistory := !allWorktrees[j].LastAccess.IsZero()

					if iHasHistory && jHasHistory {
						return allWorktrees[i].LastAccess.After(allWorktrees[j].LastAccess)
					}
					if iHasHistory != jHasHistory {
						return iHasHistory
					}
					// Both without history: alphabetical
					if allWorktrees[i].RepoName != allWorktrees[j].RepoName {
						return allWorktrees[i].RepoName < allWorktrees[j].RepoName
					}
					return allWorktrees[i].Branch < allWorktrees[j].Branch
				})

				result, err := flows.CdInteractive(flows.CdWizardParams{
					Worktrees: allWorktrees,
				})
				if err != nil {
					return err
				}
				if result.Cancelled {
					os.Exit(1)
				}

				targetPath = result.SelectedPath
				repoName = result.RepoName
				branchName = result.Branch
			} else if len(args) == 0 {
				// No arguments: return most recently accessed worktree
				hist, err := history.Load(cfg.GetHistoryPath())
				if err != nil {
					return fmt.Errorf("load history: %w", err)
				}
				if len(hist.Entries) == 0 {
					return fmt.Errorf("no worktree history (use wt cd <branch> first)")
				}

				// Clean stale entries and find first valid
				if removed := hist.RemoveStale(); removed > 0 {
					if err := hist.Save(cfg.GetHistoryPath()); err != nil {
						l := log.FromContext(ctx)
						l.Printf("Warning: failed to save history after cleanup: %v\n", err)
					}
				}

				if len(hist.Entries) == 0 {
					return fmt.Errorf("no worktree history (all entries stale)")
				}

				hist.SortByRecency()
				entry := hist.Entries[0]
				targetPath = entry.Path
				repoName = entry.RepoName
				branchName = entry.Branch
			} else {
				// Parse positional argument
				arg := args[0]

				if before, after, ok := strings.Cut(arg, ":"); ok {
					// repo:branch format
					repoName = before
					branchName = after
				} else {
					// Just branch name
					branchName = arg
				}

				if repoName != "" {
					// Explicit repo specified - find exact worktree
					repo, err := reg.FindByName(repoName)
					if err != nil {
						return err
					}
					exists, pathErr := repo.PathExists()
					if pathErr != nil {
						return fmt.Errorf("%s: cannot access path (%s): %w", repo.Name, repo.Path, pathErr)
					}
					if !exists {
						return fmt.Errorf("%s: path no longer exists (%s)\n  Update with: wt repo add <new-path> --name %s", repo.Name, repo.Path, repo.Name)
					}

					worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
					if err != nil {
						return fmt.Errorf("list worktrees: %w", err)
					}

					for _, wt := range worktrees {
						if wt.Branch == branchName {
							targetPath = wt.Path
							break
						}
					}

					if targetPath == "" {
						return fmt.Errorf("worktree not found: %s:%s", repoName, branchName)
					}
				} else {
					// Search all repos for matching branch
					l := log.FromContext(ctx)
					type match struct {
						repoName string
						path     string
					}
					var matches []match

					searchRepos := filterOrphanedRepos(l, reg.Repos)

					for _, repo := range searchRepos {
						worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
						if err != nil {
							l.Debug("skipping repo", "repo", repo.Name, "error", err)
							continue
						}

						for _, wt := range worktrees {
							if wt.Branch == branchName {
								matches = append(matches, match{
									repoName: repo.Name,
									path:     wt.Path,
								})
							}
						}
					}

					if len(matches) == 0 {
						return fmt.Errorf("worktree not found: %s", branchName)
					}

					if len(matches) > 1 {
						// Ambiguous - list matches
						var names []string
						for _, m := range matches {
							names = append(names, m.repoName+":"+branchName)
						}
						return fmt.Errorf("branch %q exists in multiple repos: %s", branchName, strings.Join(names, ", "))
					}

					targetPath = matches[0].path
					repoName = matches[0].repoName
				}
			}

			// Record access to history for wt cd
			if err := history.RecordAccess(targetPath, repoName, branchName, cfg.GetHistoryPath()); err != nil {
				l := log.FromContext(ctx)
				l.Printf("Warning: failed to record history: %v\n", err)
			}

			// Copy to clipboard if requested
			if copyToClipboard {
				l := log.FromContext(ctx)
				if err := clipboard.WriteAll(targetPath); err != nil {
					l.Printf("Warning: failed to copy to clipboard: %v\n", err)
				}
			}

			// Print path
			out.Println(targetPath)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode with fuzzy search")
	cmd.Flags().BoolVar(&copyToClipboard, "copy", false, "Copy path to clipboard")

	// Register completions
	cmd.ValidArgsFunction = completeCdArg

	return cmd
}

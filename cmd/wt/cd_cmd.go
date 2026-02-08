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

			var targetPath string

			if interactive {
				// Interactive mode: show fuzzy list of all worktrees
				l := log.FromContext(ctx)
				var allWorktrees []flows.CdWorktreeInfo

				allRepos := filterOrphanedRepos(l, reg.Repos)

				for _, repo := range allRepos {
					worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
					if err != nil {
						l.Debug("skipping repo", "repo", repo.Name, "error", err)
						continue
					}
					notes, _ := git.GetAllBranchConfig(ctx, repo.Path)
					for _, wt := range worktrees {
						allWorktrees = append(allWorktrees, flows.CdWorktreeInfo{
							RepoName: repo.Name,
							Branch:   wt.Branch,
							Path:     wt.Path,
							IsDirty:  git.IsDirty(ctx, wt.Path),
							Note:     notes[wt.Branch],
						})
					}
				}

				if len(allWorktrees) == 0 {
					return fmt.Errorf("no worktrees found")
				}

				// Sort by repo/branch for consistent ordering
				sort.Slice(allWorktrees, func(i, j int) bool {
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
			} else if len(args) == 0 {
				// No arguments: return most recently accessed worktree
				mostRecent, err := history.GetMostRecent(cfg.GetHistoryPath())
				if err != nil {
					return fmt.Errorf("load history: %w", err)
				}
				if mostRecent == "" {
					return fmt.Errorf("no worktree history (use wt cd <branch> first)")
				}

				// Verify the path still exists
				if _, err := os.Stat(mostRecent); os.IsNotExist(err) {
					return fmt.Errorf("most recent worktree no longer exists: %s", mostRecent)
				}

				targetPath = mostRecent
			} else {
				// Parse positional argument
				arg := args[0]
				var repoName, branchName string

				if idx := strings.Index(arg, ":"); idx >= 0 {
					// repo:branch format
					repoName = arg[:idx]
					branchName = arg[idx+1:]
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
				}
			}

			// Record access to history for wt cd
			if err := history.RecordAccess(targetPath, cfg.GetHistoryPath()); err != nil {
				l := log.FromContext(ctx)
				l.Debug("failed to record history", "error", err)
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

package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
	"github.com/raphi011/wt/internal/ui"
)

func newCdCmd() *cobra.Command {
	var (
		repository  string
		label       string
		interactive bool
		hookNames   []string
		noHook      bool
		env         []string
	)

	cmd := &cobra.Command{
		Use:     "cd",
		Short:   "Print repo/worktree path for shell scripting",
		GroupID: GroupUtility,
		Args:    cobra.NoArgs,
		Long: `Print the path of a repo or worktree for shell scripting.

Use with shell command substitution: cd $(wt cd -r myrepo)`,
		Example: `  cd $(wt cd -r myrepo)   # cd to repo
  cd $(wt cd -l backend)  # cd to repo with label (must match one)
  cd $(wt cd -i)          # interactive fuzzy search for worktree`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			var targetPath string
			var repoName string

			if interactive {
				// Interactive mode: show fuzzy list of all worktrees
				var allWorktrees []ui.CdWorktreeInfo

				for i := range reg.Repos {
					repo := &reg.Repos[i]
					worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
					if err != nil {
						continue
					}
					notes, _ := git.GetAllBranchConfig(ctx, repo.Path)
					for _, wt := range worktrees {
						allWorktrees = append(allWorktrees, ui.CdWorktreeInfo{
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

				result, err := ui.CdInteractive(ui.CdWizardParams{
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
			} else {
				// Non-interactive mode
				var repo *registry.Repo

				if repository != "" {
					repo, err = reg.FindByName(repository)
					if err != nil {
						return err
					}
				} else if label != "" {
					repos := reg.FindByLabel(label)
					if len(repos) == 0 {
						return fmt.Errorf("no repo with label: %s", label)
					}
					if len(repos) > 1 {
						return fmt.Errorf("label %q matches multiple repos: use -r to specify", label)
					}
					repo = repos[0]
				} else {
					// Try current repo
					repoPath := git.GetCurrentRepoMainPath(ctx)
					if repoPath == "" {
						return fmt.Errorf("not in a git repository")
					}
					repo, err = reg.FindByPath(repoPath)
					if err != nil {
						return err
					}
				}

				targetPath = repo.Path
				repoName = repo.Name
			}

			// Print path
			out.Println(targetPath)

			// Run hooks
			hookEnv, err := hooks.ParseEnvWithStdin(env)
			if err != nil {
				return err
			}

			hookMatches, err := hooks.SelectHooks(cfg.Hooks, hookNames, noHook, hooks.CommandCd)
			if err != nil {
				return err
			}

			if len(hookMatches) > 0 {
				hookCtx := hooks.Context{
					WorktreeDir: targetPath,
					RepoDir:     targetPath,
					Repo:        repoName,
					Origin:      git.GetRepoDisplayName(targetPath),
					Trigger:     "cd",
					Env:         hookEnv,
				}
				hooks.RunAllNonFatal(hookMatches, hookCtx, targetPath)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.Flags().StringVarP(&label, "label", "l", "", "Repository label (must match one)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode with fuzzy search")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip hooks")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")

	cmd.MarkFlagsMutuallyExclusive("repository", "label", "interactive")
	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")

	// Completions
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("label", completeLabels)
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)

	return cmd
}

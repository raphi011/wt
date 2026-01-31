package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

func newCdCmd() *cobra.Command {
	var (
		repository string
		label      string
		hookNames  []string
		noHook     bool
		env        []string
	)

	cmd := &cobra.Command{
		Use:   "cd",
		Short: "Print repo path for shell scripting",
		Long: `Print the path of a repo for shell scripting.

Use with shell command substitution: cd $(wt cd -r myrepo)

Examples:
  cd $(wt cd -r myrepo)   # cd to repo
  cd $(wt cd -l backend)  # cd to repo with label (must match one)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

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

			// Print path
			out.Println(repo.Path)

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
					WorktreeDir: repo.Path,
					RepoDir:     repo.Path,
					Repo:        repo.Name,
					Origin:      git.GetRepoDisplayName(repo.Path),
					Trigger:     "cd",
					Env:         hookEnv,
				}
				hooks.RunAllNonFatal(hookMatches, hookCtx, repo.Path)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&repository, "repository", "r", "", "Repository name")
	cmd.Flags().StringVarP(&label, "label", "l", "", "Repository label (must match one)")
	cmd.Flags().StringSliceVar(&hookNames, "hook", nil, "Run named hook(s)")
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "Skip hooks")
	cmd.Flags().StringSliceVarP(&env, "arg", "a", nil, "Set hook variable KEY=VALUE")

	cmd.MarkFlagsMutuallyExclusive("repository", "label")
	cmd.MarkFlagsMutuallyExclusive("hook", "no-hook")

	// Completions
	cmd.RegisterFlagCompletionFunc("repository", completeRepoNames)
	cmd.RegisterFlagCompletionFunc("label", completeLabels)
	cmd.RegisterFlagCompletionFunc("hook", completeHooks)

	return cmd
}

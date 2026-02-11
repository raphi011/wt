package main

import (
	"fmt"
	"os"
	"os/exec"
	"slices"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "exec [repo:]branch... -- <command>",
		Short:   "Run command in worktree(s)",
		Aliases: []string{"x"},
		GroupID: GroupUtility,
		Long: `Run a command in one or more worktrees.

Target worktrees using [repo:]branch arguments before --:
  - branch: finds worktree in current repo, or all repos if ambiguous
  - repo:branch: finds exact worktree in specified repo

With no targets, runs in the current worktree.`,
		Example: `  wt exec -- git status                  # In current worktree
  wt exec main -- git status             # In main worktree
  wt exec wt:main -- git status          # In main worktree of wt repo
  wt exec wt:main myrepo:dev -- make test  # In multiple worktrees`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			workDir := config.WorkDirFromContext(ctx)
			l := log.FromContext(ctx)

			// Cobra handles -- specially - ArgsLenAtDash returns index where -- appeared
			dashIdx := cmd.ArgsLenAtDash()

			var targets []string
			var cmdArgs []string

			if dashIdx == -1 {
				// No --, treat all args as command
				cmdArgs = args
			} else {
				targets = args[:dashIdx]
				cmdArgs = args[dashIdx:]
			}

			if len(cmdArgs) == 0 {
				return fmt.Errorf("no command specified (use -- before command)")
			}

			// Load registry
			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Resolve target worktrees
			type worktreeTarget struct {
				repoName string
				path     string
			}
			var worktrees []worktreeTarget

			if len(targets) == 0 {
				// No targets - use current directory
				repoPath := git.GetCurrentRepoMainPathFrom(ctx, workDir)
				if repoPath == "" {
					return fmt.Errorf("not in a git repository")
				}
				repo, err := reg.FindByPath(repoPath)
				repoName := ""
				if err == nil {
					repoName = repo.Name
				}
				worktrees = append(worktrees, worktreeTarget{repoName: repoName, path: workDir})
			} else {
				// Parse each target
				for _, target := range targets {
					repoName, branchName := parseBranchTarget(target)

					if repoName != "" {
						// Explicit repo - find exact worktree
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

						wts, err := git.ListWorktreesFromRepo(ctx, repo.Path)
						if err != nil {
							return fmt.Errorf("list worktrees for %s: %w", repoName, err)
						}

						found := false
						for _, wt := range wts {
							if wt.Branch == branchName {
								worktrees = append(worktrees, worktreeTarget{
									repoName: repoName,
									path:     wt.Path,
								})
								found = true
								break
							}
						}

						if !found {
							return fmt.Errorf("worktree not found: %s:%s", repoName, branchName)
						}
					} else {
						// No repo specified - search all repos
						var matches []worktreeTarget

						searchRepos := filterOrphanedRepos(l, reg.Repos)

						for _, repo := range searchRepos {
							wts, err := git.ListWorktreesFromRepo(ctx, repo.Path)
							if err != nil {
								continue
							}

							for _, wt := range wts {
								if wt.Branch == branchName {
									matches = append(matches, worktreeTarget{
										repoName: repo.Name,
										path:     wt.Path,
									})
								}
							}
						}

						if len(matches) == 0 {
							return fmt.Errorf("worktree not found: %s", branchName)
						}

						// Add all matches (run in all repos with this branch)
						worktrees = append(worktrees, matches...)
					}
				}
			}

			// Deduplicate by path
			seen := make(map[string]bool)
			var unique []worktreeTarget
			for _, wt := range worktrees {
				if !seen[wt.path] {
					seen[wt.path] = true
					unique = append(unique, wt)
				}
			}
			worktrees = unique

			l.Debug("exec", "command", cmdArgs[0], "worktrees", len(worktrees))

			// Execute command in each worktree
			for _, wt := range worktrees {
				label := wt.repoName
				if label == "" {
					label = wt.path
				}
				fmt.Printf("=== %s ===\n", label)

				execCmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
				execCmd.Dir = wt.path
				execCmd.Stdout = os.Stdout
				execCmd.Stderr = os.Stderr
				execCmd.Stdin = os.Stdin

				if err := execCmd.Run(); err != nil {
					l.Printf("Error in %s: %v\n", label, err)
				}
				fmt.Println()
			}

			return nil
		},
	}

	// Register completions
	cmd.ValidArgsFunction = completeExecArg

	return cmd
}

// completeExecArg provides completion for exec command targets
func completeExecArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// After --, no more completions for targets
	if slices.Contains(args, "--") {
		return nil, cobra.ShellCompDirectiveDefault
	}

	// Reuse the same completion logic as cd
	return completeCdArg(cmd, nil, toComplete)
}

package main

import (
	"errors"
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
		Use:     "exec [[scope:]branch...] -- <command>",
		Short:   "Run command in worktree(s)",
		Aliases: []string{"x"},
		GroupID: GroupUtility,
		Long: `Run a command in one or more worktrees.

Target worktrees using [scope:]branch arguments before --:
  - branch: runs in every registered repo's worktree for that branch
  - repo:branch: finds exact worktree in specified repo
  - label:branch: runs in all repos with that label

With no targets, runs in the current worktree.

All targets are attempted even if a command fails. Failures return a non-zero
exit status: the command's exit code for a single worktree, or 1 for multiple
worktrees. Commands that cannot start also return 1.`,
		Example: `  wt exec -- git status                  # In current worktree
  wt exec main -- git status             # In main worktree of every repo
  wt exec wt:main -- git status          # In main worktree of wt repo
  wt exec backend:main -- make test      # In main worktree of backend-labeled repos
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
			var resolved []WorktreeTarget

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
				resolved = append(resolved, WorktreeTarget{RepoName: repoName, Path: workDir})
			} else {
				resolved, err = resolveWorktreeTargets(ctx, reg, targets)
				if err != nil {
					return err
				}
			}

			l.Debug("exec", "command", cmdArgs[0], "worktrees", len(resolved))

			// Execute command in each worktree, collecting failures without stopping.
			var failures []error
			exitCode := 1
			for _, wt := range resolved {
				label := wt.RepoName
				if label == "" {
					label = wt.Path
				}
				fmt.Printf("=== %s ===\n", label)

				execCmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
				execCmd.Dir = wt.Path
				execCmd.Stdout = os.Stdout
				execCmd.Stderr = os.Stderr
				execCmd.Stdin = os.Stdin

				if err := execCmd.Run(); err != nil {
					failedTarget := wt.Path
					if wt.RepoName != "" && wt.Branch != "" {
						failedTarget = wt.RepoName + ":" + wt.Branch
					}
					failures = append(failures, fmt.Errorf("command failed in %s: %w", failedTarget, err))
					var exitErr *exec.ExitError
					if len(resolved) == 1 && errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
						exitCode = exitErr.ExitCode()
					}
				}
				fmt.Println()
			}

			if len(failures) > 0 {
				return &commandExitError{err: errors.Join(failures...), code: exitCode}
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

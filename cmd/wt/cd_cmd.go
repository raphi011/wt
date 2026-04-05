package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"

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
	var global bool

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

With no arguments, returns the most recently accessed worktree.

Interactive mode (-i) is repo-aware: inside a repo it shows only that
repo's worktrees. Use -g to show all repos.`,
		Example: `  cd $(wt cd)              # cd to most recently accessed worktree
  cd $(wt cd feature-x)    # cd to feature-x worktree (error if ambiguous)
  cd $(wt cd wt:feature-x) # cd to feature-x worktree in wt repo
  cd $(wt cd -i)           # interactive: current repo's worktrees
  cd $(wt cd -i -g)        # interactive: all repos' worktrees
  wt cd --copy feature-x   # copy worktree path to clipboard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg := config.FromContext(ctx)
			out := output.FromContext(ctx)

			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			histPath, err := cfg.GetHistoryPath()
			if err != nil {
				return fmt.Errorf("history path: %w", err)
			}

			var targetPath, repoName, branchName string

			switch {
			case interactive:
				targetPath, repoName, branchName, err = runCdInteractive(ctx, reg, histPath, global)
			case len(args) == 0:
				targetPath, repoName, branchName, err = runCdRecent(ctx, cfg, histPath)
			default:
				targetPath, repoName, branchName, err = runCdTarget(ctx, reg, args[0])
			}
			if err != nil {
				return err
			}

			recordHistory(ctx, cfg, targetPath, repoName, branchName)

			if copyToClipboard {
				l := log.FromContext(ctx)
				if err := clipboard.WriteAll(targetPath); err != nil {
					l.Printf("Warning: failed to copy to clipboard: %v\n", err)
				}
			}

			out.Println(targetPath)

			// Emit OSC 7 directory hint so supporting terminal emulators
			// know the new CWD. Written directly to stderr (not via logger) because
			// this is a terminal protocol escape, not a log message — it should be
			// emitted even in --quiet mode.
			hostname, err := os.Hostname()
			if err != nil {
				hostname = ""
			}
			u := url.URL{Scheme: "file", Host: hostname, Path: targetPath}
			fmt.Fprintf(os.Stderr, "\033]7;%s\033\\", u.String())

			return nil
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode with fuzzy search")
	cmd.Flags().BoolVar(&copyToClipboard, "copy", false, "Copy path to clipboard")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Show worktrees from all repos (interactive mode)")

	// Register completions
	cmd.ValidArgsFunction = completeCdArg

	return cmd
}

// runCdInteractive shows a fuzzy-searchable worktree list and returns the selected path.
// When global is false, only worktrees from the current repo are shown.
func runCdInteractive(ctx context.Context, reg *registry.Registry, histPath string, global bool) (path, repoName, branch string, err error) {
	l := log.FromContext(ctx)

	hist, err := history.Load(histPath)
	if err != nil {
		l.Printf("Warning: failed to load history: %v\n", err)
		hist = &history.History{}
	}

	var repos []registry.Repo
	if global {
		repos = reg.Repos
	} else {
		repo, err := findOrRegisterCurrentRepoFromContext(ctx, reg)
		if err != nil {
			l.Debug("could not detect current repo, showing all", "error", err)
			repos = reg.Repos
		} else {
			repos = []registry.Repo{repo}
		}
	}
	repos = filterOrphanedRepos(l, repos)

	loaded, warnings := git.LoadWorktreesForRepos(ctx, reposToRefs(repos))
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
		return "", "", "", fmt.Errorf("no worktrees found")
	}

	// Opportunistically clean stale history entries
	if removed := hist.RemoveStale(); removed > 0 {
		if err := hist.Save(histPath); err != nil {
			l.Printf("Warning: failed to save history after cleanup: %v\n", err)
		}
	}

	sortCdWorktrees(allWorktrees)

	result, err := flows.CdInteractive(flows.CdWizardParams{
		Worktrees: allWorktrees,
	})
	if err != nil {
		return "", "", "", err
	}
	if result.Cancelled {
		os.Exit(1)
	}

	return result.SelectedPath, result.RepoName, result.Branch, nil
}

// runCdRecent returns the most recently accessed worktree from history.
func runCdRecent(ctx context.Context, cfg *config.Config, histPath string) (path, repoName, branch string, err error) {
	hist, err := history.Load(histPath)
	if err != nil {
		return "", "", "", fmt.Errorf("load history: %w", err)
	}
	if len(hist.Entries) == 0 {
		return "", "", "", fmt.Errorf("no worktree history (use wt cd <branch> first)")
	}

	if removed := hist.RemoveStale(); removed > 0 {
		if err := hist.Save(histPath); err != nil {
			l := log.FromContext(ctx)
			l.Printf("Warning: failed to save history after cleanup: %v\n", err)
		}
	}

	if len(hist.Entries) == 0 {
		return "", "", "", fmt.Errorf("no worktree history (all entries stale)")
	}

	hist.SortByRecency()
	entry := hist.Entries[0]
	return entry.Path, entry.RepoName, entry.Branch, nil
}

// sortCdWorktrees sorts worktrees with history first (by LastAccess desc),
// then worktrees without history (alphabetical by repo:branch).
func sortCdWorktrees(worktrees []flows.CdWorktreeInfo) {
	sort.Slice(worktrees, func(i, j int) bool {
		iHasHistory := !worktrees[i].LastAccess.IsZero()
		jHasHistory := !worktrees[j].LastAccess.IsZero()

		if iHasHistory && jHasHistory {
			return worktrees[i].LastAccess.After(worktrees[j].LastAccess)
		}
		if iHasHistory != jHasHistory {
			return iHasHistory
		}
		if worktrees[i].RepoName != worktrees[j].RepoName {
			return worktrees[i].RepoName < worktrees[j].RepoName
		}
		return worktrees[i].Branch < worktrees[j].Branch
	})
}

// runCdTarget resolves a [repo:]branch argument to a worktree path.
func runCdTarget(ctx context.Context, reg *registry.Registry, arg string) (path, repoName, branch string, err error) {
	match, err := resolveOneWorktreeTarget(ctx, reg, arg)
	if err != nil {
		return "", "", "", err
	}
	return match.Path, match.RepoName, match.Branch, nil
}

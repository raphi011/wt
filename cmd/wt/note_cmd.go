package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/registry"
)

func newNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "note",
		Short:   "Manage branch notes",
		Aliases: []string{"n"},
		GroupID: GroupUtility,
		Long: `Manage notes on branches.

Notes are stored in git config and displayed in list output.

Target a worktree using [scope:]branch where scope can be a repo name or label.
If no target is specified, uses the current worktree's branch.`,
		Example: `  wt note set "WIP"                    # Set note on current branch
  wt note set "WIP" main               # Set note on main (all repos with main)
  wt note set "WIP" myrepo:main        # Set note on main in myrepo
  wt note set "WIP" backend:feat       # Set note in all backend repos
  wt note get                          # Get note for current branch
  wt note get myrepo:feature           # Get note for specific worktree
  wt note clear                        # Clear note from current branch`,
	}

	cmd.AddCommand(newNoteSetCmd())
	cmd.AddCommand(newNoteGetCmd())
	cmd.AddCommand(newNoteClearCmd())

	return cmd
}

func newNoteSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set <text> [scope:]branch",
		Short:             "Set a note on a branch",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeNoteArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			text := args[0]

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Resolve target(s)
			targets, err := resolveNoteTargets(ctx, reg, args[1:])
			if err != nil {
				return err
			}

			// Set note on each target
			for _, t := range targets {
				if err := git.SetBranchNote(ctx, t.RepoPath, t.Branch, text); err != nil {
					return err
				}
				fmt.Printf("Note set on %s:%s\n", t.RepoName, t.Branch)
			}

			return nil
		},
	}

	return cmd
}

func newNoteGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "get [[scope:]branch]",
		Short:             "Get the note for a branch",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeNoteArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := output.FromContext(ctx)

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Resolve target(s)
			targets, err := resolveNoteTargets(ctx, reg, args)
			if err != nil {
				return err
			}

			// Get note from each target
			for _, t := range targets {
				note, err := git.GetBranchNote(ctx, t.RepoPath, t.Branch)
				if err != nil {
					return err
				}

				if len(targets) > 1 {
					if note != "" {
						out.Printf("%s:%s: %s\n", t.RepoName, t.Branch, note)
					}
				} else {
					if note != "" {
						out.Println(note)
					}
				}
			}
			return nil
		},
	}

	return cmd
}

func newNoteClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "clear [[scope:]branch]",
		Short:             "Clear the note from a branch",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeNoteArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Load registry
			reg, err := registry.Load()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Resolve target(s)
			targets, err := resolveNoteTargets(ctx, reg, args)
			if err != nil {
				return err
			}

			// Clear note from each target
			for _, t := range targets {
				if err := git.ClearBranchNote(ctx, t.RepoPath, t.Branch); err != nil {
					return err
				}
				fmt.Printf("Note cleared on %s:%s\n", t.RepoName, t.Branch)
			}

			return nil
		},
	}

	return cmd
}

// noteTarget holds a resolved note target
type noteTarget struct {
	RepoName string
	RepoPath string
	Branch   string
}

// resolveNoteTargets resolves targets for note commands.
// If no args, uses current worktree's branch.
// Otherwise parses [scope:]branch format.
func resolveNoteTargets(ctx context.Context, reg *registry.Registry, args []string) ([]noteTarget, error) {
	if len(args) == 0 {
		// No target - use current worktree's branch
		repo, branch, err := getCurrentRepoBranch(ctx, reg)
		if err != nil {
			return nil, err
		}
		return []noteTarget{{
			RepoName: repo.Name,
			RepoPath: repo.Path,
			Branch:   branch,
		}}, nil
	}

	// Parse the target
	target := args[0]
	parsed, err := parseScopedTarget(reg, target)
	if err != nil {
		return nil, err
	}

	if len(parsed.Repos) > 0 {
		// Scoped target - use specified repos
		var targets []noteTarget
		for _, repo := range parsed.Repos {
			targets = append(targets, noteTarget{
				RepoName: repo.Name,
				RepoPath: repo.Path,
				Branch:   parsed.Branch,
			})
		}
		return targets, nil
	}

	// No scope - search all repos for matching worktree
	var targets []noteTarget
	for i := range reg.Repos {
		repo := &reg.Repos[i]
		wts, err := git.ListWorktreesFromRepo(ctx, repo.Path)
		if err != nil {
			continue
		}
		for _, wt := range wts {
			if wt.Branch == parsed.Branch {
				targets = append(targets, noteTarget{
					RepoName: repo.Name,
					RepoPath: repo.Path,
					Branch:   parsed.Branch,
				})
				break
			}
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("worktree not found: %s", parsed.Branch)
	}

	return targets, nil
}

// getCurrentRepoBranch gets the repo and current branch from current directory
func getCurrentRepoBranch(ctx context.Context, reg *registry.Registry) (*registry.Repo, string, error) {
	repoPath := git.GetCurrentRepoMainPath(ctx)
	if repoPath == "" {
		return nil, "", fmt.Errorf("not in a git repository")
	}

	repo, err := reg.FindByPath(repoPath)
	if err != nil {
		// Auto-register
		newRepo := registry.Repo{
			Path:   repoPath,
			Name:   git.GetRepoDisplayName(repoPath),
			Labels: cfg.DefaultLabels,
		}
		if err := reg.Add(newRepo); err != nil {
			return nil, "", err
		}
		if err := reg.Save(); err != nil {
			return nil, "", err
		}
		repo, err = reg.FindByPath(repoPath)
		if err != nil {
			return nil, "", err
		}
	}

	branch, err := git.GetCurrentBranch(ctx, workDir)
	if err != nil {
		return nil, "", err
	}

	return repo, branch, nil
}

// completeNoteArg provides completion for note command targets
func completeNoteArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// For set command, first arg is text, second is target
	// For get/clear, first arg is target
	cmdName := cmd.Name()
	targetArgIndex := 0
	if cmdName == "set" {
		targetArgIndex = 1
	}

	if len(args) != targetArgIndex {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Reuse the same completion logic as cd
	return completeScopedWorktreeArg(cmd, args, toComplete)
}

// completeScopedWorktreeArg provides completion for [scope:]branch format
// including both repo: and label: prefixes
func completeScopedWorktreeArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := context.Background()

	// Check if user is typing scope:branch format
	if idx := strings.Index(toComplete, ":"); idx >= 0 {
		scopeName := toComplete[:idx]
		branchPrefix := toComplete[idx+1:]

		reg, err := registry.Load()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Try repo name first
		repo, err := reg.FindByName(scopeName)
		if err == nil {
			worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			var matches []string
			for _, wt := range worktrees {
				if strings.HasPrefix(wt.Branch, branchPrefix) {
					matches = append(matches, scopeName+":"+wt.Branch)
				}
			}
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		// Try label
		labelRepos := reg.FindByLabel(scopeName)
		if len(labelRepos) > 0 {
			// Collect unique branches across all labeled repos
			branchSet := make(map[string]bool)
			for _, repo := range labelRepos {
				worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
				if err != nil {
					continue
				}
				for _, wt := range worktrees {
					if strings.HasPrefix(wt.Branch, branchPrefix) {
						branchSet[wt.Branch] = true
					}
				}
			}

			var matches []string
			for branch := range branchSet {
				matches = append(matches, scopeName+":"+branch)
			}
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string

	// Check if we're inside a git repo
	currentRepoPath := git.GetCurrentRepoMainPath(ctx)
	if currentRepoPath != "" {
		// Inside a repo - show branches from current repo first (no prefix)
		worktrees, err := git.ListWorktreesFromRepo(ctx, currentRepoPath)
		if err == nil {
			for _, wt := range worktrees {
				if strings.HasPrefix(wt.Branch, toComplete) {
					matches = append(matches, wt.Branch)
				}
			}
		}
	}

	// Also offer "repo:" and "label:" completions
	reg, err := registry.Load()
	if err != nil {
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	// Repo prefixes
	for _, repo := range reg.Repos {
		prefix := repo.Name + ":"
		if strings.HasPrefix(prefix, toComplete) || strings.HasPrefix(toComplete, repo.Name) {
			matches = append(matches, prefix)
		}
	}

	// Label prefixes
	for _, label := range reg.AllLabels() {
		prefix := label + ":"
		if strings.HasPrefix(prefix, toComplete) || strings.HasPrefix(toComplete, label) {
			matches = append(matches, prefix)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

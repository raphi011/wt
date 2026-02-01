package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/registry"
)

// repoPathFromArgs extracts the repo path from positional args.
// It checks for:
// 1. A scoped target (repo:branch) pattern - extracts repo name before colon
// 2. A bare repo name that matches a registered repo
// Falls back to current directory if neither is found.
func repoPathFromArgs(ctx context.Context, args []string) string {
	reg, err := registry.Load()
	if err != nil {
		return git.GetCurrentRepoMainPath(ctx)
	}

	for _, arg := range args {
		// Check for scope:branch pattern
		if idx := strings.Index(arg, ":"); idx >= 0 {
			repoName := arg[:idx]
			if repo, err := reg.FindByName(repoName); err == nil {
				return repo.Path
			}
		}

		// Check if arg is a bare repo name
		if repo, err := reg.FindByName(arg); err == nil {
			return repo.Path
		}
	}

	// Fall back to current directory
	return git.GetCurrentRepoMainPath(ctx)
}

// completeBranches provides branch name completion.
// It checks args for a repo reference (scope:branch or bare repo name),
// otherwise uses current directory.
func completeBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := context.Background()

	repoPath := repoPathFromArgs(ctx, args)
	if repoPath == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	branches, err := git.ListLocalBranches(ctx, repoPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string
	for _, b := range branches {
		if strings.HasPrefix(b, toComplete) {
			matches = append(matches, b)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// completeRemoteBranches provides remote branch name completion.
// It checks args for a repo reference (scope:branch or bare repo name),
// otherwise uses current directory.
func completeRemoteBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := context.Background()

	repoPath := repoPathFromArgs(ctx, args)
	if repoPath == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	branches, err := git.ListRemoteBranches(ctx, repoPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string
	for _, b := range branches {
		if strings.HasPrefix(b, toComplete) {
			matches = append(matches, b)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// completeWorktrees provides worktree completion for the specified repo.
// Supports "branch", "repo:branch", and "label:branch" formats.
func completeWorktrees(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

	// Try current repo first
	repoPath := git.GetCurrentRepoMainPath(ctx)
	if repoPath != "" {
		worktrees, err := git.ListWorktreesFromRepo(ctx, repoPath)
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
	if err == nil {
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
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// completeCdArg provides completion for `wt cd [scope:]branch`
// scope can be a repo name or label.
// Inside a repo: shows branches from current repo (no prefix) + scope:branch for others
// Outside a repo or with scope prefix: shows scope:branch combinations
func completeCdArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

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

// Register completions for checkout command
func registerCheckoutCompletions(cmd *cobra.Command) {
	// Branch argument completion
	cmd.RegisterFlagCompletionFunc("base", completeBranches)

	// Positional arg (branch name) completion - both local and remote
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Get both local and remote branches
		local, _ := completeBranches(cmd, args, toComplete)
		remote, _ := completeRemoteBranches(cmd, args, toComplete)

		// Combine and deduplicate
		seen := make(map[string]bool)
		var all []string
		for _, b := range local {
			if !seen[b] {
				seen[b] = true
				all = append(all, b)
			}
		}
		for _, b := range remote {
			if !seen[b] {
				seen[b] = true
				all = append(all, b)
			}
		}

		return all, cobra.ShellCompDirectiveNoFileComp
	}
}

package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/registry"
)

// completeBranches provides branch name completion.
// It checks if -r flag is set and uses that repo's branches, otherwise uses current directory.
func completeBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := context.Background()

	// Check if -r flag is set
	repoName, _ := cmd.Flags().GetString("repository")
	if repoName == "" {
		// Try getting string slice version (for commands with multiple repos)
		repos, _ := cmd.Flags().GetStringSlice("repository")
		if len(repos) > 0 {
			repoName = repos[0]
		}
	}

	var repoPath string

	if repoName != "" {
		// Load registry and find repo
		reg, err := registry.Load()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		repo, err := reg.FindByName(repoName)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		repoPath = repo.Path
	} else {
		// Use current directory
		repoPath = git.GetCurrentRepoMainPath(ctx)
		if repoPath == "" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}

	// Get branches from the repo
	branches, err := git.ListLocalBranches(ctx, repoPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Filter by prefix
	var matches []string
	for _, b := range branches {
		if strings.HasPrefix(b, toComplete) {
			matches = append(matches, b)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// completeRemoteBranches provides remote branch name completion.
func completeRemoteBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := context.Background()

	// Check if -r flag is set
	repoName, _ := cmd.Flags().GetString("repository")
	if repoName == "" {
		repos, _ := cmd.Flags().GetStringSlice("repository")
		if len(repos) > 0 {
			repoName = repos[0]
		}
	}

	var repoPath string

	if repoName != "" {
		reg, err := registry.Load()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		repo, err := reg.FindByName(repoName)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		repoPath = repo.Path
	} else {
		repoPath = git.GetCurrentRepoMainPath(ctx)
		if repoPath == "" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
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
// Supports both "branch" and "repo:branch" formats.
func completeWorktrees(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx := context.Background()

	// Check if user is typing repo:branch format
	if idx := strings.Index(toComplete, ":"); idx >= 0 {
		// User is typing "repo:branch" - complete branch within specified repo
		repoName := toComplete[:idx]
		branchPrefix := toComplete[idx+1:]

		reg, err := registry.Load()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		repo, err := reg.FindByName(repoName)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var matches []string
		for _, wt := range worktrees {
			if strings.HasPrefix(wt.Branch, branchPrefix) {
				matches = append(matches, repoName+":"+wt.Branch)
			}
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	// Check if -r flag is set
	repoName, _ := cmd.Flags().GetString("repository")
	if repoName == "" {
		repos, _ := cmd.Flags().GetStringSlice("repository")
		if len(repos) > 0 {
			repoName = repos[0]
		}
	}

	var matches []string

	// If repo flag is set, only complete branches from that repo
	if repoName != "" {
		reg, err := registry.Load()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		repo, err := reg.FindByName(repoName)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		for _, wt := range worktrees {
			if strings.HasPrefix(wt.Branch, toComplete) {
				matches = append(matches, wt.Branch)
			}
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	// No repo specified - try current repo first
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

	// Also offer "repo:" completions for all registered repos
	reg, err := registry.Load()
	if err == nil {
		for _, repo := range reg.Repos {
			prefix := repo.Name + ":"
			if strings.HasPrefix(prefix, toComplete) || strings.HasPrefix(toComplete, repo.Name) {
				matches = append(matches, prefix)
			}
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// completeCdArg provides completion for `wt cd [repo:]branch`
// Inside a repo: shows branches from current repo (no prefix) + repo:branch for others
// Outside a repo or with repo prefix: shows repo:branch combinations
func completeCdArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	ctx := context.Background()

	// Check if user is typing repo:branch format
	if idx := strings.Index(toComplete, ":"); idx >= 0 {
		// User is typing "repo:branch" - complete branch within specified repo
		repoName := toComplete[:idx]
		branchPrefix := toComplete[idx+1:]

		reg, err := registry.Load()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		repo, err := reg.FindByName(repoName)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var matches []string
		for _, wt := range worktrees {
			if strings.HasPrefix(wt.Branch, branchPrefix) {
				matches = append(matches, repoName+":"+wt.Branch)
			}
		}
		return matches, cobra.ShellCompDirectiveNoFileComp
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

	// Also offer "repo:" completions for all registered repos
	reg, err := registry.Load()
	if err != nil {
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	for _, repo := range reg.Repos {
		prefix := repo.Name + ":"
		if strings.HasPrefix(prefix, toComplete) || strings.HasPrefix(toComplete, repo.Name) {
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

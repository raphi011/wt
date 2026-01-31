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
func completeWorktrees(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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

	worktrees, err := git.ListWorktreesFromRepo(ctx, repoPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string
	for _, wt := range worktrees {
		if strings.HasPrefix(wt.Branch, toComplete) {
			matches = append(matches, wt.Branch)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// completeRepoNamesArg provides repo name completion for positional arguments
func completeRepoNamesArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Don't complete if we already have an arg
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	reg, err := registry.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := reg.AllRepoNames()
	var matches []string
	for _, n := range names {
		if strings.HasPrefix(n, toComplete) {
			matches = append(matches, n)
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

// Register completions for remove command
func registerRemoveCompletions(cmd *cobra.Command) {
	cmd.ValidArgsFunction = completeRepoNamesArg
}

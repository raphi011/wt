package main

import (
	"context"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/registry"
)

// repoPathFromArgs extracts the repo path from positional args.
// It checks for:
// 1. A scoped target (repo:branch) pattern - extracts repo name before colon
// 2. A bare repo name that matches a registered repo
// Falls back to current directory if neither is found.
func repoPathFromArgs(ctx context.Context, cfg *config.Config, args []string) string {
	reg, err := registry.Load(cfg.RegistryPath)
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
	cfg := config.FromContext(cmd.Context())
	ctx := context.Background()

	repoPath := repoPathFromArgs(ctx, cfg, args)
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
	cfg := config.FromContext(cmd.Context())
	ctx := context.Background()

	repoPath := repoPathFromArgs(ctx, cfg, args)
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

// completeBaseBranches provides completion for the --base flag.
// Supports both local branches and explicit remote refs (origin/branch, upstream/branch).
func completeBaseBranches(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg := config.FromContext(cmd.Context())
	ctx := context.Background()

	repoPath := repoPathFromArgs(ctx, cfg, args)
	if repoPath == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string

	// Check if user is typing a remote prefix (e.g., "origin/", "upstream/")
	if idx := strings.Index(toComplete, "/"); idx >= 0 {
		remoteName := toComplete[:idx]
		branchPrefix := toComplete[idx+1:]

		// Verify remote exists
		if git.HasRemote(ctx, repoPath, remoteName) {
			// List remote branches and filter by prefix
			remoteBranches, err := git.ListRemoteBranches(ctx, repoPath)
			if err == nil {
				prefix := remoteName + "/"
				for _, b := range remoteBranches {
					// Remote branches are already prefixed with remote name
					if strings.HasPrefix(b, prefix) {
						branchPart := strings.TrimPrefix(b, prefix)
						if strings.HasPrefix(branchPart, branchPrefix) {
							matches = append(matches, b)
						}
					}
				}
			}
			return matches, cobra.ShellCompDirectiveNoFileComp
		}
	}

	// Local branches (simple branch names)
	localBranches, err := git.ListLocalBranches(ctx, repoPath)
	if err == nil {
		for _, b := range localBranches {
			if strings.HasPrefix(b, toComplete) {
				matches = append(matches, b)
			}
		}
	}

	// Also suggest remote prefixes if user might want explicit remote refs
	remotes, err := git.ListRemotes(ctx, repoPath)
	if err == nil {
		for _, remote := range remotes {
			prefix := remote + "/"
			if strings.HasPrefix(prefix, toComplete) {
				matches = append(matches, prefix)
			}
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// completeWorktrees provides worktree completion for the specified repo.
// Supports "branch", "repo:branch", and "label:branch" formats.
func completeWorktrees(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg := config.FromContext(cmd.Context())
	ctx := context.Background()

	// Check if user is typing scope:branch format
	if idx := strings.Index(toComplete, ":"); idx >= 0 {
		scopeName := toComplete[:idx]
		branchPrefix := toComplete[idx+1:]

		reg, err := registry.Load(cfg.RegistryPath)
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
	reg, err := registry.Load(cfg.RegistryPath)
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

	cfg := config.FromContext(cmd.Context())
	ctx := context.Background()

	// Check if user is typing scope:branch format
	if idx := strings.Index(toComplete, ":"); idx >= 0 {
		scopeName := toComplete[:idx]
		branchPrefix := toComplete[idx+1:]

		reg, err := registry.Load(cfg.RegistryPath)
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
	reg, err := registry.Load(cfg.RegistryPath)
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

// completePrCheckoutArgs provides completion for `wt pr checkout [repo] <number>`.
// First arg: repo names (if not numeric). No completion for PR numbers.
func completePrCheckoutArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// If we already have an arg that looks like a PR number, no more completion
	if len(args) >= 1 {
		// Check if first arg is numeric (PR number) - no more args needed
		if _, err := strconv.Atoi(args[0]); err == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		// First arg is repo, second would be PR number - no completion
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg := config.FromContext(cmd.Context())

	// First arg: offer repo names
	reg, err := registry.Load(cfg.RegistryPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string
	for _, name := range reg.AllRepoNames() {
		if strings.HasPrefix(name, toComplete) {
			matches = append(matches, name)
		}
	}
	return matches, cobra.ShellCompDirectiveNoFileComp
}

// completeLabelAddArgs provides completion for `wt label add/remove <label> [scope...]`.
// First arg: label names. Subsequent args: scope args (repo names + labels).
func completeLabelAddArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// First arg is the label
	if len(args) == 0 {
		return completeLabels(cmd, args, toComplete)
	}

	// Subsequent args are scopes
	return completeScopeArgs(cmd, args, toComplete)
}

// completeScopeArgs provides completion for scope arguments (repo names + labels).
// Used by commands like `wt list` where positional args can be repo names or labels.
func completeScopeArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg := config.FromContext(cmd.Context())

	reg, err := registry.Load(cfg.RegistryPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var matches []string

	// Repo names
	for _, name := range reg.AllRepoNames() {
		if strings.HasPrefix(name, toComplete) {
			matches = append(matches, name)
		}
	}

	// Labels
	for _, label := range reg.AllLabels() {
		if strings.HasPrefix(label, toComplete) {
			matches = append(matches, label)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

// Register completions for checkout command
func registerCheckoutCompletions(cmd *cobra.Command) {
	// Branch argument completion for --base flag (supports both local and remote refs)
	cmd.RegisterFlagCompletionFunc("base", completeBaseBranches)

	// Positional arg completion for [scope:]branch
	// Suggests repo/label prefixes and existing branches (even with -b, as branch name inspiration)
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		cfg := config.FromContext(cmd.Context())
		ctx := context.Background()

		// If user is typing scope:branch format, complete the branch part
		if idx := strings.Index(toComplete, ":"); idx >= 0 {
			scopeName := toComplete[:idx]
			branchPrefix := toComplete[idx+1:]

			reg, err := registry.Load(cfg.RegistryPath)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			// Try repo name first
			repo, err := reg.FindByName(scopeName)
			if err == nil {
				// Suggest local branches, excluding checked-out ones
				branches, err := git.ListLocalBranches(ctx, repo.Path)
				if err != nil {
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				wtBranches := git.GetWorktreeBranches(ctx, repo.Path)

				var matches []string
				for _, b := range branches {
					if strings.HasPrefix(b, branchPrefix) && !wtBranches[b] {
						matches = append(matches, scopeName+":"+b)
					}
				}
				return matches, cobra.ShellCompDirectiveNoFileComp
			}

			// Try label
			labelRepos := reg.FindByLabel(scopeName)
			if len(labelRepos) > 0 {
				// Collect unique branches across all labeled repos, excluding checked-out ones
				branchSet := make(map[string]bool)
				for _, r := range labelRepos {
					branches, err := git.ListLocalBranches(ctx, r.Path)
					if err != nil {
						continue
					}
					wtBranches := git.GetWorktreeBranches(ctx, r.Path)
					for _, b := range branches {
						if strings.HasPrefix(b, branchPrefix) && !wtBranches[b] {
							branchSet[b] = true
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

		// No colon yet - suggest repo prefixes and optionally branches
		var matches []string

		// Load registry for repo/label prefixes
		reg, err := registry.Load(cfg.RegistryPath)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// Repo prefixes (always suggest these)
		for _, repo := range reg.Repos {
			prefix := repo.Name + ":"
			if strings.HasPrefix(prefix, toComplete) {
				matches = append(matches, prefix)
			}
		}

		// Label prefixes (always suggest these)
		for _, label := range reg.AllLabels() {
			prefix := label + ":"
			if strings.HasPrefix(prefix, toComplete) {
				matches = append(matches, prefix)
			}
		}

		// Also suggest existing branches (excluding checked-out ones)
		// Even with -b, users may want to base new branch names on existing ones
		currentRepoPath := git.GetCurrentRepoMainPath(ctx)
		if currentRepoPath != "" {
			wtBranches := git.GetWorktreeBranches(ctx, currentRepoPath)

			// Local branches
			branches, err := git.ListLocalBranches(ctx, currentRepoPath)
			if err == nil {
				for _, b := range branches {
					if strings.HasPrefix(b, toComplete) && !wtBranches[b] {
						matches = append(matches, b)
					}
				}
			}

			// Remote branches (exclude those matching checked-out local branches)
			remoteBranches, err := git.ListRemoteBranches(ctx, currentRepoPath)
			if err == nil {
				seen := make(map[string]bool)
				for _, b := range matches {
					seen[b] = true
				}
				for _, b := range remoteBranches {
					if strings.HasPrefix(b, toComplete) && !seen[b] && !wtBranches[b] {
						matches = append(matches, b)
					}
				}
			}
		}

		return matches, cobra.ShellCompDirectiveNoFileComp
	}
}

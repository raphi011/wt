package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
)

// resolveLabelRepos resolves repository paths for label operations.
// If repos is empty and inside a repo, uses current repo.
// If repos is empty and outside a repo, returns error.
// If repos is provided, resolves each by name via git.FindRepoByName.
func resolveLabelRepos(ctx context.Context, repos []string, cfg *config.Config, workDir string) ([]string, error) {
	// Determine scan directory for repo lookup
	scanDir := cfg.RepoScanDir()
	if scanDir == "" {
		scanDir = "."
	}
	scanDir, err := filepath.Abs(scanDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// If no repos specified, try to use current repo
	if len(repos) == 0 {
		// Check if inside a git repo (regular or worktree)
		if git.IsInsideRepoPath(ctx, workDir) {
			// If in a worktree, get the main repo
			if git.IsWorktree(workDir) {
				mainRepo, err := git.GetMainRepoPath(workDir)
				if err != nil {
					return nil, fmt.Errorf("failed to get main repo path: %w", err)
				}
				return []string{mainRepo}, nil
			}
			// In main repo, use toplevel
			return []string{git.GetCurrentRepoMainPathFrom(ctx, workDir)}, nil
		}

		return nil, fmt.Errorf("not inside a git repository, specify -r/--repository")
	}

	// Resolve each repo by name
	var repoPaths []string

	for _, repoName := range repos {
		repoPath, err := git.FindRepoByName(scanDir, repoName)
		if err != nil {
			// Skip repos that can't be found - errors will be handled per-operation
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", repoName, err)
			continue
		}
		repoPaths = append(repoPaths, repoPath)
	}

	if len(repoPaths) == 0 {
		return nil, fmt.Errorf("no valid repositories found")
	}

	return repoPaths, nil
}

func (c *LabelAddCmd) runLabelAdd(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	repoPaths, err := resolveLabelRepos(ctx, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	var errs []error
	for _, repoPath := range repoPaths {
		repoName := filepath.Base(repoPath)
		if err := git.AddLabel(ctx, repoPath, c.Label); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
			continue
		}
		fmt.Printf("Label %q added to %s\n", c.Label, repoName)
	}

	return errors.Join(errs...)
}

func (c *LabelRemoveCmd) runLabelRemove(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	repoPaths, err := resolveLabelRepos(ctx, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	var errs []error
	for _, repoPath := range repoPaths {
		repoName := filepath.Base(repoPath)
		if err := git.RemoveLabel(ctx, repoPath, c.Label); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
			continue
		}
		fmt.Printf("Label %q removed from %s\n", c.Label, repoName)
	}

	return errors.Join(errs...)
}

func (c *LabelListCmd) runLabelList(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	// If --global flag, list labels from all repos in directory
	if c.Global {
		return c.runLabelListGlobal(ctx)
	}

	repoPaths, err := resolveLabelRepos(ctx, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	// If multiple repos, show repo name with labels
	showRepoName := len(repoPaths) > 1

	for _, repoPath := range repoPaths {
		labels, err := git.GetLabels(ctx, repoPath)
		if err != nil {
			return fmt.Errorf("failed to get labels: %w", err)
		}

		if len(labels) == 0 {
			continue
		}

		if showRepoName {
			repoName := git.GetRepoDisplayName(repoPath)
			fmt.Printf("%s: %s\n", repoName, strings.Join(labels, ", "))
		} else {
			fmt.Println(strings.Join(labels, "\n"))
		}
	}
	return nil
}

func (c *LabelListCmd) runLabelListGlobal(ctx context.Context) error {
	cfg := c.Config
	// Use repo_dir from config if available, fallback to cwd
	scanDir := cfg.RepoScanDir()
	if scanDir == "" {
		scanDir = "."
	}

	absDir, err := filepath.Abs(scanDir)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	repos, err := git.FindAllRepos(absDir)
	if err != nil {
		return fmt.Errorf("failed to scan repos: %w", err)
	}

	// Collect all unique labels and repos per label
	labelRepos := make(map[string][]string)
	for _, repoPath := range repos {
		labels, err := git.GetLabels(ctx, repoPath)
		if err != nil {
			continue
		}
		repoName := git.GetRepoDisplayName(repoPath)
		for _, label := range labels {
			labelRepos[label] = append(labelRepos[label], repoName)
		}
	}

	if len(labelRepos) == 0 {
		return nil
	}

	for label, repos := range labelRepos {
		fmt.Printf("%s: %s\n", label, strings.Join(repos, ", "))
	}
	return nil
}

func (c *LabelClearCmd) runLabelClear(ctx context.Context) error {
	cfg := c.Config
	workDir := c.WorkDir
	repoPaths, err := resolveLabelRepos(ctx, c.Repository, cfg, workDir)
	if err != nil {
		return err
	}

	var errs []error
	for _, repoPath := range repoPaths {
		repoName := filepath.Base(repoPath)
		if err := git.ClearLabels(ctx, repoPath); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", repoName, err))
			continue
		}
		fmt.Printf("Labels cleared from %s\n", repoName)
	}

	return errors.Join(errs...)
}

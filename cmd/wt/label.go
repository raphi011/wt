package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/output"
)

// resolveLabelRepos resolves repository paths for label operations.
// If repos is empty and inside a repo, uses current repo.
// If repos is empty and outside a repo, returns error.
// If repos is provided, resolves each by name via git.FindRepoByName.
func resolveLabelRepos(ctx context.Context, repos []string, repoDir, workDir string) ([]string, error) {
	// Determine repo directory for lookup
	if repoDir == "" {
		repoDir = "."
	}
	var err error
	repoDir, err = filepath.Abs(repoDir)
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
		repoPath, err := git.FindRepoByName(repoDir, repoName)
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
	l := log.FromContext(ctx)
	repoPaths, err := resolveLabelRepos(ctx, c.Repository, c.Config.RepoScanDir(), c.WorkDir)
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
		l.Printf("Label %q added to %s\n", c.Label, repoName)
	}

	return errors.Join(errs...)
}

func (c *LabelRemoveCmd) runLabelRemove(ctx context.Context) error {
	l := log.FromContext(ctx)
	repoPaths, err := resolveLabelRepos(ctx, c.Repository, c.Config.RepoScanDir(), c.WorkDir)
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
		l.Printf("Label %q removed from %s\n", c.Label, repoName)
	}

	return errors.Join(errs...)
}

func (c *LabelListCmd) runLabelList(ctx context.Context) error {
	out := output.FromContext(ctx)

	// If --global flag, list labels from all repos in directory
	if c.Global {
		return c.runLabelListGlobal(ctx)
	}

	repoPaths, err := resolveLabelRepos(ctx, c.Repository, c.Config.RepoScanDir(), c.WorkDir)
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
			out.Printf("%s: %s\n", repoName, strings.Join(labels, ", "))
		} else {
			out.Println(strings.Join(labels, "\n"))
		}
	}
	return nil
}

func (c *LabelListCmd) runLabelListGlobal(ctx context.Context) error {
	out := output.FromContext(ctx)
	cfg := c.Config

	// Use repo_dir from config if available, fallback to cwd
	repoDir := cfg.RepoScanDir()
	if repoDir == "" {
		repoDir = "."
	}

	absDir, err := filepath.Abs(repoDir)
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
		out.Printf("%s: %s\n", label, strings.Join(repos, ", "))
	}
	return nil
}

func (c *LabelClearCmd) runLabelClear(ctx context.Context) error {
	l := log.FromContext(ctx)
	repoPaths, err := resolveLabelRepos(ctx, c.Repository, c.Config.RepoScanDir(), c.WorkDir)
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
		l.Printf("Labels cleared from %s\n", repoName)
	}

	return errors.Join(errs...)
}

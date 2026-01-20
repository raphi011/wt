package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/raphi011/wt/internal/git"
)

// resolveLabelRepo resolves the repository path for label operations.
// If inside a git repo, uses that. Otherwise uses dir flag.
func resolveLabelRepo(dir string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if inside a git repo (regular or worktree)
	if git.IsInsideRepo() {
		// If in a worktree, get the main repo
		if git.IsWorktree(cwd) {
			mainRepo, err := git.GetMainRepoPath(cwd)
			if err != nil {
				return "", fmt.Errorf("failed to get main repo path: %w", err)
			}
			return mainRepo, nil
		}
		// In main repo, use toplevel
		return git.GetCurrentRepoMainPath(), nil
	}

	// Not in repo - need dir flag
	if dir == "" {
		return "", fmt.Errorf("not inside a git repository, specify --dir")
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if dir is a git repo
	gitPath := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		return "", fmt.Errorf("%s is not a git repository", absDir)
	}

	return absDir, nil
}

func runLabelAdd(cmd *LabelAddCmd) error {
	repoPath, err := resolveLabelRepo(cmd.Dir)
	if err != nil {
		return err
	}

	if err := git.AddLabel(repoPath, cmd.Label); err != nil {
		return fmt.Errorf("failed to add label: %w", err)
	}

	repoName := filepath.Base(repoPath)
	fmt.Printf("Label %q added to %s\n", cmd.Label, repoName)
	return nil
}

func runLabelRemove(cmd *LabelRemoveCmd) error {
	repoPath, err := resolveLabelRepo(cmd.Dir)
	if err != nil {
		return err
	}

	if err := git.RemoveLabel(repoPath, cmd.Label); err != nil {
		return fmt.Errorf("failed to remove label: %w", err)
	}

	repoName := filepath.Base(repoPath)
	fmt.Printf("Label %q removed from %s\n", cmd.Label, repoName)
	return nil
}

func runLabelList(cmd *LabelListCmd) error {
	// If --all flag, list labels from all repos in directory
	if cmd.All {
		return runLabelListAll(cmd)
	}

	repoPath, err := resolveLabelRepo(cmd.Dir)
	if err != nil {
		return err
	}

	labels, err := git.GetLabels(repoPath)
	if err != nil {
		return fmt.Errorf("failed to get labels: %w", err)
	}

	if len(labels) == 0 {
		return nil
	}

	fmt.Println(strings.Join(labels, "\n"))
	return nil
}

func runLabelListAll(cmd *LabelListCmd) error {
	scanDir := cmd.Dir
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
		labels, err := git.GetLabels(repoPath)
		if err != nil {
			continue
		}
		repoName, _ := git.GetRepoNameFrom(repoPath)
		if repoName == "" {
			repoName = filepath.Base(repoPath)
		}
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

func runLabelClear(cmd *LabelClearCmd) error {
	repoPath, err := resolveLabelRepo(cmd.Dir)
	if err != nil {
		return err
	}

	if err := git.ClearLabels(repoPath); err != nil {
		return fmt.Errorf("failed to clear labels: %w", err)
	}

	repoName := filepath.Base(repoPath)
	fmt.Printf("Labels cleared from %s\n", repoName)
	return nil
}

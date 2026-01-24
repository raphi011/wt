package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
)

// RepoInfo contains information about a repository
type RepoInfo struct {
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	Branch        string   `json:"branch"`
	Labels        []string `json:"labels"`
	OriginURL     string   `json:"origin_url"`
	WorktreeCount int      `json:"worktree_count"`
}

func runRepos(cmd *ReposCmd, cfg *config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Determine scan directory from config
	scanDir := cfg.RepoScanDir()
	if scanDir == "" {
		scanDir = "."
	}

	absScanDir, err := filepath.Abs(scanDir)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Find all repositories
	repoPaths, err := git.FindAllRepos(absScanDir)
	if err != nil {
		return fmt.Errorf("failed to scan repos: %w", err)
	}

	// Gather info for each repo
	var repos []RepoInfo
	for _, repoPath := range repoPaths {
		// Get labels
		labels, _ := git.GetLabels(repoPath)

		// Filter by label if specified
		if cmd.Label != "" {
			hasLabel := false
			for _, l := range labels {
				if l == cmd.Label {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				continue
			}
		}

		// Get repo name from folder name
		name := filepath.Base(repoPath)

		// Get current branch
		branch, _ := git.GetCurrentBranch(repoPath)

		// Get origin URL
		originURL, _ := git.GetOriginURL(repoPath)

		// Count worktrees (excluding the main repo itself)
		wtInfos, _ := git.ListWorktreesFromRepo(repoPath)
		worktreeCount := 0
		if len(wtInfos) > 1 {
			worktreeCount = len(wtInfos) - 1 // Subtract 1 for the main worktree
		}

		repos = append(repos, RepoInfo{
			Name:          name,
			Path:          repoPath,
			Branch:        branch,
			Labels:        labels,
			OriginURL:     originURL,
			WorktreeCount: worktreeCount,
		})
	}

	// Sort repos
	sortRepos(repos, cmd.Sort)

	if cmd.JSON {
		data, err := json.MarshalIndent(repos, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(repos) == 0 {
		if cmd.Label != "" {
			fmt.Printf("No repositories found with label %q\n", cmd.Label)
		} else {
			fmt.Printf("No repositories found in %s\n", absScanDir)
		}
		return nil
	}

	fmt.Printf("Repositories in %s (%d)\n\n", absScanDir, len(repos))

	// Build table
	fmt.Print(formatReposTable(repos))

	return nil
}

func formatReposTable(repos []RepoInfo) string {
	if len(repos) == 0 {
		return ""
	}

	var output strings.Builder

	// Prepare row data
	type rowData struct {
		name      string
		branch    string
		labels    string
		origin    string
		worktrees string
	}
	var rowsData []rowData

	for _, repo := range repos {
		name := repo.Name
		branch := repo.Branch
		labels := strings.Join(repo.Labels, ", ")
		origin := repo.OriginURL
		worktrees := fmt.Sprintf("%d", repo.WorktreeCount)

		rowsData = append(rowsData, rowData{
			name:      name,
			branch:    branch,
			labels:    labels,
			origin:    origin,
			worktrees: worktrees,
		})
	}

	// Build header row
	headers := []string{"NAME", "BRANCH", "LABELS", "ORIGIN", "WORKTREES"}

	// Build data rows
	var rows [][]string
	for _, rd := range rowsData {
		rows = append(rows, []string{rd.name, rd.branch, rd.labels, rd.origin, rd.worktrees})
	}

	// Create table with lipgloss/table
	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	output.WriteString(t.String())
	output.WriteString("\n")

	return output.String()
}

func sortRepos(repos []RepoInfo, sortBy string) {
	switch sortBy {
	case "branch":
		slices.SortFunc(repos, func(a, b RepoInfo) int {
			return cmp.Compare(a.Branch, b.Branch)
		})
	case "worktrees":
		// Sort by worktree count descending (most worktrees first)
		slices.SortFunc(repos, func(a, b RepoInfo) int {
			return cmp.Compare(b.WorktreeCount, a.WorktreeCount)
		})
	case "label":
		// Sort by first label alphabetically (unlabeled repos last, then by name)
		slices.SortFunc(repos, func(a, b RepoInfo) int {
			la := ""
			lb := ""
			if len(a.Labels) > 0 {
				la = a.Labels[0]
			}
			if len(b.Labels) > 0 {
				lb = b.Labels[0]
			}
			// Unlabeled repos go last
			if la == "" && lb != "" {
				return 1
			}
			if la != "" && lb == "" {
				return -1
			}
			// Same label (or both empty): sort by name
			if la == lb {
				return cmp.Compare(a.Name, b.Name)
			}
			return cmp.Compare(la, lb)
		})
	default: // "name" or empty
		slices.SortFunc(repos, func(a, b RepoInfo) int {
			return cmp.Compare(a.Name, b.Name)
		})
	}
}

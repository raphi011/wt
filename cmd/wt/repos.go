package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

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

	// Determine scan directory
	scanDir := cmd.Dir
	if scanDir == "" {
		scanDir = cfg.RepoScanDir()
	}
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

		// Get repo name from origin URL (fallback to folder name)
		name, err := git.GetRepoNameFrom(repoPath)
		if err != nil {
			name = filepath.Base(repoPath)
		}

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

	// Track max widths for each column
	maxNameWidth := len("NAME")
	maxBranchWidth := len("BRANCH")
	maxLabelsWidth := len("LABELS")
	maxOriginWidth := len("ORIGIN")
	maxWTWidth := len("WORKTREES")

	// First pass: calculate widths and prepare row data
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

		// Track max widths
		if len(name) > maxNameWidth {
			maxNameWidth = len(name)
		}
		if len(branch) > maxBranchWidth {
			maxBranchWidth = len(branch)
		}
		if len(labels) > maxLabelsWidth {
			maxLabelsWidth = len(labels)
		}
		if len(origin) > maxOriginWidth {
			maxOriginWidth = len(origin)
		}
		if len(worktrees) > maxWTWidth {
			maxWTWidth = len(worktrees)
		}

		rowsData = append(rowsData, rowData{
			name:      name,
			branch:    branch,
			labels:    labels,
			origin:    origin,
			worktrees: worktrees,
		})
	}

	// Create table columns with calculated widths + spacing
	columns := []table.Column{
		{Title: "NAME", Width: maxNameWidth + 2},
		{Title: "BRANCH", Width: maxBranchWidth + 2},
		{Title: "LABELS", Width: maxLabelsWidth + 2},
		{Title: "ORIGIN", Width: maxOriginWidth + 2},
		{Title: "WORKTREES", Width: maxWTWidth},
	}

	// Build rows
	var rows []table.Row
	for _, rd := range rowsData {
		rows = append(rows, table.Row{rd.name, rd.branch, rd.labels, rd.origin, rd.worktrees})
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(len(rows)+1),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true).
		Padding(0)
	s.Cell = lipgloss.NewStyle().Padding(0)
	s.Selected = lipgloss.NewStyle().Padding(0)
	t.SetStyles(s)

	output.WriteString(t.View())
	output.WriteString("\n")

	return output.String()
}

func sortRepos(repos []RepoInfo, sortBy string) {
	switch sortBy {
	case "branch":
		sort.Slice(repos, func(i, j int) bool {
			return repos[i].Branch < repos[j].Branch
		})
	case "worktrees":
		// Sort by worktree count descending (most worktrees first)
		sort.Slice(repos, func(i, j int) bool {
			return repos[i].WorktreeCount > repos[j].WorktreeCount
		})
	case "label":
		// Sort by first label alphabetically (unlabeled repos last, then by name)
		sort.Slice(repos, func(i, j int) bool {
			li := ""
			lj := ""
			if len(repos[i].Labels) > 0 {
				li = repos[i].Labels[0]
			}
			if len(repos[j].Labels) > 0 {
				lj = repos[j].Labels[0]
			}
			// Unlabeled repos go last
			if li == "" && lj != "" {
				return false
			}
			if li != "" && lj == "" {
				return true
			}
			// Same label (or both empty): sort by name
			if li == lj {
				return repos[i].Name < repos[j].Name
			}
			return li < lj
		})
	default: // "name" or empty
		sort.Slice(repos, func(i, j int) bool {
			return repos[i].Name < repos[j].Name
		})
	}
}

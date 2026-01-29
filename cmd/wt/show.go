package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/output"
	"github.com/raphi011/wt/internal/resolve"
	"github.com/raphi011/wt/internal/ui"
)

// ShowInfo contains all information displayed by wt show
type ShowInfo struct {
	ID            int        `json:"id"`
	Path          string     `json:"path"`
	Branch        string     `json:"branch"`
	RepoName      string     `json:"repo_name"`
	MainRepo      string     `json:"main_repo"`
	OriginURL     string     `json:"origin_url"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	LastCommit    string     `json:"last_commit,omitempty"`
	CommitsAhead  int        `json:"commits_ahead"`
	CommitsBehind int        `json:"commits_behind"`
	DiffStats     *DiffStats `json:"diff_stats,omitempty"`
	IsDirty       bool       `json:"is_dirty"`
	IsMerged      bool       `json:"is_merged"`
	IsWorktree    bool       `json:"is_worktree"`
	Note          string     `json:"note,omitempty"`
	PR            *PRInfo    `json:"pr,omitempty"`
}

// Status returns a human-readable status string for the worktree.
// Priority: dirty > merged > commits ahead > clean
func (s *ShowInfo) Status() string {
	if s.IsDirty {
		return "dirty (uncommitted changes)"
	}
	if s.IsMerged {
		return "merged"
	}
	if s.CommitsAhead > 0 {
		return fmt.Sprintf("%d ahead", s.CommitsAhead)
	}
	return "clean"
}

// DiffStats for JSON output
type DiffStats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Files     int `json:"files"`
}

// PRInfo for JSON output
type PRInfo struct {
	Number       int    `json:"number"`
	State        string `json:"state"`
	IsDraft      bool   `json:"is_draft,omitempty"`
	URL          string `json:"url,omitempty"`
	Author       string `json:"author,omitempty"`
	IsApproved   bool   `json:"is_approved,omitempty"`
	CommentCount int    `json:"comment_count,omitempty"`
}

func (c *ShowCmd) runShow(ctx context.Context) error {
	worktreeDir, err := c.Config.GetAbsWorktreeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve target worktree
	info, err := c.resolveShowTarget(ctx, worktreeDir)
	if err != nil {
		return err
	}

	// Load cache with lock
	wtCache, unlock, err := cache.LoadWithLock(worktreeDir)
	if err != nil {
		return err
	}
	defer unlock()

	// Get PR info from cache or refresh
	var prInfo *forge.PRInfo
	originURL, _ := git.GetOriginURL(ctx, info.MainRepo)
	folderName := filepath.Base(info.Path)
	if originURL != "" {
		if c.Refresh {
			sp := ui.NewSpinner("Fetching PR status...")
			sp.Start()
			prInfo = fetchPRForBranch(ctx, originURL, info.MainRepo, info.Branch, folderName, wtCache, c.Config.Hosts, &c.Config.Forge)
			sp.Stop()
			// Save updated cache
			if err := cache.Save(worktreeDir, wtCache); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
			}
		} else if pr := wtCache.GetPRForBranch(folderName); pr != nil && pr.Fetched {
			prInfo = pr
		}
	}

	// Gather additional info
	showInfo := gatherShowInfo(ctx, info, prInfo)

	if c.JSON {
		return outputShowJSON(ctx, showInfo)
	}

	return outputShowText(ctx, showInfo)
}

// resolveShowTarget resolves the worktree target for show command
func (c *ShowCmd) resolveShowTarget(ctx context.Context, worktreeDir string) (*resolve.Target, error) {
	cfg := c.Config
	workDir := c.WorkDir

	// Use ByIDOrRepoOrPath for basic resolution
	target, err := resolve.ByIDOrRepoOrPath(ctx, c.ID, c.Repository, worktreeDir, cfg.RepoScanDir(), workDir)
	if err != nil {
		// Wrap error with more specific message when not inside worktree/repo
		if c.ID == 0 && c.Repository == "" && !git.IsWorktree(workDir) && !git.IsInsideRepoPath(ctx, workDir) {
			return nil, fmt.Errorf("--number or --repository required when not inside a worktree/repo (run 'wt list' to see numbers)")
		}
		return nil, err
	}

	// For show command, try to get ID from cache if not already set
	if target.ID == 0 && worktreeDir != "" {
		wtCache, _ := cache.Load(worktreeDir)
		if wtCache != nil {
			key := cache.MakeWorktreeKey(target.Path)
			if entry, ok := wtCache.Worktrees[key]; ok {
				target.ID = entry.ID
			}
		}
	}

	return target, nil
}

func gatherShowInfo(ctx context.Context, target *resolve.Target, prInfo *forge.PRInfo) *ShowInfo {
	// Determine if this is a worktree (path differs from main repo)
	isWorktree := target.Path != target.MainRepo
	info := &ShowInfo{
		ID:         target.ID,
		Path:       target.Path,
		Branch:     target.Branch,
		MainRepo:   target.MainRepo,
		IsWorktree: isWorktree,
	}

	// Get origin URL and extract repo name from it
	info.OriginURL, _ = git.GetOriginURL(ctx, target.MainRepo)
	info.RepoName = git.ExtractRepoNameFromURL(info.OriginURL)
	if info.RepoName == "" {
		// Fallback to local folder name
		info.RepoName = filepath.Base(target.MainRepo)
	}

	// Get default branch once for all comparisons (avoids 3x redundant calls)
	defaultBranch := git.GetDefaultBranch(ctx, target.MainRepo)

	// Get commits ahead
	commitsAhead, err := git.GetCommitCountWithBase(ctx, target.MainRepo, target.Branch, defaultBranch)
	if err == nil {
		info.CommitsAhead = commitsAhead
	}

	// Get commits behind
	commitsBehind, err := git.GetCommitsBehindWithBase(ctx, target.MainRepo, target.Branch, defaultBranch)
	if err == nil {
		info.CommitsBehind = commitsBehind
	}

	// Get diff stats
	diffStats, err := git.GetDiffStatsWithBase(ctx, target.MainRepo, target.Branch, defaultBranch)
	if err == nil && (diffStats.Additions > 0 || diffStats.Deletions > 0 || diffStats.Files > 0) {
		info.DiffStats = &DiffStats{
			Additions: diffStats.Additions,
			Deletions: diffStats.Deletions,
			Files:     diffStats.Files,
		}
	}

	// Get dirty status
	info.IsDirty = git.IsDirty(ctx, target.Path)

	// Set merge status based on PR
	info.IsMerged = prInfo != nil && prInfo.State == "MERGED"

	// Get last commit time
	info.LastCommit, _ = git.GetLastCommitRelative(ctx, target.Path)

	// Get branch creation time
	createdAt, err := git.GetBranchCreatedTime(ctx, target.MainRepo, target.Branch)
	if err == nil {
		info.CreatedAt = &createdAt
	}

	// Get note
	info.Note, _ = git.GetBranchNote(ctx, target.MainRepo, target.Branch)

	// Convert PR info
	if prInfo != nil && prInfo.Number > 0 {
		info.PR = &PRInfo{
			Number:       prInfo.Number,
			State:        prInfo.State,
			IsDraft:      prInfo.IsDraft,
			URL:          prInfo.URL,
			Author:       prInfo.Author,
			IsApproved:   prInfo.IsApproved,
			CommentCount: prInfo.CommentCount,
		}
	}

	return info
}

func fetchPRForBranch(ctx context.Context, originURL, mainRepo, branch, folderName string, wtCache *cache.Cache, hosts map[string]string, forgeConfig *config.ForgeConfig) *forge.PRInfo {
	// Check if branch has upstream
	upstreamBranch := git.GetUpstreamBranch(ctx, mainRepo, branch)
	if upstreamBranch == "" {
		return nil
	}

	// Detect forge
	f := forge.Detect(originURL, hosts, forgeConfig)
	if err := f.Check(ctx); err != nil {
		return nil
	}

	// Fetch PR info
	pr, err := f.GetPRForBranch(ctx, originURL, upstreamBranch)
	if err != nil {
		return nil
	}

	// Cache the result
	wtCache.SetPRForBranch(folderName, pr)

	return pr
}

func outputShowJSON(ctx context.Context, info *ShowInfo) error {
	out := output.FromContext(ctx)
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	out.Println(string(data))
	return nil
}

func outputShowText(ctx context.Context, info *ShowInfo) error {
	out := output.FromContext(ctx)

	// Styles
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	cyanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Build section header
	var worktreeHeader string
	if info.IsWorktree {
		if info.ID > 0 {
			worktreeHeader = fmt.Sprintf("Worktree (ID: %d)", info.ID)
		} else {
			worktreeHeader = "Worktree"
		}
	} else {
		worktreeHeader = "Repo"
	}

	// Build all rows
	var rows [][]string

	// Worktree info
	worktreeName := filepath.Base(info.Path)
	rows = append(rows, []string{"Name", worktreeName})
	rows = append(rows, []string{"Branch", info.Branch})
	rows = append(rows, []string{"Repo", info.RepoName})
	if info.CreatedAt != nil {
		rows = append(rows, []string{"Created", formatTimeAgo(*info.CreatedAt)})
	}
	if info.LastCommit != "" {
		rows = append(rows, []string{"Last commit", info.LastCommit})
	}

	// Commits ahead/behind
	commitStr := ""
	if info.CommitsAhead > 0 {
		commitStr = greenStyle.Render(fmt.Sprintf("%d ahead", info.CommitsAhead))
	}
	if info.CommitsBehind > 0 {
		if commitStr != "" {
			commitStr += ", "
		}
		commitStr += redStyle.Render(fmt.Sprintf("%d behind", info.CommitsBehind))
	}
	if commitStr == "" {
		commitStr = dimStyle.Render("up to date")
	}
	rows = append(rows, []string{"Commits", commitStr})

	// Diff stats
	if info.DiffStats != nil {
		diffStr := fmt.Sprintf("%s %s (%d files)",
			greenStyle.Render(fmt.Sprintf("+%d", info.DiffStats.Additions)),
			redStyle.Render(fmt.Sprintf("-%d", info.DiffStats.Deletions)),
			info.DiffStats.Files)
		rows = append(rows, []string{"Changes", diffStr})
	}

	// Status
	status := info.Status()
	statusStyle := dimStyle
	switch {
	case info.IsDirty:
		statusStyle = yellowStyle
	case info.IsMerged:
		statusStyle = greenStyle
	}
	rows = append(rows, []string{"Status", statusStyle.Render(status)})

	// Track PR section start index
	prStartIndex := -1

	// PR info
	prHeader := "PR"
	if info.PR != nil {
		// Detect forge type from URL
		if strings.Contains(info.OriginURL, "github") {
			prHeader = "GitHub PR"
		} else if strings.Contains(info.OriginURL, "gitlab") {
			prHeader = "GitLab MR"
		}

		prStartIndex = len(rows)
		stateStyle := yellowStyle
		switch info.PR.State {
		case "MERGED":
			stateStyle = greenStyle
		case "CLOSED":
			stateStyle = redStyle
		}
		prValue := fmt.Sprintf("#%d %s", info.PR.Number, stateStyle.Render(info.PR.State))
		if info.PR.IsDraft {
			prValue += " " + dimStyle.Render("[DRAFT]")
		}
		rows = append(rows, []string{"Number", prValue})

		if info.PR.Author != "" {
			rows = append(rows, []string{"Author", "@" + info.PR.Author})
		}
		if info.PR.URL != "" {
			rows = append(rows, []string{"URL", cyanStyle.Render(info.PR.URL)})
		}
		reviewStr := dimStyle.Render("none")
		if info.PR.IsApproved {
			reviewStr = greenStyle.Render("approved")
		}
		rows = append(rows, []string{"Reviews", reviewStr})
		if info.PR.CommentCount > 0 {
			rows = append(rows, []string{"Comments", fmt.Sprintf("%d", info.PR.CommentCount)})
		}
	}

	// Note
	if info.Note != "" {
		rows = append(rows, []string{"Note", info.Note})
	}

	// Calculate max key width and max value width for alignment
	maxKeyWidth := 0
	maxValueWidth := 0
	for _, row := range rows {
		if len(row[0]) > maxKeyWidth {
			maxKeyWidth = len(row[0])
		}
		// Use lipgloss.Width for accurate measurement with ANSI codes
		valWidth := lipgloss.Width(row[1])
		if valWidth > maxValueWidth {
			maxValueWidth = valWidth
		}
	}

	// Split rows if there's a PR section
	var mainRows, prRows [][]string
	if prStartIndex >= 0 {
		mainRows = rows[:prStartIndex]
		prRows = rows[prStartIndex:]
	} else {
		mainRows = rows
	}

	// Style function for table rows
	styleFunc := func(row, col int) lipgloss.Style {
		style := lipgloss.NewStyle().Padding(0, 1)
		if col == 0 {
			style = style.Bold(true).Width(maxKeyWidth + 2) // +2 for padding
		} else {
			// Ensure value column has consistent width
			style = style.Width(maxValueWidth + 2)
		}
		return style
	}

	// Render main table with all rows for consistent width
	t := table.New().
		Rows(rows...).
		Border(lipgloss.NormalBorder()).
		BorderStyle(dimStyle).
		BorderColumn(false).
		StyleFunc(styleFunc)

	// Get the full table to determine width, but we'll only print parts
	fullTableStr := t.String()
	fullTableLines := strings.Split(fullTableStr, "\n")
	contentWidth := lipgloss.Width(fullTableLines[0]) - 2 // -2 for borders

	// Helper to print centered header row
	printHeader := func(label string) {
		leftPad := (contentWidth - len(label)) / 2
		rightPad := contentWidth - len(label) - leftPad
		headerLine := dimStyle.Render("│") +
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Render(
				strings.Repeat(" ", leftPad)+label+strings.Repeat(" ", rightPad)) +
			dimStyle.Render("│")
		out.Println(headerLine)
	}

	// Helper to print data row
	printRow := func(row []string) {
		keyStyle := lipgloss.NewStyle().Bold(true).Width(maxKeyWidth+2).Padding(0, 1)
		valStyle := lipgloss.NewStyle().Padding(0, 1)

		key := keyStyle.Render(row[0])
		val := valStyle.Render(row[1])

		// Pad value to fill remaining width
		keyWidth := lipgloss.Width(key)
		valWidth := lipgloss.Width(val)
		remainingWidth := contentWidth - keyWidth - valWidth
		if remainingWidth > 0 {
			val += strings.Repeat(" ", remainingWidth)
		}

		out.Println(dimStyle.Render("│") + key + val + dimStyle.Render("│"))
	}

	// Print top border
	out.Println(dimStyle.Render("┌" + strings.Repeat("─", contentWidth) + "┐"))

	// Print Worktree header
	printHeader(worktreeHeader)

	// Print main rows
	for _, row := range mainRows {
		printRow(row)
	}

	// If PR section exists, add centered header and PR rows
	if len(prRows) > 0 {
		printHeader(prHeader)
		for _, row := range prRows {
			printRow(row)
		}
	}

	// Print bottom border
	out.Println(dimStyle.Render("└" + strings.Repeat("─", contentWidth) + "┘"))

	return nil
}

func formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}

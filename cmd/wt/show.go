package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/raphi011/wt/internal/cache"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/resolve"
	"github.com/raphi011/wt/internal/ui"
)

// ShowInfo contains all information displayed by wt show
type ShowInfo struct {
	ID           int        `json:"id"`
	Path         string     `json:"path"`
	Branch       string     `json:"branch"`
	RepoName     string     `json:"repo_name"`
	MainRepo     string     `json:"main_repo"`
	OriginURL    string     `json:"origin_url"`
	CreatedAt    *time.Time `json:"created_at,omitempty"`
	LastCommit   string     `json:"last_commit,omitempty"`
	CommitsAhead int        `json:"commits_ahead"`
	CommitsBehind int       `json:"commits_behind"`
	DiffStats    *DiffStats `json:"diff_stats,omitempty"`
	IsDirty      bool       `json:"is_dirty"`
	IsMerged     bool       `json:"is_merged"`
	Note         string     `json:"note,omitempty"`
	PR           *PRInfo    `json:"pr,omitempty"`
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

func runShow(cmd *ShowCmd, cfg *config.Config) error {
	if err := git.CheckGit(); err != nil {
		return err
	}

	// Resolve target worktree
	info, err := resolveShowTarget(cmd.ID, cmd.Dir)
	if err != nil {
		return err
	}

	scanPath := cmd.Dir
	if scanPath == "" {
		scanPath = "."
	}
	scanPath, err = filepath.Abs(scanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Acquire lock on cache
	lock := cache.NewFileLock(forge.LockPath(scanPath))
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Unlock()

	// Load cache
	wtCache, err := forge.LoadCache(scanPath)
	if err != nil {
		return fmt.Errorf("failed to load cache: %w", err)
	}

	// Get PR info from cache or refresh
	var prInfo *forge.PRInfo
	originURL, _ := git.GetOriginURL(info.MainRepo)
	if originURL != "" {
		if cmd.Refresh {
			sp := ui.NewSpinner("Fetching PR status...")
			sp.Start()
			prInfo = fetchPRForBranch(originURL, info.MainRepo, info.Branch, wtCache, cfg)
			sp.Stop()
			// Save updated cache
			if err := forge.SaveCache(scanPath, wtCache); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
			}
		} else if originCache, ok := wtCache.PRs[originURL]; ok {
			if pr, ok := originCache[info.Branch]; ok && pr != nil && pr.Fetched {
				prInfo = pr
			}
		}
	}

	// Gather additional info
	showInfo := gatherShowInfo(info, prInfo)

	if cmd.JSON {
		return outputShowJSON(showInfo)
	}

	return outputShowText(showInfo, prInfo)
}

// resolveShowTarget resolves the worktree target for show command
func resolveShowTarget(id int, dir string) (*resolve.Target, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	inWorktree := git.IsWorktree(cwd)
	inGitRepo := git.IsInsideRepo()

	// If no ID provided and inside a worktree or git repo, use current branch
	if id == 0 {
		if !inWorktree && !inGitRepo {
			return nil, fmt.Errorf("--id required when not inside a worktree (run 'wt list' to see IDs)")
		}

		branch, err := git.GetCurrentBranch(cwd)
		if err != nil {
			return nil, fmt.Errorf("failed to get current branch: %w", err)
		}

		var mainRepo string
		if inWorktree {
			mainRepo, err = git.GetMainRepoPath(cwd)
			if err != nil {
				return nil, fmt.Errorf("failed to get main repo path: %w", err)
			}
		} else {
			// Inside main repo, use cwd as main repo
			mainRepo = cwd
		}

		// Get ID from cache if available
		scanPath := dir
		if scanPath == "" {
			scanPath = "."
		}
		scanPath, _ = filepath.Abs(scanPath)
		wtCache, _ := forge.LoadCache(scanPath)
		originURL, _ := git.GetOriginURL(mainRepo)
		wtID := 0
		if wtCache != nil && originURL != "" {
			key := forge.MakeWorktreeKey(originURL, branch)
			if entry, ok := wtCache.Worktrees[key]; ok {
				wtID = entry.ID
			}
		}

		return &resolve.Target{ID: wtID, Branch: branch, MainRepo: mainRepo, Path: cwd}, nil
	}

	// Resolve by ID
	scanPath := dir
	if scanPath == "" {
		scanPath = "."
	}
	scanPath, err = filepath.Abs(scanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return resolve.ByID(id, scanPath)
}

func gatherShowInfo(target *resolve.Target, prInfo *forge.PRInfo) *ShowInfo {
	info := &ShowInfo{
		ID:       target.ID,
		Path:     target.Path,
		Branch:   target.Branch,
		MainRepo: target.MainRepo,
		RepoName: filepath.Base(target.MainRepo),
	}

	// Get origin URL
	info.OriginURL, _ = git.GetOriginURL(target.MainRepo)

	// Get commits ahead
	commitsAhead, err := git.GetCommitCount(target.MainRepo, target.Branch)
	if err == nil {
		info.CommitsAhead = commitsAhead
	}

	// Get commits behind
	commitsBehind, err := git.GetCommitsBehind(target.MainRepo, target.Branch)
	if err == nil {
		info.CommitsBehind = commitsBehind
	}

	// Get diff stats
	diffStats, err := git.GetDiffStats(target.MainRepo, target.Branch)
	if err == nil && (diffStats.Additions > 0 || diffStats.Deletions > 0 || diffStats.Files > 0) {
		info.DiffStats = &DiffStats{
			Additions: diffStats.Additions,
			Deletions: diffStats.Deletions,
			Files:     diffStats.Files,
		}
	}

	// Get dirty status
	info.IsDirty = git.IsDirty(target.Path)

	// Get merge status
	isMerged, _ := git.IsBranchMerged(target.MainRepo, target.Branch)
	info.IsMerged = isMerged
	if prInfo != nil && prInfo.State == "MERGED" {
		info.IsMerged = true
	}

	// Get last commit time
	info.LastCommit, _ = git.GetLastCommitRelative(target.Path)

	// Get branch creation time
	createdAt, err := git.GetBranchCreatedTime(target.MainRepo, target.Branch)
	if err == nil {
		info.CreatedAt = &createdAt
	}

	// Get note
	info.Note, _ = git.GetBranchNote(target.MainRepo, target.Branch)

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

func fetchPRForBranch(originURL, mainRepo, branch string, wtCache *forge.Cache, cfg *config.Config) *forge.PRInfo {
	// Check if branch has upstream
	upstreamBranch := git.GetUpstreamBranch(mainRepo, branch)
	if upstreamBranch == "" {
		return nil
	}

	// Detect forge
	f := forge.Detect(originURL, cfg.Hosts)
	if err := f.Check(); err != nil {
		return nil
	}

	// Fetch PR info
	pr, err := f.GetPRForBranch(originURL, upstreamBranch)
	if err != nil {
		return nil
	}

	// Cache the result
	if wtCache.PRs == nil {
		wtCache.PRs = make(forge.PRCache)
	}
	if wtCache.PRs[originURL] == nil {
		wtCache.PRs[originURL] = make(map[string]*forge.PRInfo)
	}
	wtCache.PRs[originURL][branch] = pr

	return pr
}

func outputShowJSON(info *ShowInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputShowText(info *ShowInfo, _ *forge.PRInfo) error {
	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true)
	labelStyle := lipgloss.NewStyle()
	valueStyle := lipgloss.NewStyle()
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	cyanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	// Header
	worktreeName := filepath.Base(info.Path)
	idStr := ""
	if info.ID > 0 {
		idStr = fmt.Sprintf(" (ID: %d)", info.ID)
	}
	fmt.Println(titleStyle.Render(worktreeName + idStr))
	fmt.Println(strings.Repeat("-", len(worktreeName)+len(idStr)))

	// Basic info
	fmt.Printf("%s %s\n", labelStyle.Render("Branch:"), valueStyle.Render(info.Branch))
	fmt.Printf("%s %s\n", labelStyle.Render("Repo:"), valueStyle.Render(info.RepoName))

	// Time info
	if info.CreatedAt != nil {
		fmt.Printf("%s %s\n", labelStyle.Render("Created:"), valueStyle.Render(formatTimeAgo(*info.CreatedAt)))
	}
	if info.LastCommit != "" {
		fmt.Printf("%s %s\n", labelStyle.Render("Last commit:"), valueStyle.Render(info.LastCommit))
	}

	fmt.Println()

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
		commitStr = "up to date"
	}
	fmt.Printf("%s %s\n", labelStyle.Render("Commits:"), commitStr)

	// Diff stats
	if info.DiffStats != nil {
		diffStr := fmt.Sprintf("%s %s (%d files)",
			greenStyle.Render(fmt.Sprintf("+%d", info.DiffStats.Additions)),
			redStyle.Render(fmt.Sprintf("-%d", info.DiffStats.Deletions)),
			info.DiffStats.Files)
		fmt.Printf("%s %s\n", labelStyle.Render("Changes:"), diffStr)
	}

	// Status
	statusStr := "clean"
	if info.IsDirty {
		statusStr = yellowStyle.Render("dirty (uncommitted changes)")
	}
	if info.IsMerged {
		statusStr = greenStyle.Render("merged")
	}
	fmt.Printf("%s %s\n", labelStyle.Render("Status:"), statusStr)

	// PR info
	if info.PR != nil {
		fmt.Println()
		stateStyle := yellowStyle
		switch info.PR.State {
		case "MERGED":
			stateStyle = greenStyle
		case "CLOSED":
			stateStyle = redStyle
		}
		prHeader := fmt.Sprintf("PR #%d (%s)", info.PR.Number, stateStyle.Render(info.PR.State))
		if info.PR.IsDraft {
			prHeader += " " + labelStyle.Render("[DRAFT]")
		}
		fmt.Println(prHeader)

		if info.PR.Author != "" {
			fmt.Printf("  %s @%s\n", labelStyle.Render("Author:"), info.PR.Author)
		}
		if info.PR.URL != "" {
			fmt.Printf("  %s %s\n", labelStyle.Render("URL:"), cyanStyle.Render(info.PR.URL))
		}
		reviewStr := "none"
		if info.PR.IsApproved {
			reviewStr = greenStyle.Render("approved")
		}
		fmt.Printf("  %s %s\n", labelStyle.Render("Reviews:"), reviewStr)
		if info.PR.CommentCount > 0 {
			fmt.Printf("  %s %d\n", labelStyle.Render("Comments:"), info.PR.CommentCount)
		}
	}

	// Note
	if info.Note != "" {
		fmt.Println()
		fmt.Printf("%s %s\n", labelStyle.Render("Note:"), info.Note)
	}

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

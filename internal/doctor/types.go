package doctor

// IssueCategory groups issues by type.
type IssueCategory string

const (
	// CategoryCache represents problems with cache data.
	CategoryCache IssueCategory = "cache"
	// CategoryGit represents problems with git worktree links.
	CategoryGit IssueCategory = "git"
	// CategoryOrphan represents untracked worktrees or ghost entries.
	CategoryOrphan IssueCategory = "orphan"
)

// Issue represents a problem detected by doctor.
type Issue struct {
	Key         string        // cache key or path
	Description string        // human-readable description
	FixAction   string        // what --fix would do
	Category    IssueCategory // issue category
	RepoPath    string        // for git repair operations
}

// IssueStats tracks counts by category.
type IssueStats struct {
	CacheValid      int // valid cache entries
	CacheIssues     int // cache entries with issues
	GitHealthy      int // worktrees with healthy git links
	GitRepairable   int // worktrees with repairable git links
	GitUnrepairable int // worktrees with unrepairable git links
	GitPrunable     int // stale git references that can be pruned
	OrphanUntracked int // worktrees on disk but not in cache
	OrphanGhost     int // entries in cache but not recognized by git
}

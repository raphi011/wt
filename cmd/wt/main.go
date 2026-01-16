package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/alexflint/go-arg"

	"github.com/raphaelgruber/wt/internal/git"
	"github.com/raphaelgruber/wt/internal/github"
	"github.com/raphaelgruber/wt/internal/ui"
)

type CreateCmd struct {
	Path   string `arg:"positional,required" help:"base directory (., .., or ~/Git/worktrees)"`
	Branch string `arg:"positional,required" help:"branch name"`
}

func (CreateCmd) Description() string {
	return `Create a new git worktree at <path>/<repo>-<branch>
Examples:
  wt create . feature-branch       # Create in current directory
  wt create .. feature-branch      # Create next to current repo
  wt create ~/Git/worktrees branch # Create in specific directory`
}

type CleanCmd struct {
	Path      string `arg:"positional" default:"." help:"directory to scan"`
	DryRun    bool   `arg:"-n,--dry-run" help:"preview without removing"`
	RefreshPR bool   `arg:"--refresh-pr" help:"force refresh PR cache"`
}

func (CleanCmd) Description() string {
	return `Cleanup merged git worktrees with PR status display.
Removes worktrees where the branch is merged AND working directory is clean.`
}

type ListCmd struct {
	Path string `arg:"positional" default:"." help:"directory to scan"`
	JSON bool   `arg:"--json" help:"output as JSON"`
}

func (ListCmd) Description() string {
	return "List all git worktrees with their status"
}

type CompletionCmd struct {
	Shell string `arg:"positional,required" help:"shell type (fish)"`
}

func (CompletionCmd) Description() string {
	return "Generate shell completion script"
}

type Args struct {
	Create     *CreateCmd     `arg:"subcommand:create" help:"create a new worktree"`
	Clean      *CleanCmd      `arg:"subcommand:clean" help:"cleanup merged worktrees"`
	List       *ListCmd       `arg:"subcommand:list" help:"list worktrees"`
	Completion *CompletionCmd `arg:"subcommand:completion" help:"generate completion script"`
}

func (Args) Description() string {
	return "Git worktree manager - create, clean, and list worktrees"
}

func main() {
	var args Args
	p := arg.MustParse(&args)

	switch {
	case args.Create != nil:
		if err := runCreate(args.Create); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Clean != nil:
		if err := runClean(args.Clean); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.List != nil:
		if err := runList(args.List); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case args.Completion != nil:
		if err := runCompletion(args.Completion); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		p.WriteHelp(os.Stdout)
		os.Exit(1)
	}
}

func runCreate(cmd *CreateCmd) error {
	fmt.Printf("Creating worktree for branch: %s...\n", cmd.Branch)

	basePath := expandPath(cmd.Path)
	path, err := git.CreateWorktree(basePath, cmd.Branch)
	if err != nil {
		return err
	}

	fmt.Printf("✓ Worktree created at: %s\n", path)
	return nil
}

func runClean(cmd *CleanCmd) error {
	// Expand path
	scanPath := cmd.Path
	if scanPath == "." {
		var err error
		scanPath, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		scanPath = expandPath(scanPath)
	}

	// Start spinner
	sp := ui.NewSpinner(fmt.Sprintf("Scanning worktrees in %s...", scanPath))
	sp.Start()

	// List worktrees
	worktrees, err := git.ListWorktrees(scanPath)
	if err != nil {
		sp.Stop()
		return err
	}

	if len(worktrees) == 0 {
		sp.Stop()
		fmt.Println("No worktrees found")
		return nil
	}

	// Group by repo for fetching
	grouped := git.GroupWorktreesByRepo(worktrees)

	// Fetch origin/master for each repo
	for repoName, wts := range grouped {
		if len(wts) == 0 {
			continue
		}
		sp.UpdateMessage(fmt.Sprintf("Fetching origin/master for %s...", repoName))
		git.FetchOriginMaster(wts[0].MainRepo)
	}

	// Load PR cache
	prCache, _ := github.LoadPRCache(scanPath)

	// Clean cache (remove deleted origins/branches)
	prCache = github.CleanPRCache(prCache, worktrees)

	// Determine which branches need fetching
	var toFetch []git.Worktree
	if cmd.RefreshPR {
		toFetch = worktrees // Fetch all
	} else {
		for _, wt := range worktrees {
			originURL, _ := github.GetOriginURL(wt.MainRepo)
			if originCache, ok := prCache[originURL]; !ok || originCache[wt.Branch] == nil {
				toFetch = append(toFetch, wt)
			}
		}
	}

	// Fetch uncached PRs in parallel
	if len(toFetch) > 0 {
		sp.UpdateMessage(fmt.Sprintf("Fetching PR status (%d branches)...", len(toFetch)))
		var prMutex sync.Mutex
		var prWg sync.WaitGroup

		for _, wt := range toFetch {
			prWg.Add(1)
			go func(wt git.Worktree) {
				defer prWg.Done()
				originURL, _ := github.GetOriginURL(wt.MainRepo)
				if originURL != "" {
					pr, _ := github.GetPRForBranch(originURL, wt.Branch)
					if pr != nil {
						prMutex.Lock()
						if prCache[originURL] == nil {
							prCache[originURL] = make(map[string]*github.PRInfo)
						}
						prCache[originURL][wt.Branch] = pr
						prMutex.Unlock()
					}
				}
			}(wt)
		}

		prWg.Wait()

		// Save updated cache
		github.SavePRCache(scanPath, prCache)
	}

	// Build prMap from cache for display
	prMap := make(map[string]*github.PRInfo)
	for _, wt := range worktrees {
		originURL, _ := github.GetOriginURL(wt.MainRepo)
		if originCache, ok := prCache[originURL]; ok {
			if pr, ok := originCache[wt.Branch]; ok {
				prMap[wt.Branch] = pr
			}
		}
	}

	// Update merge status for worktrees based on PR state
	for i := range worktrees {
		if pr, ok := prMap[worktrees[i].Branch]; ok && pr != nil {
			if pr.State == "MERGED" {
				worktrees[i].IsMerged = true
			}
		}
	}

	// Stop spinner and clear line
	sp.Stop()

	// Sort worktrees by repo name
	sort.Slice(worktrees, func(i, j int) bool {
		return worktrees[i].RepoName < worktrees[j].RepoName
	})

	// Determine which to remove
	var toRemove []git.Worktree
	toRemoveMap := make(map[string]bool)
	var skipped int

	for _, wt := range worktrees {
		if wt.IsMerged && !wt.IsDirty {
			toRemove = append(toRemove, wt)
			toRemoveMap[wt.Path] = true
		} else {
			skipped++
		}
	}

	// Display table with cleaned worktrees marked
	fmt.Print(ui.FormatWorktreesTable(worktrees, prMap, toRemoveMap, cmd.DryRun))

	// Remove worktrees
	if !cmd.DryRun && len(toRemove) > 0 {
		for _, wt := range toRemove {
			if err := git.RemoveWorktree(wt, true); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", wt.Path, err)
			}
		}

		// Prune stale references
		processedRepos := make(map[string]bool)
		for _, wt := range toRemove {
			if !processedRepos[wt.MainRepo] {
				git.PruneWorktrees(wt.MainRepo)
				processedRepos[wt.MainRepo] = true
			}
		}
	}

	// Print summary
	fmt.Print(ui.FormatSummary(len(toRemove), skipped, cmd.DryRun))

	return nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	return path
}

func runList(cmd *ListCmd) error {
	// Expand path
	scanPath := cmd.Path
	if scanPath == "." {
		var err error
		scanPath, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		scanPath = expandPath(scanPath)
	}

	// List worktrees in directory
	worktrees, err := git.ListWorktrees(scanPath)
	if err != nil {
		return err
	}

	if cmd.JSON {
		data, err := json.MarshalIndent(worktrees, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Simple text output
	if len(worktrees) == 0 {
		fmt.Println("No worktrees found")
		return nil
	}

	for _, wt := range worktrees {
		fmt.Printf("%s (%s)\n", filepath.Base(wt.Path), wt.Branch)
	}

	return nil
}

func runCompletion(cmd *CompletionCmd) error {
	switch cmd.Shell {
	case "fish":
		fmt.Print(fishCompletions)
		return nil
	default:
		return fmt.Errorf("unsupported shell: %s (supported: fish)", cmd.Shell)
	}
}

const fishCompletions = `# wt completions - supports fish autosuggestions and tab completion
complete -c wt -f

# Subcommands (shown in completions and autosuggestions)
complete -c wt -n "not __fish_seen_subcommand_from create clean list completion" -a "create" -d "Create new worktree"
complete -c wt -n "not __fish_seen_subcommand_from create clean list completion" -a "clean" -d "Cleanup merged worktrees"
complete -c wt -n "not __fish_seen_subcommand_from create clean list completion" -a "list" -d "List worktrees"
complete -c wt -n "not __fish_seen_subcommand_from create clean list completion" -a "completion" -d "Generate completion script"

# create: suggest directories first, then branch names
complete -c wt -n "__fish_seen_subcommand_from create; and not __fish_seen_argument" -a "." -d "Current directory"
complete -c wt -n "__fish_seen_subcommand_from create; and not __fish_seen_argument" -a ".." -d "Parent directory"
complete -c wt -n "__fish_seen_subcommand_from create; and not __fish_seen_argument" -a "~/Git/worktrees" -d "Worktrees directory"
complete -c wt -n "__fish_seen_subcommand_from create; and not __fish_seen_argument" -a "(__fish_complete_directories)"
complete -c wt -n "__fish_seen_subcommand_from create; and __fish_seen_argument" -a "(git branch --all --format='%(refname:short)' 2>/dev/null | string replace 'origin/' '' | sort -u)" -d "Branch name"

# clean: suggest common paths + directories
complete -c wt -n "__fish_seen_subcommand_from clean; and not __fish_seen_argument -s n -l dry-run -s p -l prune-invalid" -a "~/Git/worktrees" -d "Default worktrees directory"
complete -c wt -n "__fish_seen_subcommand_from clean; and not __fish_seen_argument -s n -l dry-run -s p -l prune-invalid" -a "." -d "Current directory"
complete -c wt -n "__fish_seen_subcommand_from clean" -a "(__fish_complete_directories)"
complete -c wt -n "__fish_seen_subcommand_from clean" -s n -l dry-run -d "Preview without removing"
complete -c wt -n "__fish_seen_subcommand_from clean" -s p -l prune-invalid -d "Remove non-worktree folders"

# list: suggest directories
complete -c wt -n "__fish_seen_subcommand_from list; and not __fish_seen_argument -l json" -a "~/Git/worktrees" -d "Worktrees directory"
complete -c wt -n "__fish_seen_subcommand_from list; and not __fish_seen_argument -l json" -a "." -d "Current directory"
complete -c wt -n "__fish_seen_subcommand_from list" -a "(__fish_complete_directories)"
complete -c wt -n "__fish_seen_subcommand_from list" -l json -d "Output as JSON"

# completion
complete -c wt -n "__fish_seen_subcommand_from completion" -a "fish" -d "Fish shell"

# Global help
complete -c wt -s h -l help -d "Show help message"
`

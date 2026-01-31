package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/registry"
)

func newDoctorCmd() *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:     "doctor",
		Short:   "Diagnose and repair issues",
		GroupID: GroupConfig,
		Args:    cobra.NoArgs,
		Long: `Diagnose and repair registry and worktree issues.

Checks:
- Registry file is valid JSON
- Registered repos exist on disk
- Registered repos are valid git repos
- Worktrees are valid (no broken/orphaned)
- External tools installed (git, gh/glab)

Examples:
  wt doctor          # Check for issues
  wt doctor --fix    # Auto-fix recoverable issues`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var issues int

			fmt.Println("Running diagnostics...")
			fmt.Println()

			// Check git is available
			if err := git.CheckGit(); err != nil {
				fmt.Printf("❌ Git not found: %v\n", err)
				issues++
			} else {
				fmt.Println("✓ Git is available")
			}

			// Check gh CLI
			if _, err := os.Stat("/usr/local/bin/gh"); err == nil {
				fmt.Println("✓ GitHub CLI (gh) is available")
			} else {
				fmt.Println("⚠ GitHub CLI (gh) not found (optional)")
			}

			// Check glab CLI
			if _, err := os.Stat("/usr/local/bin/glab"); err == nil {
				fmt.Println("✓ GitLab CLI (glab) is available")
			} else {
				fmt.Println("⚠ GitLab CLI (glab) not found (optional)")
			}

			fmt.Println()

			// Load and check registry
			reg, err := registry.Load()
			if err != nil {
				fmt.Printf("❌ Failed to load registry: %v\n", err)
				issues++
			} else {
				fmt.Printf("✓ Registry loaded (%d repos)\n", len(reg.Repos))

				// Check each repo
				var stale []string
				for _, repo := range reg.Repos {
					if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
						fmt.Printf("❌ Repo not found: %s (%s)\n", repo.Name, repo.Path)
						stale = append(stale, repo.Name)
						issues++
					} else if _, err := git.DetectRepoType(repo.Path); err != nil {
						fmt.Printf("❌ Invalid git repo: %s (%s)\n", repo.Name, repo.Path)
						issues++
					} else {
						// Check worktrees
						worktrees, err := git.ListWorktreesFromRepo(ctx, repo.Path)
						if err != nil {
							fmt.Printf("⚠ Failed to list worktrees for %s: %v\n", repo.Name, err)
						} else {
							fmt.Printf("✓ %s: %d worktrees\n", repo.Name, len(worktrees))
						}
					}
				}

				// Fix stale entries if requested
				if fix && len(stale) > 0 {
					fmt.Println()
					fmt.Printf("Removing %d stale entries...\n", len(stale))
					for _, name := range stale {
						if err := reg.Remove(name); err != nil {
							fmt.Printf("❌ Failed to remove %s: %v\n", name, err)
						} else {
							fmt.Printf("✓ Removed %s\n", name)
						}
					}
					if err := reg.Save(); err != nil {
						fmt.Printf("❌ Failed to save registry: %v\n", err)
					}
				}
			}

			fmt.Println()
			if issues > 0 {
				fmt.Printf("Found %d issue(s)\n", issues)
				if !fix {
					fmt.Println("Run 'wt doctor --fix' to auto-fix recoverable issues")
				}
				return fmt.Errorf("%d issues found", issues)
			}

			fmt.Println("All checks passed")
			return nil
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Auto-fix recoverable issues")

	return cmd
}

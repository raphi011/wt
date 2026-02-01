package flows

import (
	"fmt"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
)

// CdOptions holds the options gathered from interactive mode.
type CdOptions struct {
	SelectedPath string // Selected worktree path
	RepoName     string // Repository name of selected worktree
	Branch       string // Branch name of selected worktree
	Cancelled    bool
}

// CdWorktreeInfo contains worktree data for display in the wizard.
type CdWorktreeInfo struct {
	RepoName string
	Branch   string
	Path     string
	IsDirty  bool
	Note     string
}

// CdWizardParams contains parameters for the cd wizard.
type CdWizardParams struct {
	Worktrees []CdWorktreeInfo // All worktrees available for selection
}

// CdInteractive runs the interactive cd wizard with fuzzy search.
func CdInteractive(params CdWizardParams) (CdOptions, error) {
	if len(params.Worktrees) == 0 {
		return CdOptions{Cancelled: true}, nil
	}

	w := framework.NewWizard("Change Directory to Worktree")

	// Build options from worktrees
	options := make([]framework.Option, len(params.Worktrees))

	for i, wt := range params.Worktrees {
		// Format: "repo/branch" with optional dirty marker
		label := fmt.Sprintf("%s/%s", wt.RepoName, wt.Branch)
		if wt.IsDirty {
			label += " *"
		}

		description := ""
		if wt.Note != "" {
			description = wt.Note
		}

		options[i] = framework.Option{
			Label:       label,
			Value:       i, // Store index to retrieve full info later
			Description: description,
			Disabled:    false,
		}
	}

	// Create filterable list step (uses fuzzy search)
	selectStep := steps.NewFilterableList("worktree", "Worktree", "Select worktree", options)

	w.AddStep(selectStep)

	// Skip summary for single-step wizard - go directly after selection
	w.WithSkipSummary(true)

	// Run the wizard
	result, err := w.Run()
	if err != nil {
		return CdOptions{}, err
	}

	if result.IsCancelled() {
		return CdOptions{Cancelled: true}, nil
	}

	// Extract selected worktree
	step := result.GetStep("worktree")
	if step == nil {
		return CdOptions{Cancelled: true}, nil
	}

	fl := step.(*steps.FilterableListStep)
	val := fl.GetSelectedValue()
	if val == nil {
		return CdOptions{Cancelled: true}, nil
	}

	idx := val.(int)
	wt := params.Worktrees[idx]

	return CdOptions{
		SelectedPath: wt.Path,
		RepoName:     wt.RepoName,
		Branch:       wt.Branch,
	}, nil
}

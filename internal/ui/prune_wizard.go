package ui

import (
	"fmt"
	"strings"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/ui/wizard"
)

// PruneOptions holds the options gathered from interactive mode.
type PruneOptions struct {
	SelectedIDs []int // Selected worktree IDs to prune
	Cancelled   bool
}

// PruneWorktreeInfo contains worktree data for display in the wizard.
type PruneWorktreeInfo struct {
	ID       int
	RepoName string
	Branch   string
	Reason   string // "Merged PR", "Merged branch", "Clean", "Dirty", "Not merged", "Has commits"
	IsDirty  bool   // Whether worktree has uncommitted changes
	Worktree git.Worktree
}

// PruneWizardParams contains parameters for the prune wizard.
type PruneWizardParams struct {
	Worktrees []PruneWorktreeInfo // All worktrees with their prune status
}

// PruneInteractive runs the interactive prune wizard.
func PruneInteractive(params PruneWizardParams) (PruneOptions, error) {
	if len(params.Worktrees) == 0 {
		return PruneOptions{Cancelled: true}, nil
	}

	w := wizard.NewWizard("Prune")

	// Build options from worktrees
	options := make([]wizard.Option, len(params.Worktrees))
	var preSelected []int

	for i, wt := range params.Worktrees {
		// Format: "repo/branch (reason)"
		label := fmt.Sprintf("%s/%s", wt.RepoName, wt.Branch)
		description := wt.Reason

		// Determine if this should be pre-selected (auto-prunable)
		isPrunable := isPrunableReason(wt.Reason)

		options[i] = wizard.Option{
			Label:       label,
			Value:       wt.ID,
			Description: description,
			Disabled:    false, // All can be selected in interactive mode
		}

		if isPrunable {
			preSelected = append(preSelected, i)
		}
	}

	// Create multi-select step
	selectStep := wizard.NewFilterableList("worktrees", "Worktrees", "Select worktrees to prune", options).
		WithMultiSelect().
		SetMinMax(0, 0) // No minimum required (user can cancel)

	// Pre-select prunable worktrees
	if len(preSelected) > 0 {
		selectStep.SetSelected(preSelected)
	}

	w.AddStep(selectStep)

	// Customize summary
	w.WithSummary("Confirm removal")

	// Add info line showing count and any warnings
	w.WithInfoLine(func(wiz *wizard.Wizard) string {
		step := wiz.GetStep("worktrees")
		if step == nil {
			return ""
		}
		fl := step.(*wizard.FilterableListStep)
		count := fl.SelectedCount()
		if count == 0 {
			return "No worktrees selected"
		}

		// Check if any dirty worktrees are selected
		indices := fl.GetSelectedIndices()
		dirtyCount := 0
		for _, idx := range indices {
			if params.Worktrees[idx].IsDirty {
				dirtyCount++
			}
		}

		if dirtyCount > 0 {
			return fmt.Sprintf("%d selected (%d with uncommitted changes)", count, dirtyCount)
		}
		return fmt.Sprintf("%d selected", count)
	})

	// Run the wizard
	result, err := w.Run()
	if err != nil {
		return PruneOptions{}, err
	}

	if result.IsCancelled() {
		return PruneOptions{Cancelled: true}, nil
	}

	// Extract selected IDs
	opts := PruneOptions{}
	step := result.GetStep("worktrees")
	if step != nil {
		fl := step.(*wizard.FilterableListStep)
		indices := fl.GetSelectedIndices()
		for _, idx := range indices {
			opts.SelectedIDs = append(opts.SelectedIDs, params.Worktrees[idx].ID)
		}
	}

	return opts, nil
}

// isPrunableReason returns true if the reason indicates auto-prunable status.
func isPrunableReason(reason string) bool {
	reason = strings.ToLower(reason)
	return reason == "merged pr"
}

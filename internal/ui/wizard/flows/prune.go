package flows

import (
	"fmt"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/ui/styles"
	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
)

// pruneOptionValue holds the value stored in each Option for prune wizard.
type pruneOptionValue struct {
	ID         int
	IsPrunable bool
	Reason     string
}

// PruneOptions holds the options gathered from interactive mode.
type PruneOptions struct {
	SelectedIDs []int // Selected worktree IDs to prune
	Cancelled   bool
}

// PruneWorktreeInfo contains worktree data for display in the wizard.
type PruneWorktreeInfo struct {
	ID         int
	RepoName   string
	Branch     string
	Reason     string // "Merged PR", "Merged branch", "Clean", "Dirty", "Not merged", "Has commits"
	IsDirty    bool   // Whether worktree has uncommitted changes
	IsPrunable bool   // Whether worktree can be auto-pruned (merged PR, not dirty)
	Worktree   git.Worktree
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

	w := framework.NewWizard("Prune")

	// Build options from worktrees
	options := make([]framework.Option, len(params.Worktrees))
	var preSelected []int

	for i, wt := range params.Worktrees {
		// Format: "repo/branch"
		label := fmt.Sprintf("%s/%s", wt.RepoName, wt.Branch)

		options[i] = framework.Option{
			Label: label,
			Value: pruneOptionValue{
				ID:         wt.ID,
				IsPrunable: wt.IsPrunable,
				Reason:     wt.Reason,
			},
			Description: wt.Reason, // Fallback for default renderer
			Disabled:    false,     // All can be selected in interactive mode
		}

		// Pre-select prunable worktrees
		if wt.IsPrunable {
			preSelected = append(preSelected, i)
		}
	}

	// Create multi-select step with custom description renderer
	selectStep := steps.NewFilterableList("worktrees", "Worktrees", "Select worktrees to prune", options).
		WithMultiSelect().
		SetMinMax(0, 0). // No minimum required (user can cancel)
		WithDescriptionRenderer(pruneDescriptionRenderer)

	// Pre-select prunable worktrees
	if len(preSelected) > 0 {
		selectStep.SetSelected(preSelected)
	}

	w.AddStep(selectStep)

	// Customize summary
	w.WithSummary("Confirm removal")

	// Add info line showing count and any warnings
	w.WithInfoLine(func(wiz *framework.Wizard) string {
		step := wiz.GetStep("worktrees")
		if step == nil {
			return ""
		}
		fl := step.(*steps.FilterableListStep)
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
		fl := step.(*steps.FilterableListStep)
		indices := fl.GetSelectedIndices()
		for _, idx := range indices {
			opts.SelectedIDs = append(opts.SelectedIDs, params.Worktrees[idx].ID)
		}
	}

	return opts, nil
}

// pruneDescriptionRenderer renders the description for prune options with colored status.
// - Prunable items: reason in success (green) color
// - Non-prunable selected items: "Force" prefix in error (red) color + reason
// - Non-prunable non-selected items: reason in muted color
func pruneDescriptionRenderer(opt framework.Option, isSelected bool) string {
	val, ok := opt.Value.(pruneOptionValue)
	if !ok {
		return styles.MutedStyle.Render(opt.Description)
	}

	if val.IsPrunable {
		// Prunable: show reason in success color
		return styles.SuccessStyle.Render(val.Reason)
	}

	if isSelected {
		// Non-prunable but selected: show "Force" in error color + reason
		return styles.ErrorStyle.Render("Force") + " " + styles.MutedStyle.Render(val.Reason)
	}

	// Non-prunable and not selected: show reason in muted color
	return styles.MutedStyle.Render(val.Reason)
}

package flows

import (
	"fmt"
	"strings"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/ui/styles"
	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
)

// pruneOptionValue holds the value stored in each Option for prune wizard.
type pruneOptionValue struct {
	ID         int
	IsPrunable bool
	IsStale    bool
	IsDirty    bool
	Reason     string
	Worktree   git.Worktree
	PR         forge.PRInfo
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
	Reason     string // PR state display string (e.g. "● Merged", "○ Open", "⏳ Stale (3w)")
	IsPrunable bool   // Whether worktree can be auto-pruned (merged PR or stale)
	IsStale    bool   // Whether worktree is stale (old commit, not merged)
	IsDirty    bool   // Whether worktree has uncommitted changes (removal needs --force)
	Worktree   git.Worktree
	PR         forge.PRInfo
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
		// Format: "repo:branch"
		label := fmt.Sprintf("%s:%s", wt.RepoName, wt.Branch)

		options[i] = framework.Option{
			Label: label,
			Value: pruneOptionValue{
				ID:         wt.ID,
				IsPrunable: wt.IsPrunable,
				IsStale:    wt.IsStale,
				IsDirty:    wt.IsDirty,
				Reason:     wt.Reason,
				Worktree:   wt.Worktree,
				PR:         wt.PR,
			},
			Description: wt.Reason, // Fallback for default renderer
			Disabled:    false,     // All can be selected in interactive mode
		}

		// Pre-select prunable worktrees, except those with uncommitted changes
		if wt.IsPrunable && !wt.IsDirty {
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

	// Add info line showing count
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

// pruneDescriptionRenderer keeps PR state colors consistent with list output.
// Eligibility, age and a forced selection are separate action/safety labels.
func pruneDescriptionRenderer(opt framework.Option, isSelected bool) string {
	val, ok := opt.Value.(pruneOptionValue)
	if !ok {
		return styles.MutedStyle.Render(opt.Description)
	}
	pr := val.PR
	status := styles.FormatPRRef(pr.Number, pr.State, pr.IsDraft, "", false)
	if pr.Number == 0 {
		if stateText := styles.FormatPRState(pr.State, pr.IsDraft); stateText != "" {
			status = styles.PRStateStyle(pr.State, pr.IsDraft).Render(stateText)
		}
	}
	if status == "" && !val.IsStale {
		status = styles.MutedStyle.Render(val.Reason)
	}
	parts := []string{}
	if val.IsPrunable {
		parts = append(parts, styles.SuccessStyle.Render("Eligible"))
	} else if isSelected {
		parts = append(parts, styles.ErrorStyle.Render("Force"))
	}
	if val.IsDirty {
		parts = append(parts, styles.WarningStyle.Render("Dirty"))
	}
	if val.IsStale {
		parts = append(parts, styles.WarningStyle.Render(val.Reason))
	}
	if status != "" {
		parts = append(parts, status)
	}
	return strings.Join(parts, " • ")
}

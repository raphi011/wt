package ui

import (
	"fmt"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/ui/wizard"
)

// PrCheckoutOptions holds the options gathered from interactive mode.
type PrCheckoutOptions struct {
	Cancelled     bool
	SelectedRepo  string   // Selected repo path (when outside a repo)
	SelectedPR    int      // Selected PR number
	SelectedHooks []string // Hook names to run (empty if NoHook is true)
	NoHook        bool     // True if no hooks selected
}

// PRFetcher is a function that fetches open PRs for a repo path.
type PRFetcher func(repoPath string) ([]forge.OpenPR, error)

// PrCheckoutWizardParams contains parameters for the PR checkout wizard.
type PrCheckoutWizardParams struct {
	AvailableRepos  []string   // All available repo paths
	RepoNames       []string   // Display names for repos
	PreSelectedRepo int        // Index of pre-selected repo (-1 if none)
	FetchPRs        PRFetcher  // Function to fetch PRs for a repo
	AvailableHooks  []HookInfo // Available hooks
	HooksFromCLI    bool       // True if --hook or --no-hook was passed (skip hooks step)
}

// PrCheckoutInteractive runs the interactive PR checkout wizard.
func PrCheckoutInteractive(params PrCheckoutWizardParams) (PrCheckoutOptions, error) {
	w := wizard.NewWizard("PR Checkout")

	// Track repo paths/names for the wizard
	repoPaths := params.AvailableRepos
	repoNames := params.RepoNames

	// Variable to store the selected repo path for PR fetching
	var selectedRepoPath string

	// Only show repo selection step if there are multiple repos to choose from
	showRepoStep := len(repoPaths) > 1

	// Step 1: Repo selection (single-select) - skip if only one repo
	if showRepoStep {
		repoOptions := make([]wizard.Option, len(repoNames))
		for i, name := range repoNames {
			repoOptions[i] = wizard.Option{
				Label: name,
				Value: repoPaths[i],
			}
		}
		repoStep := wizard.NewSingleSelect("repo", "Repository", "Select a repository", repoOptions)

		// Pre-select repo if provided
		if params.PreSelectedRepo >= 0 && params.PreSelectedRepo < len(repoPaths) {
			repoStep.SetCursor(params.PreSelectedRepo)
		}

		w.AddStep(repoStep)
	}

	// Step 2: PR selection (single-select with two-row display)
	prStep := wizard.NewSingleSelect("pr", "PR", "Select a PR to checkout", nil)
	w.AddStep(prStep)

	// Step 3: Hooks (only when available and not set via CLI)
	hasHooks := len(params.AvailableHooks) > 0 && !params.HooksFromCLI
	if hasHooks {
		hookOptions := make([]wizard.Option, len(params.AvailableHooks))
		var preSelectedHooks []int
		for i, hook := range params.AvailableHooks {
			label := hook.Name
			if hook.Description != "" {
				label = hook.Name + " - " + hook.Description
			}
			hookOptions[i] = wizard.Option{
				Label: label,
				Value: hook.Name,
			}
			if hook.IsDefault {
				preSelectedHooks = append(preSelectedHooks, i)
			}
		}
		hookStep := wizard.NewFilterableList("hooks", "Hooks", "Select hooks to run after checkout", hookOptions).
			WithMultiSelect().
			SetMinMax(0, 0)
		if len(preSelectedHooks) > 0 {
			hookStep.SetSelected(preSelectedHooks)
		}
		w.AddStep(hookStep)
	}

	// Callbacks
	// When repo selection completes, fetch PRs
	if showRepoStep && params.FetchPRs != nil {
		w.OnComplete("repo", func(wiz *wizard.Wizard) {
			repoStep := wiz.GetStep("repo").(*wizard.SingleSelectStep)
			selectedIdx := repoStep.GetSelectedIndex()
			if selectedIdx < 0 || selectedIdx >= len(repoPaths) {
				return
			}

			selectedRepoPath = repoPaths[selectedIdx]
			prs, err := params.FetchPRs(selectedRepoPath)
			if err != nil {
				// Show error in options
				prStepUpdate := wiz.GetStep("pr").(*wizard.SingleSelectStep)
				prStepUpdate.SetOptions([]wizard.Option{
					{Label: fmt.Sprintf("Error: %v", err), Disabled: true},
				})
				return
			}

			if len(prs) == 0 {
				prStepUpdate := wiz.GetStep("pr").(*wizard.SingleSelectStep)
				prStepUpdate.SetOptions([]wizard.Option{
					{Label: "No open PRs found", Disabled: true},
				})
				return
			}

			// Build PR options with two-row display
			prOptions := make([]wizard.Option, len(prs))
			for i, pr := range prs {
				desc := fmt.Sprintf("@%s (%s)", pr.Author, pr.Branch)
				if pr.IsDraft {
					desc += " [draft]"
				}
				prOptions[i] = wizard.Option{
					Label:       pr.Title,
					Description: desc,
					Value:       pr.Number,
				}
			}

			prStepUpdate := wiz.GetStep("pr").(*wizard.SingleSelectStep)
			prStepUpdate.SetOptions(prOptions)
		})
	}

	// If no repo step (single repo), fetch PRs immediately before wizard runs
	if !showRepoStep && len(repoPaths) > 0 && params.FetchPRs != nil {
		selectedRepoPath = repoPaths[0]
		prs, err := params.FetchPRs(selectedRepoPath)
		if err != nil {
			return PrCheckoutOptions{}, fmt.Errorf("failed to fetch PRs: %w", err)
		}

		if len(prs) == 0 {
			return PrCheckoutOptions{}, fmt.Errorf("no open PRs found")
		}

		// Build PR options with two-row display
		prOptions := make([]wizard.Option, len(prs))
		for i, pr := range prs {
			desc := fmt.Sprintf("@%s (%s)", pr.Author, pr.Branch)
			if pr.IsDraft {
				desc += " [draft]"
			}
			prOptions[i] = wizard.Option{
				Label:       pr.Title,
				Description: desc,
				Value:       pr.Number,
			}
		}
		prStep.SetOptions(prOptions)
	}

	// Info line showing selected repo
	if showRepoStep {
		w.WithInfoLine(func(wiz *wizard.Wizard) string {
			repoStep := wiz.GetStep("repo")
			if repoStep == nil || !repoStep.IsComplete() {
				return ""
			}
			v := repoStep.Value()
			if v.Label == "" {
				return ""
			}
			return "Repository: " + v.Label
		})
	}

	// Run the wizard
	result, err := w.Run()
	if err != nil {
		return PrCheckoutOptions{}, err
	}

	if result.IsCancelled() {
		return PrCheckoutOptions{Cancelled: true}, nil
	}

	// Extract values
	opts := PrCheckoutOptions{}

	// Get selected repo
	if showRepoStep {
		opts.SelectedRepo = result.GetString("repo")
	} else if len(repoPaths) > 0 {
		// Single repo case - use the first (only) repo
		opts.SelectedRepo = repoPaths[0]
	}

	// Get selected PR number
	prValue := result.GetValue("pr")
	if num, ok := prValue.Raw.(int); ok {
		opts.SelectedPR = num
	}

	// Hooks
	if hasHooks {
		opts.SelectedHooks = result.GetStrings("hooks")
		opts.NoHook = len(opts.SelectedHooks) == 0
	}

	return opts, nil
}

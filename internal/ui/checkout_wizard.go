package ui

import (
	"fmt"
	"strings"

	"github.com/raphi011/wt/internal/ui/wizard"
)

// CheckoutOptions holds the options gathered from interactive mode.
type CheckoutOptions struct {
	Branch        string
	NewBranch     bool
	IsWorktree    bool // True if selected branch already has a worktree (hooks only)
	Fetch         bool
	Cancelled     bool
	SelectedRepos []string // Selected repo paths (when outside a repo)
	SelectedHooks []string // Hook names to run (empty if NoHook is true)
	NoHook        bool     // True if no hooks selected
}

// BranchInfo contains branch info including worktree status.
type BranchInfo struct {
	Name       string
	InWorktree bool
}

// BranchFetchResult contains branches with their worktree status.
type BranchFetchResult struct {
	Branches []BranchInfo
}

// BranchFetcher is a function that fetches branches for a repo path.
type BranchFetcher func(repoPath string) BranchFetchResult

// HookInfo contains hook display info for the wizard.
type HookInfo struct {
	Name        string
	Description string
	IsDefault   bool // Has on=["checkout"]
}

// CheckoutWizardParams contains parameters for the checkout wizard.
type CheckoutWizardParams struct {
	Branches         []BranchInfo // Existing branches with worktree status
	AvailableRepos   []string     // All available repo paths
	RepoNames        []string     // Display names for repos
	PreSelectedRepos []int        // Indices of pre-selected repos (e.g., current repo when inside one)
	FetchBranches    BranchFetcher
	AvailableHooks   []HookInfo
	HooksFromCLI     bool // True if --hook or --no-hook was passed (skip hooks step)
}

// CheckoutInteractive runs the interactive checkout wizard.
func CheckoutInteractive(params CheckoutWizardParams) (CheckoutOptions, error) {
	w := wizard.NewWizard("Checkout")

	// Track repo paths/names for the wizard
	repoPaths := params.AvailableRepos
	repoNames := params.RepoNames
	hasRepos := len(repoPaths) > 0

	// Keep track of current branches for worktree lookup
	currentBranches := params.Branches

	// Step 1: Repos (only when available)
	if hasRepos {
		repoOptions := make([]wizard.Option, len(repoNames))
		for i, name := range repoNames {
			repoOptions[i] = wizard.Option{
				Label: name,
				Value: repoPaths[i],
			}
		}
		repoStep := wizard.NewMultiSelect("repos", "Repos", "Select repositories", repoOptions)
		repoStep.SetMinMax(1, 0) // At least one repo required

		// Pre-select repos if provided
		if len(params.PreSelectedRepos) > 0 {
			repoStep.SetSelected(params.PreSelectedRepos)
		}

		w.AddStep(repoStep)
	}

	// Step 2: Branch (combined mode + branch selection)
	// Supports creating new branch via filter or selecting existing
	branchOptions := buildBranchOptions(params.Branches)
	branchStep := wizard.NewFilterableList("branch", "Branch", "Select or create a branch", branchOptions).
		WithCreateFromFilter(func(filter string) string {
			return fmt.Sprintf("+ Create %q", filter)
		}).
		WithValueLabel(func(value string, isNew bool, opt wizard.Option) string {
			if isNew {
				return value + " (new)"
			}
			if strings.HasSuffix(opt.Label, "(checked out)") {
				return value + " (hooks only)"
			}
			return value
		}).
		WithRuneFilter(wizard.RuneFilterNoSpaces)
	w.AddStep(branchStep)

	// Step 3: Options (fetch) - only for new branches
	fetchOptions := []wizard.Option{
		{Label: "Yes", Value: true},
		{Label: "No", Value: false},
	}
	fetchStep := wizard.NewSingleSelect("fetch", "Fetch", "Fetch from origin first?", fetchOptions)
	w.AddStep(fetchStep)

	// Step 4: Hooks (only when available and not set via CLI)
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
		hookStep := wizard.NewMultiSelect("hooks", "Hooks", "Select hooks to run after checkout", hookOptions)
		hookStep.SetMinMax(0, 0) // No minimum required (can select none)
		if len(preSelectedHooks) > 0 {
			hookStep.SetSelected(preSelectedHooks)
		}
		w.AddStep(hookStep)
	}

	// Skip conditions
	// Skip "fetch" step when checking out existing branch (not creating new)
	w.SkipWhen("fetch", func(wiz *wizard.Wizard) bool {
		step := wiz.GetStep("branch").(*wizard.FilterableListStep)
		return !step.IsCreateSelected()
	})

	// Callbacks
	// When repos selection completes, fetch branches from first selected repo
	if hasRepos && params.FetchBranches != nil {
		w.OnComplete("repos", func(wiz *wizard.Wizard) {
			repoStep := wiz.GetStep("repos").(*wizard.MultiSelectStep)
			indices := repoStep.GetSelectedIndices()
			if len(indices) == 0 {
				return
			}

			// Fetch branches from first selected repo
			firstRepoPath := repoPaths[indices[0]]
			result := params.FetchBranches(firstRepoPath)

			// Update current branches for worktree lookup
			currentBranches = result.Branches

			// Update branch step with fetched branches
			branchStep := wiz.GetStep("branch").(*wizard.FilterableListStep)
			branchOpts := buildBranchOptions(result.Branches)
			branchStep.SetOptions(branchOpts)
		})
	}

	// Info line showing selected repos
	if hasRepos {
		w.WithInfoLine(func(wiz *wizard.Wizard) string {
			repoStep := wiz.GetStep("repos")
			if repoStep == nil || !repoStep.IsComplete() {
				return ""
			}
			v := repoStep.Value()
			if v.Label == "" {
				return ""
			}
			return "Selected: " + v.Label
		})
	}

	// Run the wizard
	result, err := w.Run()
	if err != nil {
		return CheckoutOptions{}, err
	}

	if result.IsCancelled() {
		return CheckoutOptions{Cancelled: true}, nil
	}

	// Extract values
	opts := CheckoutOptions{}

	// Get selected repos
	if hasRepos {
		opts.SelectedRepos = result.GetStrings("repos")
	}

	// Branch - get from the combined branch step
	branchStepResult := result.GetStep("branch").(*wizard.FilterableListStep)
	opts.Branch = result.GetString("branch")
	opts.NewBranch = branchStepResult.IsCreateSelected()

	// Check if selected branch is in worktree
	if !opts.NewBranch {
		for _, b := range currentBranches {
			if strings.EqualFold(b.Name, opts.Branch) && b.InWorktree {
				opts.IsWorktree = true
				break
			}
		}
	}

	// Fetch (only relevant for new branches)
	if opts.NewBranch {
		opts.Fetch = result.GetBool("fetch")
	}

	// Hooks
	if hasHooks {
		opts.SelectedHooks = result.GetStrings("hooks")
		opts.NoHook = len(opts.SelectedHooks) == 0
	}

	return opts, nil
}

// buildBranchOptions creates Option slice from branches with worktree info.
func buildBranchOptions(branches []BranchInfo) []wizard.Option {
	opts := make([]wizard.Option, len(branches))
	for i, branch := range branches {
		label := branch.Name
		if branch.InWorktree {
			label += " (checked out)"
		}
		opts[i] = wizard.Option{
			Label: label,
			Value: branch.Name,
		}
	}
	return opts
}

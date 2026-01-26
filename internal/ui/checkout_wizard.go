package ui

import (
	"github.com/raphi011/wt/internal/ui/wizard"
)

// CheckoutOptions holds the options gathered from interactive mode.
type CheckoutOptions struct {
	Branch        string
	NewBranch     bool
	Fetch         bool
	Cancelled     bool
	SelectedRepos []string // Selected repo paths (when outside a repo)
}

// BranchFetchResult contains branches and which ones are in worktrees.
type BranchFetchResult struct {
	Branches         []string        // All branches
	WorktreeBranches map[string]bool // Branches already checked out in worktrees
}

// BranchFetcher is a function that fetches branches for a repo path.
type BranchFetcher func(repoPath string) BranchFetchResult

// CheckoutWizardParams contains parameters for the checkout wizard.
type CheckoutWizardParams struct {
	Branches         []string        // Existing branches for selection
	WorktreeBranches map[string]bool // Branches already in worktrees (unselectable)
	AvailableRepos   []string        // All available repo paths
	RepoNames        []string        // Display names for repos
	PreSelectedRepos []int           // Indices of pre-selected repos (e.g., current repo when inside one)
	FetchBranches    BranchFetcher   // Optional callback to fetch branches after repo selection
}

// CheckoutInteractive runs the interactive checkout wizard.
func CheckoutInteractive(params CheckoutWizardParams) (CheckoutOptions, error) {
	w := wizard.NewWizard("Checkout")

	// Track repo paths/names for the wizard
	repoPaths := params.AvailableRepos
	repoNames := params.RepoNames
	hasRepos := len(repoPaths) > 0

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

	// Step 2: Mode
	modeOptions := []wizard.Option{
		{Label: "Create new branch", Value: true},
		{Label: "Checkout existing branch", Value: false},
	}
	modeStep := wizard.NewSingleSelect("mode", "Mode", "Create new branch or checkout existing?", modeOptions)
	w.AddStep(modeStep)

	// Step 3: Branch
	// This will be dynamically updated based on mode selection
	branchOptions := buildBranchOptions(params.Branches, params.WorktreeBranches)
	branchStep := wizard.NewFilterableList("branch", "Branch", "Select a branch", branchOptions)

	// Also create text input for new branch name
	newBranchStep := wizard.NewTextInput("newbranch", "Branch", "Enter branch name:", "feature/my-feature")

	w.AddStep(branchStep)
	w.AddStep(newBranchStep)

	// Step 4: Options (fetch)
	fetchOptions := []wizard.Option{
		{Label: "Yes", Value: true},
		{Label: "No", Value: false},
	}
	fetchStep := wizard.NewSingleSelect("fetch", "Options", "Fetch from origin first?", fetchOptions)
	w.AddStep(fetchStep)

	// Skip conditions
	// Skip "repos" step if already pre-selected and no changes needed
	// (This is handled by the wizard - if step is completed, user can still go back)

	// Skip "branch" step when creating new branch (use newbranch instead)
	w.SkipWhen("branch", func(wiz *wizard.Wizard) bool {
		return wiz.GetBool("mode") // mode=true means "create new branch"
	})

	// Skip "newbranch" step when checking out existing branch
	w.SkipWhen("newbranch", func(wiz *wizard.Wizard) bool {
		return !wiz.GetBool("mode") // mode=false means "checkout existing"
	})

	// Skip "fetch" step when checking out existing branch
	w.SkipWhen("fetch", func(wiz *wizard.Wizard) bool {
		return !wiz.GetBool("mode")
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

			// Disable "checkout existing" if multiple repos selected
			modeStep := wiz.GetStep("mode").(*wizard.SingleSelectStep)
			if len(indices) > 1 {
				modeStep.DisableOption(1, "single repo only")
			} else {
				modeStep.EnableAllOptions()
			}

			// Fetch branches from first selected repo
			firstRepoPath := repoPaths[indices[0]]
			result := params.FetchBranches(firstRepoPath)

			// Update branch step with fetched branches
			branchStep := wiz.GetStep("branch").(*wizard.FilterableListStep)
			branchOpts := buildBranchOptions(result.Branches, result.WorktreeBranches)
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

	// Mode
	opts.NewBranch = result.GetBool("mode")

	// Branch name
	if opts.NewBranch {
		opts.Branch = result.GetString("newbranch")
		opts.Fetch = result.GetBool("fetch")
	} else {
		opts.Branch = result.GetString("branch")
		opts.Fetch = false
	}

	return opts, nil
}

// buildBranchOptions creates Option slice from branches, marking worktree branches as disabled.
func buildBranchOptions(branches []string, worktreeBranches map[string]bool) []wizard.Option {
	opts := make([]wizard.Option, len(branches))
	for i, branch := range branches {
		opts[i] = wizard.Option{
			Label:    branch,
			Value:    branch,
			Disabled: worktreeBranches[branch],
		}
		if worktreeBranches[branch] {
			opts[i].Description = "in worktree"
		}
	}
	return opts
}


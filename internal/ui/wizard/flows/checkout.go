package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
)

// CheckoutOptions holds the options gathered from interactive mode.
type CheckoutOptions struct {
	Branch        string
	NewBranch     bool
	Base          string // Base branch for new branch creation
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
	Branches      []BranchInfo
	DefaultBranch string // Default branch name (e.g. "main", "master")
}

// BranchFetcher is a function that fetches branches for a repo path.
type BranchFetcher func(ctx context.Context, repoPath string) (BranchFetchResult, error)

// HookInfo contains hook display info for the wizard.
type HookInfo struct {
	Name        string
	Description string
	IsDefault   bool // Runs by default for the checkout
}

// addHookStep adds a hook selection step to the wizard if hooks are available
// and not already set via CLI flags.
func addHookStep(w *framework.Wizard, hooks []HookInfo) {
	hookOptions := make([]framework.Option, len(hooks))
	var preSelectedHooks []int
	for i, hook := range hooks {
		label := hook.Name
		if hook.Description != "" {
			label = hook.Name + " - " + hook.Description
		}
		hookOptions[i] = framework.Option{
			Label: label,
			Value: hook.Name,
		}
		if hook.IsDefault {
			preSelectedHooks = append(preSelectedHooks, i)
		}
	}
	hookStep := steps.NewFilterableList("hooks", "Hooks", "Select hooks to run after checkout", hookOptions).
		WithMultiSelect().
		SetMinMax(0, 0) // No minimum required (can select none)
	if len(preSelectedHooks) > 0 {
		hookStep.SetSelected(preSelectedHooks)
	}
	w.AddStep(hookStep)
}

// CheckoutWizardParams contains parameters for the checkout wizard.
type CheckoutWizardParams struct {
	Context          context.Context
	Branches         []BranchInfo  // Existing branches with worktree status
	AvailableRepos   []string      // All available repo paths
	RepoNames        []string      // Display names for repos
	PreSelectedRepos []int         // Indices of pre-selected repos (e.g., current repo when inside one)
	FetchBranches    BranchFetcher // Function to fetch branches for a repo
	AvailableHooks   []HookInfo
	HooksFromCLI     bool   // True if --hook or --no-hook was passed (skip hooks step)
	DefaultBranch    string // Default branch name for pre-selection in base step
	BaseFromCLI      bool   // True if --base was explicitly passed (skip base step)
}

// buildCheckoutWizard constructs the flow without running subprocesses.
func buildCheckoutWizard(params CheckoutWizardParams) (*framework.Wizard, *steps.FilterableListStep) {
	w := framework.NewWizard("Checkout")

	// Track repo paths/names for the wizard
	repoPaths := params.AvailableRepos
	repoNames := params.RepoNames
	hasRepos := len(repoPaths) > 0

	// Step 1: Repos (only when available)
	if hasRepos {
		repoOptions := make([]framework.Option, len(repoNames))
		for i, name := range repoNames {
			repoOptions[i] = framework.Option{
				Label: name,
				Value: repoPaths[i],
			}
		}
		repoStep := steps.NewFilterableList("repos", "Repos", "Select repositories", repoOptions).
			WithMultiSelect().
			SetMinMax(1, 0) // At least one repo required

		// Pre-select repos if provided
		if len(params.PreSelectedRepos) > 0 {
			repoStep.SetSelected(params.PreSelectedRepos)
		}

		w.AddStep(repoStep)
	}

	// Step 2: Branch (combined mode + branch selection)
	// Supports creating new branch via filter or selecting existing
	branchOptions := buildBranchOptions(params.Branches)
	branchStep := steps.NewFilterableList("branch", "Branch", "Select or create a branch", branchOptions).
		WithCreateFromFilter(func(filter string) string {
			return fmt.Sprintf("+ Create %q", filter)
		}).
		WithValueLabel(func(value string, isNew bool, _ framework.Option) string {
			if isNew {
				return value + " (new)"
			}
			return value
		}).
		WithRuneFilter(framework.RuneFilterNoSpaces).
		WithEmptyMessage("No matching branches")

	// Step 3: Base branch (only when creating new branch and not set via CLI)
	// Use plain branch names without worktree decoration — the user is selecting
	// a branch to fork from, not opening a worktree.
	baseOptions := buildBaseBranchOptions(params.Branches)
	baseStep := steps.NewFilterableList("base", "Base Branch", "Select a base branch to create from", baseOptions).
		WithRuneFilter(framework.RuneFilterNoSpaces).
		WithEmptyMessage("No matching branches")

	// Pre-select default branch
	if params.DefaultBranch != "" {
		for i, opt := range baseOptions {
			if opt.Value == params.DefaultBranch {
				baseStep.SetCursor(i)
				break
			}
		}
	}

	if params.FetchBranches != nil {
		loading := steps.NewLoadingStep(branchStep, steps.LoadingConfig[BranchFetchResult]{
			Context: params.Context,
			Key: func() (string, error) {
				if !hasRepos {
					return "", fmt.Errorf("no repository available")
				}
				indices := w.GetStep("repos").(*steps.FilterableListStep).GetSelectedIndices()
				if len(indices) == 0 {
					return "", fmt.Errorf("select a repository first")
				}
				return repoPaths[indices[0]], nil
			},
			Fetch: params.FetchBranches,
			Apply: func(result BranchFetchResult) bool {
				branchStep.SetOptions(buildBranchOptions(result.Branches))
				baseStep.Reset()
				baseOptions := buildBaseBranchOptions(result.Branches)
				baseStep.SetOptions(baseOptions)
				for i, opt := range baseOptions {
					if opt.Value == result.DefaultBranch {
						baseStep.SetCursor(i)
						break
					}
				}
				return len(result.Branches) == 0
			},
			LoadingText: "Loading branches…",
			EmptyText:   "No branches found. Type a name to create the first branch, or refresh with Ctrl+R.",
		})
		w.AddStep(loading)
	} else {
		w.AddStep(branchStep)
	}
	if hasRepos {
		previousSelection := strings.Join(w.GetStrings("repos"), "\x00")
		w.OnComplete("repos", func(wiz *framework.Wizard) {
			selection := strings.Join(wiz.GetStrings("repos"), "\x00")
			if selection != previousSelection {
				wiz.GetStep("branch").Reset()
				baseStep.Reset()
			}
			previousSelection = selection
		})
	}

	w.AddStep(baseStep)

	// Skip base step when selecting existing branch or --base passed on CLI
	w.SkipWhen("base", func(wiz *framework.Wizard) bool {
		if params.BaseFromCLI {
			return true
		}
		return !branchStep.IsCreateSelected() || branchStep.OptionsCount() == 0
	})

	// Step 4: Hooks (only when available and not set via CLI)
	hasHooks := len(params.AvailableHooks) > 0 && !params.HooksFromCLI
	if hasHooks {
		addHookStep(w, params.AvailableHooks)
	}

	return w, branchStep
}

// CheckoutInteractive runs the wizard with command-context cancellation.
func CheckoutInteractive(params CheckoutWizardParams) (CheckoutOptions, error) {
	w, branchStep := buildCheckoutWizard(params)
	// Run the wizard
	ctx := params.Context
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := w.RunContext(ctx)
	if err != nil {
		return CheckoutOptions{}, err
	}

	if result.IsCancelled() {
		return CheckoutOptions{Cancelled: true}, nil
	}

	// Extract values
	opts := CheckoutOptions{}

	// Get selected repos
	if len(params.AvailableRepos) > 0 {
		opts.SelectedRepos = result.GetStrings("repos")
	}

	// Branch - get from the combined branch step
	opts.Branch = result.GetString("branch")
	opts.NewBranch = branchStep.IsCreateSelected()

	// Base branch
	if !params.BaseFromCLI {
		opts.Base = result.GetString("base")
	}

	// Hooks
	if len(params.AvailableHooks) > 0 && !params.HooksFromCLI {
		opts.SelectedHooks = result.GetStrings("hooks")
		opts.NoHook = len(opts.SelectedHooks) == 0
	}

	return opts, nil
}

// buildBaseBranchOptions creates Option slice from branches using plain names
// (no worktree decoration), suitable for the base branch selection step.
func buildBaseBranchOptions(branches []BranchInfo) []framework.Option {
	var opts []framework.Option
	for _, branch := range branches {
		opts = append(opts, framework.Option{
			Label: branch.Name,
			Value: branch.Name,
		})
	}
	return opts
}

// buildBranchOptions creates Option slice from branches, appending " (worktree)" to branches that already have a worktree.
func buildBranchOptions(branches []BranchInfo) []framework.Option {
	var opts []framework.Option
	for _, branch := range branches {
		label := branch.Name
		if branch.InWorktree {
			label = branch.Name + " (worktree)"
		}
		opts = append(opts, framework.Option{
			Label:      label,
			SearchText: branch.Name,
			Value:      branch.Name,
		})
	}
	return opts
}

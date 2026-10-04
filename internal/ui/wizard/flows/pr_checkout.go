package flows

import (
	"context"
	"fmt"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
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
type PRFetcher func(ctx context.Context, repoPath string) ([]forge.OpenPR, error)

// PrCheckoutWizardParams contains parameters for the PR checkout wizard.
type PrCheckoutWizardParams struct {
	Context         context.Context
	AvailableRepos  []string   // All available repo paths
	RepoNames       []string   // Display names for repos
	PreSelectedRepo int        // Index of pre-selected repo (-1 if none)
	FetchPRs        PRFetcher  // Function to fetch PRs for a repo
	AvailableHooks  []HookInfo // Available hooks
	HooksFromCLI    bool       // True if --hook or --no-hook was passed (skip hooks step)
}

// buildPrCheckoutWizard constructs the flow without running subprocesses.
func buildPrCheckoutWizard(params PrCheckoutWizardParams) *framework.Wizard {
	w := framework.NewWizard("PR Checkout")

	// Track repo paths/names for the wizard
	repoPaths := params.AvailableRepos
	repoNames := params.RepoNames

	// Only show repo selection step if there are multiple repos to choose from
	showRepoStep := len(repoPaths) > 1

	// Step 1: Repo selection (single-select) - skip if only one repo
	if showRepoStep {
		repoOptions := make([]framework.Option, len(repoNames))
		for i, name := range repoNames {
			repoOptions[i] = framework.Option{
				Label: name,
				Value: repoPaths[i],
			}
		}
		repoStep := steps.NewSingleSelect("repo", "Repository", "Select a repository", repoOptions)

		// Pre-select repo if provided
		if params.PreSelectedRepo >= 0 && params.PreSelectedRepo < len(repoPaths) {
			repoStep.SetCursor(params.PreSelectedRepo)
		}

		w.AddStep(repoStep)
	}

	// Step 2: PR selection (single-select with two-row display)
	prStep := steps.NewSingleSelect("pr", "PR", "Select a PR to checkout", nil)
	if params.FetchPRs != nil {
		w.AddStep(steps.NewLoadingStep(prStep, steps.LoadingConfig[[]forge.OpenPR]{
			Context: params.Context,
			Key: func() (string, error) {
				if showRepoStep {
					return w.GetString("repo"), nil
				}
				if len(repoPaths) == 0 {
					return "", fmt.Errorf("no repository available")
				}
				return repoPaths[0], nil
			},
			Fetch: params.FetchPRs,
			Apply: func(prs []forge.OpenPR) bool {
				prStep.SetOptions(buildPROptions(prs))
				return len(prs) == 0
			},
			LoadingText: "Loading open PRs…",
			EmptyText:   "No open PRs found. Refresh with Ctrl+R or go back to choose another repository.",
		}))
	} else {
		w.AddStep(prStep)
	}

	// Step 3: Hooks (only when available and not set via CLI)
	hasHooks := len(params.AvailableHooks) > 0 && !params.HooksFromCLI
	if hasHooks {
		addHookStep(w, params.AvailableHooks)
	}

	// Info line showing selected repo
	if showRepoStep {
		w.WithInfoLine(func(wiz *framework.Wizard) string {
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

	return w
}

// PrCheckoutInteractive runs the wizard with command-context cancellation.
func PrCheckoutInteractive(params PrCheckoutWizardParams) (PrCheckoutOptions, error) {
	w := buildPrCheckoutWizard(params)
	ctx := params.Context
	if ctx == nil {
		ctx = context.Background()
	}
	repoPaths := params.AvailableRepos
	showRepoStep := len(repoPaths) > 1
	hasHooks := len(params.AvailableHooks) > 0 && !params.HooksFromCLI
	// Run the wizard
	result, err := w.RunContext(ctx)
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

func buildPROptions(prs []forge.OpenPR) []framework.Option {
	options := make([]framework.Option, len(prs))
	for i, pr := range prs {
		desc := fmt.Sprintf("@%s (%s)", pr.Author, pr.Branch)
		if pr.IsDraft {
			desc += " [draft]"
		}
		options[i] = framework.Option{Label: pr.Title, Description: desc, Value: pr.Number}
	}
	return options
}

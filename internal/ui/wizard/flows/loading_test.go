package flows

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/ui/wizard/framework"
	"github.com/raphi011/wt/internal/ui/wizard/steps"
)

func wizardKey(code rune, mod tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: mod}
}

func TestPRLoadingSwitchCancelsAndIgnoresLateResults(t *testing.T) {
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	w := buildPrCheckoutWizard(PrCheckoutWizardParams{
		AvailableRepos: []string{"A", "B"}, RepoNames: []string{"A", "B"},
		FetchPRs: func(ctx context.Context, repo string) ([]forge.OpenPR, error) {
			if repo == "A" {
				started <- ctx
				<-release
			}
			return []forge.OpenPR{{Number: 1, Title: "PR from " + repo}}, nil
		},
	})
	defer w.Close()
	w.Init()
	_, cmdA := w.Update(wizardKey(tea.KeyEnter, 0))
	resultA := make(chan tea.Msg, 1)
	go func() { resultA <- cmdA() }()
	var ctxA context.Context
	select {
	case ctxA = <-started:
	case <-time.After(time.Second):
		t.Fatal("fetch did not start")
	}
	// UI updates run while the command is still blocked.
	w.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	if !strings.Contains(w.GetStep("pr").View(), "Loading") {
		t.Fatal("expected loading state")
	}
	w.Update(wizardKey(tea.KeyEnter, 0))
	if w.CurrentStepID() != "pr" {
		t.Fatal("loading state allowed selection")
	}
	w.Update(wizardKey(tea.KeyLeft, tea.ModAlt))
	select {
	case <-ctxA.Done():
	default:
		t.Fatal("back did not cancel obsolete fetch")
	}
	w.Update(wizardKey(tea.KeyDown, 0))
	_, cmdB := w.Update(wizardKey(tea.KeyEnter, 0))
	w.Update(cmdB())
	if !strings.Contains(w.GetStep("pr").View(), "PR from B") {
		t.Fatal("B did not load")
	}
	close(release)
	w.Update(<-resultA)
	if strings.Contains(w.GetStep("pr").View(), "PR from A") || !strings.Contains(w.GetStep("pr").View(), "PR from B") {
		t.Fatal("late A result replaced B")
	}
}

func TestPRLoadingRetryCacheAndRefresh(t *testing.T) {
	calls := 0
	w := buildPrCheckoutWizard(PrCheckoutWizardParams{
		AvailableRepos: []string{"A", "B"}, RepoNames: []string{"A", "B"},
		FetchPRs: func(ctx context.Context, repo string) ([]forge.OpenPR, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("offline")
			}
			return []forge.OpenPR{{Number: calls, Title: "Ready", Author: "author", Branch: "topic", IsDraft: true}}, nil
		},
	})
	defer w.Close()
	w.Init()
	_, cmd := w.Update(wizardKey(tea.KeyEnter, 0))
	w.Update(cmd())
	if !strings.Contains(w.GetStep("pr").View(), "offline") {
		t.Fatal("missing error")
	}
	_, cmd = w.Update(wizardKey('r', tea.ModCtrl))
	w.Update(cmd())
	if calls != 2 || !strings.Contains(w.GetStep("pr").View(), "[draft]") {
		t.Fatal("retry did not apply PR metadata")
	}
	w.Update(wizardKey(tea.KeyLeft, tea.ModAlt))
	_, cmd = w.Update(wizardKey(tea.KeyEnter, 0))
	if cmd != nil || calls != 2 {
		t.Fatal("returning to unchanged repo fetched again")
	}
	_, cmd = w.Update(wizardKey('r', tea.ModCtrl))
	w.Update(cmd())
	if calls != 3 {
		t.Fatal("explicit refresh reused stale cache")
	}
	w.Update(wizardKey(tea.KeyEnter, 0))
	if w.GetValue("pr").Raw != 3 {
		t.Fatalf("PR value = %v", w.GetValue("pr").Raw)
	}
}

func TestBranchLoadingDefaultsEmptyCreationAndCache(t *testing.T) {
	calls := 0
	w, branch := buildCheckoutWizard(CheckoutWizardParams{
		AvailableRepos: []string{"A", "B"}, RepoNames: []string{"A", "B"}, PreSelectedRepos: []int{0},
		FetchBranches: func(ctx context.Context, repo string) (BranchFetchResult, error) {
			calls++
			if repo == "B" {
				return BranchFetchResult{}, nil
			}
			return BranchFetchResult{Branches: []BranchInfo{{Name: "topic"}, {Name: "main", InWorktree: true}}, DefaultBranch: "main"}, nil
		},
	})
	defer w.Close()
	cmd := w.Init()
	if calls != 0 || w.CurrentStepID() != "branch" {
		t.Fatal("construction/Init fetched synchronously")
	}
	w.Update(cmd())
	if base := w.GetStep("base").(*steps.FilterableListStep); base.GetCursor() != 1 {
		t.Fatal("default base not selected")
	}
	w.Update(tea.PasteMsg{Content: "main"})
	w.Update(wizardKey(tea.KeyEnter, 0))
	if branch.IsCreateSelected() || w.GetString("branch") != "main" || w.CurrentStepID() != "summary" {
		t.Fatal("existing branch behavior changed")
	}
	w.Update(wizardKey(tea.KeyLeft, tea.ModAlt)) // summary -> branch
	w.Update(wizardKey(tea.KeyLeft, tea.ModAlt)) // branch -> repos
	repos := w.GetStep("repos").(*steps.FilterableListStep)
	repos.SetSelected([]int{0})
	w.Update(wizardKey(tea.KeyEnter, 0))
	// Returning to the same repository retains input and selection.
	if calls != 1 {
		t.Fatal("same repository fetched again")
	}

	if w.GetString("branch") != "main" {
		t.Fatal("unchanged selection was lost")
	}
	w.Update(wizardKey(tea.KeyLeft, tea.ModAlt))
	// Toggle off A and select B.
	w.Update(wizardKey(tea.KeySpace, 0))
	w.Update(wizardKey(tea.KeyDown, 0))
	w.Update(wizardKey(tea.KeySpace, 0))
	_, cmd = w.Update(wizardKey(tea.KeyEnter, 0))
	w.Update(cmd())
	if !strings.Contains(w.GetStep("branch").View(), "No branches found") || branch.IsComplete() {
		t.Fatal("empty repository retained old branch")
	}
	w.Update(tea.PasteMsg{Content: "first"})
	w.Update(wizardKey(tea.KeyEnter, 0))
	if !branch.IsCreateSelected() || w.CurrentStepID() != "summary" {
		t.Fatal("first branch creation required a nonexistent base")
	}
}

func TestLoadingCancellationOnExitAndParentContext(t *testing.T) {
	for _, how := range []string{"ctrl+c", "esc", "parent"} {
		t.Run(how, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered := make(chan context.Context, 1)
			w := buildPrCheckoutWizard(PrCheckoutWizardParams{
				Context: parent, AvailableRepos: []string{"A"},
				FetchPRs: func(ctx context.Context, _ string) ([]forge.OpenPR, error) {
					entered <- ctx
					<-ctx.Done()
					return nil, ctx.Err()
				},
			})
			defer w.Close()
			cmd := w.Init()
			result := make(chan tea.Msg, 1)
			go func() { result <- cmd() }()
			ctx := <-entered
			switch how {
			case "parent":
				cancel()
			case "esc":
				w.Update(wizardKey(tea.KeyEscape, 0))
			default:
				w.Update(wizardKey('c', tea.ModCtrl))
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("fetch did not cancel")
			}
			w.Update(<-result)
			if how != "parent" && !w.IsCancelled() {
				t.Fatal("wizard did not cancel")
			}
			if w.GetStep("pr").IsComplete() {
				t.Fatal("cancelled result completed a step")
			}
		})
	}
}

var _ framework.Step = (*steps.LoadingStep[BranchFetchResult])(nil)

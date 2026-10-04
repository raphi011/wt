package steps

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

func TestLoadingCacheIsScopedToRepository(t *testing.T) {
	repo, calls := "A", 0
	selector := NewSingleSelect("items", "Items", "", nil)
	loader := NewLoadingStep(selector, LoadingConfig[[]framework.Option]{
		Key: func() (string, error) { return repo, nil },
		Fetch: func(ctx context.Context, repo string) ([]framework.Option, error) {
			calls++
			return []framework.Option{{Label: repo, Value: repo}}, nil
		},
		Apply: func(options []framework.Option) bool { selector.SetOptions(options); return len(options) == 0 },
	})
	loader.Update(loader.Init()())
	repo = "B"
	loader.Update(loader.Init()())
	repo = "A"
	if cmd := loader.Init(); cmd != nil {
		t.Fatal("cached repository fetched again")
	}
	opt, _ := selector.GetOption(0)
	if calls != 2 || opt.Value != "A" {
		t.Fatalf("cache returned wrong repo: calls=%d option=%+v", calls, opt)
	}
}

func TestRefreshCancelsPendingRequestAndRejectsSameRepoLateResult(t *testing.T) {
	calls := 0
	selector := NewSingleSelect("items", "Items", "", nil)
	loader := NewLoadingStep(selector, LoadingConfig[int]{
		Key:   func() (string, error) { return "A", nil },
		Fetch: func(ctx context.Context, _ string) (int, error) { calls++; return calls, nil },
		Apply: func(value int) bool {
			selector.SetOptions([]framework.Option{{Label: "result", Value: value}})
			return false
		},
	})
	// First result has been computed but has not yet been delivered to Update.
	old := loader.Init()()
	_, cmd, _ := loader.Update(keyMsg("ctrl+r"))
	loader.Update(cmd())
	loader.Update(old)
	loader.Update(keyMsg("enter"))
	if calls != 2 || loader.Value().Raw != 2 {
		t.Fatal("obsolete same-repository result replaced refresh result")
	}
}

func TestCancelledCommandDoesNotStartFetch(t *testing.T) {
	called := false
	loader := NewLoadingStep(NewSingleSelect("items", "Items", "", nil), LoadingConfig[int]{
		Key:   func() (string, error) { return "A", nil },
		Fetch: func(ctx context.Context, _ string) (int, error) { called = true; return 1, nil },
		Apply: func(int) bool { return false },
	})
	cmd := loader.Init()
	loader.Deactivate()
	loader.Update(cmd())
	if called || loader.IsComplete() {
		t.Fatal("obsolete command performed I/O")
	}
}

func TestLoadingKeyErrorCanRecoverWithRetry(t *testing.T) {
	valid := false
	selector := NewSingleSelect("items", "Items", "", nil)
	loader := NewLoadingStep(selector, LoadingConfig[int]{
		Key: func() (string, error) {
			if !valid {
				return "", errors.New("choose a repository")
			}
			return "repo", nil
		},
		Fetch: func(context.Context, string) (int, error) { return 42, nil },
		Apply: func(value int) bool {
			selector.SetOptions([]framework.Option{{Label: "ready", Value: value}})
			return false
		},
	})
	if cmd := loader.Init(); cmd != nil {
		t.Fatal("invalid repository key launched a fetch")
	}
	if !strings.Contains(loader.View(), "choose a repository") || loader.IsComplete() || loader.Value().Raw != nil {
		t.Fatal("key error was not actionable/incomplete")
	}
	valid = true
	_, cmd, _ := loader.Update(keyMsg("ctrl+r"))
	if cmd == nil {
		t.Fatal("retry did not restart loading after key correction")
	}
	loader.Update(cmd())
	loader.Update(keyMsg("enter"))
	if loader.Value().Raw != 42 || !loader.IsComplete() {
		t.Fatal("retry did not restore usable selection")
	}
}

func TestLoadingParentCancellationDiscardsFetchedResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	applied := false
	loader := NewLoadingStep(NewSingleSelect("items", "Items", "", nil), LoadingConfig[int]{
		Context: ctx, Key: func() (string, error) { return "repo", nil },
		Fetch: func(context.Context, string) (int, error) { cancel(); return 42, nil },
		Apply: func(int) bool { applied = true; return false },
	})
	loader.Update(loader.Init()())
	if applied || loader.IsComplete() || loader.Value().Raw != nil {
		t.Fatal("cancelled fetch applied its result")
	}
	if !strings.Contains(loader.View(), "context canceled") {
		t.Fatal("parent cancellation was not propagated")
	}
}

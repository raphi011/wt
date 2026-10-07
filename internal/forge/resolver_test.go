package forge_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/forge/forgetest"
)

func TestResolverDetectionAndOverride(t *testing.T) {
	t.Parallel()
	resolver := forge.ResolverFromContext(context.Background())
	for _, tc := range []struct{ url, name, want string }{
		{"https://github.com/org/repo", "", "github"},
		{"git@gitlab.com:org/repo.git", "", "gitlab"},
		{"https://code.example/org/repo", "", "gitlab"},
		{"https://gitlab.com/org/repo", "github", "github"},
		{"https://github.com/org/repo", "gitlab", "gitlab"},
	} {
		f := resolver(tc.url, tc.name, map[string]string{"code.example": "gitlab"}, &config.ForgeConfig{})
		if f.Name() != tc.want {
			t.Errorf("resolve(%q, %q) = %q, want %q", tc.url, tc.name, f.Name(), tc.want)
		}
	}
}

type checkedForge struct {
	*forgetest.Forge
	checks atomic.Int32
	err    error
}

func (f *checkedForge) Check(ctx context.Context) error {
	f.checks.Add(1)
	return f.err
}

func TestSessionInjectedAdapterChecksOnce(t *testing.T) {
	t.Parallel()
	for _, checkErr := range []error{nil, errors.New("authentication failed")} {
		f := &checkedForge{Forge: forgetest.New(), err: checkErr}
		resolver := forge.Resolver(func(string, string, map[string]string, *config.ForgeConfig) forge.Forge { return f })
		ctx := forge.WithResolver(context.Background(), resolver)
		session := forge.NewSession(forge.ResolverFromContext(ctx), nil, nil)
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				got, err := session.Resolve(ctx, "https://example.test/org/repo")
				if got != f || !errors.Is(err, checkErr) {
					t.Errorf("Resolve = %v, %v; want injected adapter, %v", got, err, checkErr)
				}
			})
		}
		wg.Wait()
		if f.checks.Load() != 1 {
			t.Fatalf("Check called %d times, want 1", f.checks.Load())
		}
	}
}

func TestSessionCancelledBeforeCheck(t *testing.T) {
	t.Parallel()
	f := &checkedForge{Forge: forgetest.New()}
	resolver := forge.Resolver(func(string, string, map[string]string, *config.ForgeConfig) forge.Forge { return f })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := forge.NewSession(resolver, nil, nil).Resolve(ctx, "https://example.test/org/repo")
	if !errors.Is(err, context.Canceled) || f.checks.Load() != 0 {
		t.Fatalf("cancelled Resolve = %v, checks = %d", err, f.checks.Load())
	}
}

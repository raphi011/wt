//go:build integration

package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/forge/forgetest"
	"github.com/raphi011/wt/internal/git"
)

// localCloneForge keeps forge state in memory and clones a local git source.
type localCloneForge struct {
	*forgetest.Forge
	source string
}

func (f *localCloneForge) CloneRepo(ctx context.Context, repoSpec, destPath string) (string, error) {
	dest := filepath.Join(destPath, filepath.Base(repoSpec))
	if err := git.CloneRegular(ctx, f.source, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func (f *localCloneForge) CloneBareRepo(ctx context.Context, repoSpec, destPath string) (string, error) {
	dest := filepath.Join(destPath, filepath.Base(repoSpec))
	if err := git.CloneBareWithWorktreeSupport(ctx, f.source, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func injectTestForge(ctx context.Context, f forge.Forge) context.Context {
	return forge.WithResolver(ctx, func(repoURL, name string, hosts map[string]string, cfg *config.ForgeConfig) forge.Forge {
		return f
	})
}

func withRepoPR(t *testing.T, ctx context.Context, repoPath, state string) (context.Context, *forgetest.Forge) {
	t.Helper()
	originURL, err := git.GetOriginURL(ctx, repoPath)
	if err != nil {
		t.Fatalf("get origin URL: %v", err)
	}
	f := forgetest.New()
	f.SetPR(originURL, "feature", forge.PRInfo{Number: 1, State: state, URL: "https://example.test/test-repo/pull/1", Author: "test"})
	return injectTestForge(ctx, f), f
}

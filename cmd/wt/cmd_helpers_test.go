package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/fs"
	"github.com/raphi011/wt/internal/hooks"
	"github.com/raphi011/wt/internal/log"
)

func TestWithHooks_AfterWorkDir(t *testing.T) {
	t.Parallel()

	tmpDir := fs.ResolvePath(t.TempDir())
	repoDir := filepath.Join(tmpDir, "repo")
	wtDir := filepath.Join(tmpDir, "wt")
	for _, dir := range []string{repoDir, wtDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	beforeOut := filepath.Join(tmpDir, "before.txt")
	afterOut := filepath.Join(tmpDir, "after.txt")

	ctx := log.WithLogger(context.Background(), log.New(io.Discard, false, false))
	p := hookParams{
		HooksCfg: config.HooksConfig{Hooks: map[string]config.Hook{
			"before": {Command: "pwd -P > '" + beforeOut + "'", On: []string{"before:merge"}},
			"after":  {Command: "pwd -P > '" + afterOut + "'", On: []string{"merge"}},
		}},
		WtPath:       wtDir,
		AfterWorkDir: repoDir,
		RepoPath:     repoDir,
		Trigger:      hooks.CommandMerge,
	}

	// fn removes the worktree, like `wt pr merge` does after merging.
	err := withHooks(ctx, p, func() error {
		return os.Remove(wtDir)
	})
	if err != nil {
		t.Fatalf("withHooks: %v", err)
	}

	for file, want := range map[string]string{beforeOut: wtDir, afterOut: repoDir} {
		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("hook did not run: %v", err)
		}
		if strings.TrimSpace(string(got)) != want {
			t.Errorf("%s: hook ran in %q, want %q", filepath.Base(file), strings.TrimSpace(string(got)), want)
		}
	}
}

package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/registry"
)

func TestPRInteractiveArguments(t *testing.T) {
	for _, tc := range []struct {
		args      []string
		wantError bool
	}{
		{[]string{"-i"}, false}, {[]string{"-i", "repo"}, false},
		{[]string{"-i", "repo", "123"}, true}, {nil, true},
		{[]string{"123"}, false}, {[]string{"repo", "123"}, false},
	} {
		cmd := newPrCheckoutCmd()
		if err := cmd.ParseFlags(tc.args); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Args(cmd, cmd.Flags().Args()); (err != nil) != tc.wantError {
			t.Errorf("args %v: %v", tc.args, err)
		}
	}
}

func TestPRWizardFetchersUseForgeAndCancelSubprocess(t *testing.T) {
	for _, forgeName := range []string{"github", "gitlab"} {
		t.Run(forgeName, func(t *testing.T) {
			repoPath := t.TempDir()
			for _, args := range [][]string{{"init", "--quiet", repoPath}, {"-C", repoPath, "remote", "add", "origin", "https://" + forgeName + ".com/org/repo.git"}} {
				if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
					t.Fatalf("fixture: %v: %s", err, out)
				}
			}
			fakeBin := t.TempDir()
			tool, result := "gh", `[{"number":7,"title":"Test PR","headRefName":"topic","author":{"login":"author"},"isDraft":true}]`
			if forgeName == "gitlab" {
				tool, result = "glab", `[{"iid":7,"title":"Test PR","source_branch":"topic","author":{"username":"author"},"draft":true}]`
			}
			script := "#!/bin/sh\nif [ \"$1\" = auth ]; then exit 0; fi\nif [ \"$FAKE_PR_BLOCK\" = 1 ]; then\n : > \"$FAKE_PR_STARTED\"\n exec /bin/sleep 30\nfi\nprintf '%s' '" + result + "'\n"
			if err := os.WriteFile(filepath.Join(fakeBin, tool), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
			cfg := config.Default()
			ctx := config.WithConfig(context.Background(), &cfg)
			ctx = config.WithResolver(ctx, config.NewResolver(&cfg))
			ctx = config.WithWorkDir(ctx, repoPath)
			ctx = log.WithLogger(ctx, log.New(io.Discard, false, false))
			reg := &registry.Registry{Repos: []registry.Repo{{Name: "repo", Path: repoPath}}}
			params, err := prCheckoutWizardParams(ctx, reg, "repo", "", hookFlags{})
			if err != nil {
				t.Fatal(err)
			}
			prs, err := params.FetchPRs(ctx, repoPath)
			if err != nil || len(prs) != 1 || prs[0].Number != 7 || prs[0].Branch != "topic" || !prs[0].IsDraft {
				t.Fatalf("%s fetch = %+v, %v", forgeName, prs, err)
			}
			started := filepath.Join(fakeBin, "started")
			t.Setenv("FAKE_PR_BLOCK", "1")
			t.Setenv("FAKE_PR_STARTED", started)
			fetchCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := params.FetchPRs(fetchCtx, repoPath); done <- err }()
			deadline := time.Now().Add(3 * time.Second)
			for {
				if _, err := os.Stat(started); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("fake forge process did not start")
				}
				time.Sleep(10 * time.Millisecond)
			}
			cancel()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled fetch succeeded")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("cancellation did not stop forge subprocess")
			}
		})
	}
}

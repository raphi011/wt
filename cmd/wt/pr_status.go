package main

import (
	"context"
	"fmt"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/log"
	"github.com/raphi011/wt/internal/prstatus"
	"github.com/raphi011/wt/internal/ui/progress"
)

// loadWorktreePRStatus keeps terminal progress and diagnostics at the command boundary.
func loadWorktreePRStatus(ctx context.Context, worktrees []git.Worktree, refresh, reset bool) (*prstatus.Result, error) {
	var pb *progress.ProgressBar
	defer func() {
		if pb != nil {
			pb.Stop()
		}
	}()
	status, err := prstatus.Load(ctx, worktrees, config.FromContext(ctx), prstatus.Options{
		Refresh: refresh,
		Reset:   reset,
		Progress: func(completed, total, failed int) {
			if pb == nil {
				pb = progress.NewProgressBar(total, "Fetching PR status...", progress.WithContext(ctx))
				pb.Start()
			}
			msg := "Fetching PR status..."
			if failed > 0 {
				msg = fmt.Sprintf("Fetching PR status... (%d failed)", failed)
			}
			pb.SetProgress(completed, msg)
		},
	})
	if err != nil {
		return nil, err
	}
	l := log.FromContext(ctx)
	if reset {
		l.Println("Cache reset: PR info cleared")
	}
	if len(status.FailedBranches) > 0 {
		l.Printf("Warning: failed to fetch PR status for: %v\n", status.FailedBranches)
	}
	if status.SaveError != nil {
		l.Printf("Warning: failed to save PR cache: %v\n", status.SaveError)
	}
	return status, nil
}

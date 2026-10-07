package prstatus_test

import (
	"testing"

	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/prstatus"
)

func TestBranchIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wt   git.Worktree
		want string
	}{
		{git.Worktree{Branch: "local", UpstreamBranch: "source"}, "source"},
		{git.Worktree{Branch: "feature"}, "feature"},
		{git.Worktree{Branch: "feature", UpstreamBranch: "feature"}, "feature"},
	} {
		if got := prstatus.Branch(tc.wt); got != tc.want {
			t.Errorf("Branch(%+v) = %q, want %q", tc.wt, got, tc.want)
		}
	}
}

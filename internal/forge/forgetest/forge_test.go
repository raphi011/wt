package forgetest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/raphi011/wt/internal/forge"
	"github.com/raphi011/wt/internal/forge/forgetest"
)

func TestPRLookupIsolation(t *testing.T) {
	t.Parallel()
	f := forgetest.New()
	ctx := context.Background()
	f.SetPR("repo-a", "topic", forge.PRInfo{Number: 1, State: forge.PRStateOpen})
	f.SetPR("repo-b", "topic", forge.PRInfo{Number: 2, State: forge.PRStateOpen})
	for _, tc := range []struct {
		repo, branch string
		number       int
	}{{"repo-a", "topic", 1}, {"repo-b", "topic", 2}, {"repo-a", "missing", 0}} {
		pr, err := f.GetPRForBranch(ctx, tc.repo, tc.branch)
		if err != nil || pr.Number != tc.number || !pr.Fetched {
			t.Fatalf("lookup %s:%s = %+v, %v", tc.repo, tc.branch, pr, err)
		}
		pr.State = forge.PRStateClosed
	}
	pr, err := f.GetPRForBranch(ctx, "repo-a", "topic")
	if err != nil || pr.State != forge.PRStateOpen {
		t.Fatalf("lookup mutated stored PR: %+v, %v", pr, err)
	}
	branch, err := f.GetPRBranch(ctx, "repo-b", 2)
	if err != nil || branch != "topic" {
		t.Fatalf("branch lookup = %q, %v", branch, err)
	}
}

func TestCreateAndMerge(t *testing.T) {
	t.Parallel()
	f := forgetest.New()
	ctx := context.Background()
	params := forge.CreatePRParams{Head: "topic", Base: "main", Title: "Change", Draft: true}
	result, err := f.CreatePR(ctx, "repo", params)
	if err != nil {
		t.Fatal(err)
	}
	prs, err := f.ListOpenPRs(ctx, "repo")
	if err != nil || len(prs) != 1 || prs[0].Title != params.Title || !prs[0].IsDraft {
		t.Fatalf("open PRs = %+v, %v", prs, err)
	}
	if err := f.MergePR(ctx, "repo", result.Number, "squash"); err != nil {
		t.Fatal(err)
	}
	pr, err := f.GetPRForBranch(ctx, "repo", "topic")
	if err != nil || pr.State != forge.PRStateMerged {
		t.Fatalf("merged PR = %+v, %v", pr, err)
	}
	if creates, merges := f.Creates(), f.Merges(); len(creates) != 1 || creates[0].Params != params || len(merges) != 1 || merges[0].Number != result.Number || merges[0].Strategy != "squash" {
		t.Fatalf("calls = %+v, %+v", creates, merges)
	}
	prs, err = f.ListOpenPRs(ctx, "repo")
	if err != nil || len(prs) != 0 {
		t.Fatalf("merged PR remains open: %+v, %v", prs, err)
	}
	if err := f.MergePR(ctx, "other-repo", result.Number, "squash"); err == nil {
		t.Fatal("merged unknown PR")
	}
}

func TestCancelledOperations(t *testing.T) {
	t.Parallel()
	f := forgetest.New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.CreatePR(ctx, "repo", forge.CreatePRParams{Head: "topic"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("create: %v", err)
	}
	if err := f.MergePR(ctx, "repo", 1, "squash"); !errors.Is(err, context.Canceled) {
		t.Fatalf("merge: %v", err)
	}
	if _, err := f.GetPRForBranch(ctx, "repo", "topic"); !errors.Is(err, context.Canceled) {
		t.Fatalf("lookup: %v", err)
	}
	if len(f.Creates()) != 0 || len(f.Merges()) != 0 {
		t.Fatal("cancelled operations recorded")
	}
}

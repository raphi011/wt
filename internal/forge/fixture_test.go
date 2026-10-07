package forge

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/log"
)

// The fixtures in testdata/ are unmodified stdout of the gh/glab commands the
// forge implementations run, recorded against cli/cli and gitlab-org/cli.
// These tests replace the CLI on PATH, so they cannot run in parallel.

// fakeCLI puts the fake gh and glab from testdata/bin first on PATH. They
// print stdout, exit with exitCode, and record their arguments (one per line)
// in the returned file.
func fakeCLI(t *testing.T, stdout string, exitCode int) string {
	t.Helper()

	binDir, err := filepath.Abs(filepath.Join("testdata", "bin"))
	if err != nil {
		t.Fatalf("failed to resolve fake CLI dir: %v", err)
	}

	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	stdoutFile := filepath.Join(dir, "stdout")
	if err := os.WriteFile(stdoutFile, []byte(stdout), 0o644); err != nil {
		t.Fatalf("failed to write fake CLI stdout: %v", err)
	}
	t.Setenv("WT_FAKE_ARGS", argsFile)
	t.Setenv("WT_FAKE_STDOUT", stdoutFile)
	t.Setenv("WT_FAKE_EXIT", strconv.Itoa(exitCode))
	fakeAPI(t, "{}\n", 0)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return argsFile
}

// fakeAPI sets what the fake CLI prints and exits with for "api" calls.
// Call it after fakeCLI.
func fakeAPI(t *testing.T, stdout string, exitCode int) {
	t.Helper()

	stdoutFile := filepath.Join(t.TempDir(), "api-stdout")
	if err := os.WriteFile(stdoutFile, []byte(stdout), 0o644); err != nil {
		t.Fatalf("failed to write fake CLI api stdout: %v", err)
	}
	t.Setenv("WT_FAKE_API_STDOUT", stdoutFile)
	t.Setenv("WT_FAKE_API_EXIT", strconv.Itoa(exitCode))
}

// fixture returns the content of a recorded CLI output in testdata/.
func fixture(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	return string(content)
}

// recordedArgs returns the arguments the fake CLI was called with.
func recordedArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	content, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("failed to read recorded args: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}

// forgeFixtures describes one forge implementation and its recorded output.
type forgeFixtures struct {
	cli     string
	forge   Forge
	repoURL string

	listBranch string
	view       string
	viewFork   string
	listOpen   string

	wantPR     PRInfo
	wantBranch string
	wantOpen   []OpenPR
}

var fixtureForges = []forgeFixtures{
	{
		cli:        "gh",
		forge:      &GitHub{},
		repoURL:    "https://github.com/cli/cli",
		listBranch: "github/pr_list_branch.json",
		view:       "github/pr_view.json",
		viewFork:   "github/pr_view_fork.json",
		listOpen:   "github/pr_list_open.json",
		wantPR: PRInfo{
			Number:       14542,
			State:        PRStateMerged,
			URL:          "https://github.com/cli/cli/pull/14542",
			Author:       "app/dependabot",
			CommentCount: 1,
			HasReviews:   true,
			IsApproved:   true,
			Fetched:      true,
		},
		wantBranch: "dependabot/go_modules/github.com/klauspost/compress-1.20.1",
		wantOpen: []OpenPR{
			{Number: 14617, Title: "fix(auth): fail fast when --with-token is used with terminal stdin", Author: "bingtang9", Branch: "fix/auth-login-with-token-tty-stdin"},
			{Number: 14616, Title: "Clarify issue view comment output modes", Author: "app/copilot-swe-agent", Branch: "copilot/gh-skill-document-issue-view", IsDraft: true},
			{Number: 14583, Title: "Add ACCESSIBILITY.md", Author: "BagToad", Branch: "bagtoad/add-accessibility-md"},
		},
	},
	{
		cli:        "glab",
		forge:      &GitLab{},
		repoURL:    "https://gitlab.com/gitlab-org/cli",
		listBranch: "gitlab/mr_list_branch.json",
		view:       "gitlab/mr_view.json",
		viewFork:   "gitlab/mr_view_fork.json",
		listOpen:   "gitlab/mr_list_open.json",
		wantPR: PRInfo{
			Number:       4015,
			State:        PRStateOpen,
			URL:          "https://gitlab.com/gitlab-org/cli/-/merge_requests/4015",
			Author:       "unstable-code",
			CommentCount: 9,
			Fetched:      true,
		},
		wantBranch: "jmc-remove-glab-mcp",
		wantOpen: []OpenPR{
			{Number: 4015, Title: "fix(iteration): show the real total in list header", Author: "unstable-code", Branch: "unstable-code/fix-iteration-list-total"},
			{Number: 4014, Title: "Draft: feat(mcp): remove the experimental glab mcp command", Author: "jay_mccure", Branch: "jmc-remove-glab-mcp", IsDraft: true},
			{Number: 4013, Title: "Draft: govern: add fallback periodic sync via launchd/systemd", Author: "jeanvdw", Branch: "632450-fallback-cron", IsDraft: true},
		},
	},
}

func TestForge_GetPRForBranch_Fixture(t *testing.T) {
	for _, f := range fixtureForges {
		t.Run(f.cli, func(t *testing.T) {
			fakeCLI(t, fixture(t, f.listBranch), 0)

			got, err := f.forge.GetPRForBranch(context.Background(), f.repoURL, "some-branch")
			if err != nil {
				t.Fatalf("GetPRForBranch failed: %v", err)
			}
			if got.CachedAt.IsZero() {
				t.Error("CachedAt should be set")
			}
			got.CachedAt = f.wantPR.CachedAt
			if *got != f.wantPR {
				t.Errorf("GetPRForBranch =\n%+v, want\n%+v", *got, f.wantPR)
			}
		})
	}
}

func TestForge_GetPRForBranch_NoPR(t *testing.T) {
	for _, f := range fixtureForges {
		t.Run(f.cli, func(t *testing.T) {
			fakeCLI(t, "[]\n", 0)

			got, err := f.forge.GetPRForBranch(context.Background(), f.repoURL, "some-branch")
			if err != nil {
				t.Fatalf("GetPRForBranch failed: %v", err)
			}
			if !got.Fetched || got.Number != 0 || got.State != "" {
				t.Errorf("GetPRForBranch = %+v, want a fetched marker without a PR", *got)
			}
		})
	}
}

func TestForge_GetPRForBranch_Errors(t *testing.T) {
	for _, f := range fixtureForges {
		t.Run(f.cli+"/malformed output", func(t *testing.T) {
			fakeCLI(t, "not json", 0)

			_, err := f.forge.GetPRForBranch(context.Background(), f.repoURL, "some-branch")
			if err == nil || !strings.Contains(err.Error(), "failed to parse "+f.cli+" output") {
				t.Errorf("GetPRForBranch error = %v, want a parse error", err)
			}
		})
		t.Run(f.cli+"/command fails", func(t *testing.T) {
			fakeCLI(t, "", 1)

			_, err := f.forge.GetPRForBranch(context.Background(), f.repoURL, "some-branch")
			if err == nil || !strings.Contains(err.Error(), f.cli+" command failed") {
				t.Errorf("GetPRForBranch error = %v, want a command error", err)
			}
		})
	}
}

func TestForge_GetPRBranch_Fixture(t *testing.T) {
	for _, f := range fixtureForges {
		t.Run(f.cli, func(t *testing.T) {
			fakeCLI(t, fixture(t, f.view), 0)

			got, err := f.forge.GetPRBranch(context.Background(), f.repoURL, 1)
			if err != nil {
				t.Fatalf("GetPRBranch failed: %v", err)
			}
			if got != f.wantBranch {
				t.Errorf("GetPRBranch = %q, want %q", got, f.wantBranch)
			}
		})
		t.Run(f.cli+"/fork", func(t *testing.T) {
			fakeCLI(t, fixture(t, f.viewFork), 0)

			_, err := f.forge.GetPRBranch(context.Background(), f.repoURL, 1)
			if err == nil || !strings.Contains(err.Error(), "is from a fork") {
				t.Errorf("GetPRBranch error = %v, want a fork error", err)
			}
		})
		t.Run(f.cli+"/no branch", func(t *testing.T) {
			fakeCLI(t, "{}\n", 0)

			_, err := f.forge.GetPRBranch(context.Background(), f.repoURL, 1)
			if err == nil || !strings.Contains(err.Error(), "has no") {
				t.Errorf("GetPRBranch error = %v, want a missing branch error", err)
			}
		})
		t.Run(f.cli+"/malformed output", func(t *testing.T) {
			fakeCLI(t, "not json", 0)

			_, err := f.forge.GetPRBranch(context.Background(), f.repoURL, 1)
			if err == nil || !strings.Contains(err.Error(), "failed to parse "+f.cli+" output") {
				t.Errorf("GetPRBranch error = %v, want a parse error", err)
			}
		})
		t.Run(f.cli+"/command fails", func(t *testing.T) {
			fakeCLI(t, "", 1)

			_, err := f.forge.GetPRBranch(context.Background(), f.repoURL, 1)
			if err == nil || !strings.Contains(err.Error(), f.cli+" command failed") {
				t.Errorf("GetPRBranch error = %v, want a command error", err)
			}
		})
	}
}

func TestForge_ListOpenPRs_Fixture(t *testing.T) {
	for _, f := range fixtureForges {
		t.Run(f.cli, func(t *testing.T) {
			fakeCLI(t, fixture(t, f.listOpen), 0)

			got, err := f.forge.ListOpenPRs(context.Background(), f.repoURL)
			if err != nil {
				t.Fatalf("ListOpenPRs failed: %v", err)
			}
			if !slices.Equal(got, f.wantOpen) {
				t.Errorf("ListOpenPRs =\n%+v, want\n%+v", got, f.wantOpen)
			}
		})
		t.Run(f.cli+"/malformed output", func(t *testing.T) {
			fakeCLI(t, "not json", 0)

			_, err := f.forge.ListOpenPRs(context.Background(), f.repoURL)
			if err == nil || !strings.Contains(err.Error(), "failed to parse "+f.cli+" output") {
				t.Errorf("ListOpenPRs error = %v, want a parse error", err)
			}
		})
	}
}

func TestForge_CreatePR_Fixture(t *testing.T) {
	tests := []struct {
		cli      string
		forge    Forge
		repoURL  string
		stdout   string
		wantURL  string
		wantArgs []string
	}{
		{
			cli:     "gh",
			forge:   &GitHub{},
			repoURL: "https://github.com/org/repo",
			stdout:  "https://github.com/org/repo/pull/12\n",
			wantURL: "https://github.com/org/repo/pull/12",
			wantArgs: []string{"pr", "create", "-R", "org/repo", "--title", "Title", "--body", "Body",
				"--base", "main", "--head", "feature", "--draft"},
		},
		{
			cli:     "glab",
			forge:   &GitLab{},
			repoURL: "https://gitlab.com/org/repo",
			stdout:  "\nCreating draft merge request for feature into main in org/repo\n\nhttps://gitlab.com/org/repo/-/merge_requests/12\n",
			wantURL: "https://gitlab.com/org/repo/-/merge_requests/12",
			wantArgs: []string{"mr", "create", "-R", "org/repo", "--title", "Title", "--description", "Body", "--yes",
				"--target-branch", "main", "--source-branch", "feature", "--draft"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.cli, func(t *testing.T) {
			argsFile := fakeCLI(t, tt.stdout, 0)

			got, err := tt.forge.CreatePR(context.Background(), tt.repoURL, CreatePRParams{
				Title: "Title", Body: "Body", Base: "main", Head: "feature", Draft: true,
			})
			if err != nil {
				t.Fatalf("CreatePR failed: %v", err)
			}
			if got.Number != 12 || got.URL != tt.wantURL {
				t.Errorf("CreatePR = %+v, want #12 %s", *got, tt.wantURL)
			}
			if args := recordedArgs(t, argsFile); !slices.Equal(args, tt.wantArgs) {
				t.Errorf("%s args =\n%q, want\n%q", tt.cli, args, tt.wantArgs)
			}
		})
		t.Run(tt.cli+"/command fails", func(t *testing.T) {
			fakeCLI(t, "", 1)

			_, err := tt.forge.CreatePR(context.Background(), tt.repoURL, CreatePRParams{Title: "Title"})
			if err == nil || !strings.Contains(err.Error(), "create failed") {
				t.Errorf("CreatePR error = %v, want a command error", err)
			}
		})
	}
}

func TestForge_MergePR_Args(t *testing.T) {
	tests := []struct {
		cli      string
		forge    Forge
		repoURL  string
		strategy string
		wantArgs []string
	}{
		{"gh", &GitHub{}, "https://github.com/org/repo", "squash", []string{"pr", "merge", "7", "-R", "org/repo", "--squash", "--delete-branch"}},
		{"gh", &GitHub{}, "https://github.com/org/repo", "rebase", []string{"pr", "merge", "7", "-R", "org/repo", "--rebase", "--delete-branch"}},
		{"gh", &GitHub{}, "https://github.com/org/repo", "merge", []string{"pr", "merge", "7", "-R", "org/repo", "--merge", "--delete-branch"}},
		{"glab", &GitLab{}, "https://gitlab.com/org/repo", "squash", []string{"mr", "merge", "7", "-R", "org/repo", "--remove-source-branch", "--squash"}},
		{"glab", &GitLab{}, "https://gitlab.com/org/repo", "merge", []string{"mr", "merge", "7", "-R", "org/repo", "--remove-source-branch"}},
	}

	for _, tt := range tests {
		t.Run(tt.cli+"/"+tt.strategy, func(t *testing.T) {
			argsFile := fakeCLI(t, "", 0)

			if err := tt.forge.MergePR(context.Background(), tt.repoURL, 7, tt.strategy); err != nil {
				t.Fatalf("MergePR failed: %v", err)
			}
			if args := recordedArgs(t, argsFile); !slices.Equal(args, tt.wantArgs) {
				t.Errorf("%s args =\n%q, want\n%q", tt.cli, args, tt.wantArgs)
			}
		})
		t.Run(tt.cli+"/"+tt.strategy+"/command fails", func(t *testing.T) {
			fakeCLI(t, "", 1)

			err := tt.forge.MergePR(context.Background(), tt.repoURL, 7, tt.strategy)
			if err == nil || !strings.Contains(err.Error(), "merge failed") {
				t.Errorf("MergePR error = %v, want a merge error", err)
			}
		})
	}
}

func TestGitLab_GetPRForBranch_ReviewStatus(t *testing.T) {
	tests := []struct {
		name           string
		approvals      string
		wantHasReviews bool
		wantIsApproved bool
	}{
		{"approved", "gitlab/mr_approvals_approved.json", true, true},
		{"approvals outstanding", "gitlab/mr_approvals_partial.json", true, false},
		// GitLab reports approved=true when no approval is required
		{"nobody approved, none required", "gitlab/mr_approvals_none_required.json", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsFile := fakeCLI(t, fixture(t, "gitlab/mr_list_branch.json"), 0)
			fakeAPI(t, fixture(t, tt.approvals), 0)

			got, err := (&GitLab{}).GetPRForBranch(context.Background(), "https://gitlab.com/gitlab-org/cli", "some-branch")
			if err != nil {
				t.Fatalf("GetPRForBranch failed: %v", err)
			}
			if got.HasReviews != tt.wantHasReviews || got.IsApproved != tt.wantIsApproved {
				t.Errorf("HasReviews, IsApproved = %v, %v, want %v, %v", got.HasReviews, got.IsApproved, tt.wantHasReviews, tt.wantIsApproved)
			}

			wantArgs := []string{"api", "projects/gitlab-org%2Fcli/merge_requests/4015/approvals"}
			if args := recordedArgs(t, argsFile+".api"); !slices.Equal(args, wantArgs) {
				t.Errorf("glab args = %q, want %q", args, wantArgs)
			}
		})
	}
}

func TestGitLab_GetPRForBranch_ApprovalsUnavailable(t *testing.T) {
	fakeCLI(t, fixture(t, "gitlab/mr_list_branch.json"), 0)
	fakeAPI(t, "", 1)

	var logs strings.Builder
	ctx := log.WithLogger(context.Background(), log.New(&logs, false, false))

	got, err := (&GitLab{}).GetPRForBranch(ctx, "https://gitlab.com/gitlab-org/cli", "some-branch")
	if err != nil {
		t.Fatalf("GetPRForBranch failed: %v", err)
	}
	if got.Number != 4015 || got.HasReviews || got.IsApproved {
		t.Errorf("GetPRForBranch = %+v, want MR 4015 without review status", *got)
	}
	if !strings.Contains(logs.String(), "review status") {
		t.Errorf("a warning about the review status should be logged, got: %q", logs.String())
	}
}

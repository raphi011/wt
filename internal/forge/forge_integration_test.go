//go:build integration

package forge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

var (
	testRepo      string // e.g. "raphi011/wt-test"
	testClonePath string // shared clone directory for write tests
)

func TestMain(m *testing.M) {
	testRepo = os.Getenv("WT_TEST_GITHUB_REPO")
	if testRepo == "" {
		os.Exit(0) // skip all tests
	}

	// Clone once to temp dir for write tests
	tmpDir, err := os.MkdirTemp("", "forge-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	gh := &GitHub{}
	testClonePath, err = gh.CloneRepo(testRepo, tmpDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to clone test repo: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(tmpDir) // cleanup
	os.Exit(code)
}

func skipIfNoGitHub(t *testing.T) {
	if testRepo == "" {
		t.Skip("WT_TEST_GITHUB_REPO not set")
	}
}

// closeGitHubPR closes a PR using gh CLI
func closeGitHubPR(t *testing.T, repo string, number int) {
	t.Helper()
	c := exec.Command("gh", "pr", "close", fmt.Sprintf("%d", number), "-R", repo)
	if err := c.Run(); err != nil {
		t.Logf("warning: failed to close PR #%d: %v", number, err)
	}
}

// deleteRemoteBranch deletes a remote branch
func deleteRemoteBranch(t *testing.T, clonePath, branch string) {
	t.Helper()
	c := exec.Command("git", "-C", clonePath, "push", "origin", "--delete", branch)
	if err := c.Run(); err != nil {
		t.Logf("warning: failed to delete remote branch %s: %v", branch, err)
	}
}

// Read-only tests - can run in parallel

func TestGitHub_Check(t *testing.T) {
	skipIfNoGitHub(t)
	t.Parallel()

	gh := &GitHub{}
	err := gh.Check()
	if err != nil {
		t.Errorf("Check() error = %v, want nil", err)
	}
}

func TestGitHub_GetPRForBranch_Main(t *testing.T) {
	skipIfNoGitHub(t)
	t.Parallel()

	gh := &GitHub{}
	repoURL := "https://github.com/" + testRepo

	pr, err := gh.GetPRForBranch(repoURL, "main")
	if err != nil {
		t.Fatalf("GetPRForBranch() error = %v", err)
	}
	if !pr.Fetched {
		t.Error("GetPRForBranch() pr.Fetched = false, want true")
	}
	// main branch typically has no open PR, so Number should be 0
	// but we just verify the call succeeded
}

func TestGitHub_GetPRForBranch_NonExistent(t *testing.T) {
	skipIfNoGitHub(t)
	t.Parallel()

	gh := &GitHub{}
	repoURL := "https://github.com/" + testRepo

	pr, err := gh.GetPRForBranch(repoURL, "nonexistent-branch-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("GetPRForBranch() error = %v", err)
	}
	if !pr.Fetched {
		t.Error("GetPRForBranch() pr.Fetched = false, want true")
	}
	if pr.Number != 0 {
		t.Errorf("GetPRForBranch() pr.Number = %d, want 0 (no PR)", pr.Number)
	}
}

func TestGitHub_CloneRepo(t *testing.T) {
	skipIfNoGitHub(t)
	t.Parallel()

	gh := &GitHub{}
	tmpDir := t.TempDir()

	clonePath, err := gh.CloneRepo(testRepo, tmpDir)
	if err != nil {
		t.Fatalf("CloneRepo() error = %v", err)
	}

	// Verify .git exists
	gitDir := filepath.Join(clonePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Errorf("CloneRepo() .git dir not found at %s", gitDir)
	}
}

func TestGitHub_CloneRepo_InvalidSpec(t *testing.T) {
	skipIfNoGitHub(t)
	t.Parallel()

	gh := &GitHub{}
	tmpDir := t.TempDir()

	_, err := gh.CloneRepo("invalid-spec-no-slash", tmpDir)
	if err == nil {
		t.Error("CloneRepo() with invalid spec should return error")
	}

	_, err = gh.CloneRepo("/repo", tmpDir)
	if err == nil {
		t.Error("CloneRepo() with empty org should return error")
	}

	_, err = gh.CloneRepo("org/", tmpDir)
	if err == nil {
		t.Error("CloneRepo() with empty repo should return error")
	}
}

// Write tests - combined into single test to ensure sequential execution
// and proper cleanup (t.Cleanup runs after ALL subtests complete)

func TestGitHub_PRWorkflow(t *testing.T) {
	skipIfNoGitHub(t)

	gh := &GitHub{}
	repoURL := "https://github.com/" + testRepo

	// Create unique branch name
	testBranch := fmt.Sprintf("test-wt-%d", time.Now().UnixNano())
	var prNumber int

	// Cleanup runs after all subtests complete
	t.Cleanup(func() {
		if prNumber > 0 {
			closeGitHubPR(t, testRepo, prNumber)
		}
		deleteRemoteBranch(t, testClonePath, testBranch)
	})

	// Setup: create branch and push
	t.Run("Setup", func(t *testing.T) {
		// Fetch latest main and create branch from it
		c := exec.Command("git", "-C", testClonePath, "fetch", "origin", "main")
		if err := c.Run(); err != nil {
			t.Fatalf("git fetch failed: %v", err)
		}
		c = exec.Command("git", "-C", testClonePath, "checkout", "-B", testBranch, "origin/main")
		if err := c.Run(); err != nil {
			t.Fatalf("git checkout -B failed: %v", err)
		}

		// Create empty commit
		c = exec.Command("git", "-C", testClonePath, "commit", "--allow-empty", "-m", "Test commit for integration test")
		if err := c.Run(); err != nil {
			t.Fatalf("git commit failed: %v", err)
		}

		// Push branch
		c = exec.Command("git", "-C", testClonePath, "push", "-u", "origin", testBranch)
		if err := c.Run(); err != nil {
			t.Fatalf("git push failed: %v", err)
		}
	})

	t.Run("CreatePR", func(t *testing.T) {
		// Change to clone directory (gh pr create needs to run from within repo)
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get current directory: %v", err)
		}
		if err := os.Chdir(testClonePath); err != nil {
			t.Fatalf("failed to chdir to clone: %v", err)
		}
		defer os.Chdir(origDir)

		// Create PR (not draft so it can be merged)
		result, err := gh.CreatePR(repoURL, CreatePRParams{
			Title: "Test PR - " + testBranch,
			Body:  "Automated integration test PR. Will be merged automatically.",
		})
		if err != nil {
			t.Fatalf("CreatePR() error = %v", err)
		}
		if result.Number <= 0 {
			t.Errorf("CreatePR() number = %d, want > 0", result.Number)
		}
		if result.URL == "" {
			t.Error("CreatePR() URL is empty")
		}

		prNumber = result.Number
		t.Logf("Created PR #%d: %s", result.Number, result.URL)
	})

	t.Run("ViewPR", func(t *testing.T) {
		if prNumber == 0 {
			t.Skip("No PR created")
		}

		// View PR without opening browser
		err := gh.ViewPR(repoURL, prNumber, false)
		if err != nil {
			t.Errorf("ViewPR() error = %v", err)
		}
	})

	t.Run("GetPRBranch", func(t *testing.T) {
		if prNumber == 0 {
			t.Skip("No PR created")
		}

		branch, err := gh.GetPRBranch(repoURL, prNumber)
		if err != nil {
			t.Fatalf("GetPRBranch() error = %v", err)
		}
		if branch != testBranch {
			t.Errorf("GetPRBranch() = %q, want %q", branch, testBranch)
		}
	})

	t.Run("MergePR", func(t *testing.T) {
		if prNumber == 0 {
			t.Skip("No PR created")
		}

		// Merge the PR with squash strategy
		err := gh.MergePR(repoURL, prNumber, "squash")
		if err != nil {
			t.Errorf("MergePR() error = %v", err)
		}

		// Clear prNumber since it's now merged (no need to close in cleanup)
		prNumber = 0
	})
}

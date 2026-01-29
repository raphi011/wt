//go:build integration

package forge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// forgeTestConfig holds configuration for testing a specific forge.
type forgeTestConfig struct {
	name      string // "github" or "gitlab"
	forge     Forge  // the forge instance
	repoSpec  string // e.g. "raphi011/wt-test"
	repoURL   string // e.g. "https://github.com/raphi011/wt-test"
	clonePath string // path to cloned repo for write tests
}

var testForges []forgeTestConfig

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "forge-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	// Check GitHub
	if repo := os.Getenv("WT_TEST_GITHUB_REPO"); repo != "" {
		gh := &GitHub{}
		ghDir := filepath.Join(tmpDir, "github")
		if err := os.MkdirAll(ghDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create github dir: %v\n", err)
			os.Exit(1)
		}
		clonePath, err := gh.CloneRepo(ctx, repo, ghDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to clone GitHub test repo: %v\n", err)
			os.Exit(1)
		}
		testForges = append(testForges, forgeTestConfig{
			name:      "github",
			forge:     gh,
			repoSpec:  repo,
			repoURL:   "https://github.com/" + repo,
			clonePath: clonePath,
		})
	}

	// Check GitLab
	if repo := os.Getenv("WT_TEST_GITLAB_REPO"); repo != "" {
		gl := &GitLab{}
		glDir := filepath.Join(tmpDir, "gitlab")
		if err := os.MkdirAll(glDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create gitlab dir: %v\n", err)
			os.Exit(1)
		}
		clonePath, err := gl.CloneRepo(ctx, repo, glDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to clone GitLab test repo: %v\n", err)
			os.Exit(1)
		}
		testForges = append(testForges, forgeTestConfig{
			name:      "gitlab",
			forge:     gl,
			repoSpec:  repo,
			repoURL:   "https://gitlab.com/" + repo,
			clonePath: clonePath,
		})
	}

	// Skip all tests if no forge configured
	if len(testForges) == 0 {
		os.Exit(0)
	}

	os.Exit(m.Run())
}

// closePR closes a PR/MR using the appropriate CLI.
func closePR(t *testing.T, fc *forgeTestConfig, number int) {
	t.Helper()
	var c *exec.Cmd
	switch fc.name {
	case "github":
		c = exec.Command("gh", "pr", "close", fmt.Sprintf("%d", number), "-R", fc.repoSpec)
	case "gitlab":
		c = exec.Command("glab", "mr", "close", fmt.Sprintf("%d", number), "-R", fc.repoSpec)
	default:
		t.Logf("warning: unknown forge %s, cannot close PR", fc.name)
		return
	}
	if err := c.Run(); err != nil {
		t.Logf("warning: failed to close PR #%d: %v", number, err)
	}
}

// configureGitCredentials sets up git credential helper for the forge.
func configureGitCredentials(t *testing.T, clonePath, forgeName string) {
	t.Helper()
	var credentialHelper string
	switch forgeName {
	case "github":
		credentialHelper = "!gh auth git-credential"
	case "gitlab":
		credentialHelper = "!glab auth git-credential"
	default:
		t.Fatalf("unknown forge: %s", forgeName)
	}

	for _, cfg := range [][]string{
		{"user.email", "test@example.com"},
		{"user.name", "Test User"},
		{"credential.helper", ""},            // Clear any existing helper
		{"credential.helper", credentialHelper},
	} {
		c := exec.Command("git", "-C", clonePath, "config", cfg[0], cfg[1])
		if err := c.Run(); err != nil {
			t.Fatalf("git config %s failed: %v", cfg[0], err)
		}
	}
}

// deleteRemoteBranch deletes a remote branch.
func deleteRemoteBranch(t *testing.T, clonePath, branch string) {
	t.Helper()
	c := exec.Command("git", "-C", clonePath, "push", "origin", "--delete", branch)
	if err := c.Run(); err != nil {
		t.Logf("warning: failed to delete remote branch %s: %v", branch, err)
	}
}

// Read-only tests - can run in parallel

// TestForge_Check verifies that forge CLI is properly configured
// and authenticated.
//
// Scenario: User has forge CLI installed and authenticated
// Expected: Check() returns nil (no error)
func TestForge_Check(t *testing.T) {
	for _, fc := range testForges {
		t.Run(fc.name, func(t *testing.T) {
			t.Parallel()
			err := fc.forge.Check(context.Background())
			if err != nil {
				t.Errorf("Check() error = %v, want nil", err)
			}
		})
	}
}

// TestForge_GetPRForBranch_Main verifies fetching PR info for the main branch.
//
// Scenario: User checks PR status for main branch (typically no open PR)
// Expected: GetPRForBranch() succeeds with Fetched=true
func TestForge_GetPRForBranch_Main(t *testing.T) {
	for _, fc := range testForges {
		t.Run(fc.name, func(t *testing.T) {
			t.Parallel()
			pr, err := fc.forge.GetPRForBranch(context.Background(), fc.repoURL, "main")
			if err != nil {
				t.Fatalf("GetPRForBranch() error = %v", err)
			}
			if !pr.Fetched {
				t.Error("GetPRForBranch() pr.Fetched = false, want true")
			}
			// main branch typically has no open PR, so Number should be 0
			// but we just verify the call succeeded
		})
	}
}

// TestForge_GetPRForBranch_NonExistent verifies fetching PR info for
// a branch that doesn't exist.
//
// Scenario: User checks PR status for non-existent branch
// Expected: GetPRForBranch() succeeds with Fetched=true and Number=0
func TestForge_GetPRForBranch_NonExistent(t *testing.T) {
	for _, fc := range testForges {
		t.Run(fc.name, func(t *testing.T) {
			t.Parallel()
			pr, err := fc.forge.GetPRForBranch(context.Background(), fc.repoURL, "nonexistent-branch-"+fmt.Sprintf("%d", time.Now().UnixNano()))
			if err != nil {
				t.Fatalf("GetPRForBranch() error = %v", err)
			}
			if !pr.Fetched {
				t.Error("GetPRForBranch() pr.Fetched = false, want true")
			}
			if pr.Number != 0 {
				t.Errorf("GetPRForBranch() pr.Number = %d, want 0 (no PR)", pr.Number)
			}
		})
	}
}

// TestForge_CloneRepo verifies cloning a repository via forge CLI.
//
// Scenario: User clones a repo to temp directory
// Expected: Repo cloned with .git directory created
func TestForge_CloneRepo(t *testing.T) {
	for _, fc := range testForges {
		t.Run(fc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()

			clonePath, err := fc.forge.CloneRepo(context.Background(), fc.repoSpec, tmpDir)
			if err != nil {
				t.Fatalf("CloneRepo() error = %v", err)
			}

			// Verify .git exists
			gitDir := filepath.Join(clonePath, ".git")
			if _, err := os.Stat(gitDir); os.IsNotExist(err) {
				t.Errorf("CloneRepo() .git dir not found at %s", gitDir)
			}
		})
	}
}

// TestForge_CloneRepo_InvalidSpec verifies error handling for invalid repo specs.
//
// Scenario: User tries to clone with invalid spec (no slash, empty org/repo)
// Expected: CloneRepo() returns error for all invalid specs
func TestForge_CloneRepo_InvalidSpec(t *testing.T) {
	for _, fc := range testForges {
		t.Run(fc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			ctx := context.Background()

			_, err := fc.forge.CloneRepo(ctx, "invalid-spec-no-slash", tmpDir)
			if err == nil {
				t.Error("CloneRepo() with invalid spec should return error")
			}

			_, err = fc.forge.CloneRepo(ctx, "/repo", tmpDir)
			if err == nil {
				t.Error("CloneRepo() with empty org should return error")
			}

			_, err = fc.forge.CloneRepo(ctx, "org/", tmpDir)
			if err == nil {
				t.Error("CloneRepo() with empty repo should return error")
			}
		})
	}
}

// Write tests - combined into single test to ensure sequential execution
// and proper cleanup (t.Cleanup runs after ALL subtests complete)

// TestForge_PRWorkflow verifies the full PR lifecycle: create, view, get branch, merge.
// This test runs subtests sequentially to manage a single PR.
//
// Scenario: Create test branch, push, create PR, view PR, get PR branch, merge PR
// Expected: All operations succeed, PR is created and merged
func TestForge_PRWorkflow(t *testing.T) {
	for _, fc := range testForges {
		t.Run(fc.name, func(t *testing.T) {
			// No t.Parallel() - subtests must run sequentially
			ctx := context.Background()

			// Create unique branch name
			testBranch := fmt.Sprintf("test-wt-%d", time.Now().UnixNano())
			var prNumber int

			// Cleanup runs after all subtests complete
			t.Cleanup(func() {
				if prNumber > 0 {
					closePR(t, &fc, prNumber)
				}
				deleteRemoteBranch(t, fc.clonePath, testBranch)
			})

			// Setup: create branch and push
			t.Run("Setup", func(t *testing.T) {
				configureGitCredentials(t, fc.clonePath, fc.name)

				// Fetch latest main and create branch from it
				c := exec.Command("git", "-C", fc.clonePath, "fetch", "origin", "main")
				if err := c.Run(); err != nil {
					t.Fatalf("git fetch failed: %v", err)
				}
				c = exec.Command("git", "-C", fc.clonePath, "checkout", "-B", testBranch, "origin/main")
				if err := c.Run(); err != nil {
					t.Fatalf("git checkout -B failed: %v", err)
				}

				// Create empty commit
				c = exec.Command("git", "-C", fc.clonePath, "commit", "--allow-empty", "-m", "Test commit for integration test")
				if err := c.Run(); err != nil {
					t.Fatalf("git commit failed: %v", err)
				}

				// Push branch
				c = exec.Command("git", "-C", fc.clonePath, "push", "-u", "origin", testBranch)
				if err := c.Run(); err != nil {
					t.Fatalf("git push failed: %v", err)
				}
			})

			t.Run("CreatePR", func(t *testing.T) {
				// Change to clone directory (CLI tools may need to run from within repo)
				origDir, err := os.Getwd()
				if err != nil {
					t.Fatalf("failed to get current directory: %v", err)
				}
				if err := os.Chdir(fc.clonePath); err != nil {
					t.Fatalf("failed to chdir to clone: %v", err)
				}
				defer os.Chdir(origDir)

				// Create PR (not draft so it can be merged)
				result, err := fc.forge.CreatePR(ctx, fc.repoURL, CreatePRParams{
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
				err := fc.forge.ViewPR(ctx, fc.repoURL, prNumber, false)
				if err != nil {
					t.Errorf("ViewPR() error = %v", err)
				}
			})

			t.Run("GetPRBranch", func(t *testing.T) {
				if prNumber == 0 {
					t.Skip("No PR created")
				}

				branch, err := fc.forge.GetPRBranch(ctx, fc.repoURL, prNumber)
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

				// Merge the PR with squash strategy (supported by both GitHub and GitLab)
				err := fc.forge.MergePR(ctx, fc.repoURL, prNumber, "squash")
				if err != nil {
					t.Errorf("MergePR() error = %v", err)
				}

				// Clear prNumber since it's now merged (no need to close in cleanup)
				prNumber = 0
			})
		})
	}
}

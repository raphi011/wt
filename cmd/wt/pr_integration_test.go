//go:build integration

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/git"
	"github.com/raphi011/wt/internal/history"
	"github.com/raphi011/wt/internal/registry"
)

// TestPrCheckout_InvalidPRNumber tests error when first arg is not a valid PR number.
//
// Scenario: User runs `wt pr checkout notanumber`
// Expected: Returns error about invalid PR number
func TestPrCheckout_InvalidPRNumber(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"notanumber"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid PR number, got nil")
	}
	if !strings.Contains(err.Error(), "invalid PR number") {
		t.Errorf("expected error about invalid PR number, got %q", err.Error())
	}
}

// TestPrCheckout_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt pr checkout nonexistent 123`
// Expected: Returns error about repo not found in registry
func TestPrCheckout_RepoNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}

	// Work from a non-repo directory
	otherDir := filepath.Join(tmpDir, "other")
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, otherDir)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found in registry") {
		t.Errorf("expected error about repo not found in registry, got %q", err.Error())
	}
}

// TestPrCheckout_InvalidPRNumberWithRepo tests error when second arg is not a valid PR number.
//
// Scenario: User runs `wt pr checkout myrepo notanumber`
// Expected: Returns error about invalid PR number
func TestPrCheckout_InvalidPRNumberWithRepo(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	tmpDir = resolvePath(t, tmpDir)

	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"myrepo", "notanumber"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid PR number, got nil")
	}
	if !strings.Contains(err.Error(), "invalid PR number") {
		t.Errorf("expected error about invalid PR number, got %q", err.Error())
	}
}

// TestPrCreate_NotInGitRepo tests error when running pr create outside a git repo.
//
// Scenario: User runs `wt pr create` from a non-git directory with no repo arg
// Expected: Returns "not in a git repository" error
func TestPrCreate_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, nonGitDir)

	cmd := newPrCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--title", "test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestPrCreate_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt pr create nonexistent`
// Expected: Returns "not found" error
func TestPrCreate_RepoNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent", "--title", "test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %q", err.Error())
	}
}

// TestPrMerge_NotInGitRepo tests error when running pr merge outside a git repo.
//
// Scenario: User runs `wt pr merge` from a non-git directory with no repo arg
// Expected: Returns "not in a git repository" error
func TestPrMerge_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, nonGitDir)

	cmd := newPrMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestPrMerge_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt pr merge nonexistent`
// Expected: Returns "not found" error
func TestPrMerge_RepoNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %q", err.Error())
	}
}

// TestPrView_NotInGitRepo tests error when running pr view outside a git repo.
//
// Scenario: User runs `wt pr view` from a non-git directory with no repo arg
// Expected: Returns "not in a git repository" error
func TestPrView_NotInGitRepo(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	nonGitDir := filepath.Join(tmpDir, "not-a-repo")
	if err := os.MkdirAll(nonGitDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, nonGitDir)

	cmd := newPrViewCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("expected 'not in a git repository' error, got %q", err.Error())
	}
}

// TestPrView_RepoNotFound tests error when specified repo doesn't exist.
//
// Scenario: User runs `wt pr view nonexistent`
// Expected: Returns "not found" error
func TestPrView_RepoNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{Repos: []registry.Repo{}}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrViewCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got %q", err.Error())
	}
}

// TestPrCreate_BodyAndBodyFileMutuallyExclusive tests that --body and --body-file cannot both be used.
//
// Scenario: User runs `wt pr create --title test --body "text" --body-file file.txt`
// Expected: Returns cobra mutual exclusivity error
func TestPrCreate_BodyAndBodyFileMutuallyExclusive(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newPrCreateCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--title", "test", "--body", "inline body", "--body-file", "file.txt"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "body") || !strings.Contains(err.Error(), "body-file") {
		t.Errorf("expected error about body/body-file mutual exclusivity, got %q", err.Error())
	}
}

// TestPrCheckout_HookNoHookMutuallyExclusive tests that --hook and --no-hook cannot both be used.
//
// Scenario: User runs `wt pr checkout --hook myhook --no-hook 123`
// Expected: Returns cobra mutual exclusivity error
func TestPrCheckout_HookNoHookMutuallyExclusive(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--hook", "myhook", "--no-hook", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "hook") || !strings.Contains(err.Error(), "no-hook") {
		t.Errorf("expected error about hook/no-hook mutual exclusivity, got %q", err.Error())
	}
}

// TestPrMerge_HookNoHookMutuallyExclusive tests that --hook and --no-hook cannot both be used.
//
// Scenario: User runs `wt pr merge --hook myhook --no-hook`
// Expected: Returns cobra mutual exclusivity error
func TestPrMerge_HookNoHookMutuallyExclusive(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, repoPath)

	cmd := newPrMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--hook", "myhook", "--no-hook"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "hook") || !strings.Contains(err.Error(), "no-hook") {
		t.Errorf("expected error about hook/no-hook mutual exclusivity, got %q", err.Error())
	}
}

// TestPrCheckout_OrgRepoAlreadyInRegistry tests that org/repo format finds a
// registered repo by matching any of its remote URLs.
//
// Scenario: User runs `wt pr checkout test/myrepo 123` and "myrepo" is registered
//
//	with origin https://github.com/test/myrepo.git
//
// Expected: Finds the existing repo (proceeds past lookup, fails at forge/PR fetch)
func TestPrCheckout_OrgRepoAlreadyInRegistry(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	// Use test/myrepo to match the origin set by setupTestRepo
	cmd.SetArgs([]string{"test/myrepo", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (forge/PR fetch), got nil")
	}
	// Should find the repo and fail later at forge detection or PR fetch
	if strings.Contains(err.Error(), "not found in registry") {
		t.Errorf("should have found repo via remote URL, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "already exists") {
		t.Errorf("should not return 'already exists' error, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "no registered repo") {
		t.Errorf("should have found repo via remote URL, got %q", err.Error())
	}
	// Should reach forge detection (origin URL is a fake URL)
	if strings.Contains(err.Error(), "no registered repo") || strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error from forge detection stage, got lookup error: %q", err.Error())
	}
}

// TestPrCheckout_OrgRepoMatchesByRemote tests that org/repo format finds a
// registered repo even when the registry name differs from the repo slug.
//
// Scenario: Repo registered as "protectedaccounts" but origin is
//
//	https://github.com/n26/de.tech26.protectedaccounts.git
//
// Expected: `wt pr checkout n26/de.tech26.protectedaccounts 123` finds it
func TestPrCheckout_OrgRepoMatchesByRemote(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	// setupTestRepo sets origin to https://github.com/test/<name>.git
	// We override it to simulate a dotted repo name that differs from the registry name
	repoPath := setupTestRepo(t, tmpDir, "protectedaccounts")

	// Override origin to use a different org/repo path
	setRemote := exec.Command("git", "remote", "set-url", "origin", "https://github.com/n26/de.tech26.protectedaccounts.git")
	setRemote.Dir = repoPath
	if out, err := setRemote.CombinedOutput(); err != nil {
		t.Fatalf("failed to set origin: %v\n%s", err, out)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "protectedaccounts", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"n26/de.tech26.protectedaccounts", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (forge/PR fetch), got nil")
	}
	// Should find the repo via remote URL match and fail at forge detection
	if strings.Contains(err.Error(), "not found in registry") {
		t.Errorf("should have found repo via remote URL, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "no registered repo") {
		t.Errorf("should have found repo via remote URL, got %q", err.Error())
	}
}

// TestPrCheckout_OrgRepoCaseInsensitiveMatch tests that remote URL matching
// is case-insensitive.
//
// Scenario: Origin is https://github.com/N26/MyRepo.git, user types n26/myrepo
// Expected: Finds the repo despite case difference
func TestPrCheckout_OrgRepoCaseInsensitiveMatch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	// Set origin with mixed case
	setRemote := exec.Command("git", "remote", "set-url", "origin", "https://github.com/N26/MyRepo.git")
	setRemote.Dir = repoPath
	if out, err := setRemote.CombinedOutput(); err != nil {
		t.Fatalf("failed to set origin: %v\n%s", err, out)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	// Use lowercase to test case-insensitive matching
	cmd.SetArgs([]string{"n26/myrepo", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (forge/PR fetch), got nil")
	}
	// Should find the repo despite case mismatch
	if strings.Contains(err.Error(), "no registered repo") {
		t.Errorf("case-insensitive match should have found repo, got %q", err.Error())
	}
}

// TestPrCheckout_OrgRepoMultipleMatches tests that an error is returned when
// multiple registered repos have remotes matching the same org/repo.
//
// Scenario: Two repos both have origin pointing to test/shared-repo
// Expected: Error listing both repo names
func TestPrCheckout_OrgRepoMultipleMatches(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath1 := setupTestRepo(t, tmpDir, "repo1")
	repoPath2 := setupTestRepo(t, tmpDir, "repo2")

	// Set both repos to have the same origin
	for _, rp := range []string{repoPath1, repoPath2} {
		setRemote := exec.Command("git", "remote", "set-url", "origin", "https://github.com/test/shared-repo.git")
		setRemote.Dir = rp
		if out, err := setRemote.CombinedOutput(); err != nil {
			t.Fatalf("failed to set origin for %s: %v\n%s", rp, err, out)
		}
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "repo1", Path: repoPath1},
			{Name: "repo2", Path: repoPath2},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"test/shared-repo", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for multiple matches, got nil")
	}
	if !strings.Contains(err.Error(), "multiple registered repos match") {
		t.Errorf("expected 'multiple registered repos match' error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "repo1") || !strings.Contains(err.Error(), "repo2") {
		t.Errorf("expected error to list both repo names, got %q", err.Error())
	}
}

// TestPrCheckout_OrgRepoNoMatchWithoutCloneFlag tests that org/repo format
// fails with a helpful error when no registered repo matches and --clone is not set.
//
// Scenario: User runs `wt pr checkout unknown/repo 123` with no matching remote
// Expected: Error with suggestion to use --clone
func TestPrCheckout_OrgRepoNoMatchWithoutCloneFlag(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"unknown/repo", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unmatched org/repo without --clone, got nil")
	}
	if !strings.Contains(err.Error(), "no registered repo has a remote matching") {
		t.Errorf("expected 'no registered repo' error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "--clone") {
		t.Errorf("expected suggestion to use --clone, got %q", err.Error())
	}
}

// TestPrCheckout_OrgRepoWithCloneFlag tests that --clone allows cloning
// when no registered repo matches.
//
// Scenario: User runs `wt pr checkout --clone unknown/repo 123`
// Expected: Attempts to clone (fails at forge detection, not at "no registered repo")
func TestPrCheckout_OrgRepoWithCloneFlag(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--clone", "unknown/repo", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (forge detection), got nil")
	}
	// Should NOT get "no registered repo" — it should proceed to clone
	if strings.Contains(err.Error(), "no registered repo") {
		t.Errorf("--clone should bypass 'no registered repo' error, got %q", err.Error())
	}
}

// TestPrCheckout_OrgRepoMatchesByUpstreamRemote tests that org/repo format
// matches against non-origin remotes (e.g. upstream).
//
// Scenario: Repo has origin pointing elsewhere, but upstream matches org/repo
// Expected: Finds the repo via the upstream remote
func TestPrCheckout_OrgRepoMatchesByUpstreamRemote(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	// Origin points to something else; add an upstream that matches
	setOrigin := exec.Command("git", "remote", "set-url", "origin", "https://github.com/other/unrelated.git")
	setOrigin.Dir = repoPath
	if out, err := setOrigin.CombinedOutput(); err != nil {
		t.Fatalf("failed to set origin: %v\n%s", err, out)
	}
	addUpstream := exec.Command("git", "remote", "add", "upstream", "https://github.com/test/target-repo.git")
	addUpstream.Dir = repoPath
	if out, err := addUpstream.CombinedOutput(); err != nil {
		t.Fatalf("failed to add upstream: %v\n%s", err, out)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"test/target-repo", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (forge/PR fetch), got nil")
	}
	// Should find the repo via upstream remote, not fail at lookup
	if strings.Contains(err.Error(), "no registered repo") {
		t.Errorf("should have matched via upstream remote, got %q", err.Error())
	}
}

// TestPrCheckout_CloneFlagWithExistingMatch tests that --clone does not trigger
// cloning when a registered repo already matches the org/repo by remote URL.
//
// Scenario: User runs `wt pr checkout --clone test/myrepo 123` and myrepo is registered
// Expected: Uses existing repo (match takes precedence over --clone)
func TestPrCheckout_CloneFlagWithExistingMatch(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepo(t, tmpDir, "myrepo")

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "myrepo", Path: repoPath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, tmpDir)

	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	// --clone is set, but test/myrepo already matches the registered repo
	cmd.SetArgs([]string{"--clone", "test/myrepo", "123"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (forge/PR fetch), got nil")
	}
	// Should NOT attempt to clone — the existing match takes precedence
	if strings.Contains(err.Error(), "clone") {
		t.Errorf("should use existing repo instead of cloning, got %q", err.Error())
	}
	// Should NOT get "no registered repo"
	if strings.Contains(err.Error(), "no registered repo") {
		t.Errorf("should have found repo via remote URL, got %q", err.Error())
	}
}

// TestPrCheckout_AlreadyCheckedOut tests that pr checkout opens the worktree
// of a PR branch that is already checked out.
//
// Scenario: User runs `wt pr checkout 1 --forge github` with set_upstream on,
// the PR branch "feature" already has a worktree
// Expected: The worktree is opened, no second worktree is created and no upstream is set
func TestPrCheckout_AlreadyCheckedOut(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	cfg, repoPath, tmpDir := setupPrCheckoutRepo(t)
	cfg.Checkout.SetUpstream = new(true)
	wtPath := createTestWorktree(t, repoPath, "feature")

	ctx, logs := testContextWithLog(t, cfg, repoPath)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"1", "--forge", "github"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("pr checkout command failed: %v", err)
	}

	want := fmt.Sprintf("Opened worktree: %s (feature)\n", wtPath)
	if !strings.Contains(logs.String(), want) {
		t.Errorf("stderr should contain %q, got: %q", want, logs.String())
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "test-repo-feature")); err == nil {
		t.Error("no second worktree should be created")
	}
	if upstream := git.GetUpstreamBranch(ctx, repoPath, "feature"); upstream != "" {
		t.Errorf("opening a worktree should not set an upstream, got %q", upstream)
	}
}

// TestPrCheckout_PreservesFiles tests that pr checkout preserves files like checkout does.
//
// Scenario: preserve.paths lists ".env", user runs `wt pr checkout 1 --forge github`
// with and without --no-preserve
// Expected: ".env" is linked into the new worktree unless --no-preserve is passed
func TestPrCheckout_PreservesFiles(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	for _, noPreserve := range []bool{false, true} {
		t.Run(fmt.Sprintf("no-preserve=%v", noPreserve), func(t *testing.T) {
			cfg, repoPath, tmpDir := setupPrCheckoutRepo(t)
			cfg.Preserve = config.PreserveConfig{Paths: []string{".env"}}
			if err := os.WriteFile(filepath.Join(repoPath, ".env"), []byte("SECRET=abc\n"), 0644); err != nil {
				t.Fatalf("failed to write .env: %v", err)
			}

			args := []string{"1", "--forge", "github"}
			if noPreserve {
				args = append(args, "--no-preserve")
			}
			ctx := testContextWithConfig(t, cfg, repoPath)
			cmd := newPrCheckoutCmd()
			cmd.SetContext(ctx)
			cmd.SetArgs(args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("pr checkout command failed: %v", err)
			}

			_, err := os.Lstat(filepath.Join(tmpDir, "test-repo-feature", ".env"))
			if preserved := err == nil; preserved == noPreserve {
				t.Errorf(".env preserved = %v with --no-preserve = %v", preserved, noPreserve)
			}
		})
	}
}

// TestPrCheckout_CloneRegular tests pr checkout into a fresh regular clone.
//
// Scenario: User runs `wt pr checkout --clone --clone-mode regular test/test-repo 1 --forge github`
// Expected: The repo is cloned and registered, the PR branch is checked out
// in the clone itself and reported as an opened worktree
func TestPrCheckout_CloneRegular(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	tmpDir := resolvePath(t, t.TempDir())
	sourcePath, originPath := setupTestRepoWithOrigin(t, tmpDir, "source")
	if out, err := runGitCommand(sourcePath, "push", "origin", "main:feature"); err != nil {
		t.Fatalf("failed to push feature branch: %v\n%s", err, out)
	}
	t.Setenv("WT_TEST_CLONE_SOURCE", originPath)

	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	if err := saveRegistry(&registry.Registry{}, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx, logs := testContextWithLog(t, cfg, workDir)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--clone", "--clone-mode", "regular", "test/test-repo", "1", "--forge", "github"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("pr checkout command failed: %v", err)
	}

	clonePath := filepath.Join(workDir, "test-repo")
	branch, err := runGitCommand(clonePath, "branch", "--show-current")
	if err != nil {
		t.Fatalf("failed to read current branch: %v", err)
	}
	if strings.TrimSpace(branch) != "feature" {
		t.Errorf("clone should be on the PR branch, got %q", branch)
	}
	want := fmt.Sprintf("Opened worktree: %s (feature)\n", clonePath)
	if !strings.Contains(logs.String(), want) {
		t.Errorf("stderr should contain %q, got: %q", want, logs.String())
	}
	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if _, err := reg.FindByName("test-repo"); err != nil {
		t.Errorf("clone should be registered: %v", err)
	}
}

// TestPrCheckout_CloneNameConflict tests pr checkout --clone when the repo name is taken.
//
// Scenario: User runs `wt pr checkout --clone test/test-repo 1 --forge github`
// while another repo is registered as test-repo
// Expected: Command fails, the clone is removed and the registry is unchanged
func TestPrCheckout_CloneNameConflict(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	tmpDir := resolvePath(t, t.TempDir())
	sourcePath, originPath := setupTestRepoWithOrigin(t, tmpDir, "source")
	t.Setenv("WT_TEST_CLONE_SOURCE", originPath)

	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("failed to create work dir: %v", err)
	}
	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}
	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: sourcePath},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg := &config.Config{RegistryPath: regFile}
	ctx := testContextWithConfig(t, cfg, workDir)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--clone", "--clone-mode", "regular", "test/test-repo", "1", "--forge", "github"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when name is already registered")
	}

	if _, err := os.Stat(filepath.Join(workDir, "test-repo")); !os.IsNotExist(err) {
		t.Errorf("clone should be removed after failed registration, stat error: %v", err)
	}

	reg, err := registry.Load(regFile)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}
	if len(reg.Repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(reg.Repos))
	}
}

// setupPrCheckoutRepo creates a registered repo whose local origin has a
// "feature" branch, so pr checkout can fetch the PR branch without network.
// Returns the config, the repo path and the temp dir the worktree is created in.
func setupPrCheckoutRepo(t *testing.T) (cfg *config.Config, repoPath, tmpDir string) {
	t.Helper()

	tmpDir = resolvePath(t, t.TempDir())

	repoPath, _ = setupTestRepoWithOrigin(t, tmpDir, "test-repo")
	if out, err := runGitCommand(repoPath, "push", "origin", "main:feature"); err != nil {
		t.Fatalf("failed to push feature branch: %v\n%s", err, out)
	}

	regFile := filepath.Join(tmpDir, ".wt", "repos.json")
	if err := os.MkdirAll(filepath.Dir(regFile), 0755); err != nil {
		t.Fatalf("failed to create registry directory: %v", err)
	}

	reg := &registry.Registry{
		Repos: []registry.Repo{
			{Name: "test-repo", Path: repoPath, WorktreeFormat: "../{repo}-{branch}"},
		},
	}
	if err := saveRegistry(reg, regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	cfg = &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: "../{repo}-{branch}",
		},
	}
	return cfg, repoPath, tmpDir
}

// TestPrCheckout_HookWithArg tests that --arg values reach hooks run by pr checkout.
//
// Scenario: User runs `wt pr checkout 1 --forge github -a val=hello`, PR head branch exists on origin
// Expected: Worktree is created for the PR branch and the checkout hook runs with the variable substituted
func TestPrCheckout_HookWithArg(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	cfg, repoPath, tmpDir := setupPrCheckoutRepo(t)
	outputPath := filepath.Join(tmpDir, "hook-output.txt")
	cfg.Hooks = config.HooksConfig{
		Hooks: map[string]config.Hook{
			"show": {Command: "echo {val} > " + outputPath, On: []string{"checkout"}},
		},
	}

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"1", "--forge", "github", "-a", "val=hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("pr checkout command failed: %v", err)
	}

	wtPath := filepath.Join(tmpDir, "test-repo-feature")
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist at %s", wtPath)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read hook output: %v", err)
	}
	if strings.TrimSpace(string(content)) != "hello" {
		t.Errorf("expected hook output 'hello', got %q", string(content))
	}
}

// TestPrCheckout_InvalidArg tests that pr checkout reports a malformed --arg.
//
// Scenario: User runs `wt pr checkout 1 --forge github -a =value`
// Expected: Returns error about the empty key
func TestPrCheckout_InvalidArg(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	cfg, repoPath, _ := setupPrCheckoutRepo(t)

	ctx := testContextWithConfig(t, cfg, repoPath)
	cmd := newPrCheckoutCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"1", "--forge", "github", "-a", "=value"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed --arg")
	}
	if !strings.Contains(err.Error(), "key cannot be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPrMerge_HookWithArg tests that --arg values reach hooks run by pr merge.
//
// Scenario: User runs `wt pr merge --keep -a val=hello` in a worktree whose PR is already merged
// Expected: The merge hook runs with the variable substituted
func TestPrMerge_HookWithArg(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	cfg, _, wtPath := setupMergedWorktree(t)
	outputPath := filepath.Join(filepath.Dir(cfg.RegistryPath), "hook-output.txt")
	cfg.Hooks = config.HooksConfig{
		Hooks: map[string]config.Hook{
			"show": {Command: "echo {val} > " + outputPath, On: []string{"merge"}},
		},
	}

	ctx := testContextWithConfig(t, cfg, wtPath)
	cmd := newPrMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--keep", "-a", "val=hello"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("pr merge command failed: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read hook output: %v", err)
	}
	if strings.TrimSpace(string(content)) != "hello" {
		t.Errorf("expected hook output 'hello', got %q", string(content))
	}
}

// TestPrMerge_InvalidArg tests that pr merge reports a malformed --arg.
//
// Scenario: User runs `wt pr merge --keep -a =value` in a worktree whose PR is already merged
// Expected: Returns error about the empty key and keeps the worktree
func TestPrMerge_InvalidArg(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	cfg, _, wtPath := setupMergedWorktree(t)

	ctx := testContextWithConfig(t, cfg, wtPath)
	cmd := newPrMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--keep", "-a", "=value"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for malformed --arg")
	}
	if !strings.Contains(err.Error(), "key cannot be empty") {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("worktree should be kept: %v", err)
	}
}

// TestPrMerge_RemovesHistoryEntry tests that pr merge forgets the removed worktree.
//
// Scenario: User runs `wt pr merge` in a worktree whose PR is already merged
// Expected: The worktree is removed and its history entry is dropped
func TestPrMerge_RemovesHistoryEntry(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	cfg, repoPath, wtPath := setupMergedWorktree(t)
	cfg.HistoryPath = filepath.Join(t.TempDir(), "history.json")

	if err := history.RecordAccess(wtPath, "test-repo", "feature", cfg.HistoryPath); err != nil {
		t.Fatalf("failed to record history: %v", err)
	}
	if err := history.RecordAccess(repoPath, "test-repo", "main", cfg.HistoryPath); err != nil {
		t.Fatalf("failed to record history: %v", err)
	}

	ctx := testContextWithConfig(t, cfg, wtPath)
	cmd := newPrMergeCmd()
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("pr merge command failed: %v", err)
	}

	if _, err := os.Stat(wtPath); err == nil {
		t.Error("worktree should be removed after pr merge")
	}
	hist, err := history.Load(cfg.HistoryPath)
	if err != nil {
		t.Fatalf("failed to load history: %v", err)
	}
	if hist.FindByPath(wtPath) != nil {
		t.Error("removed worktree should be removed from history")
	}
	if hist.FindByPath(repoPath) == nil {
		t.Error("history entry of the main repo should be kept")
	}
}

// TestPrMerge_DeleteLocalBranch tests that pr merge follows prune.delete_local_branches.
//
// Scenario: User runs `wt pr merge` in a worktree whose PR is already merged,
// with delete_local_branches on and off
// Expected: The local branch is deleted only when delete_local_branches is on
func TestPrMerge_DeleteLocalBranch(t *testing.T) {
	// Not parallel: fakeGHMergedPR changes PATH via t.Setenv
	fakeGHMergedPR(t)

	for _, deleteBranches := range []bool{true, false} {
		t.Run(fmt.Sprintf("delete_local_branches=%v", deleteBranches), func(t *testing.T) {
			cfg, repoPath, wtPath := setupMergedWorktree(t)
			cfg.Prune.DeleteLocalBranches = deleteBranches

			ctx := testContextWithConfig(t, cfg, wtPath)
			cmd := newPrMergeCmd()
			cmd.SetContext(ctx)
			cmd.SetArgs([]string{})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("pr merge command failed: %v", err)
			}

			if _, err := os.Stat(wtPath); err == nil {
				t.Error("worktree should be removed after pr merge")
			}
			_, err := runGitCommand(repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/feature")
			if exists := err == nil; exists == deleteBranches {
				t.Errorf("branch exists = %v with delete_local_branches = %v", exists, deleteBranches)
			}
		})
	}
}

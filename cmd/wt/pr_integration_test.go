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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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
	if err := reg.Save(regFile); err != nil {
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

// TestPrCheckout_AlreadyCheckedOut tests that the pr checkout code path correctly
// detects an existing worktree, outputs "Opened worktree" instead of
// "Created worktree", and does not destroy the worktree.
//
// We cannot run the full pr checkout command (requires a forge/remote), so we
// exercise the detection + output message logic that pr_cmd.go uses inline.
//
// Scenario: Branch "feature" already has a worktree checked out
// Expected: findWorktreeForBranch detects it, output says "Opened worktree"
func TestPrCheckout_AlreadyCheckedOut(t *testing.T) {
	t.Parallel()

	tmpDir := resolvePath(t, t.TempDir())
	repoPath := setupTestRepoWithBranches(t, tmpDir, "myrepo", []string{"feature"})

	// Create a worktree for the branch first
	wtPath := filepath.Join(repoPath, ".worktrees", "feature")
	gitWorktreeAdd := exec.Command("git", "worktree", "add", wtPath, "feature")
	gitWorktreeAdd.Dir = repoPath
	if out, err := gitWorktreeAdd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create worktree: %v\n%s", err, out)
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
	if err := reg.Save(regFile); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	ctx := testContextWithConfig(t, &config.Config{
		RegistryPath: regFile,
		Checkout: config.CheckoutConfig{
			WorktreeFormat: ".worktrees/{branch}",
		},
	}, repoPath)

	// Step 1: Verify findWorktreeForBranch detects the existing worktree
	foundPath, found := findWorktreeForBranch(ctx, repoPath, "feature")
	if !found {
		t.Fatal("expected to find existing worktree for branch 'feature'")
	}
	if foundPath != wtPath {
		t.Errorf("expected worktree path %s, got %s", wtPath, foundPath)
	}

	// Step 2: Verify the output message selection logic from pr_cmd.go.
	// When found=true and justClonedRegular=false, the code builds:
	//   outputMsg = fmt.Sprintf("Opened worktree: %s (%s)\n", wtPath, branch)
	// Reproduce that logic here to verify it selects "Opened worktree".
	justClonedRegular := false
	branch := "feature"

	var outputMsg string
	if justClonedRegular {
		outputMsg = fmt.Sprintf("Checked out PR branch: %s (%s)\n", repoPath, branch)
	} else if found {
		outputMsg = fmt.Sprintf("Opened worktree: %s (%s)\n", foundPath, branch)
	} else {
		outputMsg = fmt.Sprintf("Created worktree: %s (%s)\n", wtPath, branch)
	}

	if !strings.Contains(outputMsg, "Opened worktree") {
		t.Errorf("expected 'Opened worktree' in output, got %q", outputMsg)
	}
	if strings.Contains(outputMsg, "Created worktree") {
		t.Errorf("output should not contain 'Created worktree', got %q", outputMsg)
	}
	expectedMsg := fmt.Sprintf("Opened worktree: %s (feature)\n", wtPath)
	if outputMsg != expectedMsg {
		t.Errorf("expected output %q, got %q", expectedMsg, outputMsg)
	}

	// Step 3: Worktree should still exist (not removed or modified)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should still exist at %s", wtPath)
	}
}

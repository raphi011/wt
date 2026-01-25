# Integration Tests

This document describes the integration test setup for `wt`.

## Overview

Integration tests verify `wt` commands against real git repositories and external services. They are separated from unit tests using the `//go:build integration` build tag.

## Running Tests

```bash
# Run all integration tests (requires gh auth)
make test-integration

# Run specific test package
WT_TEST_GITHUB_REPO=raphi011/wt-test go test -tags=integration -v ./internal/forge/...

# Run without forge tests (no GitHub auth needed)
go test -tags=integration ./cmd/wt/...
```

## Test Categories

### 1. Command Integration Tests (`cmd/wt/*_integration_test.go`)

Tests for CLI commands against local git repositories created in temp directories.

**Covered:**
- `wt add` - Creating worktrees from existing/new branches
- `wt cd` - Navigating to worktrees by ID, repo, or label
- `wt list` - Listing worktrees with various filters
- `wt prune` - Removing merged worktrees
- `wt doctor` - Diagnosing and fixing worktree issues
- `wt note` - Setting/getting notes on worktrees
- `wt exec` - Executing commands across worktrees
- `wt hook` - Running hooks on worktrees

**Setup:** Tests create temporary git repositories with branches and worktrees. No external services required.

### 2. Forge Integration Tests (`internal/forge/forge_integration_test.go`)

Tests for GitHub CLI operations against a real GitHub repository.

**Covered:**
- `Check()` - Verify gh CLI is authenticated
- `GetPRForBranch()` - Query PR status for branches
- `CloneRepo()` - Clone repositories
- `CreatePR()` - Create pull requests
- `ViewPR()` - View PR details
- `GetPRBranch()` - Get branch name from PR number
- `MergePR()` - Merge PRs with squash strategy

**Not Covered:**
- GitLab operations (planned for future)
- Rebase and merge commit strategies (only squash tested)
- Cross-repository/fork PRs
- PR reviews and approvals
- Draft PR conversion

## Setup Requirements

### Local Development

1. **gh CLI authenticated:**
   ```bash
   gh auth login
   ```

2. **Access to test repository:**
   The forge tests use `raphi011/wt-test` as the test repository. For local testing, you need push access to this repo or can fork and update `WT_TEST_GITHUB_REPO`.

### CI (GitHub Actions)

The CI workflow (`.github/workflows/ci.yml`) requires:

1. **`GH_TOKEN` secret:** A Personal Access Token with:
   - Repository access to `raphi011/wt-test`
   - Permissions: `Contents` (read/write), `Pull requests` (read/write)

2. **Environment variables:**
   ```yaml
   env:
     GH_TOKEN: ${{ secrets.GH_TOKEN }}
     WT_TEST_GITHUB_REPO: raphi011/wt-test
   ```

## Test Repository

The forge tests use a dedicated test repository: `raphi011/wt-test`

**Why a separate repo?**
- Tests create real PRs and merge them
- Avoids polluting the main repo with test commits
- Can have permissive settings (no branch protection)

**Repository requirements:**
- Public or accessible to test token
- `main` branch as default
- No branch protection rules (tests need to merge without reviews)
- Allow squash merges

## Test Patterns

### Parallel Execution

- Read-only tests use `t.Parallel()` for concurrent execution
- Write tests (PR workflow) run sequentially to avoid conflicts
- Command tests use `t.Parallel()` with isolated temp directories

### Working Directory Injection

Commands accept `workDir` parameter instead of using `os.Getwd()`. This enables parallel test execution without `os.Chdir()` race conditions.

### Cleanup

- `t.Cleanup()` ensures resources are cleaned up even on failure
- Forge tests close any unclosed PRs and delete test branches
- Temp directories are automatically removed by `t.TempDir()`

### Skip Conditions

Tests skip gracefully when requirements aren't met:

```go
func skipIfNoGitHub(t *testing.T) {
    if os.Getenv("WT_TEST_GITHUB_REPO") == "" {
        t.Skip("WT_TEST_GITHUB_REPO not set")
    }
}
```

## Adding New Tests

### Command Tests

1. Create test in `cmd/wt/<command>_integration_test.go`
2. Add build tag: `//go:build integration`
3. Use `t.Parallel()` and `t.TempDir()` for isolation
4. Use helper functions from `integration_test_helpers.go`

### Forge Tests

1. Add to `internal/forge/forge_integration_test.go`
2. Read-only tests: use `t.Parallel()`, call `skipIfNoGitHub(t)`
3. Write tests: add as subtest in `TestGitHub_PRWorkflow`
4. Ensure cleanup in `t.Cleanup()`

## Limitations

- **GitHub only:** GitLab integration tests not yet implemented
- **Single test repo:** All forge tests share one repository
- **No mock mode:** Tests require real GitHub access
- **Rate limits:** Excessive test runs may hit GitHub API limits
- **Network dependent:** Tests fail if GitHub is unreachable

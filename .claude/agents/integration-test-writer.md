# Integration Test Writer Agent

Use this agent when writing or updating integration tests in `cmd/wt/*_integration_test.go`.

## Test File Structure

Integration tests use the build tag `//go:build integration` and are in `cmd/wt/*_integration_test.go`.

## Required Test Documentation

All integration test functions MUST have doc comments in this format:
```go
// TestCommandName_Scenario describes what the test verifies in one line.
//
// Scenario: User runs `wt command args`
// Expected: Description of expected outcome
func TestCommandName_Scenario(t *testing.T) {
```

These comments are extracted by `make testdoc` to generate `docs/TESTS.md`.

## Parallel Test Safety

**All tests MUST be parallel-safe.** Tests run concurrently with `t.Parallel()`.

### Process-Wide State to Avoid

Never modify these in tests - they affect all goroutines:
- `os.Setenv()` - especially `HOME`
- `os.Chdir()` - changes working directory globally

### Registry Isolation Pattern

Use `cfg.RegistryPath` to give each test its own isolated registry file:

```go
func TestSomething(t *testing.T) {
    t.Parallel()  // ALWAYS first statement

    tmpDir := t.TempDir()
    tmpDir = resolvePath(t, tmpDir)  // Required on macOS for symlink resolution

    // Create isolated registry file
    regFile := filepath.Join(tmpDir, ".wt", "repos.json")
    os.MkdirAll(filepath.Dir(regFile), 0755)

    // Set config with isolated registry path
    oldCfg := cfg
    cfg = &config.Config{RegistryPath: regFile}
    defer func() { cfg = oldCfg }()

    // Create and save test registry
    reg := &registry.Registry{
        Repos: []registry.Repo{
            {Name: "test-repo", Path: repoPath},
        },
    }
    if err := reg.Save(regFile); err != nil {
        t.Fatalf("failed to save registry: %v", err)
    }

    // ... test logic ...
}
```

### Working Directory Isolation Pattern

Use the package-level `workDir` variable instead of `os.Chdir()`:

```go
func TestSomething(t *testing.T) {
    t.Parallel()

    // ... setup ...

    // Set isolated working directory
    oldWorkDir := workDir
    workDir = repoPath
    defer func() { workDir = oldWorkDir }()

    // ... test logic ...
}
```

For tests that need to simulate being outside a repo:
```go
otherDir := filepath.Join(tmpDir, "other")
os.MkdirAll(otherDir, 0755)

oldWorkDir := workDir
workDir = otherDir  // Not inside a git repo
defer func() { workDir = oldWorkDir }()
```

## Complete Test Template

```go
// TestCommand_Scenario tests specific behavior.
//
// Scenario: User runs `wt command args`
// Expected: Description of expected result
func TestCommand_Scenario(t *testing.T) {
    t.Parallel()

    // 1. Create temp directory with symlink resolution
    tmpDir := t.TempDir()
    tmpDir = resolvePath(t, tmpDir)

    // 2. Setup test repo(s)
    repoPath := setupTestRepo(t, tmpDir, "test-repo")
    // Or with branches:
    // repoPath := setupTestRepoWithBranches(t, tmpDir, "test-repo", []string{"feature"})

    // 3. Create isolated registry
    regFile := filepath.Join(tmpDir, ".wt", "repos.json")
    os.MkdirAll(filepath.Dir(regFile), 0755)

    reg := &registry.Registry{
        Repos: []registry.Repo{
            {Name: "test-repo", Path: repoPath},
        },
    }
    if err := reg.Save(regFile); err != nil {
        t.Fatalf("failed to save registry: %v", err)
    }

    // 4. Set config with any needed options
    oldCfg := cfg
    cfg = &config.Config{
        RegistryPath: regFile,
        // Add other config as needed
    }
    defer func() { cfg = oldCfg }()

    // 5. Set working directory
    oldWorkDir := workDir
    workDir = repoPath
    defer func() { workDir = oldWorkDir }()

    // 6. Create and execute command
    ctx := testContext(t)
    cmd := newSomeCmd()
    cmd.SetContext(ctx)
    cmd.SetArgs([]string{"arg1", "arg2"})

    // 7. Assert result
    if err := cmd.Execute(); err != nil {
        t.Fatalf("command failed: %v", err)
    }

    // 8. Verify expected state
    // ...
}
```

## Helper Functions

Available in `integration_test_helpers.go`:

- `testContext(t)` - Creates context with test logger/output
- `testContextWithOutput(t)` - Returns context and output buffer for verification
- `resolvePath(t, path)` - Resolves symlinks (required on macOS)
- `setupTestRepo(t, dir, name)` - Creates a git repo with initial commit
- `setupTestRepoWithBranches(t, dir, name, branches)` - Creates repo with branches
- `setupBareInGitRepo(t, dir, name)` - Creates bare-in-.git structured repo
- `createTestWorktree(t, repoPath, branch)` - Creates a worktree
- `runGitCommand(repoPath, args...)` - Runs git command and returns output

## Common Patterns

### Testing error cases
```go
err := cmd.Execute()
if err == nil {
    t.Fatal("expected error for invalid input")
}
if !strings.Contains(err.Error(), "expected message") {
    t.Errorf("expected error about X, got: %v", err)
}
```

### Testing command output
```go
ctx, out := testContextWithOutput(t)
cmd.SetContext(ctx)
// ...
if err := cmd.Execute(); err != nil {
    t.Fatalf("command failed: %v", err)
}
if !strings.Contains(out.String(), "expected output") {
    t.Errorf("expected output to contain X, got: %s", out.String())
}
```

### Testing file operations
```go
// Verify file exists
if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
    t.Error("expected file to exist")
}

// Verify file was removed
if _, err := os.Stat(removedPath); err == nil {
    t.Error("expected file to be removed")
}
```

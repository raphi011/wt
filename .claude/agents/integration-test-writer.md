# Test Writer Agent

Use this agent when writing or updating:
- Integration tests in `cmd/wt/*_integration_test.go`
- Wizard/interactive mode unit tests in `internal/ui/wizard/**/*_test.go`

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

---

# Wizard/Interactive Mode Tests

Unit tests for the interactive wizard system in `internal/ui/wizard/`.

## Testing Approach

Test step components by calling `Update()` directly with synthetic `tea.KeyPressMsg`. No TTY/terminal needed.

**Advantages:**
- Fast execution
- Tests logic without rendering overhead
- Works in CI without terminal

## Required Helpers

Add these helpers to each test file:

```go
// keyMsg creates a tea.KeyPressMsg from a string key.
func keyMsg(key string) tea.KeyPressMsg {
    switch key {
    case "enter":
        return tea.KeyPressMsg{Code: tea.KeyEnter}
    case "up":
        return tea.KeyPressMsg{Code: tea.KeyUp}
    case "down":
        return tea.KeyPressMsg{Code: tea.KeyDown}
    case "left":
        return tea.KeyPressMsg{Code: tea.KeyLeft}
    case "right":
        return tea.KeyPressMsg{Code: tea.KeyRight}
    case "home":
        return tea.KeyPressMsg{Code: tea.KeyHome}
    case "end":
        return tea.KeyPressMsg{Code: tea.KeyEnd}
    case "pgup":
        return tea.KeyPressMsg{Code: tea.KeyPgUp}
    case "pgdown":
        return tea.KeyPressMsg{Code: tea.KeyPgDown}
    case "esc":
        return tea.KeyPressMsg{Code: tea.KeyEscape}
    case "space":
        return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
    case "backspace":
        return tea.KeyPressMsg{Code: tea.KeyBackspace}
    case "tab":
        return tea.KeyPressMsg{Code: tea.KeyTab}
    default:
        // Single character key
        if len(key) == 1 {
            r := rune(key[0])
            return tea.KeyPressMsg{Code: r, Text: key}
        }
        return tea.KeyPressMsg{}
    }
}

// updateStep performs Update and returns the concrete type with type assertion.
func updateStep[T framework.Step](t *testing.T, s T, msg tea.KeyPressMsg) (T, framework.StepResult) {
    t.Helper()
    result, _, stepResult := s.Update(msg)
    concrete, ok := result.(T)
    if !ok {
        t.Fatalf("Update returned unexpected type: %T", result)
    }
    return concrete, stepResult
}
```

## Step Test Patterns

### SingleSelectStep Tests

```go
func TestSingleSelectStep_Navigation(t *testing.T) {
    options := []framework.Option{
        {Label: "Option 1", Value: "opt1"},
        {Label: "Option 2", Value: "opt2"},
    }

    t.Run("navigate down moves cursor", func(t *testing.T) {
        step := NewSingleSelect("test", "Test", "Select:", options)

        updateStep(t, step, keyMsg("down"))
        if step.GetCursor() != 1 {
            t.Errorf("Cursor = %d, want 1", step.GetCursor())
        }
    })

    t.Run("enter selects current option", func(t *testing.T) {
        step := NewSingleSelect("test", "Test", "Select:", options)

        _, result := updateStep(t, step, keyMsg("enter"))
        if result != framework.StepSubmitIfReady {
            t.Errorf("Result = %v, want StepSubmitIfReady", result)
        }
        if !step.IsComplete() {
            t.Error("Step should be complete")
        }
    })

    t.Run("disabled options are skipped", func(t *testing.T) {
        opts := []framework.Option{
            {Label: "Enabled", Value: "e1"},
            {Label: "Disabled", Value: "d1", Disabled: true},
            {Label: "Enabled 2", Value: "e2"},
        }
        step := NewSingleSelect("test", "Test", "Select:", opts)

        updateStep(t, step, keyMsg("down"))
        // Should skip disabled option
        if step.GetCursor() != 2 {
            t.Errorf("Cursor = %d, want 2 (skipped disabled)", step.GetCursor())
        }
    })
}
```

### FilterableListStep Tests

```go
func TestFilterableListStep_Filtering(t *testing.T) {
    options := []framework.Option{
        {Label: "apple", Value: "apple"},
        {Label: "banana", Value: "banana"},
    }

    t.Run("typing updates filter", func(t *testing.T) {
        step := NewFilterableList("test", "Test", "Select", options)

        updateStep(t, step, keyMsg("a"))
        if step.GetFilter() != "a" {
            t.Errorf("Filter = %q, want %q", step.GetFilter(), "a")
        }
    })

    t.Run("filter narrows options", func(t *testing.T) {
        step := NewFilterableList("test", "Test", "Select", options)

        updateStep(t, step, keyMsg("a"))
        updateStep(t, step, keyMsg("p"))
        // "ap" matches only "apple"
        if step.FilteredCount() != 1 {
            t.Errorf("FilteredCount = %d, want 1", step.FilteredCount())
        }
    })

    t.Run("multi-select space toggles", func(t *testing.T) {
        step := NewFilterableList("test", "Test", "Select", options).
            WithMultiSelect()

        updateStep(t, step, keyMsg("space"))
        if step.SelectedCount() != 1 {
            t.Errorf("SelectedCount = %d, want 1", step.SelectedCount())
        }

        updateStep(t, step, keyMsg("space"))
        if step.SelectedCount() != 0 {
            t.Errorf("SelectedCount after toggle = %d, want 0", step.SelectedCount())
        }
    })
}
```

### TextInputStep Tests

```go
func TestTextInputStep_Input(t *testing.T) {
    t.Run("typing updates value", func(t *testing.T) {
        step := NewTextInput("test", "Test", "Enter:", "")
        step.Init() // IMPORTANT: Focus the input first!

        updateStep(t, step, keyMsg("h"))
        updateStep(t, step, keyMsg("i"))

        if step.GetValue() != "hi" {
            t.Errorf("Value = %q, want %q", step.GetValue(), "hi")
        }
    })

    t.Run("validation prevents empty submit", func(t *testing.T) {
        step := NewTextInput("test", "Test", "Enter:", "")

        _, result := updateStep(t, step, keyMsg("enter"))
        if result == framework.StepSubmitIfReady {
            t.Error("Should not submit with empty value")
        }
    })
}
```

## Wizard Orchestration Tests

Use a mock step for testing wizard behavior:

```go
type mockStep struct {
    id        string
    title     string
    complete  bool
    value     StepValue
}

func newMockStep(id, title string) *mockStep {
    return &mockStep{id: id, title: title}
}

func (s *mockStep) ID() string    { return s.id }
func (s *mockStep) Title() string { return s.title }
func (s *mockStep) Init() tea.Cmd { return nil }

func (s *mockStep) Update(msg tea.KeyPressMsg) (Step, tea.Cmd, StepResult) {
    switch msg.String() {
    case "left":
        return s, nil, StepBack
    case "right":
        s.complete = true
        return s, nil, StepAdvance
    case "enter":
        s.complete = true
        return s, nil, StepSubmitIfReady
    }
    return s, nil, StepContinue
}

func (s *mockStep) View() string              { return s.title }
func (s *mockStep) Help() string              { return "mock" }
func (s *mockStep) Value() StepValue          { return s.value }
func (s *mockStep) IsComplete() bool          { return s.complete }
func (s *mockStep) Reset()                    { s.complete = false }
func (s *mockStep) HasClearableInput() bool   { return false }
func (s *mockStep) ClearInput() tea.Cmd       { return nil }
```

### Wizard Test Examples

```go
func updateWizard(t *testing.T, w *Wizard, key string) *Wizard {
    t.Helper()
    m, _ := w.Update(keyMsg(key))
    return m.(*Wizard)
}

func TestWizard_Navigation(t *testing.T) {
    t.Run("enter advances to next step", func(t *testing.T) {
        step1 := newMockStep("step1", "Step 1")
        step2 := newMockStep("step2", "Step 2")

        w := NewWizard("Test").AddStep(step1).AddStep(step2)
        w.Init()

        w = updateWizard(t, w, "enter")

        if w.CurrentStepID() != "step2" {
            t.Errorf("CurrentStepID = %s, want step2", w.CurrentStepID())
        }
    })

    t.Run("SkipWhen skips step", func(t *testing.T) {
        step1 := newMockStep("step1", "Step 1")
        step2 := newMockStep("step2", "Step 2")
        step3 := newMockStep("step3", "Step 3")

        w := NewWizard("Test").
            AddStep(step1).
            AddStep(step2).
            AddStep(step3).
            SkipWhen("step2", func(w *Wizard) bool { return true })

        w.Init()
        w = updateWizard(t, w, "enter")

        // Should skip step2, go to step3
        if w.CurrentStepID() != "step3" {
            t.Errorf("CurrentStepID = %s, want step3", w.CurrentStepID())
        }
    })

    t.Run("esc cancels wizard", func(t *testing.T) {
        step1 := newMockStep("step1", "Step 1")
        w := NewWizard("Test").AddStep(step1)
        w.Init()

        w = updateWizard(t, w, "esc")

        if !w.IsCancelled() {
            t.Error("Wizard should be cancelled")
        }
    })
}
```

## Key Test Scenarios

### SingleSelectStep
- [ ] Navigation: up/down/j/k/home/end/pgup/pgdown
- [ ] Selection: enter selects, right advances
- [ ] Disabled options: cursor skips, cannot select
- [ ] Value(): returns correct selection
- [ ] Reset(): clears selection

### FilterableListStep
- [ ] Filter: typing updates, backspace clears
- [ ] Fuzzy matching: partial matches work
- [ ] Multi-select: space toggles, SetMinMax enforces
- [ ] Create-from-filter: appears when no exact match
- [ ] ClearInput(): clears filter

### TextInputStep
- [ ] Input: typing works (after Init/Focus)
- [ ] Validation: custom validators, empty rejection
- [ ] Submission: enter submits, right advances
- [ ] ClearInput(): clears text

### Wizard
- [ ] Navigation: enter advances, left goes back
- [ ] SkipWhen: conditions evaluated, backward also skips
- [ ] OnComplete: callbacks fire with wizard access
- [ ] Summary: enter confirms, left goes back
- [ ] WithSkipSummary: completes after last step
- [ ] Cancel: esc/ctrl+c, clears input first if clearable
- [ ] Pre-filled: Init skips to first incomplete

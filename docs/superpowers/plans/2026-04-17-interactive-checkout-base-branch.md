# Interactive Checkout Base Branch Selection — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a base branch selection step to `wt checkout -i` wizard when creating a new branch.

**Architecture:** Add a single-select `FilterableListStep` for base branch between the branch and hooks steps, skipped via `SkipWhen` when the user selects an existing branch or `--base` was passed on CLI. Reuse existing branch options and add a `SetCursor` method for pre-selection.

**Tech Stack:** Go, bubbletea wizard framework, FilterableListStep component

---

### Task 1: Add `SetCursor` method to `FilterableListStep`

**Files:**
- Modify: `internal/ui/wizard/steps/filterable_list.go`
- Test: `internal/ui/wizard/steps/filterable_list_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/ui/wizard/steps/filterable_list_test.go`, add:

```go
func TestFilterableListStep_SetCursor(t *testing.T) {
	t.Parallel()

	options := []framework.Option{
		{Label: "alpha", Value: "alpha"},
		{Label: "beta", Value: "beta"},
		{Label: "gamma", Value: "gamma"},
	}
	step := NewFilterableList("test", "Test", "Pick one", options)

	step.SetCursor(2)

	if step.GetCursor() != 2 {
		t.Errorf("GetCursor() = %d, want 2", step.GetCursor())
	}
}

func TestFilterableListStep_SetCursor_ClampsBounds(t *testing.T) {
	t.Parallel()

	options := []framework.Option{
		{Label: "alpha", Value: "alpha"},
		{Label: "beta", Value: "beta"},
	}
	step := NewFilterableList("test", "Test", "Pick one", options)

	step.SetCursor(10)
	if step.GetCursor() != 1 {
		t.Errorf("GetCursor() after out-of-bounds = %d, want 1", step.GetCursor())
	}

	step.SetCursor(-1)
	if step.GetCursor() != 0 {
		t.Errorf("GetCursor() after negative = %d, want 0", step.GetCursor())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/wizard/steps/ -run TestFilterableListStep_SetCursor -v`
Expected: FAIL — `SetCursor` method does not exist.

- [ ] **Step 3: Implement `SetCursor`**

In `internal/ui/wizard/steps/filterable_list.go`, add after the `GetCursor` method (line ~658):

```go
// SetCursor sets the cursor position, clamping to valid bounds.
func (s *FilterableListStep) SetCursor(idx int) *FilterableListStep {
	maxIdx := len(s.filtered) - 1
	if maxIdx < 0 {
		maxIdx = 0
	}
	if idx < 0 {
		idx = 0
	}
	if idx > maxIdx {
		idx = maxIdx
	}
	s.cursor = idx
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/wizard/steps/ -run TestFilterableListStep_SetCursor -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/wizard/steps/filterable_list.go internal/ui/wizard/steps/filterable_list_test.go
git commit -m "feat: add SetCursor method to FilterableListStep"
```

---

### Task 2: Add base branch step to checkout wizard

**Files:**
- Modify: `internal/ui/wizard/flows/checkout.go`
- Test: `internal/ui/wizard/flows/checkout_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/ui/wizard/flows/checkout_test.go`, add:

```go
func TestCheckoutOptions_IncludesBase(t *testing.T) {
	opts := CheckoutOptions{
		Branch:    "feature-x",
		NewBranch: true,
		Base:      "main",
	}

	if opts.Base != "main" {
		t.Errorf("Base = %q, want %q", opts.Base, "main")
	}
}

func TestCheckoutWizardParams_IncludesDefaultBranch(t *testing.T) {
	params := CheckoutWizardParams{
		DefaultBranch: "main",
		BaseFromCLI:   false,
	}

	if params.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", params.DefaultBranch, "main")
	}
	if params.BaseFromCLI {
		t.Error("BaseFromCLI should be false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/wizard/flows/ -run "TestCheckoutOptions_IncludesBase|TestCheckoutWizardParams_IncludesDefaultBranch" -v`
Expected: FAIL — `Base` field does not exist on `CheckoutOptions`, `DefaultBranch`/`BaseFromCLI` do not exist on `CheckoutWizardParams`.

- [ ] **Step 3: Add fields to structs**

In `internal/ui/wizard/flows/checkout.go`, add `Base` field to `CheckoutOptions`:

```go
type CheckoutOptions struct {
	Branch        string
	NewBranch     bool
	Base          string   // Base branch for new branch creation
	Cancelled     bool
	SelectedRepos []string
	SelectedHooks []string
	NoHook        bool
}
```

Add `DefaultBranch` and `BaseFromCLI` fields to `CheckoutWizardParams`:

```go
type CheckoutWizardParams struct {
	Branches         []BranchInfo
	AvailableRepos   []string
	RepoNames        []string
	PreSelectedRepos []int
	FetchBranches    BranchFetcher
	AvailableHooks   []HookInfo
	HooksFromCLI     bool
	DefaultBranch    string // Default branch name for pre-selection in base step
	BaseFromCLI      bool   // True if --base was explicitly passed (skip base step)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui/wizard/flows/ -run "TestCheckoutOptions_IncludesBase|TestCheckoutWizardParams_IncludesDefaultBranch" -v`
Expected: PASS

- [ ] **Step 5: Add the base step to `CheckoutInteractive`**

In `internal/ui/wizard/flows/checkout.go`, in the `CheckoutInteractive` function, add after the branch step (after line 137 `w.AddStep(branchStep)`) and before the hooks step:

```go
	// Step 3: Base branch (only when creating new branch and not set via CLI)
	baseStep := steps.NewFilterableList("base", "Base Branch", "Select a base branch to create from", branchOptions).
		WithRuneFilter(framework.RuneFilterNoSpaces).
		WithEmptyMessage("No matching branches")

	// Pre-select default branch
	if params.DefaultBranch != "" {
		for i, opt := range branchOptions {
			if opt.Value == params.DefaultBranch {
				baseStep.SetCursor(i)
				break
			}
		}
	}

	w.AddStep(baseStep)

	// Skip base step when selecting existing branch or --base passed on CLI
	w.SkipWhen("base", func(wiz *framework.Wizard) bool {
		if params.BaseFromCLI {
			return true
		}
		branchStepResult, ok := wiz.GetStep("branch").(*steps.FilterableListStep)
		if !ok {
			return true
		}
		return !branchStepResult.IsCreateSelected()
	})
```

- [ ] **Step 6: Update `OnComplete("repos", ...)` callback to also update base step**

In the existing `OnComplete("repos", ...)` callback (around line 148-170), add after the `branchStepUpdate.SetOptions(branchOpts)` line:

```go
				// Update base step with same branches
				baseStepUpdate, ok := wiz.GetStep("base").(*steps.FilterableListStep)
				if !ok {
					return
				}
				baseStepUpdate.SetOptions(branchOpts)
```

- [ ] **Step 7: Extract base value from wizard result**

In the result extraction section of `CheckoutInteractive` (around line 183-200), add after the branch extraction and before the hooks extraction:

```go
	// Base branch
	if !params.BaseFromCLI {
		opts.Base = result.GetString("base")
	}
```

- [ ] **Step 8: Run all checkout flow tests**

Run: `go test ./internal/ui/wizard/flows/ -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/ui/wizard/flows/checkout.go internal/ui/wizard/flows/checkout_test.go
git commit -m "feat: add base branch step to checkout wizard"
```

---

### Task 3: Wire base branch into `runCheckoutWizard` and apply result

**Files:**
- Modify: `cmd/wt/checkout_cmd.go`

- [ ] **Step 1: Pass `--base` CLI state and default branch to wizard params**

In `cmd/wt/checkout_cmd.go`, in the `RunE` function, capture the `--base` explicit flag (add after `fetchExplicit` on line 58):

```go
				baseExplicit := cmd.Flags().Changed("base")
```

Then update `runCheckoutWizard` call (line 73) to pass the new flag:

```go
				wizOpts, err := runCheckoutWizard(ctx, reg, hf.HookNames, hf.NoHook, baseExplicit)
```

- [ ] **Step 2: Update `runCheckoutWizard` signature and detect default branch**

Update the `runCheckoutWizard` function signature (line 576):

```go
func runCheckoutWizard(ctx context.Context, reg *registry.Registry, cliHooks []string, cliNoHook bool, baseFromCLI bool) (flows.CheckoutOptions, error) {
```

Then, after the `initialBranches` block (after line 627), add:

```go
	// Detect default branch from first selected/available repo
	var defaultBranch string
	if len(preSelectedRepos) > 0 {
		defaultBranch = git.GetDefaultBranch(ctx, repoPaths[preSelectedRepos[0]])
	} else if len(repoPaths) > 0 {
		defaultBranch = git.GetDefaultBranch(ctx, repoPaths[0])
	}
```

Then update the `params` struct (around line 640) to include the new fields:

```go
	params := flows.CheckoutWizardParams{
		Branches:         initialBranches,
		AvailableRepos:   repoPaths,
		RepoNames:        repoNames,
		PreSelectedRepos: preSelectedRepos,
		FetchBranches:    fetchBranches,
		AvailableHooks:   availableHooks,
		HooksFromCLI:     len(cliHooks) > 0 || cliNoHook,
		DefaultBranch:    defaultBranch,
		BaseFromCLI:      baseFromCLI,
	}
```

- [ ] **Step 3: Apply wizard base result to the `base` variable**

In the interactive mode block (after line 85 `hf.NoHook = wizOpts.NoHook`), add:

```go
				if wizOpts.Base != "" {
					base = wizOpts.Base
				}
```

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/wt`
Expected: Compiles successfully.

- [ ] **Step 5: Run all unit tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/wt/checkout_cmd.go
git commit -m "feat: wire base branch wizard step into checkout command"
```

---

### Task 4: Manual smoke test

- [ ] **Step 1: Build the binary**

Run: `just build`

- [ ] **Step 2: Test interactive new branch with base selection**

Run: `./wt checkout -i`
- Select a repo (if multiple)
- Type a new branch name that doesn't exist → select "Create"
- Verify the base branch step appears with branches listed
- Verify the default branch is pre-selected
- Select a base branch and continue
- Verify worktree is created with correct base

- [ ] **Step 3: Test interactive existing branch (base step skipped)**

Run: `./wt checkout -i`
- Select an existing branch from the list
- Verify the base branch step is NOT shown
- Verify checkout works normally

- [ ] **Step 4: Test `--base` flag skips wizard step**

Run: `./wt checkout -i --base develop`
- Create a new branch in the wizard
- Verify the base branch step is NOT shown (skipped because `--base` was explicit)
- Verify the branch is created from `develop`

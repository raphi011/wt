# Interactive Checkout Base Branch Selection

## Overview

Extend `wt checkout -i` to include a base branch selection step when creating a new branch. Currently, `--base` is only available as a CLI flag; this adds it to the interactive wizard flow.

## Wizard Flow (Updated)

1. **Repos** — select repositories (unchanged)
2. **Branch** — select or create a branch (unchanged)
3. **Base** — select base branch to create from (**new step**)
4. **Hooks** — select hooks to run (unchanged)

## Base Branch Step

- **Component:** `FilterableListStep` (single-select)
- **Options:** Same branch list already fetched for the branch step (reuse `buildBranchOptions`)
- **Pre-selection:** Default branch (detected via `git.GetDefaultBranch`) pre-selected
- **Skip condition:** `SkipWhen("base", ...)` — skip when `branchStep.IsCreateSelected()` returns `false` (i.e., user selected an existing branch, not creating a new one), OR when `--base` was explicitly passed on the CLI
- **Filter:** `RuneFilterNoSpaces` (consistent with branch step)

## Data Flow Changes

### `CheckoutOptions` (flows/checkout.go)

Add `Base string` field to hold the selected base branch.

### `CheckoutWizardParams` (flows/checkout.go)

Add `DefaultBranch string` field. The caller (`runCheckoutWizard`) resolves this from the first selected repo using `git.GetDefaultBranch`. Also add `BaseFromCLI bool` to indicate `--base` was explicitly set on CLI (skip the wizard step).

### `runCheckoutWizard` (checkout_cmd.go)

- Detect the default branch from the first selected repo (or current repo)
- Pass it via `CheckoutWizardParams.DefaultBranch`

### `CheckoutInteractive` (flows/checkout.go)

- Add a `FilterableListStep` for "base" after the branch step
- Use `SkipWhen("base", ...)` to skip when the branch step didn't create a new branch
- Pre-select the default branch in the options list
- On repos change (`OnComplete("repos", ...)`): update base step options alongside the existing branch step update, and update pre-selection to the new repo's default branch

### `checkout_cmd.go` interactive block

Apply `wizOpts.Base` to the `base` variable:

```go
base = wizOpts.Base
```

### Summary

The base branch appears automatically in the wizard summary since it's a standard wizard step with a value. When skipped (existing branch), it won't appear.

## What Doesn't Change

- Non-interactive `--base` flag behavior
- `createWorktreeForBranch` logic (already handles base ref resolution)
- Hook step behavior
- Branch step behavior

# Test Documentation

Generated: 2026-01-31

## Summary

| Command | Tests |
|---------|-------|
| [add](#add) | 4 |
| [clone](#clone) | 6 |
| [forge](#forge) | 6 |
| [hook](#hook) | 4 |
| [migrate](#migrate) | 4 |
| [remove](#remove) | 2 |
| [wt checkout](#wt-checkout) | 6 |
| [wt label](#wt-label) | 4 |
| [wt list](#wt-list) | 4 |
| [wt prune](#wt-prune) | 4 |
| [wt repos](#wt-repos) | 4 |
| **Total** | **48** |

## add

| Test | Description |
|------|-------------|
| `TestAdd_RegisterRepo` | Tests registering an existing git repo. |
| `TestAdd_WithLabels` | Tests registering a repo with labels. |
| `TestAdd_DuplicatePath` | Tests that adding the same path twice fails. |
| `TestAdd_NotAGitRepo` | Tests that adding a non-git directory fails. |

## clone

| Test | Description |
|------|-------------|
| `TestClone_RegularRepo` | Tests cloning a regular repository. |
| `TestClone_BareRepo` | Tests cloning a bare repository. |
| `TestClone_WithLabels` | Tests cloning with labels. |
| `TestClone_WithCustomName` | Tests cloning with a custom display name. |
| `TestClone_DestinationExists` | Tests that cloning to an existing path fails. |
| `TestClone_AutoName` | Tests cloning without destination extracts name from URL. |

## forge

| Test | Description |
|------|-------------|
| `TestForge_Check` | Verifies that forge CLI is properly configured |
| `TestForge_GetPRForBranch_Main` | Verifies fetching PR info for the main branch. |
| `TestForge_GetPRForBranch_NonExistent` | Verifies fetching PR info for |
| `TestForge_CloneRepo` | Verifies cloning a repository via forge CLI. |
| `TestForge_CloneRepo_InvalidSpec` | Verifies error handling for invalid repo specs. |
| `TestForge_PRWorkflow` | Verifies the full PR lifecycle: create, view, get branch, merge. |

## hook

| Test | Description |
|------|-------------|
| `TestHook_RunHook` | Tests running a configured hook. |
| `TestHook_UnknownHook` | Tests running an unknown hook. |
| `TestHook_DryRun` | Tests dry-run mode. |
| `TestHook_WithEnvVar` | Tests hook with environment variable. |

## migrate

| Test | Description |
|------|-------------|
| `TestMigrate_ImportRepos` | Tests importing repos from a directory. |
| `TestMigrate_SkipsExisting` | Tests that migrate skips already registered repos. |
| `TestMigrate_DryRun` | Tests dry-run mode. |
| `TestMigrate_NonExistentDir` | Tests migrating from non-existent directory. |

## remove

| Test | Description |
|------|-------------|
| `TestRemove_UnregisterRepo` | Tests unregistering a repo. |
| `TestRemove_NonExistent` | Tests removing a non-existent repo. |

## wt checkout

| Test | Description |
|------|-------------|
| `TestCheckout_ExistingBranch` | Tests checking out an existing branch. |
| `TestCheckout_NewBranch` | Tests creating a new branch. |
| `TestCheckout_ByRepoName` | Tests checkout in a specific repo by name. |
| `TestCheckout_ByLabel` | Tests checkout in repos by label. |
| `TestCheckout_SlashBranchName` | Tests checkout with slash in branch name. |
| `TestCheckout_NotInRepo` | Tests that checkout fails when not in repo and no -r flag. |

## wt label

| Test | Description |
|------|-------------|
| `TestLabel_Add` | Tests adding a label to a repo. |
| `TestLabel_Remove` | Tests removing a label from a repo. |
| `TestLabel_List` | Tests listing labels for a repo. |
| `TestLabel_Clear` | Tests clearing all labels from a repo. |

## wt list

| Test | Description |
|------|-------------|
| `TestList_EmptyRepo` | Tests listing worktrees when none exist. |
| `TestList_WithWorktrees` | Tests listing existing worktrees. |
| `TestList_ByRepoName` | Tests listing worktrees for a specific repo. |
| `TestList_JSON` | Tests JSON output format. |

## wt prune

| Test | Description |
|------|-------------|
| `TestPrune_NoWorktrees` | Tests pruning when no worktrees exist. |
| `TestPrune_WithWorktree` | Tests pruning a worktree. |
| `TestPrune_DryRun` | Tests dry-run mode. |
| `TestPrune_ByRepoName` | Tests pruning in a specific repo. |

## wt repos

| Test | Description |
|------|-------------|
| `TestRepos_ListEmpty` | Tests listing repos when none are registered. |
| `TestRepos_ListRepos` | Tests listing registered repos. |
| `TestRepos_FilterByLabel` | Tests filtering repos by label. |
| `TestRepos_JSON` | Tests JSON output. |


# Test Documentation

Generated: 2026-02-01

## Summary

| Command | Tests |
|---------|-------|
| [forge](#forge) | 6 |
| [hook](#hook) | 4 |
| [repoadd](#repoadd) | 6 |
| [repoclone](#repoclone) | 5 |
| [repolist](#repolist) | 4 |
| [repomakebare](#repomakebare) | 13 |
| [reporemove](#reporemove) | 2 |
| [wt checkout](#wt-checkout) | 6 |
| [wt label](#wt-label) | 4 |
| [wt list](#wt-list) | 4 |
| [wt prune](#wt-prune) | 4 |
| **Total** | **58** |

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

## repoadd

| Test | Description |
|------|-------------|
| `TestRepoAdd_RegisterRepo` | Tests registering an existing git repo. |
| `TestRepoAdd_WithLabels` | Tests registering a repo with labels. |
| `TestRepoAdd_DuplicatePath` | Tests that adding the same path twice fails. |
| `TestRepoAdd_NotAGitRepo` | Tests that adding a non-git directory fails. |
| `TestRepoAdd_MultiplePaths` | Tests adding multiple repos at once. |
| `TestRepoAdd_SkipsNonGitDirs` | Tests that non-git directories are skipped. |

## repoclone

| Test | Description |
|------|-------------|
| `TestRepoClone_BareRepo` | Tests cloning a repository as bare (default behavior). |
| `TestRepoClone_WithLabels` | Tests cloning with labels. |
| `TestRepoClone_WithCustomName` | Tests cloning with a custom display name. |
| `TestRepoClone_DestinationExists` | Tests that cloning to an existing path fails. |
| `TestRepoClone_AutoName` | Tests cloning without destination extracts name from URL. |

## repolist

| Test | Description |
|------|-------------|
| `TestRepoList_ListEmpty` | Tests listing repos when none are registered. |
| `TestRepoList_ListRepos` | Tests listing registered repos. |
| `TestRepoList_FilterByLabel` | Tests filtering repos by label. |
| `TestRepoList_JSON` | Tests JSON output. |

## repomakebare

| Test | Description |
|------|-------------|
| `TestRepoMakeBare_BasicMigration` | Tests basic migration from regular repo to bare-in-.git. |
| `TestRepoMakeBare_WithCustomName` | Tests migration with custom display name. |
| `TestRepoMakeBare_WithLabels` | Tests migration with labels. |
| `TestRepoMakeBare_WithWorktreeFormat` | Tests migration with worktree format. |
| `TestRepoMakeBare_DryRun` | Tests dry run mode. |
| `TestRepoMakeBare_WithExistingWorktrees` | Tests migration with existing worktrees. |
| `TestRepoMakeBare_IsWorktree` | Tests error when path is a worktree. |
| `TestRepoMakeBare_AlreadyBare` | Tests error when repo is already bare-in-.git. |
| `TestRepoMakeBare_NotGitRepo` | Tests error when path is not a git repo. |
| `TestRepoMakeBare_HasSubmodules` | Tests error when repo has submodules. |
| `TestRepoMakeBare_AlreadyRegistered` | Tests migration of already registered repo. |
| `TestRepoMakeBare_NameConflict` | Tests error when name conflicts with existing repo. |
| `TestRepoMakeBare_ByPath` | Tests migration when providing explicit path argument. |

## reporemove

| Test | Description |
|------|-------------|
| `TestRepoRemove_UnregisterRepo` | Tests unregistering a repo. |
| `TestRepoRemove_NonExistent` | Tests removing a non-existent repo. |

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


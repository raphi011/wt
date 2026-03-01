# Test Documentation

Generated: 2026-03-01

## Summary

| Command | Tests |
|---------|-------|
| [completebasebranches](#completebasebranches) | 1 |
| [completebranches](#completebranches) | 1 |
| [completion](#completion) | 3 |
| [confighooks](#confighooks) | 4 |
| [configinit](#configinit) | 1 |
| [configshow](#configshow) | 2 |
| [findignoredfiles](#findignoredfiles) | 1 |
| [findsourceworktree](#findsourceworktree) | 1 |
| [forge](#forge) | 9 |
| [hook](#hook) | 6 |
| [init](#init) | 4 |
| [prcheckout](#prcheckout) | 5 |
| [prcreate](#prcreate) | 3 |
| [preservefiles](#preservefiles) | 1 |
| [prmerge](#prmerge) | 3 |
| [prview](#prview) | 2 |
| [repoadd](#repoadd) | 6 |
| [repoclone](#repoclone) | 10 |
| [repoconvertbare](#repoconvertbare) | 17 |
| [repoconvertregular](#repoconvertregular) | 4 |
| [repolist](#repolist) | 5 |
| [reporemove](#reporemove) | 5 |
| [wt cd](#wt-cd) | 8 |
| [wt checkout](#wt-checkout) | 45 |
| [wt exec](#wt-exec) | 11 |
| [wt label](#wt-label) | 13 |
| [wt list](#wt-list) | 12 |
| [wt note](#wt-note) | 8 |
| [wt prune](#wt-prune) | 18 |
| **Total** | **209** |

## completebasebranches

| Test | Description |
|------|-------------|
| `TestCompleteBaseBranches_WithContext` | Tests that completeBaseBranches uses cmd.Context() |

## completebranches

| Test | Description |
|------|-------------|
| `TestCompleteBranches_WithContext` | Tests that completeBranches uses cmd.Context() |

## completion

| Test | Description |
|------|-------------|
| `TestCompletion_Fish` | Tests that fish completion generation succeeds. |
| `TestCompletion_Bash` | Tests that bash completion generation succeeds. |
| `TestCompletion_Zsh` | Tests that zsh completion generation succeeds. |

## confighooks

| Test | Description |
|------|-------------|
| `TestConfigHooks_NoHooks` | Tests hooks display when none are configured. |
| `TestConfigHooks_WithHooks` | Tests hooks display when hooks are configured. |
| `TestConfigHooks_JSON` | Tests JSON output of hooks. |
| `TestConfigHooks_JSON_Empty` | Tests JSON output when no hooks configured. |

## configinit

| Test | Description |
|------|-------------|
| `TestConfigInit_Stdout` | Tests printing default config to stdout. |

## configshow

| Test | Description |
|------|-------------|
| `TestConfigShow_Basic` | Tests basic config display. |
| `TestConfigShow_JSON` | Tests JSON output of config show. |

## findignoredfiles

| Test | Description |
|------|-------------|
| `TestFindIgnoredFiles` | _No documentation_ |

## findsourceworktree

| Test | Description |
|------|-------------|
| `TestFindSourceWorktree` | _No documentation_ |

## forge

| Test | Description |
|------|-------------|
| `TestForge_Check` | Verifies that forge CLI is properly configured |
| `TestForge_GetPRForBranch_Main` | Verifies fetching PR info for the main branch. |
| `TestForge_GetPRForBranch_NonExistent` | Verifies fetching PR info for |
| `TestForge_CloneRepo` | Verifies cloning a repository via forge CLI. |
| `TestForge_CloneRepo_InvalidSpec` | Verifies error handling for invalid repo specs. |
| `TestForge_ListOpenPRs` | Verifies listing open PRs for a repository. |
| `TestForge_CloneBareRepo` | Verifies cloning a repository as a bare repo. |
| `TestForge_CloneBareRepo_InvalidSpec` | Verifies error handling for invalid repo specs |
| `TestForge_PRWorkflow` | Verifies the full PR lifecycle: create, view, get branch, merge. |

## hook

| Test | Description |
|------|-------------|
| `TestHook_RunHook` | Tests running a configured hook. |
| `TestHook_UnknownHook` | Tests running an unknown hook. |
| `TestHook_DryRun` | Tests dry-run mode. |
| `TestHook_WithEnvVar` | Tests hook with environment variable. |
| `TestHook_WithRepoBranchFormat` | Tests hook with repo:branch format. |
| `TestHook_RepoBranchFormat_BranchNotFound` | Tests error when branch in repo:branch format is not found. |

## init

| Test | Description |
|------|-------------|
| `TestInit_Bash` | Tests that init bash succeeds and outputs a shell wrapper. |
| `TestInit_Zsh` | Tests that init zsh succeeds and outputs a shell wrapper. |
| `TestInit_Fish` | Tests that init fish succeeds and outputs a shell wrapper. |
| `TestInit_UnsupportedShell` | Tests error for an unsupported shell. |

## prcheckout

| Test | Description |
|------|-------------|
| `TestPrCheckout_InvalidPRNumber` | Tests error when first arg is not a valid PR number. |
| `TestPrCheckout_RepoNotFound` | Tests error when specified repo doesn't exist. |
| `TestPrCheckout_InvalidPRNumberWithRepo` | Tests error when second arg is not a valid PR number. |
| `TestPrCheckout_HookNoHookMutuallyExclusive` | Tests that --hook and --no-hook cannot both be used. |
| `TestPrCheckout_OrgRepoAlreadyInRegistry` | Tests error when org/repo format is used |

## prcreate

| Test | Description |
|------|-------------|
| `TestPrCreate_NotInGitRepo` | Tests error when running pr create outside a git repo. |
| `TestPrCreate_RepoNotFound` | Tests error when specified repo doesn't exist. |
| `TestPrCreate_BodyAndBodyFileMutuallyExclusive` | Tests that --body and --body-file cannot both be used. |

## preservefiles

| Test | Description |
|------|-------------|
| `TestPreserveFiles` | _No documentation_ |

## prmerge

| Test | Description |
|------|-------------|
| `TestPrMerge_NotInGitRepo` | Tests error when running pr merge outside a git repo. |
| `TestPrMerge_RepoNotFound` | Tests error when specified repo doesn't exist. |
| `TestPrMerge_HookNoHookMutuallyExclusive` | Tests that --hook and --no-hook cannot both be used. |

## prview

| Test | Description |
|------|-------------|
| `TestPrView_NotInGitRepo` | Tests error when running pr view outside a git repo. |
| `TestPrView_RepoNotFound` | Tests error when specified repo doesn't exist. |

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
| `TestRepoClone_MasterDefaultBranch` | Tests cloning a repo with master as default branch. |
| `TestRepoClone_WithLabels` | Tests cloning with labels. |
| `TestRepoClone_WithCustomName` | Tests cloning with a custom display name. |
| `TestRepoClone_DestinationExists` | Tests that cloning to an existing path fails. |
| `TestRepoClone_AutoName` | Tests cloning without destination extracts name from URL. |
| `TestRepoClone_ShortFormWithoutDefaultOrg` | Tests that short-form without org fails. |
| `TestRepoClone_ShortFormAutoExtractRepoName` | Tests that short-form extracts repo name. |
| `TestRepoClone_ExplicitBranchSkipsAutoDetect` | Tests that -b flag overrides auto-detection. |
| `TestRepoClone_EmptyRepoSkipsWorktree` | Tests that empty repos don't create worktrees. |

## repoconvertbare

| Test | Description |
|------|-------------|
| `TestRepoConvertBare_BasicMigration` | Tests basic migration from regular repo to bare-in-.git. |
| `TestRepoConvertBare_WithCustomName` | Tests migration with custom display name. |
| `TestRepoConvertBare_WithLabels` | Tests migration with labels. |
| `TestRepoConvertBare_WithWorktreeFormat` | Tests migration with worktree format. |
| `TestRepoConvertBare_WithSiblingFormat` | Tests migration with sibling worktree format. |
| `TestRepoConvertBare_SiblingFormatWithExistingWorktrees` | Tests migration with sibling format and existing worktrees. |
| `TestRepoConvertBare_DryRun` | Tests dry run mode. |
| `TestRepoConvertBare_WithExistingWorktrees` | Tests migration with existing worktrees. |
| `TestRepoConvertBare_IsWorktree` | Tests error when path is a worktree. |
| `TestRepoConvertBare_AlreadyBare` | Tests error when repo is already bare-in-.git. |
| `TestRepoConvertBare_NotGitRepo` | Tests error when path is not a git repo. |
| `TestRepoConvertBare_HasSubmodules` | Tests error when repo has submodules. |
| `TestRepoConvertBare_AlreadyRegistered` | Tests migration of already registered repo. |
| `TestRepoConvertBare_NameConflict` | Tests error when name conflicts with existing repo. |
| `TestRepoConvertBare_ByPath` | Tests migration when providing explicit path argument. |
| `TestRepoConvertBare_WorktreeMetadataNameMismatch` | Tests migration when worktree folder name |
| `TestRepoConvertBare_PreservesUpstream` | Tests that upstream tracking is preserved during migration. |

## repoconvertregular

| Test | Description |
|------|-------------|
| `TestRepoConvertRegular_BasicConversion` | Tests basic conversion from bare to regular. |
| `TestRepoConvertRegular_AlreadyRegular` | Tests error when repo is already regular. |
| `TestRepoConvertRegular_RoundTrip` | Tests bare→regular→bare round-trip. |
| `TestRepoConvertRegular_DryRun` | Tests dry run for bare→regular conversion. |

## repolist

| Test | Description |
|------|-------------|
| `TestRepoList_ListEmpty` | Tests listing repos when none are registered. |
| `TestRepoList_ListRepos` | Tests listing registered repos. |
| `TestRepoList_FilterByLabel` | Tests filtering repos by label. |
| `TestRepoList_LabelNotFound` | Tests error when filtering by nonexistent label. |
| `TestRepoList_JSON` | Tests JSON output. |

## reporemove

| Test | Description |
|------|-------------|
| `TestRepoRemove_UnregisterRepo` | Tests unregistering a repo. |
| `TestRepoRemove_NonExistent` | Tests removing a non-existent repo. |
| `TestRepoRemove_OutputShowsCorrectName` | Tests that the output message shows the |
| `TestRepoRemove_DeleteForce` | Tests removing a repo with --delete --force flags. |
| `TestRepoRemove_ByPath` | Tests removing a repo by its full path instead of name. |

## wt cd

| Test | Description |
|------|-------------|
| `TestCd_BranchName` | Tests resolving a worktree by branch name. |
| `TestCd_RepoBranch` | Tests resolving a worktree by repo:branch format. |
| `TestCd_BranchNotFound` | Tests error when branch doesn't exist. |
| `TestCd_RepoNotFound` | Tests error when specified repo doesn't exist. |
| `TestCd_AmbiguousBranch` | Tests error when same branch exists in multiple repos. |
| `TestCd_NoArgs_NoHistory` | Tests error when no args and no history. |
| `TestCd_NoArgs_WithHistory` | Tests returning the most recent worktree. |
| `TestCd_RecordsHistory` | Tests that cd writes to history after resolving a worktree. |

## wt checkout

| Test | Description |
|------|-------------|
| `TestCheckout_ExistingBranch` | Tests checking out an existing branch. |
| `TestCheckout_NewBranch` | Tests creating a new branch. |
| `TestCheckout_ByRepoName` | Tests checkout in a specific repo by name. |
| `TestCheckout_ByLabel` | Tests checkout in repos by label. |
| `TestCheckout_SlashBranchName` | Tests checkout with slash in branch name. |
| `TestCheckout_NotInRepo` | Tests that checkout fails when not in repo. |
| `TestCheckout_NewBranchPushesAndSetsUpstream` | Tests that new branches are pushed and get upstream set. |
| `TestCheckout_ExistingBranchWithRemoteSetsUpstream` | Tests upstream for existing remote branches. |
| `TestCheckout_LocalOnlyBranchNoUpstream` | Tests that local-only branches don't get upstream. |
| `TestCheckout_SetUpstreamDisabled` | Tests that upstream is not set when disabled. |
| `TestCheckout_NoOriginNoUpstream` | Tests checkout works without origin remote. |
| `TestCheckout_AlreadyCheckedOut_ScopedTarget` | Tests checkout blocking with repo:branch syntax. |
| `TestCheckout_AlreadyCheckedOut` | Tests that checkout fails for already checked-out branches. |
| `TestCheckout_BaseBranch` | Tests creating a new branch from a specific base. |
| `TestCheckout_Fetch` | Tests that --fetch fetches before creating branch. |
| `TestCheckout_FetchExistingBranch` | Tests that --fetch fetches the target branch for existing branches. |
| `TestCheckout_FetchWithBase` | Tests that --fetch with --base fetches the specified base branch. |
| `TestCheckout_AutoStash` | Tests that --autostash stashes and applies changes. |
| `TestCheckout_Note` | Tests that --note sets a note on the branch. |
| `TestCheckout_Hook` | Tests that --hook runs a specific hook after checkout. |
| `TestCheckout_NoHook` | Tests that --no-hook skips default hooks. |
| `TestCheckout_HookWithArg` | Tests that --arg passes variables to hooks. |
| `TestCheckout_DefaultHookRuns` | Tests that default hooks run automatically. |
| `TestCheckout_RecordsHistory` | Tests that checkout records to history. |
| `TestCheckout_NewBranchEmptyRepo` | Tests creating a new branch on an empty (no commits) repo. |
| `TestCheckout_NewBranchEmptyRepoWithFetch` | Tests that --fetch is safely skipped on empty repos. |
| `TestCheckout_NewBranchEmptyRepoLocalBaseRef` | Tests empty repo with BaseRef="local" config. |
| `TestCheckout_NewBranchInvalidBaseRef` | Tests that an invalid base ref on a non-empty repo returns an error. |
| `TestCheckout_HistoryEnablesCdNoArgs` | Tests that wt cd (no args) works after checkout. |
| `TestCheckout_ExplicitUpstreamRemoteRef` | Tests --base with upstream/branch syntax. |
| `TestCheckout_LocalBaseRefWithFetchWarning` | Tests that --fetch with local base_ref prints warning. |
| `TestCheckout_ExplicitOriginRemoteRef` | Tests --base with origin/branch syntax. |
| `TestCheckout_PreserveFiles` | Tests that git-ignored files matching preserve patterns |
| `TestCheckout_PreserveNoOverwrite` | Tests that preserved files never overwrite |
| `TestCheckout_NoPreserveFlag` | Tests that --no-preserve skips file preservation. |
| `TestCheckout_PreserveExclude` | Tests that exclude patterns filter out matching path segments. |
| `TestCheckout_AutoStash_NoChanges` | Tests that --autostash with clean working tree succeeds. |
| `TestCheckout_AutoStash_UntrackedFiles` | Tests that untracked files are stashed and popped. |
| `TestCheckout_AutoStash_StagedAndModified` | Tests autostash with a mix of staged and modified files. |
| `TestCheckout_AutoStash_BareInGitRepo` | Tests that --autostash works with bare-in-.git repos. |
| `TestCheckout_AutoStash_BareInGitRepo_NoChanges` | Tests that --autostash with a clean |
| `TestCheckout_AutoStash_NotInTargetRepo` | Tests that --autostash errors when the user |
| `TestCheckout_AutoStash_SecondaryWorktree` | Tests that --autostash works when the user |
| `TestCheckout_AutoStash_Subdirectory` | Tests that --autostash works when the user |
| `TestCheckout_AutoStash_LabelTarget` | Tests that --autostash errors when used |

## wt exec

| Test | Description |
|------|-------------|
| `TestExec_NoCommand` | Tests error when no command is given after --. |
| `TestExec_InCurrentWorktree` | Tests running a command in the current directory. |
| `TestExec_ByBranch` | Tests running a command in a specific worktree by branch name. |
| `TestExec_ByRepoBranch` | Tests running a command with repo:branch targeting. |
| `TestExec_MultipleTargets` | Tests running a command in multiple worktrees. |
| `TestExec_BranchNotFound` | Tests error when target branch doesn't exist. |
| `TestExec_Deduplication` | Tests that the same target is only executed once. |
| `TestExec_NotInGitRepo` | Tests error when running exec with no targets from outside a git repo. |
| `TestExec_FailingCommand` | Tests that a non-zero exit command is handled gracefully. |
| `TestExec_RepoNotFound` | Tests error when targeting a non-existent repo. |
| `TestExec_ByRepoScope` | TestExec_ByLabelScope tests running a command in worktrees matched by label scope. |

## wt label

| Test | Description |
|------|-------------|
| `TestLabel_Add` | Tests adding a label to a repo. |
| `TestLabel_Remove` | Tests removing a label from a repo. |
| `TestLabel_List` | Tests listing labels for a repo. |
| `TestLabel_Clear` | Tests clearing all labels from a repo. |
| `TestLabel_Add_ByLabelScope` | Tests adding a label using a label as scope. |
| `TestLabel_Add_DuplicateLabel` | Tests that adding a label that already exists is idempotent. |
| `TestLabel_Remove_LabelNotFound` | Tests removing a label that doesn't exist on the repo. |
| `TestLabel_List_Global` | Tests listing all labels across repos with --global flag. |
| `TestLabel_List_NoLabels` | Tests listing labels for a repo that has no labels. |
| `TestLabel_List_MultipleRepos` | Tests listing labels for multiple repos at once. |
| `TestLabel_Add_CurrentRepo` | Tests adding a label when no scope is provided. |
| `TestLabel_Remove_NotInGitRepo` | Tests error when removing a label from outside a git repo. |
| `TestLabel_Clear_CurrentRepo` | Tests clearing labels when no scope is provided. |

## wt list

| Test | Description |
|------|-------------|
| `TestList_EmptyRepo` | Tests listing worktrees when none exist. |
| `TestList_WithWorktrees` | Tests listing existing worktrees. |
| `TestList_ByRepoName` | Tests listing worktrees for a specific repo. |
| `TestList_ByLabel` | Tests listing worktrees filtered by label. |
| `TestList_MultipleScopes` | Tests listing worktrees for multiple scopes. |
| `TestList_ScopeNotFound` | Tests error when scope doesn't exist. |
| `TestList_JSON` | Tests JSON output format. |
| `TestList_OrphanedRepoFiltered` | Tests that orphaned repos are silently skipped. |
| `TestList_SortByBranch` | Tests sorting worktrees by branch name. |
| `TestList_SortByRepo` | Tests sorting worktrees by repo name. |
| `TestList_Global` | Tests the --global flag shows all repos. |
| `TestList_GlobalFromNonRepo` | Tests --global from outside any git repo. |

## wt note

| Test | Description |
|------|-------------|
| `TestNoteSet_CurrentBranch` | Tests setting a note on the current branch. |
| `TestNoteGet_CurrentBranch` | Tests getting a note from the current branch. |
| `TestNoteGet_NoNote` | Tests getting a note when none is set. |
| `TestNoteClear_CurrentBranch` | Tests clearing a note from the current branch. |
| `TestNoteSet_ExplicitRepoBranch` | Tests setting a note via repo:branch target. |
| `TestNoteGet_ExplicitBranch` | Tests getting a note via repo:branch target. |
| `TestNote_BranchNotFound` | Tests error when target branch doesn't exist. |
| `TestNoteSet_LabelScope` | Tests setting a note on all repos matching a label. |

## wt prune

| Test | Description |
|------|-------------|
| `TestPrune_NoWorktrees` | Tests pruning when no worktrees exist. |
| `TestPrune_WithWorktree` | Tests pruning a worktree. |
| `TestPrune_DryRun` | Tests dry-run mode. |
| `TestPrune_ByRepoName` | Tests pruning in a specific repo. |
| `TestPrune_WithRepoBranchFormat` | Tests pruning with repo:branch format. |
| `TestPrune_RepoBranchFormat_RepoNotFound` | Tests error when repo in repo:branch format is not found. |
| `TestPrune_DeleteBranchesFlag` | Tests that --delete-branches deletes local branch. |
| `TestPrune_NoDeleteBranchesDefault` | Tests that branches are kept by default. |
| `TestPrune_ConfigDeleteBranches` | Tests that config option enables branch deletion. |
| `TestPrune_NoDeleteBranchesOverridesConfig` | Tests that --no-delete-branches overrides config. |
| `TestPrune_DeleteBranches_UnmergedBranch` | Tests that unmerged branches survive safe delete. |
| `TestPrune_DryRun_DoesNotDeleteBranch` | Tests that dry-run preserves both worktree and branch. |
| `TestPrune_DeleteBranchesFlag_OverridesConfigFalse` | Tests that --delete-branches flag |
| `TestPrune_UnscopedTarget_OnlyCurrentRepo` | Tests that `wt prune feature -f` (without -g) |
| `TestPrune_UnscopedTarget_GlobalFlag` | Tests that `wt prune feature -f -g` |
| `TestPrune_UnscopedTarget_NotInRepo_FallsBackToAll` | Tests that running from a |
| `TestPrune_ForceDeleteBranch_MergedPRState` | Tests that branches with unmerged commits |
| `TestPrune_LocalConfigOverridesDeleteBranches` | Tests that a per-repo .wt.toml |


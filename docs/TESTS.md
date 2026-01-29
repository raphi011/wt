# Test Documentation

Generated: 2026-01-29

## Summary

| Command | Tests |
|---------|-------|
| [forge](#forge) | 6 |
| [wt cd](#wt-cd) | 16 |
| [wt checkout](#wt-checkout) | 26 |
| [wt doctor](#wt-doctor) | 12 |
| [wt exec](#wt-exec) | 16 |
| [wt label](#wt-label) | 25 |
| [wt list](#wt-list) | 21 |
| [wt mv](#wt-mv) | 28 |
| [wt note](#wt-note) | 18 |
| [wt prune](#wt-prune) | 15 |
| [wt repos](#wt-repos) | 15 |
| **Total** | **198** |

## forge

| Test | Description |
|------|-------------|
| `TestForge_Check` | Verifies that forge CLI is properly configured |
| `TestForge_GetPRForBranch_Main` | Verifies fetching PR info for the main branch. |
| `TestForge_GetPRForBranch_NonExistent` | Verifies fetching PR info for |
| `TestForge_CloneRepo` | Verifies cloning a repository via forge CLI. |
| `TestForge_CloneRepo_InvalidSpec` | Verifies error handling for invalid repo specs. |
| `TestForge_PRWorkflow` | Verifies the full PR lifecycle: create, view, get branch, merge. |

## wt cd

| Test | Description |
|------|-------------|
| `TestCd_ByWorktreeID` | Verifies navigating to a worktree by its numeric ID. |
| `TestCd_ByWorktreeIDWithProjectFlag` | Verifies -p flag returns main repo path |
| `TestCd_ByRepoName` | Verifies navigating to a repo by its name using -r flag. |
| `TestCd_ByLabel` | Verifies navigating to a repo by its label using -l flag. |
| `TestCd_MultipleWorktrees` | Verifies ID-based navigation with multiple worktrees |
| `TestCd_ErrorNoTargetSpecified` | Verifies error when no target (ID, repo, or |
| `TestCd_ErrorInvalidID` | Verifies error when specified ID doesn't exist. |
| `TestCd_ErrorRepoNotFound` | Verifies error when specified repo doesn't exist. |
| `TestCd_ErrorLabelNotFound` | Verifies error when no repos match the label. |
| `TestCd_ErrorMultipleReposMatchLabel` | Verifies error when label matches |
| `TestCd_NoHookFlag` | Verifies --no-hook flag skips hook execution. |
| `TestCd_MultipleReposSameWorktreeDir` | Verifies navigation works when multiple |
| `TestCd_WorktreeDirFromEnvOrConfig` | Verifies worktree_dir config is used |
| `TestCd_BranchWithSlashesInName` | Verifies navigation works for branches |
| `TestCd_RepoUsesWorktreeDirIfNoRepoDir` | Verifies -r flag scans worktree_dir |
| `TestCd_LabelUsesWorktreeDirIfNoRepoDir` | Verifies -l flag scans worktree_dir |

## wt checkout

| Test | Description |
|------|-------------|
| `TestCheckout_ExistingBranchInsideRepo` | Verifies worktree creation for an existing |
| `TestCheckout_NewBranchInsideRepo` | Verifies creating a new branch with -b flag |
| `TestCheckout_NewBranchFromCustomBase` | Verifies creating a new branch from a |
| `TestCheckout_WithNote` | Verifies setting a branch note during checkout using |
| `TestCheckout_WorktreeAlreadyExists` | Verifies graceful handling when the target |
| `TestCheckout_MultiRepoByName` | Verifies checking out the same branch in multiple |
| `TestCheckout_MultiRepoByLabel` | Verifies checking out branches in repos matching |
| `TestCheckout_ErrorMissingBranchInsideRepo` | Verifies error when no branch name provided. |
| `TestCheckout_ErrorOutsideRepoWithoutRepoFlag` | Verifies error when running |
| `TestCheckout_ErrorRepoNotFound` | Verifies error when specified repository |
| `TestCheckout_ErrorLabelNotFound` | Verifies error when no repos match |
| `TestCheckout_CustomWorktreeFormat` | Verifies worktree naming respects |
| `TestCheckout_WorktreeDirDefaultsToCurrent` | Verifies that when worktree_dir |
| `TestCheckout_InsideRepoWithRepoFlag` | Verifies that -r flag overrides current |
| `TestCheckout_NewBranchMultiRepo` | Verifies creating new branches simultaneously |
| `TestCheckout_BranchWithSlashes` | Verifies branch names with slashes (e.g., feature/name) |
| `TestCheckout_ErrorBranchDoesNotExist` | Verifies error when checking out a |
| `TestCheckout_ErrorBranchAlreadyCheckedOut` | Verifies graceful handling when |
| `TestCheckout_CombineRepoAndLabel` | Verifies combining -r and -l flags to target |
| `TestCheckout_MultipleLabels` | Verifies targeting repos with multiple labels |
| `TestCheckout_InsideRepoWithLabelOnly` | Verifies that -l flag alone doesn't |
| `TestCheckout_NoHookFlag` | Verifies that --no-hook flag skips hook execution. |
| `TestCheckout_PartialFailureMultiRepo` | Verifies that failures in some repos |
| `TestCheckout_AutoStash` | Verifies --autostash flag stashes changes and applies |
| `TestCheckout_AutoStashNoChanges` | Verifies --autostash is a no-op when there |
| `TestCheckout_AutoStashWithStagedChanges` | Verifies --autostash handles both |

## wt doctor

| Test | Description |
|------|-------------|
| `TestDoctor_NoIssues` | Verifies that doctor reports no issues for healthy setup. |
| `TestDoctor_StaleEntry` | Verifies that doctor detects and fixes stale cache entries |
| `TestDoctor_DuplicateIDs` | Verifies that doctor fixes duplicate worktree IDs |
| `TestDoctor_MissingMetadata` | Verifies that doctor populates missing cache metadata |
| `TestDoctor_OrphanWorktree` | Verifies that doctor adds untracked worktrees |
| `TestDoctor_Reset` | Verifies that --reset rebuilds the cache from scratch |
| `TestDoctor_BrokenGitLink` | Verifies that doctor repairs broken git links |
| `TestDoctor_WorktreeMoved` | Verifies that doctor repairs broken git links |
| `TestDoctor_BothMoved` | Verifies that doctor repairs broken git links |
| `TestDoctor_MultipleRepos` | Verifies doctor handles multiple repos correctly. |
| `TestDoctor_PathMismatch` | Verifies that doctor fixes cache entries with |
| `TestDoctor_EmptyWorktreeDir` | Verifies doctor handles empty directories gracefully. |

## wt exec

| Test | Description |
|------|-------------|
| `TestExec_ByWorktreeID` | Verifies executing a command in a worktree by numeric ID. |
| `TestExec_MultipleIDs` | Verifies executing a command in multiple worktrees by ID. |
| `TestExec_ByRepoName` | Verifies executing a command in a repo by name. |
| `TestExec_ByLabel` | Verifies executing a command in repos matching a label. |
| `TestExec_MultipleRepos` | Verifies executing a command in multiple repos by name. |
| `TestExec_MultipleLabels` | Verifies executing a command in repos matching multiple labels. |
| `TestExec_CombineRepoAndLabel` | Verifies combining -r and -l flags. |
| `TestExec_ErrorNoTargetSpecified` | Verifies error when no target is specified. |
| `TestExec_ErrorNoCommandSpecified` | Verifies error when no command is provided. |
| `TestExec_ErrorInvalidID` | Verifies error when specified ID doesn't exist. |
| `TestExec_ErrorRepoNotFound` | Verifies error when specified repo doesn't exist. |
| `TestExec_ErrorLabelNotFound` | Verifies error when no repos match the label. |
| `TestExec_CommandFails` | Verifies error propagation when command fails. |
| `TestExec_PartialFailure` | Verifies error reporting when command fails in some worktrees. |
| `TestExec_CommandWithArguments` | Verifies command execution with multiple arguments. |
| `TestExec_StripLeadingDashes` | Verifies that leading "--" is stripped from commands. |

## wt label

| Test | Description |
|------|-------------|
| `TestLabelAdd_CurrentRepo` | Verifies adding a label to the current repo |
| `TestLabelAdd_ByRepoName` | Verifies adding a label using -r flag |
| `TestLabelAdd_MultipleRepos` | Verifies adding the same label to multiple repos |
| `TestLabelAdd_DuplicateIsIdempotent` | Verifies that adding an existing label |
| `TestLabelAdd_AddsToExistingLabels` | Verifies that adding a label preserves |
| `TestLabelAdd_ErrorOutsideRepoWithoutFlag` | Verifies error when running |
| `TestLabelAdd_ErrorRepoNotFound` | Verifies error when specified repo |
| `TestLabelRemove_CurrentRepo` | Verifies removing a label from the current repo |
| `TestLabelRemove_ByRepoName` | Verifies removing a label using -r flag |
| `TestLabelRemove_MultipleRepos` | Verifies removing a label from multiple repos |
| `TestLabelRemove_NonexistentLabel` | Verifies that removing a label that |
| `TestLabelRemove_PreservesOtherLabels` | Verifies that removing a label |
| `TestLabelList_CurrentRepo` | Verifies listing labels from the current repo |
| `TestLabelList_ByRepoName` | Verifies listing labels using -r flag |
| `TestLabelList_MultipleRepos` | Verifies listing labels from multiple repos |
| `TestLabelList_Global` | Verifies listing labels from all repos using -g flag. |
| `TestLabelList_NoLabels` | Verifies that listing labels on repo with no labels |
| `TestLabelList_ErrorOutsideRepoWithoutFlag` | Verifies error when running |
| `TestLabelClear_CurrentRepo` | Verifies clearing all labels from the current repo |
| `TestLabelClear_ByRepoName` | Verifies clearing labels using -r flag |
| `TestLabelClear_MultipleRepos` | Verifies clearing labels from multiple repos |
| `TestLabelClear_NoLabels` | Verifies that clearing labels on repo with no labels |
| `TestLabelClear_ErrorOutsideRepoWithoutFlag` | Verifies error when running |
| `TestLabelAdd_InsideWorktree` | Verifies that running label add from inside |
| `TestLabelList_InsideWorktree` | Verifies that running label list from inside |

## wt list

| Test | Description |
|------|-------------|
| `TestList_InsideRepo` | Verifies that listing worktrees from inside a repo |
| `TestList_OutsideRepo` | Verifies that listing worktrees from outside any repo |
| `TestList_Global` | Verifies that -g flag lists worktrees from all repos |
| `TestList_NoWorktrees` | Verifies that listing worktrees in empty directory |
| `TestList_MultipleWorktreesSameRepo` | Verifies listing multiple worktrees |
| `TestList_FilterByRepo` | Verifies that -r flag filters worktrees |
| `TestList_FilterByMultipleRepos` | Verifies filtering by multiple repos |
| `TestList_FilterByRepoOverridesCurrentRepo` | Verifies that -r flag overrides |
| `TestList_FilterByRepoNotFound` | Verifies that filtering by non-existent repo |
| `TestList_FilterByLabel` | Verifies that -l flag filters worktrees |
| `TestList_FilterByMultipleLabels` | Verifies filtering by multiple labels |
| `TestList_FilterByLabelOverridesCurrentRepo` | Verifies that -l flag overrides |
| `TestList_FilterByLabelNotFound` | Verifies that filtering by non-existent label |
| `TestList_CombineRepoAndLabel` | Verifies combining -r and -l flags (OR condition). |
| `TestList_FilterFromMultipleRepos` | Verifies filtering shows all worktrees |
| `TestList_SortByID` | Verifies that -s id sorts worktrees by ID in ascending order. |
| `TestList_SortByRepo` | Verifies that -s repo sorts worktrees alphabetically by repo name. |
| `TestList_SortByBranch` | Verifies that -s branch sorts worktrees alphabetically by branch name. |
| `TestList_SortByCommit` | Verifies that -s commit sorts worktrees by last commit date |
| `TestList_JSONOutput` | Verifies that --json flag outputs valid JSON with required fields. |
| `TestList_JSONOutputMultiple` | Verifies that --json outputs multiple worktrees |

## wt mv

| Test | Description |
|------|-------------|
| `TestMv_MoveWorktreesToWorktreeDir` | Verifies that worktrees and their repos |
| `TestMv_MoveReposToRepoDir` | Verifies that repos are moved to repo_dir |
| `TestMv_MoveReposToWorktreeDirWhenNoRepoDir` | Verifies that repos are moved |
| `TestMv_CollisionAddsNumberedSuffix` | Verifies that when the destination path |
| `TestMv_MultipleCollisionsIncrementSuffix` | Verifies that collision suffixes |
| `TestMv_SkipRepoIfTargetExists` | Verifies that repos are skipped (not moved) |
| `TestMv_DryRunDoesNotMove` | Verifies that --dry-run previews moves |
| `TestMv_FilterByRepository` | Verifies that -r flag filters which repos are moved. |
| `TestMv_MovesDirtyWorktree` | Verifies that worktrees with uncommitted changes |
| `TestMv_SkipIfAlreadyAtDestination` | Verifies that worktrees already at the |
| `TestMv_WorktreeReferenceUpdatedAfterRepoMove` | Verifies that worktree .git files |
| `TestMv_MultipleWorktreesFromSameRepo` | Verifies that all worktrees from |
| `TestMv_EmptyDirectory` | Verifies that running mv on an empty directory |
| `TestMv_DestinationDoesNotExist` | Verifies that mv fails with error when |
| `TestMv_NoWorktreeDirConfigured` | Verifies that mv fails with error when |
| `TestMv_CustomFormatRenamesWorktree` | Verifies that --format flag renames |
| `TestMv_FolderFormatPlaceholder` | Verifies that {repo} placeholder uses |
| `TestMv_NestedWorktreeMovedToWorktreeDir` | Verifies that worktrees nested |
| `TestMv_MultipleNestedWorktrees` | Verifies that multiple worktrees nested |
| `TestMv_MixedNestedAndExternalWorktrees` | Verifies that both nested worktrees |
| `TestMv_NestedDirtyWorktreeMoved` | Verifies that nested worktrees with |
| `TestMv_FormatChangeRenamesWorktreesInPlace` | Verifies that worktrees already |
| `TestMv_FormatChangeWithCollision` | Verifies that format changes that would |
| `TestMv_PathArgumentSingleWorktree` | Verifies that PATH argument moves only |
| `TestMv_PathArgumentSingleRepo` | Verifies that PATH argument for a repo moves |
| `TestMv_PathArgumentFolder` | Verifies that PATH argument for a folder moves |
| `TestMv_PathArgumentRepoWithNestedWorktree` | Verifies that PATH argument for a repo |
| `TestMv_PathDoesNotExist` | Verifies that mv fails with error when |

## wt note

| Test | Description |
|------|-------------|
| `TestNoteSet_ByID` | Verifies setting a branch note by worktree ID. |
| `TestNoteSet_ByRepoName` | Verifies setting a branch note by repo name. |
| `TestNoteSet_InsideWorktree` | Verifies setting a note when running from inside a worktree. |
| `TestNoteSet_InsideMainRepo` | Verifies setting a note when running from inside main repo. |
| `TestNoteSet_OverwriteExisting` | Verifies that setting a note overwrites existing note. |
| `TestNoteGet_ByID` | Verifies retrieving a branch note by worktree ID. |
| `TestNoteGet_ByRepoName` | Verifies retrieving a branch note by repo name. |
| `TestNoteGet_InsideWorktree` | Verifies getting a note from inside a worktree. |
| `TestNoteGet_NoNoteExists` | Verifies graceful handling when no note exists. |
| `TestNoteGet_DefaultSubcommand` | Verifies that "get" is the default subcommand. |
| `TestNoteClear_ByID` | Verifies clearing a branch note by worktree ID. |
| `TestNoteClear_ByRepoName` | Verifies clearing a branch note by repo name. |
| `TestNoteClear_InsideWorktree` | Verifies clearing a note from inside a worktree. |
| `TestNoteClear_NonExistentNote` | Verifies graceful handling when clearing a non-existent note. |
| `TestNoteGet_ErrorInvalidID` | Verifies error when specified ID doesn't exist. |
| `TestNoteGet_ErrorRepoNotFound` | Verifies error when specified repo doesn't exist. |
| `TestNoteSet_ErrorInvalidID` | Verifies error when specified ID doesn't exist. |
| `TestNoteClear_ErrorInvalidID` | Verifies error when specified ID doesn't exist. |

## wt prune

| Test | Description |
|------|-------------|
| `TestPrune_NoPR_NotRemoved` | Verifies that locally merged branches without PRs |
| `TestPrune_SkipsDirty` | Verifies that dirty worktrees are skipped during auto-prune. |
| `TestPrune_SkipsUnmergedWithCommits` | Verifies that branches with unmerged commits |
| `TestPrune_CleanBranch_NotRemoved` | Verifies that clean branches (0 commits ahead) |
| `TestPrune_DryRun` | Verifies --dry-run flag shows what would be pruned without |
| `TestPrune_ByID_RequiresForce` | Verifies that pruning by ID requires -f flag for safety. |
| `TestPrune_MultipleIDs` | Verifies pruning multiple worktrees by ID. |
| `TestPrune_InsideRepoOnly_NoPR` | Verifies that without -g flag, prune only |
| `TestPrune_Global_NoPR` | Verifies that -g flag considers all repos. |
| `TestPrune_ErrorForceWithoutID` | Verifies error when -f is used without -n. |
| `TestPrune_ErrorVerboseWithID` | Verifies error when --verbose is used with -n. |
| `TestPrune_ErrorInvalidID` | Verifies error when specified ID doesn't exist. |
| `TestPrune_ErrorResetCacheWithID` | Verifies error when --reset-cache is used with -n. |
| `TestPrune_ByID_DryRun` | Verifies dry run with -n flag doesn't remove worktree. |
| `TestPrune_ForceRemovesDirty` | Verifies that -f flag removes dirty worktrees. |

## wt repos

| Test | Description |
|------|-------------|
| `TestRepos_ListsReposInDirectory` | Verifies that `wt repos` lists all repositories |
| `TestRepos_SingleRepo` | Verifies listing a single repository with count display. |
| `TestRepos_NoReposFound` | Verifies message when no repos exist in directory. |
| `TestRepos_FilterByLabel` | Verifies filtering repos by label with -l flag. |
| `TestRepos_FilterByLabel_NoMatch` | Verifies message when no repos match label filter. |
| `TestRepos_SortByName` | Verifies sorting repos alphabetically by name. |
| `TestRepos_SortByBranch` | Verifies sorting repos by current branch name. |
| `TestRepos_SortByWorktrees` | Verifies sorting repos by worktree count descending. |
| `TestRepos_SortByLabel` | Verifies sorting repos by label alphabetically (unlabeled last). |
| `TestRepos_JSONOutput` | Verifies JSON output contains all required fields. |
| `TestRepos_JSONOutputEmpty` | Verifies JSON output is empty array (not null) when no repos. |
| `TestRepos_ShowsCorrectWorktreeCount` | Verifies worktree count is accurate. |
| `TestRepos_ZeroWorktreesForRepoWithoutWorktrees` | Verifies worktree count is 0 |
| `TestRepos_ShowsMultipleLabels` | Verifies multiple labels are shown in JSON output. |
| `TestRepos_TableShowsLabels` | Verifies labels are shown in table output. |


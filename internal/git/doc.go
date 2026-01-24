// Package git provides git operations via shell commands.
//
// All operations use [os/exec.Command] to call the git CLI directly rather than
// using Go git libraries. This approach is simpler, more reliable, and ensures
// compatibility with user configurations (SSH keys, credential helpers, aliases).
//
// # Worktree Operations
//
// Core worktree management:
//
//   - [AddWorktree]: Create worktrees for new or existing branches
//   - [RemoveWorktree]: Remove worktrees with optional force flag
//   - [MoveWorktree]: Relocate worktrees to new paths
//   - [ListWorktrees]: Scan directory for worktrees with batched git calls
//   - [GetWorktreeInfo]: Get detailed info for a single worktree
//
// # Repository Operations
//
// Repository and branch queries:
//
//   - [GetOriginURL], [GetRepoName]: Extract repository information
//   - [GetCurrentBranch], [BranchExists]: Branch operations
//   - [IsBranchMerged], [GetCommitCount]: Merge status checks
//   - [GetDefaultBranch]: Detect main/master branch
//
// # Repository Discovery
//
// Finding repositories by name or label:
//
//   - [FindRepoByName]: Search for repo by name in a directory
//   - [FindAllRepos]: List all git repos in a directory
//   - [FindSimilarRepos]: Suggest similar names when repo not found
//
// # Link Validation and Repair
//
// Git worktrees have bidirectional links between the worktree's .git file
// and the main repo's .git/worktrees/ directory. These can break when repos
// are moved. Functions for diagnosis and repair:
//
//   - [IsWorktreeLinkValid]: Check if links are intact
//   - [CanRepairWorktree]: Check if repair is possible
//   - [RepairWorktree]: Fix broken links via "git worktree repair"
//   - [ListPrunableWorktrees]: Find stale git references
//
// # Batching for Performance
//
// [ListWorktrees] uses batched git calls to minimize subprocess overhead:
// one "git worktree list --porcelain" per repo instead of individual calls
// per worktree. For 10 worktrees across 2 repos, this reduces calls from
// ~40 to ~8 (or ~18 with dirty checks enabled).
package git

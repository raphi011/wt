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
//   - [LoadWorktreesForRepos]: Load worktrees from multiple repos in parallel
//
// # Repository Operations
//
// Repository and branch queries:
//
//   - [GetOriginURL], [GetRepoName]: Extract repository information
//   - [GetCurrentBranch], [BranchExists]: Branch operations
//   - [GetCommitCount]: Commits ahead of default branch
//   - [GetDefaultBranch]: Detect main/master branch
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
package git

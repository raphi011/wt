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
//   - [CreateWorktree], [CreateWorktreeNewBranch]: Create worktrees for existing or new branches
//   - [RemoveWorktree]: Remove worktrees with optional force flag
//   - [PruneWorktrees]: Clean up stale worktree references
//   - [LoadWorktreesForRepos]: Load worktrees from multiple repos in parallel
//
// # Repository Operations
//
// Repository and branch queries:
//
//   - [GetOriginURL], [GetRepoDisplayName]: Extract repository information
//   - [GetCurrentBranch], [LocalBranchExists], [RemoteBranchExists], [RefExists]: Branch operations
//   - [GetCommitMeta]: Batch-fetch commit metadata (age, timestamp) for SHAs
//   - [GetDefaultBranch]: Detect main/master branch
//   - [CloneRegular], [CloneBareWithWorktreeSupport]: Clone repositories
//
// # Migration
//
// Convert between regular and bare-in-.git repository formats:
//
//   - [ValidateMigration], [MigrateToBare]: Regular → bare conversion
//   - [ValidateMigrationToRegular], [MigrateToRegular]: Bare → regular conversion
package git

// Package resolve provides target resolution for worktree commands.
//
// Many wt commands operate on specific worktrees or repos. This package
// handles looking up targets and validating they still exist on disk.
//
// # Targeting Methods
//
// Commands support several targeting methods:
//
//   - -r/--repository: Target by repository name
//   - -l/--label: Target repos by label
//   - --branch: Target specific worktree by branch name
//   - -i/--interactive: Interactive selection
//
// # Usage Pattern
//
// Commands that use target resolution follow this pattern:
//
//   - Repo targeting: wt exec -r myrepo, wt cd -r myrepo
//   - Label targeting: wt exec -l backend, wt hook -l frontend
//   - Branch targeting: wt prune --branch feature, wt hook --branch feat
//   - Context-aware: wt note, wt pr create (default to current repo/branch)
//
// The -r/--repository and -l/--label flags are used consistently across
// commands for repo-based targeting.
package resolve

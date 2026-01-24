// Package hooks provides post-operation hook execution with placeholder substitution.
//
// Hooks are shell commands defined in config that run after wt operations like
// add, pr checkout, prune, or merge. They enable workflow automation such as
// opening editors, installing dependencies, or sending notifications.
//
// # Hook Selection
//
// Hooks can run automatically or manually:
//
//   - Automatic: Hooks with "on" config matching the command type run automatically
//   - Manual: Use --hook=name to run a specific hook, --no-hook to skip all
//
// Example config:
//
//	[hooks.vscode]
//	command = "code {path}"
//	on = ["add", "pr"]  # auto-run for add and pr commands
//
//	[hooks.cleanup]
//	command = "echo 'Done with {branch}'"
//	# no "on" - only runs via --hook=cleanup
//
// # Placeholder Substitution
//
// Static placeholders available in all hooks:
//
//   - {path}: Absolute worktree path
//   - {branch}: Branch name
//   - {repo}: Repository name from git origin
//   - {folder}: Main repo folder name
//   - {main-repo}: Main repo path
//   - {trigger}: Command that triggered the hook (add, pr, prune, merge)
//
// Custom variables via --arg key=value:
//
//   - {key}: Value from --arg key=value
//   - {key:-default}: Value with fallback if not provided
//
// # Execution Context
//
// Hooks run with the working directory set to:
//   - Worktree path for add/pr hooks
//   - Main repo path for prune hooks (worktree is deleted)
//
// Hook failures are logged but don't stop batch operations (RunForEach).
// Use [RunAll] when hook failure should stop execution.
//
// # Stdin Support
//
// Use --arg key=- to read stdin content into a variable:
//
//	echo "my content" | wt hook myhook --arg content=-
package hooks

// Package hooks provides hook execution with placeholder substitution.
//
// Hooks are shell commands defined in config that run before or after wt operations
// like checkout, pr checkout, prune, or merge. They enable workflow automation such as
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
//	command = "code {worktree-dir}"
//	on = ["checkout"]  # auto-run for checkout commands
//
//	[hooks.cleanup]
//	command = "echo 'Done with {branch}'"
//	# no "on" - only runs via --hook=cleanup
//
// # Execution Order
//
// Hooks run in alphabetical order by name. Use naming prefixes to control ordering
// (e.g., "01-install", "02-lint", "99-editor").
//
// # Placeholder Substitution
//
// Static placeholders available in all hooks:
//
//   - {worktree-dir}: Absolute worktree path
//   - {repo-dir}: Absolute main repo path
//   - {branch}: Branch name
//   - {repo}: Registered repo name (as shown in wt repo list)
//   - {trigger}: Command that triggered the hook (checkout, prune, merge, run)
//   - {action}: Checkout subtype: create, open, pr, or manual (for wt hook)
//   - {phase}: Hook timing: before or after
//
// Custom variables via --arg key=value or --arg key (bare boolean):
//
//   - {key}: Value from --arg key=value
//   - {key:-default}: Value with fallback if not provided
//   - {key:+text}: Expands to text if key is set and non-empty, otherwise empty
//
// # Execution Context
//
// Hooks run with the working directory set to:
//   - Worktree path for checkout hooks (both before and after)
//   - Repo root for pr checkout hooks (both before and after)
//   - Worktree path for before:prune hooks (worktree still exists)
//   - Main repo path for after:prune hooks (worktree is deleted)
//   - Repo root for merge hooks (both before and after)
//
// Before hooks abort the operation on failure. After hook failures are logged
// but don't stop batch operations ([RunForEach]).
// Use [RunSingle] for individual hook execution where errors are returned to the caller.
//
// # Stdin Support
//
// Use --arg key=- to read stdin content into a variable:
//
//	echo "my content" | wt hook myhook --arg content=-
package hooks

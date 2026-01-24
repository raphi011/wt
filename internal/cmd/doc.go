// Package cmd provides helpers for executing shell commands with proper error handling.
//
// This package wraps [os/exec.Cmd] to capture stderr and include it in error
// messages, making command failures more informative for users.
//
// # Usage
//
//	cmd := exec.Command("git", "status")
//	if err := cmd.Run(cmd); err != nil {
//	    // err contains stderr output if available
//	    return fmt.Errorf("git failed: %w", err)
//	}
//
//	// For commands that return output:
//	output, err := cmd.Output(exec.Command("git", "branch"))
//	if err != nil {
//	    // err contains stderr output
//	}
//
// # Design Notes
//
// The wt tool shells out to git/gh/glab CLIs rather than using Go libraries.
// This approach is simpler, more reliable, and ensures compatibility with
// user configurations (SSH keys, credential helpers, etc.).
package cmd

//go:build windows

package hooks

// shellCommand returns the shell and arguments for running a command string.
func shellCommand(command string) (string, []string) {
	return "cmd", []string{"/c", command}
}

package git

import (
	"os/exec"

	"github.com/raphi011/wt/internal/cmd"
)

// runCmd executes a command and returns stderr in the error message if it fails
func runCmd(c *exec.Cmd) error {
	return cmd.Run(c)
}

// outputCmd executes a command and returns stdout, with stderr in error if it fails
func outputCmd(c *exec.Cmd) ([]byte, error) {
	return cmd.Output(c)
}

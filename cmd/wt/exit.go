package main

import "errors"

// errCancelled is returned when the user cancels an interactive prompt whose
// result is consumed by a shell (e.g. cd $(wt cd -i)). It exits with status 1
// without printing an error.
var errCancelled = errors.New("cancelled")

// commandExitError carries an exit status through Cobra while preserving the cause.
type commandExitError struct {
	err  error
	code int
}

func (e *commandExitError) Error() string { return e.err.Error() }
func (e *commandExitError) Unwrap() error { return e.err }

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*commandExitError](err); ok {
		return exitErr.code
	}
	return 1
}

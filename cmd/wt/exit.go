package main

import "errors"

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
	var exitErr *commandExitError
	if errors.As(err, &exitErr) {
		return exitErr.code
	}
	return 1
}

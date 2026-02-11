package main

import (
	"fmt"
	"runtime"
)

// Version information - set by goreleaser
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	Execute()
}

// versionString returns the version string.
func versionString() string {
	return fmt.Sprintf("wt %s (%s, %s, %s)", version, commit[:min(7, len(commit))], date, runtime.Version())
}

package main

import "fmt"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func (Args) Version() string {
	return fmt.Sprintf("wt %s (%s, %s)", version, commit[:min(7, len(commit))], date)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

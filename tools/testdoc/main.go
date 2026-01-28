// Package main provides a CLI tool to generate markdown documentation
// from Go test functions and their doc comments.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	var (
		rootDir         string
		outputFile      string
		integrationOnly bool
	)

	flag.StringVar(&rootDir, "root", ".", "root directory to scan for test files")
	flag.StringVar(&outputFile, "out", "docs/TESTS.md", "output markdown file")
	flag.BoolVar(&integrationOnly, "integration", false, "only include integration tests (*_integration_test.go)")
	flag.Parse()

	// Resolve root directory to absolute path
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving root directory: %v\n", err)
		os.Exit(1)
	}

	// Parse all test files
	packages, err := ParseTestFiles(absRoot, integrationOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing test files: %v\n", err)
		os.Exit(1)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Render to markdown
	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := RenderMarkdown(f, packages); err != nil {
		fmt.Fprintf(os.Stderr, "error rendering markdown: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s with %d packages\n", outputFile, len(packages))
}

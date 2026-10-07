package main

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

// CommandTests groups tests by command.
type CommandTests struct {
	Command string
	Tests   []TestFunc
}

// RenderMarkdown writes the test documentation as markdown.
func RenderMarkdown(w io.Writer, packages []TestPackage) error {
	// Header
	if _, err := fmt.Fprintf(w, "# Test Documentation\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Generated: %s\n\n", time.Now().Format("2006-01-02")); err != nil {
		return err
	}

	// Collect all tests and group by command
	commandMap := make(map[string][]TestFunc)

	for _, pkg := range packages {
		for _, file := range pkg.Files {
			for _, test := range file.Tests {
				cmd := extractCommand(test.Name)
				commandMap[cmd] = append(commandMap[cmd], test)
			}
		}
	}

	// Sort commands
	var commands []string
	for cmd := range commandMap {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)

	// Summary
	if _, err := fmt.Fprintf(w, "## Summary\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| Command | Tests |\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "|---------|-------|\n"); err != nil {
		return err
	}

	totalTests := 0
	for _, cmd := range commands {
		tests := commandMap[cmd]
		if _, err := fmt.Fprintf(w, "| [%s](#%s) | %d |\n", cmd, toAnchor(cmd), len(tests)); err != nil {
			return err
		}
		totalTests += len(tests)
	}
	if _, err := fmt.Fprintf(w, "| **Total** | **%d** |\n\n", totalTests); err != nil {
		return err
	}

	// Render each command section
	for _, cmd := range commands {
		tests := commandMap[cmd]
		if err := renderCommandSection(w, cmd, tests); err != nil {
			return err
		}
	}

	return nil
}

func renderCommandSection(w io.Writer, cmd string, tests []TestFunc) error {
	if _, err := fmt.Fprintf(w, "## %s\n\n", cmd); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| Test | Description |\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "|------|-------------|\n"); err != nil {
		return err
	}

	for _, test := range tests {
		desc := extractDescription(test.Doc, test.Name)
		// Escape pipes in description for markdown table
		desc = strings.ReplaceAll(desc, "|", "\\|")
		if _, err := fmt.Fprintf(w, "| `%s` | %s |\n", test.Name, desc); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "\n")
	return err
}

// extractCommand extracts the command name from a test function name.
// Examples:
//   - TestCd_BasicNavigation -> wt cd
//   - TestCheckout_NewBranch -> wt checkout
//   - TestListWorktrees_Empty -> wt list
//   - TestForge_DetectGitHub -> forge
func extractCommand(testName string) string {
	// Remove "Test" prefix
	name := strings.TrimPrefix(testName, "Test")

	// Find the command part (before first underscore)
	parts := strings.SplitN(name, "_", 2)
	if len(parts) == 0 {
		return "other"
	}

	cmd := parts[0]

	// Map test prefixes to commands
	cmdMap := map[string]string{
		"Cd":            "wt cd",
		"Checkout":      "wt checkout",
		"Exec":          "wt exec",
		"Label":         "wt label",
		"LabelAdd":      "wt label",
		"LabelClear":    "wt label",
		"LabelList":     "wt label",
		"LabelRemove":   "wt label",
		"List":          "wt list",
		"ListWorktrees": "wt list",
		"Mv":            "wt mv",
		"Note":          "wt note",
		"NoteSet":       "wt note",
		"NoteGet":       "wt note",
		"NoteClear":     "wt note",
		"Prune":         "wt prune",
		"Repo":          "wt repo",
		"Forge":         "forge",
		"GitHub":        "forge",
		"MRCache":       "forge",
	}

	if mapped, ok := cmdMap[cmd]; ok {
		return mapped
	}

	// Default: lowercase the command
	return strings.ToLower(cmd)
}

// extractDescription gets the first line of the doc comment as description.
// It strips the test function name from the beginning if present.
func extractDescription(doc string, testName string) string {
	if doc == "" {
		return "_No documentation_"
	}

	lines := strings.SplitSeq(doc, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Strip test name prefix if present (e.g., "TestCd_ByID verifies..." -> "verifies...")
			line = strings.TrimPrefix(line, testName+" ")
			// Capitalize first letter
			if len(line) > 0 {
				line = strings.ToUpper(line[:1]) + line[1:]
			}
			return line
		}
	}

	return "_No documentation_"
}

// toAnchor converts a command name to a markdown anchor.
func toAnchor(cmd string) string {
	// Replace spaces with hyphens and lowercase
	anchor := strings.ReplaceAll(cmd, " ", "-")
	// Remove special characters
	re := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	anchor = re.ReplaceAllString(anchor, "")
	return strings.ToLower(anchor)
}

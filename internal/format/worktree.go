package format

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultWorktreeFormat is the default format for worktree folder names
const DefaultWorktreeFormat = "{repo}-{branch}"

// ValidPlaceholders lists all supported placeholders
var ValidPlaceholders = []string{"{repo}", "{branch}", "{folder}"}

// FormatParams contains the values for placeholder substitution
type FormatParams struct {
	GitOrigin  string // repo name from git remote get-url origin
	BranchName string // branch name as provided
	FolderName string // actual folder name of git repo on disk
}

// placeholderRegex matches {placeholder-name} patterns
var placeholderRegex = regexp.MustCompile(`\{[a-z-]+\}`)

// ValidateFormat checks if a format string is valid
// Returns error if format contains unknown placeholders
func ValidateFormat(format string) error {
	matches := placeholderRegex.FindAllString(format, -1)
	for _, match := range matches {
		if !isValidPlaceholder(match) {
			return fmt.Errorf("unknown placeholder %q in format %q (valid: %s)",
				match, format, strings.Join(ValidPlaceholders, ", "))
		}
	}

	// Check that at least one placeholder is present
	hasPlaceholder := false
	for _, p := range ValidPlaceholders {
		if strings.Contains(format, p) {
			hasPlaceholder = true
			break
		}
	}
	if !hasPlaceholder {
		return fmt.Errorf("format %q must contain at least one placeholder (%s)",
			format, strings.Join(ValidPlaceholders, ", "))
	}

	return nil
}

// isValidPlaceholder checks if a placeholder is in the valid list
func isValidPlaceholder(placeholder string) bool {
	for _, valid := range ValidPlaceholders {
		if placeholder == valid {
			return true
		}
	}
	return false
}

// FormatWorktreeName applies the format template to generate a worktree folder name
func FormatWorktreeName(format string, params FormatParams) string {
	result := format
	result = strings.ReplaceAll(result, "{repo}", SanitizeForPath(params.GitOrigin))
	result = strings.ReplaceAll(result, "{branch}", SanitizeForPath(params.BranchName))
	result = strings.ReplaceAll(result, "{folder}", SanitizeForPath(params.FolderName))
	return result
}

// SanitizeForPath replaces characters that are problematic in file paths
// Replaces: / \ : * ? " < > | with -
func SanitizeForPath(name string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return replacer.Replace(name)
}

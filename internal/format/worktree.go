package format

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// DefaultWorktreeFormat is the default format for worktree folder names
const DefaultWorktreeFormat = "{repo}-{branch}"

// ValidPlaceholders lists all supported placeholders
var ValidPlaceholders = []string{"{repo}", "{branch}", "{origin}"}

// FormatParams contains the values for placeholder substitution
type FormatParams struct {
	RepoName   string // folder name of git repo
	BranchName string // branch name as provided
	Origin     string // repo name from git origin URL (falls back to RepoName if empty)
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
	return slices.Contains(ValidPlaceholders, placeholder)
}

// FormatWorktreeName applies the format template to generate a worktree folder name
func FormatWorktreeName(format string, params FormatParams) string {
	result := format
	result = strings.ReplaceAll(result, "{repo}", SanitizeForPath(params.RepoName))
	result = strings.ReplaceAll(result, "{branch}", SanitizeForPath(params.BranchName))
	result = strings.ReplaceAll(result, "{origin}", SanitizeForPath(params.Origin))
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

// PathExistsFunc is a function that checks if a path exists.
// Used for dependency injection in tests.
type PathExistsFunc func(path string) bool

// UniqueWorktreePath returns a unique path by appending a numbered suffix if needed.
// If basePath doesn't exist, it returns basePath unchanged.
// If basePath exists, it tries basePath-1, basePath-2, etc. until finding a unique path.
func UniqueWorktreePath(basePath string, exists PathExistsFunc) string {
	if !exists(basePath) {
		return basePath
	}

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d", basePath, i)
		if !exists(candidate) {
			return candidate
		}
	}
}

// RelativeTime formats a time as a human-readable relative string.
// Returns "Xs ago", "Xm ago", "Xh ago", "yesterday", or date for older.
func RelativeTime(t time.Time) string {
	return RelativeTimeFrom(t, time.Now())
}

// RelativeTimeFrom formats a time relative to a reference time.
// Useful for testing with a fixed "now".
func RelativeTimeFrom(t, now time.Time) string {
	diff := now.Sub(t)

	if diff < time.Minute {
		secs := int(diff.Seconds())
		if secs <= 1 {
			return "just now"
		}
		return fmt.Sprintf("%ds ago", secs)
	}

	if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	}

	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	}

	// Check if yesterday (same calendar day - 1)
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	daysDiff := int(nowDate.Sub(tDate).Hours() / 24)

	if daysDiff == 1 {
		return "yesterday"
	}

	if daysDiff < 7 {
		return fmt.Sprintf("%dd ago", daysDiff)
	}

	return t.Format("2006-01-02")
}

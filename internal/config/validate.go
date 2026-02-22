package config

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Valid enum values for configuration fields.
var (
	ValidForgeTypes       = []string{"github", "gitlab"}
	ValidMergeStrategies  = []string{"squash", "rebase", "merge"}
	ValidBaseRefs         = []string{"local", "remote"}
	ValidDefaultSortModes = []string{"created", "repo", "branch"}
)

// validateEnum checks that value (if non-empty) is one of the allowed values.
// Returns a formatted error mentioning the field name and allowed options.
func validateEnum(value, field string, allowed []string) error {
	if value == "" {
		return nil
	}
	if !slices.Contains(allowed, value) {
		return fmt.Errorf("invalid %s %q: must be %s", field, value, formatOptions(allowed))
	}
	return nil
}

// validatePreservePatterns checks that all patterns are valid filepath.Match syntax.
func validatePreservePatterns(patterns []string, contextInfo string) error {
	for i, pat := range patterns {
		if _, err := filepath.Match(pat, ""); err != nil {
			if contextInfo != "" {
				return fmt.Errorf("invalid preserve.patterns[%d] %q in %s: %w", i, pat, contextInfo, err)
			}
			return fmt.Errorf("invalid preserve.patterns[%d] %q: %w", i, pat, err)
		}
	}
	return nil
}

// formatOptions formats a list of allowed values for error messages.
// E.g., ["a", "b", "c"] -> `"a", "b", or "c"`
func formatOptions(opts []string) string {
	quoted := make([]string, len(opts))
	for i, o := range opts {
		quoted[i] = fmt.Sprintf("%q", o)
	}
	if len(quoted) <= 2 {
		return strings.Join(quoted, " or ")
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + ", or " + quoted[len(quoted)-1]
}

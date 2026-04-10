package config

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/raphi011/wt/internal/hooktrigger"
)

// Valid enum values for configuration fields.
var (
	ValidForgeTypes       = []string{"github", "gitlab"}
	ValidMergeStrategies  = []string{"squash", "rebase", "merge"}
	ValidBaseRefs         = []string{"local", "remote"}
	ValidDefaultSortModes = []string{"date", "repo", "branch"}
	ValidCloneModes       = []string{"bare", "regular"}
)

// ValidateCloneMode validates a clone mode value against ValidCloneModes.
// Exported for use in CLI flag validation.
func ValidateCloneMode(mode string) error {
	return validateEnum(mode, "clone-mode", ValidCloneModes)
}

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

// ValidateHookTriggers validates all "on" values in hook config.
func ValidateHookTriggers(hooksMap map[string]Hook) error {
	for name, hook := range hooksMap {
		for _, on := range hook.On {
			if _, err := hooktrigger.ParseTrigger(on); err != nil {
				return fmt.Errorf("invalid hook trigger %q in hook %q: %w", on, name, err)
			}
		}
	}
	return nil
}

// validatePreservePaths checks that all paths are relative and don't escape the repo root.
func validatePreservePaths(paths []string, contextInfo string) error {
	for i, p := range paths {
		cleaned := filepath.Clean(p)

		var reason string
		switch {
		case p == "":
			reason = "must not be empty"
		case cleaned == ".":
			reason = "must be a file path, not directory root"
		case filepath.IsAbs(p):
			reason = "must be a relative path"
		case strings.HasPrefix(cleaned, ".."):
			reason = "must not escape repo root"
		}

		if reason != "" {
			if contextInfo != "" {
				return fmt.Errorf("invalid preserve.paths[%d] %q in %s: %s", i, p, contextInfo, reason)
			}
			return fmt.Errorf("invalid preserve.paths[%d] %q: %s", i, p, reason)
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

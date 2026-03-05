//go:build darwin

package fs

import (
	"os"
	"path/filepath"
	"strings"
)

// canonicalizeCase resolves symlinks and normalizes path casing on macOS.
//
// APFS is case-insensitive by default (case-sensitive volumes exist but
// normalization is harmless on those), so /Users/foo/Git and /Users/foo/git
// refer to the same directory. filepath.EvalSymlinks does NOT normalize case,
// so two paths to the same directory can differ in casing and fail string
// comparison. This function walks each component and matches against the
// actual directory listing to recover the true on-disk casing.
func canonicalizeCase(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}

	// Must be absolute after EvalSymlinks
	if !filepath.IsAbs(resolved) {
		return resolved
	}

	components := strings.Split(resolved, string(filepath.Separator))
	// components[0] is "" for absolute paths (before the leading /)

	built := string(filepath.Separator)
	for _, comp := range components[1:] {
		entries, err := os.ReadDir(built)
		if err != nil {
			// Can't read parent — return what EvalSymlinks gave us
			return resolved
		}

		matched := false
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), comp) {
				built = filepath.Join(built, entry.Name())
				matched = true
				break
			}
		}
		if !matched {
			// Component not found — return EvalSymlinks result
			return resolved
		}
	}

	return built
}

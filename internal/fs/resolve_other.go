//go:build !darwin

package fs

import "path/filepath"

// canonicalizeCase resolves symlinks only. Case normalization is not
// implemented for non-darwin platforms.
func canonicalizeCase(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

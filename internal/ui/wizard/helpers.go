package wizard

import "strings"

// DeleteLastWord removes the last word from a string (for alt+backspace).
func DeleteLastWord(s string) string {
	if s == "" {
		return s
	}
	// Trim trailing spaces first
	s = strings.TrimRight(s, " ")
	// Find last space
	lastSpace := strings.LastIndex(s, " ")
	if lastSpace == -1 {
		return "" // No space found, delete everything
	}
	return s[:lastSpace+1]
}

// FilterOptions returns indices of options matching the filter string.
func FilterOptions(options []Option, filter string) []int {
	if filter == "" {
		indices := make([]int, len(options))
		for i := range options {
			indices[i] = i
		}
		return indices
	}

	filter = strings.ToLower(filter)
	var filtered []int
	for i, opt := range options {
		if strings.Contains(strings.ToLower(opt.Label), filter) {
			filtered = append(filtered, i)
		}
	}
	return filtered
}

// IsPrintable returns true if the key string is a single printable character.
func IsPrintable(key string) bool {
	return len(key) == 1 && key[0] >= 32 && key[0] <= 126
}

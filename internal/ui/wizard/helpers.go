package wizard

import (
	"strings"
	"unicode"
)

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

// RuneFilter determines which runes are allowed in input.
type RuneFilter func(r rune) bool

// RuneFilterNone allows all printable characters.
func RuneFilterNone(r rune) bool {
	return unicode.IsPrint(r)
}

// RuneFilterNoSpaces allows printable characters except spaces.
// Use for branch names, identifiers, etc.
func RuneFilterNoSpaces(r rune) bool {
	return unicode.IsPrint(r) && r != ' '
}

// FilterRunes returns characters from a rune slice that pass the filter.
// If filter is nil, defaults to RuneFilterNone (all printable).
func FilterRunes(runes []rune, filter RuneFilter) string {
	if filter == nil {
		filter = RuneFilterNone
	}
	var result strings.Builder
	for _, r := range runes {
		if filter(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

package framework

import (
	"strings"
	"unicode"
)

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

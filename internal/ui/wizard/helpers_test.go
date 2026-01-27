package wizard

import (
	"testing"
)

func TestFilterRunes(t *testing.T) {
	tests := []struct {
		name   string
		runes  []rune
		filter RuneFilter
		want   string
	}{
		{
			name:   "single ASCII character with nil filter",
			runes:  []rune{'a'},
			filter: nil,
			want:   "a",
		},
		{
			name:   "multi-character paste with nil filter",
			runes:  []rune{'f', 'e', 'a', 't', 'u', 'r', 'e', '-', 'b', 'r', 'a', 'n', 'c', 'h'},
			filter: nil,
			want:   "feature-branch",
		},
		{
			name:   "paste with spaces using RuneFilterNoSpaces",
			runes:  []rune{'f', 'e', 'a', 't', 'u', 'r', 'e', ' ', 'b', 'r', 'a', 'n', 'c', 'h'},
			filter: RuneFilterNoSpaces,
			want:   "featurebranch",
		},
		{
			name:   "paste with newlines and tabs filtered out",
			runes:  []rune{'h', 'e', 'l', 'l', 'o', '\n', 'w', 'o', 'r', 'l', 'd', '\t', '!'},
			filter: nil,
			want:   "helloworld!",
		},
		{
			name:   "unicode characters",
			runes:  []rune{'c', 'a', 'f', 'e', '\u0301'}, // cafe with combining acute accent
			filter: nil,
			want:   "cafe\u0301",
		},
		{
			name:   "empty input",
			runes:  []rune{},
			filter: nil,
			want:   "",
		},
		{
			name:   "all spaces with RuneFilterNoSpaces",
			runes:  []rune{' ', ' ', ' '},
			filter: RuneFilterNoSpaces,
			want:   "",
		},
		{
			name:   "mixed content with RuneFilterNoSpaces",
			runes:  []rune{'f', 'i', 'x', '/', 'b', 'u', 'g', ' ', '1', '2', '3'},
			filter: RuneFilterNoSpaces,
			want:   "fix/bug123",
		},
		{
			name:   "spaces allowed with RuneFilterNone",
			runes:  []rune{'h', 'e', 'l', 'l', 'o', ' ', 'w', 'o', 'r', 'l', 'd'},
			filter: RuneFilterNone,
			want:   "hello world",
		},
		{
			name:   "control characters filtered out",
			runes:  []rune{'a', '\x00', 'b', '\x1F', 'c'},
			filter: nil,
			want:   "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterRunes(tt.runes, tt.filter)
			if got != tt.want {
				t.Errorf("FilterRunes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuneFilterNone(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'a', true},
		{'Z', true},
		{'0', true},
		{' ', true},
		{'-', true},
		{'\n', false},
		{'\t', false},
		{'\x00', false},
	}

	for _, tt := range tests {
		if got := RuneFilterNone(tt.r); got != tt.want {
			t.Errorf("RuneFilterNone(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestRuneFilterNoSpaces(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'a', true},
		{'Z', true},
		{'0', true},
		{' ', false},
		{'-', true},
		{'\n', false},
		{'\t', false},
	}

	for _, tt := range tests {
		if got := RuneFilterNoSpaces(tt.r); got != tt.want {
			t.Errorf("RuneFilterNoSpaces(%q) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

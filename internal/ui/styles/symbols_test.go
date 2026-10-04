package styles

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/raphi011/wt/internal/config"
	"github.com/raphi011/wt/internal/forge"
)

func TestSetNerdfont(t *testing.T) {
	// Test default (off)
	SetNerdfont(false)
	if currentSymbols != defaultSymbols {
		t.Errorf("expected default symbols, got %+v", currentSymbols)
	}

	// Test enabled
	SetNerdfont(true)
	if currentSymbols != nerdfontSymbols {
		t.Errorf("expected nerdfont symbols, got %+v", currentSymbols)
	}

	// Reset
	SetNerdfont(false)
}

func TestFormatPRState(t *testing.T) {
	SetNerdfont(false)

	tests := []struct {
		state    string
		isDraft  bool
		expected string
	}{
		{forge.PRStateMerged, false, "● Merged"},
		{forge.PRStateOpen, false, "○ Open"},
		{forge.PRStateOpen, true, "◌ Draft"},
		{forge.PRStateClosed, false, "✕ Closed"},
		{"", false, ""},
		{"UNKNOWN", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := FormatPRState(tt.state, tt.isDraft)
			if got != tt.expected {
				t.Errorf("FormatPRState(%q, %v) = %q, want %q",
					tt.state, tt.isDraft, got, tt.expected)
			}
		})
	}
}

func TestFormatPRState_Nerdfont(t *testing.T) {
	SetNerdfont(true)
	defer SetNerdfont(false)

	tests := []struct {
		state    string
		isDraft  bool
		expected string
	}{
		{forge.PRStateMerged, false, "\ueafe Merged"},
		{forge.PRStateOpen, false, "\uea64 Open"},
		{forge.PRStateOpen, true, "\uebdb Draft"},
		{forge.PRStateClosed, false, "\uebda Closed"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := FormatPRState(tt.state, tt.isDraft)
			if got != tt.expected {
				t.Errorf("FormatPRState(%q, %v) = %q, want %q",
					tt.state, tt.isDraft, got, tt.expected)
			}
		})
	}
}

func TestPRStateSymbol(t *testing.T) {
	SetNerdfont(false)

	tests := []struct {
		state    string
		isDraft  bool
		expected string
	}{
		{forge.PRStateMerged, false, "●"},
		{forge.PRStateOpen, false, "○"},
		{forge.PRStateOpen, true, "◌"},
		{forge.PRStateClosed, false, "✕"},
		{"", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := PRStateSymbol(tt.state, tt.isDraft)
			if got != tt.expected {
				t.Errorf("PRStateSymbol(%q, %v) = %q, want %q",
					tt.state, tt.isDraft, got, tt.expected)
			}
		})
	}
}

func TestFormatPRRef(t *testing.T) {
	tests := []struct {
		name     string
		number   int
		state    string
		isDraft  bool
		url      string
		contains string // substring that must appear
		empty    bool   // expect empty string
	}{
		{"zero number", 0, forge.PRStateOpen, false, "", "", true},
		{"open PR", 42, forge.PRStateOpen, false, "", "#42", false},
		{"merged PR", 10, forge.PRStateMerged, false, "", "#10", false},
		{"closed PR", 5, forge.PRStateClosed, false, "", "#5", false},
		{"draft PR", 7, forge.PRStateOpen, true, "", "#7", false},
		{"with URL", 99, forge.PRStateOpen, false, "https://github.com/org/repo/pull/99", "#99", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPRRef(tt.number, tt.state, tt.isDraft, tt.url, false)
			if tt.empty {
				if got != "" {
					t.Errorf("FormatPRRef() = %q, want empty", got)
				}
				return
			}
			stripped := ansi.Strip(got)
			if !strings.Contains(stripped, tt.contains) {
				t.Errorf("FormatPRRef() stripped = %q, want to contain %q", stripped, tt.contains)
			}
		})
	}
}

func TestFormatPRRef_Hyperlink(t *testing.T) {
	url := "https://github.com/org/repo/pull/42"
	got := FormatPRRef(42, forge.PRStateOpen, false, url, true)

	// OSC 8 hyperlinks use \x1b]8;; prefix
	if !strings.Contains(got, "\x1b]8;;") {
		t.Errorf("FormatPRRef with URL should contain OSC 8 sequence, got %q", got)
	}
	if !strings.Contains(got, url) {
		t.Errorf("FormatPRRef with URL should contain the URL, got %q", got)
	}

	// With URL, should be underlined (SGR 4 = underline, combined with color codes)
	if !strings.Contains(got, "\x1b[4;") {
		t.Errorf("FormatPRRef with URL should be underlined, got %q", got)
	}

	// Without URL, no OSC 8 and no underline
	noURL := FormatPRRef(42, forge.PRStateOpen, false, "", false)
	if strings.Contains(noURL, "\x1b]8;;") {
		t.Errorf("FormatPRRef without URL should not contain OSC 8 sequence, got %q", noURL)
	}
	if strings.Contains(noURL, "\x1b[4;") {
		t.Errorf("FormatPRRef without URL should not be underlined, got %q", noURL)
	}
}

func TestFormatStaleReason(t *testing.T) {
	tests := []struct {
		commitAge string
		expected  string
	}{
		{"3w", "⏳ Stale (3w)"},
		{"2mo", "⏳ Stale (2mo)"},
		{"", "⏳ Stale"},
	}

	for _, tt := range tests {
		t.Run(tt.commitAge, func(t *testing.T) {
			got := FormatStaleReason(tt.commitAge)
			if got != tt.expected {
				t.Errorf("FormatStaleReason(%q) = %q, want %q", tt.commitAge, got, tt.expected)
			}
		})
	}
}

func TestPRReferencesIncludeStateAndRespectLinkCapability(t *testing.T) {
	for _, nerdfont := range []bool{false, true} {
		SetNerdfont(nerdfont)
		for _, tc := range []struct {
			state string
			draft bool
			text  string
		}{
			{forge.PRStateOpen, false, "Open"}, {forge.PRStateOpen, true, "Draft"},
			{forge.PRStateMerged, true, "Merged"}, {forge.PRStateClosed, true, "Closed"},
			{"", true, ""}, {"UNKNOWN", false, ""},
		} {
			for _, links := range []bool{false, true} {
				ref := FormatPRRef(123, tc.state, tc.draft, "https://example.com/123", links)
				want := "#123"
				if tc.text != "" {
					want += " " + PRStateSymbol(tc.state, tc.draft) + " " + tc.text
				}
				if ansi.Strip(ref) != want {
					t.Fatalf("ref=%q want %q", ansi.Strip(ref), want)
				}
				if strings.Contains(ref, "\x1b]8;") != links {
					t.Fatalf("OSC 8 not gated: %q", ref)
				}
			}
		}
	}
	SetNerdfont(false)
}

func TestPRStatesRemainDistinctWithoutColor(t *testing.T) {
	saved := themeConfig
	Init(config.ThemeConfig{Name: "none"})
	defer Init(saved)
	for _, tc := range []struct {
		state string
		draft bool
		text  string
	}{
		{forge.PRStateOpen, false, "#42 ○ Open"}, {forge.PRStateOpen, true, "#42 ◌ Draft"},
		{forge.PRStateMerged, false, "#42 ● Merged"}, {forge.PRStateClosed, false, "#42 ✕ Closed"},
	} {
		if got := FormatPRRef(42, tc.state, tc.draft, "https://example.com/42", false); got != tc.text {
			t.Fatalf("colorless ref=%q want %q", got, tc.text)
		}
		linked := FormatPRRef(42, tc.state, tc.draft, "https://example.com/42", true)
		if !strings.Contains(linked, "\x1b]8;") || ansi.Strip(linked) != tc.text {
			t.Fatalf("colorless terminal lost link/text: %q", linked)
		}
		if strings.Contains(linked, "38;") {
			t.Fatalf("colorless link emitted color: %q", linked)
		}
	}
}

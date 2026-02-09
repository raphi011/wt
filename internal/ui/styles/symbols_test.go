package styles

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/raphi011/wt/internal/forge"
)

func TestSetNerdfont(t *testing.T) {
	// Test default (off)
	SetNerdfont(false)
	if NerdfontEnabled() {
		t.Error("expected nerdfont to be disabled")
	}
	if PRMergedSymbol() != "●" {
		t.Errorf("expected default merged symbol, got %q", PRMergedSymbol())
	}

	// Test enabled
	SetNerdfont(true)
	if !NerdfontEnabled() {
		t.Error("expected nerdfont to be enabled")
	}
	if PRMergedSymbol() != "\ueafe" {
		t.Errorf("expected nerdfont merged symbol, got %q", PRMergedSymbol())
	}

	// Reset
	SetNerdfont(false)
}

func TestPRSymbols(t *testing.T) {
	SetNerdfont(false)

	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{"PRMergedSymbol", PRMergedSymbol, "●"},
		{"PROpenSymbol", PROpenSymbol, "○"},
		{"PRClosedSymbol", PRClosedSymbol, "✕"},
		{"PRDraftSymbol", PRDraftSymbol, "◌"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(); got != tt.expected {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
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
			got := FormatPRRef(tt.number, tt.state, tt.isDraft, tt.url)
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
	got := FormatPRRef(42, forge.PRStateOpen, false, url)

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
	noURL := FormatPRRef(42, forge.PRStateOpen, false, "")
	if strings.Contains(noURL, "\x1b]8;;") {
		t.Errorf("FormatPRRef without URL should not contain OSC 8 sequence, got %q", noURL)
	}
	if strings.Contains(noURL, "\x1b[4;") {
		t.Errorf("FormatPRRef without URL should not be underlined, got %q", noURL)
	}
}

func TestCurrentSymbols(t *testing.T) {
	SetNerdfont(false)
	symbols := CurrentSymbols()

	if symbols.PRMerged != "●" {
		t.Errorf("expected default PRMerged symbol")
	}

	SetNerdfont(true)
	symbols = CurrentSymbols()

	if symbols.PRMerged != "\ueafe" {
		t.Errorf("expected nerdfont PRMerged symbol")
	}

	SetNerdfont(false)
}

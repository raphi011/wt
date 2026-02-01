package styles

import "testing"

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
		{"MERGED", false, "● Merged"},
		{"OPEN", false, "○ Open"},
		{"OPEN", true, "◌ Draft"},
		{"CLOSED", false, "✕ Closed"},
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
		{"MERGED", false, "\ueafe Merged"},
		{"OPEN", false, "\uea64 Open"},
		{"OPEN", true, "\uebdb Draft"},
		{"CLOSED", false, "\uebda Closed"},
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
		{"MERGED", false, "●"},
		{"OPEN", false, "○"},
		{"OPEN", true, "◌"},
		{"CLOSED", false, "✕"},
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

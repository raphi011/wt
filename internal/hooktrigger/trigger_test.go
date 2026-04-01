package hooktrigger

import (
	"strings"
	"testing"
)

func TestParseTrigger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    ParsedTrigger
		wantErr string
	}{
		// Single segment — defaults to phase=after
		{"bare checkout", "checkout", ParsedTrigger{Phase: "after", Trigger: "checkout"}, ""},
		{"bare prune", "prune", ParsedTrigger{Phase: "after", Trigger: "prune"}, ""},
		{"bare merge", "merge", ParsedTrigger{Phase: "after", Trigger: "merge"}, ""},
		{"bare all", "all", ParsedTrigger{Phase: "after", Trigger: "all"}, ""},

		// Two segments: timing:trigger
		{"before checkout", "before:checkout", ParsedTrigger{Phase: "before", Trigger: "checkout"}, ""},
		{"after prune", "after:prune", ParsedTrigger{Phase: "after", Trigger: "prune"}, ""},

		// Two segments: trigger:subtype
		{"checkout create", "checkout:create", ParsedTrigger{Phase: "after", Trigger: "checkout", Subtype: "create"}, ""},
		{"checkout open", "checkout:open", ParsedTrigger{Phase: "after", Trigger: "checkout", Subtype: "open"}, ""},
		{"checkout pr", "checkout:pr", ParsedTrigger{Phase: "after", Trigger: "checkout", Subtype: "pr"}, ""},

		// Three segments: timing:trigger:subtype
		{"before checkout create", "before:checkout:create", ParsedTrigger{Phase: "before", Trigger: "checkout", Subtype: "create"}, ""},
		{"after checkout pr", "after:checkout:pr", ParsedTrigger{Phase: "after", Trigger: "checkout", Subtype: "pr"}, ""},

		// Error: empty
		{"empty string", "", ParsedTrigger{}, "empty trigger value"},

		// Error: too many segments
		{"four segments", "a:b:c:d", ParsedTrigger{}, "too many segments"},

		// Error: empty segments in middle
		{"empty trigger", "before:", ParsedTrigger{}, "empty trigger"},
		{"empty subtype", "checkout:", ParsedTrigger{}, "empty trigger"},
		{"empty trigger three", "before::create", ParsedTrigger{}, "empty trigger"},

		// Error: unknown timing
		{"unknown timing", "sometimes:checkout", ParsedTrigger{}, "unknown timing"},
		{"unknown timing three", "sometimes:checkout:create", ParsedTrigger{}, "unknown timing"},

		// Error: removed trigger
		{"removed cd", "cd", ParsedTrigger{}, "no longer a valid trigger"},
		{"removed cd with subtype", "cd:create", ParsedTrigger{}, "no longer a valid trigger"},

		// Error: invalid trigger name
		{"invalid trigger", "deploy", ParsedTrigger{}, "not a valid trigger"},

		// Error: subtypes on non-checkout triggers
		{"prune with subtype", "prune:create", ParsedTrigger{}, "does not support subtypes"},
		{"merge with subtype", "merge:open", ParsedTrigger{}, "does not support subtypes"},
		{"all with subtype", "all:create", ParsedTrigger{}, "does not support subtypes"},

		// Error: invalid subtype for checkout
		{"invalid checkout subtype", "checkout:deploy", ParsedTrigger{}, "unknown subtype"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTrigger(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseTrigger(%q) = %v, want error containing %q", tt.input, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseTrigger(%q) error = %q, want containing %q", tt.input, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTrigger(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseTrigger(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		parsed  ParsedTrigger
		trigger string
		subtype string
		want    bool
	}{
		{"all matches checkout", ParsedTrigger{Trigger: "all"}, "checkout", "create", true},
		{"all matches prune", ParsedTrigger{Trigger: "all"}, "prune", "", true},
		{"checkout matches checkout", ParsedTrigger{Trigger: "checkout"}, "checkout", "create", true},
		{"checkout no subtype matches any", ParsedTrigger{Trigger: "checkout"}, "checkout", "pr", true},
		{"checkout:create matches create", ParsedTrigger{Trigger: "checkout", Subtype: "create"}, "checkout", "create", true},
		{"checkout:create no match open", ParsedTrigger{Trigger: "checkout", Subtype: "create"}, "checkout", "open", false},
		{"checkout no match prune", ParsedTrigger{Trigger: "checkout"}, "prune", "", false},
		{"prune matches prune", ParsedTrigger{Trigger: "prune"}, "prune", "", true},
		{"prune no match checkout", ParsedTrigger{Trigger: "prune"}, "checkout", "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.parsed.Matches(tt.trigger, tt.subtype)
			if got != tt.want {
				t.Errorf("Matches(%q, %q) = %v, want %v", tt.trigger, tt.subtype, got, tt.want)
			}
		})
	}
}

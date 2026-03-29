package hooks

import (
	"strings"
	"testing"
)

func TestParseTrigger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantPhase  string
		wantType   string
		wantSub    string
		wantErr    bool
		errContain string
	}{
		// Valid inputs
		{name: "bare trigger", input: "checkout", wantPhase: "after", wantType: "checkout", wantSub: ""},
		{name: "trigger with subtype", input: "checkout:create", wantPhase: "after", wantType: "checkout", wantSub: "create"},
		{name: "checkout:open", input: "checkout:open", wantPhase: "after", wantType: "checkout", wantSub: "open"},
		{name: "checkout:pr", input: "checkout:pr", wantPhase: "after", wantType: "checkout", wantSub: "pr"},
		{name: "before:checkout", input: "before:checkout", wantPhase: "before", wantType: "checkout", wantSub: ""},
		{name: "before:checkout:pr", input: "before:checkout:pr", wantPhase: "before", wantType: "checkout", wantSub: "pr"},
		{name: "after:checkout", input: "after:checkout", wantPhase: "after", wantType: "checkout", wantSub: ""},
		{name: "after:checkout:create", input: "after:checkout:create", wantPhase: "after", wantType: "checkout", wantSub: "create"},
		{name: "prune", input: "prune", wantPhase: "after", wantType: "prune", wantSub: ""},
		{name: "before:prune", input: "before:prune", wantPhase: "before", wantType: "prune", wantSub: ""},
		{name: "merge", input: "merge", wantPhase: "after", wantType: "merge", wantSub: ""},
		{name: "before:merge", input: "before:merge", wantPhase: "before", wantType: "merge", wantSub: ""},
		{name: "all", input: "all", wantPhase: "after", wantType: "all", wantSub: ""},
		{name: "before:all", input: "before:all", wantPhase: "before", wantType: "all", wantSub: ""},
		{name: "after:all", input: "after:all", wantPhase: "after", wantType: "all", wantSub: ""},

		// Invalid inputs
		{name: "cd is removed", input: "cd", wantErr: true, errContain: "no longer a valid trigger"},
		{name: "pr is not a trigger", input: "pr", wantErr: true, errContain: "not a valid trigger"},
		{name: "unknown subtype", input: "checkout:foo", wantErr: true, errContain: "unknown subtype"},
		{name: "prune no subtypes", input: "prune:create", wantErr: true, errContain: "does not support subtypes"},
		{name: "merge no subtypes", input: "merge:pr", wantErr: true, errContain: "does not support subtypes"},
		{name: "unknown timing", input: "sometimes:checkout", wantErr: true, errContain: "unknown timing"},
		{name: "empty string", input: "", wantErr: true, errContain: "empty trigger"},
		{name: "before: empty trigger", input: "before:", wantErr: true, errContain: "empty trigger"},
		{name: "trailing colon", input: "before:checkout:", wantErr: true, errContain: "empty subtype"},
		{name: "too many segments", input: "a:b:c:d", wantErr: true, errContain: "too many segments"},
		{name: "all no subtypes", input: "all:pr", wantErr: true, errContain: "does not support subtypes"},
		{name: "before:cd", input: "before:cd", wantErr: true, errContain: "no longer a valid trigger"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseTrigger(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseTrigger(%q) = %v, want error containing %q", tt.input, got, tt.errContain)
				}
				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ParseTrigger(%q) error = %q, want containing %q", tt.input, err.Error(), tt.errContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTrigger(%q) unexpected error: %v", tt.input, err)
			}
			if got.Phase != tt.wantPhase {
				t.Errorf("Phase = %q, want %q", got.Phase, tt.wantPhase)
			}
			if got.Trigger != tt.wantType {
				t.Errorf("Trigger = %q, want %q", got.Trigger, tt.wantType)
			}
			if got.Subtype != tt.wantSub {
				t.Errorf("Subtype = %q, want %q", got.Subtype, tt.wantSub)
			}
		})
	}
}

func TestParsedTrigger_Matches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		on      string
		trigger string
		subtype string
		want    bool
	}{
		{name: "checkout matches create", on: "checkout", trigger: "checkout", subtype: "create", want: true},
		{name: "checkout matches open", on: "checkout", trigger: "checkout", subtype: "open", want: true},
		{name: "checkout matches pr", on: "checkout", trigger: "checkout", subtype: "pr", want: true},
		{name: "checkout:pr matches pr", on: "checkout:pr", trigger: "checkout", subtype: "pr", want: true},
		{name: "checkout:pr no match create", on: "checkout:pr", trigger: "checkout", subtype: "create", want: false},
		{name: "checkout:create no match open", on: "checkout:create", trigger: "checkout", subtype: "open", want: false},
		{name: "before:checkout matches pr", on: "before:checkout", trigger: "checkout", subtype: "pr", want: true},
		{name: "before:checkout:pr no match create", on: "before:checkout:pr", trigger: "checkout", subtype: "create", want: false},
		{name: "all matches checkout:pr", on: "all", trigger: "checkout", subtype: "pr", want: true},
		{name: "all matches prune", on: "all", trigger: "prune", subtype: "", want: true},
		{name: "all matches merge", on: "all", trigger: "merge", subtype: "", want: true},
		{name: "before:all matches prune", on: "before:all", trigger: "prune", subtype: "", want: true},
		{name: "prune no match checkout", on: "prune", trigger: "checkout", subtype: "create", want: false},
		{name: "merge no match prune", on: "merge", trigger: "prune", subtype: "", want: false},
		{name: "checkout no match prune", on: "checkout", trigger: "prune", subtype: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsed, err := ParseTrigger(tt.on)
			if err != nil {
				t.Fatalf("ParseTrigger(%q) error: %v", tt.on, err)
			}
			got := parsed.Matches(tt.trigger, tt.subtype)
			if got != tt.want {
				t.Errorf("ParsedTrigger(%q).Matches(%q, %q) = %v, want %v", tt.on, tt.trigger, tt.subtype, got, tt.want)
			}
		})
	}
}

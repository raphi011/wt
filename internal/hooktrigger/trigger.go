package hooktrigger

import (
	"fmt"
	"slices"
	"strings"
)

// ParsedTrigger represents a parsed hook "on" value with the structure [phase:]trigger[:subtype].
type ParsedTrigger struct {
	Phase   string // "before" or "after"
	Trigger string // "checkout", "prune", "merge", or "all"
	Subtype string // checkout only: "create", "open", "pr", or "" (match all)
}

var (
	validTimings  = []string{"before", "after"}
	validTriggers = []string{"checkout", "prune", "merge", "all"}
	validSubtypes = map[string][]string{
		"checkout": {"create", "open", "pr"},
	}
	removedTriggers = map[string]string{
		"cd": `"cd" is no longer a valid trigger, use "wt hook <name>" instead`,
	}
)

func isRemovedTrigger(s string) bool {
	_, ok := removedTriggers[s]
	return ok
}

// ParseTrigger parses an "on" value string into a ParsedTrigger.
func ParseTrigger(s string) (ParsedTrigger, error) {
	if s == "" {
		return ParsedTrigger{}, fmt.Errorf("empty trigger value")
	}

	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return ParsedTrigger{}, fmt.Errorf("too many segments in %q (max 3)", s)
	}

	for i, p := range parts {
		if i > 0 && p == "" {
			if i == 1 {
				return ParsedTrigger{}, fmt.Errorf("empty trigger in %q", s)
			}
			return ParsedTrigger{}, fmt.Errorf("empty subtype in %q", s)
		}
	}

	var phase, trigger, subtype string

	switch len(parts) {
	case 1:
		phase = "after"
		trigger = parts[0]
	case 2:
		if slices.Contains(validTimings, parts[0]) {
			phase = parts[0]
			trigger = parts[1]
		} else if slices.Contains(validTriggers, parts[0]) || isRemovedTrigger(parts[0]) {
			phase = "after"
			trigger = parts[0]
			subtype = parts[1]
		} else {
			// Treat as unknown timing prefix (e.g. "sometimes:checkout")
			return ParsedTrigger{}, fmt.Errorf("unknown timing %q (valid: %s)", parts[0], strings.Join(validTimings, ", "))
		}
	case 3:
		if !slices.Contains(validTimings, parts[0]) {
			return ParsedTrigger{}, fmt.Errorf("unknown timing %q (valid: %s)", parts[0], strings.Join(validTimings, ", "))
		}
		phase = parts[0]
		trigger = parts[1]
		subtype = parts[2]
	}

	if msg, ok := removedTriggers[trigger]; ok {
		return ParsedTrigger{}, fmt.Errorf("%s", msg)
	}

	if !slices.Contains(validTriggers, trigger) {
		return ParsedTrigger{}, fmt.Errorf("%q is not a valid trigger (valid: %s)", trigger, strings.Join(validTriggers, ", "))
	}

	if subtype != "" {
		allowed, hasSubtypes := validSubtypes[trigger]
		if !hasSubtypes {
			return ParsedTrigger{}, fmt.Errorf("trigger %q does not support subtypes", trigger)
		}
		if !slices.Contains(allowed, subtype) {
			return ParsedTrigger{}, fmt.Errorf("unknown subtype %q for %s (valid: %s)", subtype, trigger, strings.Join(allowed, ", "))
		}
	}

	return ParsedTrigger{
		Phase:   phase,
		Trigger: trigger,
		Subtype: subtype,
	}, nil
}

// Matches returns true if this parsed trigger matches the given command trigger and subtype.
func (p ParsedTrigger) Matches(trigger, subtype string) bool {
	if p.Trigger == "all" {
		return true
	}
	if p.Trigger != trigger {
		return false
	}
	if p.Subtype == "" {
		return true
	}
	return p.Subtype == subtype
}

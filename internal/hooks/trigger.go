package hooks

import "github.com/raphi011/wt/internal/hooktrigger"

// ParsedTrigger represents a parsed hook "on" value with the structure [phase:]trigger[:subtype].
// It is an alias for hooktrigger.ParsedTrigger to avoid an import cycle between config and hooks.
type ParsedTrigger = hooktrigger.ParsedTrigger

// ParseTrigger parses an "on" value string into a ParsedTrigger.
func ParseTrigger(s string) (ParsedTrigger, error) {
	return hooktrigger.ParseTrigger(s)
}

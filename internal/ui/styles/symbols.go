package styles

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/raphi011/wt/internal/forge"
)

// Symbols holds the icon/symbol set based on nerdfont configuration
type Symbols struct {
	PRMerged string
	PROpen   string
	PRClosed string
	PRDraft  string
}

// Default symbols (ASCII-safe)
var defaultSymbols = Symbols{
	PRMerged: "●",
	PROpen:   "○",
	PRClosed: "✕",
	PRDraft:  "◌",
}

// Nerd font symbols
var nerdfontSymbols = Symbols{
	PRMerged: "\ueafe", // nf-oct-git_merge
	PROpen:   "\uea64", // nf-oct-git_pull_request
	PRClosed: "\uebda", // nf-oct-git_pull_request_closed
	PRDraft:  "\uebdb", // nf-oct-git_pull_request_draft
}

// currentSymbols holds the active symbol set
var currentSymbols = defaultSymbols

// SetNerdfont enables or disables nerd font symbols
func SetNerdfont(enabled bool) {
	if enabled {
		currentSymbols = nerdfontSymbols
	} else {
		currentSymbols = defaultSymbols
	}
}

// FormatPRState returns a formatted string with symbol and state.
// state should be forge.PRStateMerged, forge.PRStateOpen, forge.PRStateClosed, or empty.
// isDraft indicates if the PR is a draft (only applies to OPEN state).
func FormatPRState(state string, isDraft bool) string {
	text := PRStateText(state, isDraft)
	if text == "" {
		return ""
	}
	return PRStateSymbol(state, isDraft) + " " + text
}

// PRStateText names a cached state independently of colors and configured icons.
func PRStateText(state string, isDraft bool) string {
	switch state {
	case forge.PRStateMerged:
		return "Merged"
	case forge.PRStateOpen:
		if isDraft {
			return "Draft"
		}
		return "Open"
	case forge.PRStateClosed:
		return "Closed"
	default:
		return ""
	}
}

// PRStateStyle is shared by tables and interactive pruning. Action/safety labels
// use their own styles so, for example, a merged state remains purple everywhere.
func PRStateStyle(state string, isDraft bool) lipgloss.Style {
	switch state {
	case forge.PRStateOpen:
		if isDraft {
			return MutedStyle
		}
		return SuccessStyle
	case forge.PRStateMerged:
		return MergedStyle
	case forge.PRStateClosed:
		return ErrorStyle
	default:
		return NormalStyle
	}
}

// FormatPRRef renders a number and its cached state. The caller supplies the
// actual destination's hyperlink capability; a URL alone never enables OSC 8.
// Missing numbers remain empty and unknown states retain only their number.
func FormatPRRef(number int, state string, isDraft bool, url string, hyperlinks bool) string {
	if number == 0 {
		return ""
	}
	style := PRStateStyle(state, isDraft)
	text := fmt.Sprintf("#%d", number)
	if stateText := FormatPRState(state, isDraft); stateText != "" {
		text += " " + stateText
	}
	if hyperlinks && url != "" {
		return ansi.SetHyperlink(url) + style.Underline(true).Render(text) + ansi.ResetHyperlink()
	}
	return style.Render(text)
}

// FormatStaleReason returns a formatted stale reason string with the commit age.
func FormatStaleReason(commitAge string) string {
	if commitAge == "" {
		return "⏳ Stale"
	}
	return "⏳ Stale (" + commitAge + ")"
}

// PRStateSymbol returns just the symbol for a PR state
func PRStateSymbol(state string, isDraft bool) string {
	switch state {
	case forge.PRStateMerged:
		return currentSymbols.PRMerged
	case forge.PRStateOpen:
		if isDraft {
			return currentSymbols.PRDraft
		}
		return currentSymbols.PROpen
	case forge.PRStateClosed:
		return currentSymbols.PRClosed
	default:
		return ""
	}
}

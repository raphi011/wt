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

// useNerdfont tracks whether nerd font symbols are enabled
var useNerdfont bool

// currentSymbols holds the active symbol set
var currentSymbols = defaultSymbols

// SetNerdfont enables or disables nerd font symbols
func SetNerdfont(enabled bool) {
	useNerdfont = enabled
	if enabled {
		currentSymbols = nerdfontSymbols
	} else {
		currentSymbols = defaultSymbols
	}
}

// NerdfontEnabled returns whether nerd font symbols are enabled
func NerdfontEnabled() bool {
	return useNerdfont
}

// CurrentSymbols returns the current symbol set
func CurrentSymbols() Symbols {
	return currentSymbols
}

// PRMergedSymbol returns the symbol for merged PRs
func PRMergedSymbol() string {
	return currentSymbols.PRMerged
}

// PROpenSymbol returns the symbol for open PRs
func PROpenSymbol() string {
	return currentSymbols.PROpen
}

// PRClosedSymbol returns the symbol for closed PRs
func PRClosedSymbol() string {
	return currentSymbols.PRClosed
}

// PRDraftSymbol returns the symbol for draft PRs
func PRDraftSymbol() string {
	return currentSymbols.PRDraft
}

// FormatPRState returns a formatted string with symbol and state.
// state should be forge.PRStateMerged, forge.PRStateOpen, forge.PRStateClosed, or empty.
// isDraft indicates if the PR is a draft (only applies to OPEN state).
func FormatPRState(state string, isDraft bool) string {
	switch state {
	case forge.PRStateMerged:
		return currentSymbols.PRMerged + " Merged"
	case forge.PRStateOpen:
		if isDraft {
			return currentSymbols.PRDraft + " Draft"
		}
		return currentSymbols.PROpen + " Open"
	case forge.PRStateClosed:
		return currentSymbols.PRClosed + " Closed"
	default:
		return ""
	}
}

// FormatPRRef returns a colored #<number> string with an OSC 8 hyperlink.
// Returns empty string if number == 0.
func FormatPRRef(number int, state string, isDraft bool, url string) string {
	if number == 0 {
		return ""
	}

	var style lipgloss.Style
	switch state {
	case forge.PRStateOpen:
		if isDraft {
			style = MutedStyle
		} else {
			style = SuccessStyle
		}
	case forge.PRStateMerged:
		style = MergedStyle
	case forge.PRStateClosed:
		style = ErrorStyle
	default:
		style = NormalStyle
	}

	text := fmt.Sprintf("#%d", number)

	if url != "" {
		styled := style.Underline(true).Render(text)
		return ansi.SetHyperlink(url) + styled + ansi.ResetHyperlink()
	}
	return style.Render(text)
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

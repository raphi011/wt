package framework

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// Keys holds the bindings shared by wizard steps and their help text.
var Keys = struct {
	Cancel, Clear, Back, Advance, ListBack, ListAdvance, Confirm, Refresh key.Binding
}{
	Cancel:      key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "cancel")),
	Clear:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	Back:        key.NewBinding(key.WithKeys("alt+left"), key.WithHelp("alt+←", "back")),
	Advance:     key.NewBinding(key.WithKeys("alt+right"), key.WithHelp("alt+→", "next")),
	ListBack:    key.NewBinding(key.WithKeys("left", "alt+left"), key.WithHelp("←/alt+←", "back")),
	ListAdvance: key.NewBinding(key.WithKeys("right", "alt+right"), key.WithHelp("→/alt+→", "next")),
	Refresh:     key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh/retry")),
	Confirm:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
}

// Navigation leaves plain arrows and word-navigation keys to focused inputs.
func Navigation(msg tea.KeyPressMsg, inputFocused bool) StepResult {
	back, advance := Keys.ListBack, Keys.ListAdvance
	if inputFocused {
		back, advance = Keys.Back, Keys.Advance
	}
	switch {
	case key.Matches(msg, back):
		return StepBack
	case key.Matches(msg, advance):
		return StepAdvance
	case key.Matches(msg, Keys.Confirm):
		return StepSubmitIfReady
	default:
		return StepContinue
	}
}

// NavigationHelp uses the same bindings as Navigation.
func NavigationHelp(inputFocused bool) string {
	back, advance := Keys.ListBack, Keys.ListAdvance
	if inputFocused {
		back, advance = Keys.Back, Keys.Advance
	}
	return BindingHelp(back, advance, Keys.Confirm)
}

func BindingHelp(bindings ...key.Binding) string {
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		h := binding.Help()
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return strings.Join(parts, " • ")
}

// CancellationHelp explains Escape's current action and immediate cancellation.
func CancellationHelp(clearable bool, inputName string) string {
	escape := Keys.Clear
	if clearable {
		escape.SetHelp(escape.Help().Key, "clear "+inputName)
	}
	return BindingHelp(escape, Keys.Cancel)
}

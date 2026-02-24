package prompt

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func keyPress(key string) tea.KeyPressMsg {
	if len(key) == 1 {
		return tea.KeyPressMsg{Code: rune(key[0])}
	}
	switch key {
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	default:
		return tea.KeyPressMsg{Code: rune(key[0])}
	}
}

func TestConfirmModel_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		key       string
		confirmed bool
		done      bool
		cancelled bool
		wantCmd   bool
	}{
		{"y confirms", "y", true, true, false, true},
		{"Y confirms", "Y", true, true, false, true},
		{"n declines", "n", false, true, false, true},
		{"N declines", "N", false, true, false, true},
		{"enter defaults no", "enter", false, true, false, true},
		{"ctrl+c cancels", "ctrl+c", false, true, true, true},
		{"esc cancels", "esc", false, true, true, true},
		{"q cancels", "q", false, true, true, true},
		{"unhandled is no-op", "x", false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := confirmModel{prompt: "Continue?"}
			updated, cmd := m.Update(keyPress(tt.key))
			um := updated.(confirmModel)

			if um.confirmed != tt.confirmed {
				t.Errorf("confirmed = %v, want %v", um.confirmed, tt.confirmed)
			}
			if um.done != tt.done {
				t.Errorf("done = %v, want %v", um.done, tt.done)
			}
			if um.cancelled != tt.cancelled {
				t.Errorf("cancelled = %v, want %v", um.cancelled, tt.cancelled)
			}
			if (cmd != nil) != tt.wantCmd {
				t.Errorf("cmd nil = %v, want nil = %v", cmd == nil, !tt.wantCmd)
			}
		})
	}
}

func TestConfirmModel_ViewNotDone(t *testing.T) {
	t.Parallel()

	m := confirmModel{prompt: "Delete files?"}
	view := m.View()
	if view.Content == "" {
		t.Error("View().Content should not be empty when not done")
	}
}

func TestConfirmModel_ViewDone(t *testing.T) {
	t.Parallel()

	m := confirmModel{prompt: "Delete files?", done: true}
	// View() should not panic; when done, the content wraps an empty string
	_ = m.View()
}

func TestConfirmModel_Init(t *testing.T) {
	t.Parallel()

	m := confirmModel{prompt: "test"}
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil cmd")
	}
}

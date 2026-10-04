package output

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestHyperlinkCapabilities(t *testing.T) {
	cases := []struct {
		name string
		tty  bool
		env  map[string]string
		want bool
	}{
		{"iTerm", true, map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "iTerm.app"}, true},
		{"WezTerm", true, map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "WezTerm"}, true},
		{"Ghostty", true, map[string]string{"TERM": "xterm-ghostty"}, true},
		{"kitty", true, map[string]string{"TERM": "xterm-kitty"}, true},
		{"VTE", true, map[string]string{"TERM": "xterm-256color", "VTE_VERSION": "5000"}, true},
		{"old VTE", true, map[string]string{"TERM": "xterm", "VTE_VERSION": "4900"}, false},
		{"invalid VTE", true, map[string]string{"TERM": "xterm", "VTE_VERSION": "unknown"}, false},
		{"unknown color terminal", true, map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"}, false},
		{"Apple Terminal", true, map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "Apple_Terminal"}, false},
		{"dumb", true, map[string]string{"TERM": "dumb", "TERM_PROGRAM": "iTerm.app"}, false},
		{"missing TERM", true, map[string]string{"TERM_PROGRAM": "iTerm.app"}, false},
		{"screen", true, map[string]string{"TERM": "screen-256color", "TERM_PROGRAM": "WezTerm"}, false},
		{"tmux", true, map[string]string{"TERM": "xterm-kitty", "TMUX": "/tmp/socket"}, false},
		{"NO_COLOR independent", true, map[string]string{"TERM": "xterm-kitty", "NO_COLOR": "1"}, true},
		{"forced color pipe", false, map[string]string{"TERM": "xterm-kitty", "CLICOLOR_FORCE": "1"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := supportsHyperlinks(tc.tty, func(key string) string { return tc.env[key] }); got != tc.want {
				t.Fatalf("support=%v want %v", got, tc.want)
			}
		})
	}
}

func TestHyperlinksUseActualDestination(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	var buf bytes.Buffer
	if FromContext(WithPrinter(context.Background(), &buf)).HyperlinksSupported() {
		t.Fatal("buffer enabled links")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	if FromContext(WithPrinter(context.Background(), writer)).HyperlinksSupported() {
		t.Fatal("pipe enabled links")
	}
}

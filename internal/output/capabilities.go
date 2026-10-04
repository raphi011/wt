package output

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"
)

// HyperlinksSupported checks the data destination independently of color policy.
// Unknown terminals and multiplexers get plain references rather than assuming
// that a color-capable terminal also understands OSC 8. No terminal query runs.
func (p *Printer) HyperlinksSupported() bool {
	f, ok := p.w.(*os.File)
	return ok && supportsHyperlinks(term.IsTerminal(f.Fd()), os.Getenv)
}

func supportsHyperlinks(tty bool, getenv func(string) string) bool {
	if !tty {
		return false
	}
	terminal := getenv("TERM")
	if terminal == "" || terminal == "dumb" || terminal == "linux" ||
		strings.HasPrefix(terminal, "screen") || strings.HasPrefix(terminal, "tmux") || getenv("TMUX") != "" {
		return false
	}
	switch getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm", "ghostty":
		return true
	}
	if terminal == "xterm-kitty" || terminal == "xterm-ghostty" {
		return true
	}
	version, err := strconv.Atoi(getenv("VTE_VERSION"))
	return err == nil && version >= 5000
}

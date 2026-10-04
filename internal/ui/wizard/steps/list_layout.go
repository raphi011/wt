package steps

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/raphi011/wt/internal/ui/wizard/framework"
)

// renderList shares row-aware scrolling across selectors. Each item is already
// width-constrained; descriptions may occupy several rows. Always include the
// focused label, even when its description is taller than the available area.
func renderList(count, cursor, width, height int, row func(int) string) string {
	if count == 0 || height <= 0 {
		return ""
	}
	cursor = min(max(0, cursor), count-1)
	budget := height
	indicators := height >= 4
	if indicators {
		budget -= 2
	}
	focused := framework.Fit(row(cursor), width, budget)
	start, end, used := cursor, cursor+1, lipgloss.Height(focused)
	rows := []string{focused}
	for start > 0 {
		prev := row(start - 1)
		h := lipgloss.Height(prev)
		if used+h > budget {
			break
		}
		rows = append([]string{prev}, rows...)
		start--
		used += h
	}
	for end < count {
		next := row(end)
		h := lipgloss.Height(next)
		if used+h > budget {
			break
		}
		rows = append(rows, next)
		end++
		used += h
	}
	if indicators && start > 0 {
		rows = append([]string{framework.OptionDescriptionStyle().Render("  ↑ more above")}, rows...)
	}
	if indicators && end < count {
		rows = append(rows, framework.OptionDescriptionStyle().Render("  ↓ more below"))
	}
	return framework.Fit(strings.Join(rows, "\n"), width, height)
}

func optionRow(prefix, label, description string, width int) string {
	first := framework.Fit(prefix+label, width, 1)
	if ansi.Strip(description) == "" {
		return first
	}
	indent := min(4, max(0, width-1))
	desc := framework.Wrap(description, max(1, width-indent), 3)
	return first + "\n" + strings.Repeat(" ", indent) + strings.ReplaceAll(desc, "\n", "\n"+strings.Repeat(" ", indent))
}

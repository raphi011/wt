package prompt

import (
	"os"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/raphi011/wt/internal/ui/styles"
)

// SelectResult holds the result of a selection prompt.
type SelectResult struct {
	Value     string
	Index     int
	Cancelled bool
}

type listItem struct {
	title string
	index int
}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return "" }
func (i listItem) FilterValue() string { return i.title }

type selectModel struct {
	list      list.Model
	done      bool
	cancelled bool
	selected  int
}

func (m selectModel) Init() tea.Cmd {
	return nil
}

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(listItem); ok {
				m.selected = item.index
			}
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc", "q":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(m.list.View())
}

// Select shows a list selection prompt and returns the user's selection.
func Select(prompt string, options []string) (SelectResult, error) {
	if len(options) == 0 {
		return SelectResult{Cancelled: true}, nil
	}

	items := make([]list.Item, len(options))
	for i, opt := range options {
		items[i] = listItem{title: opt, index: i}
	}

	// Custom delegate with minimal styling
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	// Style the selected item
	selectedStyle := lipgloss.NewStyle().
		Foreground(styles.Accent).
		Bold(true)
	delegate.Styles.SelectedTitle = selectedStyle

	l := list.New(items, delegate, 60, min(len(options)+6, 20))
	l.Title = prompt
	l.SetShowStatusBar(false)
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	model := selectModel{
		list:     l,
		selected: -1,
	}
	p := tea.NewProgram(model, tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return SelectResult{}, err
	}
	m := finalModel.(selectModel)

	if m.cancelled || m.selected < 0 || m.selected >= len(options) {
		return SelectResult{Cancelled: true}, nil
	}

	return SelectResult{
		Value: options[m.selected],
		Index: m.selected,
	}, nil
}

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Result types for interactive prompts
type ConfirmResult struct {
	Confirmed bool
	Cancelled bool
}

type TextInputResult struct {
	Value     string
	Cancelled bool
}

type SelectResult struct {
	Value     string
	Index     int
	Cancelled bool
}

// --- Confirm Prompt ---

type confirmModel struct {
	prompt    string
	confirmed bool
	done      bool
	cancelled bool
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.confirmed = true
			m.done = true
			return m, tea.Quit
		case "n", "N":
			m.confirmed = false
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			// Default to no
			m.confirmed = false
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s [y/N] ", m.prompt)
}

// Confirm shows a yes/no prompt and returns the user's choice.
func Confirm(prompt string) (ConfirmResult, error) {
	model := confirmModel{prompt: prompt}
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return ConfirmResult{}, err
	}
	m := finalModel.(confirmModel)
	return ConfirmResult{
		Confirmed: m.confirmed,
		Cancelled: m.cancelled,
	}, nil
}

// --- Text Input ---

type textInputModel struct {
	textInput textinput.Model
	prompt    string
	done      bool
	cancelled bool
}

func (m textInputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m textInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m textInputModel) View() string {
	if m.done {
		return ""
	}
	return fmt.Sprintf("%s\n%s", m.prompt, m.textInput.View())
}

// TextInput shows a text input prompt and returns the user's input.
func TextInput(prompt, placeholder string) (TextInputResult, error) {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	model := textInputModel{
		textInput: ti,
		prompt:    prompt,
	}
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return TextInputResult{}, err
	}
	m := finalModel.(textInputModel)
	return TextInputResult{
		Value:     m.textInput.Value(),
		Cancelled: m.cancelled,
	}, nil
}

// --- List Selection ---

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
	case tea.KeyMsg:
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

func (m selectModel) View() string {
	if m.done {
		return ""
	}
	return m.list.View()
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
		Foreground(lipgloss.Color("170")).
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
	p := tea.NewProgram(model)
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

// --- Interactive Checkout ---

// CheckoutOptions holds the options gathered from interactive mode.
type CheckoutOptions struct {
	Branch    string
	NewBranch bool
	Fetch     bool
	Cancelled bool
}

// CheckoutInteractive runs the interactive checkout flow.
// It asks the user whether to create a new branch or checkout existing,
// then gathers the branch name accordingly.
// targetRepos optionally lists the repositories being targeted (for display).
func CheckoutInteractive(existingBranches []string, targetRepos []string) (CheckoutOptions, error) {
	// Show target repos if specified
	if len(targetRepos) > 0 {
		repoList := strings.Join(targetRepos, ", ")
		fmt.Printf("Target repositories: %s\n\n", repoList)
	}

	// Ask if creating new branch
	result, err := Confirm("Create a new branch?")
	if err != nil {
		return CheckoutOptions{}, err
	}
	if result.Cancelled {
		return CheckoutOptions{Cancelled: true}, nil
	}

	opts := CheckoutOptions{NewBranch: result.Confirmed}

	if opts.NewBranch {
		// Ask for new branch name
		input, err := TextInput("Enter branch name:", "feature/my-feature")
		if err != nil {
			return CheckoutOptions{}, err
		}
		if input.Cancelled || strings.TrimSpace(input.Value) == "" {
			return CheckoutOptions{Cancelled: true}, nil
		}
		opts.Branch = strings.TrimSpace(input.Value)

		// Ask if should fetch first
		fetchResult, err := Confirm("Fetch from origin first?")
		if err != nil {
			return CheckoutOptions{}, err
		}
		if fetchResult.Cancelled {
			return CheckoutOptions{Cancelled: true}, nil
		}
		opts.Fetch = fetchResult.Confirmed
	} else {
		// Select from existing branches
		if len(existingBranches) == 0 {
			return CheckoutOptions{}, fmt.Errorf("no existing branches found")
		}

		selection, err := Select("Select a branch:", existingBranches)
		if err != nil {
			return CheckoutOptions{}, err
		}
		if selection.Cancelled {
			return CheckoutOptions{Cancelled: true}, nil
		}
		opts.Branch = selection.Value
	}

	return opts, nil
}

package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/raphi011/wt/internal/git"
)

var (
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Bold(true)
	unselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
)

// SelectorResult contains the result of the selection
type SelectorResult struct {
	Worktree  git.Worktree
	Selected  bool // true if user selected, false if cancelled
	Cancelled bool
}

// selectorModel is the bubbletea model for worktree selection
type selectorModel struct {
	worktrees []git.Worktree
	filtered  []git.Worktree
	textInput textinput.Model
	cursor    int
	selected  *git.Worktree
	cancelled bool
	maxHeight int
}

func newSelectorModel(worktrees []git.Worktree) selectorModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40
	ti.PromptStyle = cursorStyle
	ti.TextStyle = lipgloss.NewStyle()

	return selectorModel{
		worktrees: worktrees,
		filtered:  worktrees,
		textInput: ti,
		cursor:    0,
		maxHeight: 10,
	}
}

func (m selectorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				m.selected = &m.filtered[m.cursor]
			}
			return m, tea.Quit

		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	// Handle text input
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Filter worktrees based on input
	m.filtered = m.filterWorktrees(m.textInput.Value())

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}

	return m, cmd
}

func (m selectorModel) filterWorktrees(query string) []git.Worktree {
	if query == "" {
		return m.worktrees
	}

	query = strings.ToLower(query)
	var filtered []git.Worktree

	for _, wt := range m.worktrees {
		// Match against folder name, branch, and repo name
		folder := strings.ToLower(filepath.Base(wt.Path))
		branch := strings.ToLower(wt.Branch)
		repo := strings.ToLower(wt.RepoName)

		if fuzzyMatch(folder, query) || fuzzyMatch(branch, query) || fuzzyMatch(repo, query) {
			filtered = append(filtered, wt)
		}
	}

	return filtered
}

// fuzzyMatch performs a simple fuzzy match - all query chars must appear in order
func fuzzyMatch(s, query string) bool {
	sIdx := 0
	for _, qChar := range query {
		found := false
		for sIdx < len(s) {
			if rune(s[sIdx]) == qChar {
				found = true
				sIdx++
				break
			}
			sIdx++
		}
		if !found {
			return false
		}
	}
	return true
}

func (m selectorModel) View() string {
	var sb strings.Builder

	// Show search input
	sb.WriteString("Select worktree:\n")
	sb.WriteString(m.textInput.View())
	sb.WriteString("\n\n")

	// Show filtered results
	if len(m.filtered) == 0 {
		sb.WriteString(dimStyle.Render("  No matches found"))
		sb.WriteString("\n")
	} else {
		// Calculate visible range
		start := 0
		end := len(m.filtered)
		if end > m.maxHeight {
			// Center the cursor in the visible area
			halfHeight := m.maxHeight / 2
			start = m.cursor - halfHeight
			if start < 0 {
				start = 0
			}
			end = start + m.maxHeight
			if end > len(m.filtered) {
				end = len(m.filtered)
				start = max(0, end-m.maxHeight)
			}
		}

		for i := start; i < end; i++ {
			wt := m.filtered[i]
			folder := filepath.Base(wt.Path)
			line := fmt.Sprintf("%s (%s)", folder, wt.Branch)

			if i == m.cursor {
				sb.WriteString(cursorStyle.Render("> "))
				sb.WriteString(selectedStyle.Render(line))
			} else {
				sb.WriteString("  ")
				sb.WriteString(unselectedStyle.Render(line))
			}
			sb.WriteString("\n")
		}

		// Show scroll indicator
		if len(m.filtered) > m.maxHeight {
			sb.WriteString(dimStyle.Render(fmt.Sprintf("\n  %d/%d", m.cursor+1, len(m.filtered))))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("↑/↓ navigate • enter select • esc cancel"))

	return sb.String()
}

// RunSelector shows an interactive fuzzy search selector for worktrees
// Returns the selected worktree or nil if cancelled
func RunSelector(worktrees []git.Worktree) (*SelectorResult, error) {
	if len(worktrees) == 0 {
		return &SelectorResult{Cancelled: true}, nil
	}

	model := newSelectorModel(worktrees)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m := finalModel.(selectorModel)
	if m.cancelled {
		return &SelectorResult{Cancelled: true}, nil
	}
	if m.selected != nil {
		return &SelectorResult{Worktree: *m.selected, Selected: true}, nil
	}
	return &SelectorResult{Cancelled: true}, nil
}

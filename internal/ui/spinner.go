package ui

import (
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
)

// Spinner wraps a Bubbletea spinner for simple non-interactive use
type Spinner struct {
	program *tea.Program
	model   *spinnerModel
	mu      sync.Mutex
}

// spinnerModel is the internal Bubbletea model
type spinnerModel struct {
	spinner spinner.Model
	message string
	done    bool
}

func (m spinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m spinnerModel) View() string {
	if m.done || m.message == "" {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	s := spinner.New()
	s.Spinner = spinner.Dot

	model := &spinnerModel{
		spinner: s,
		message: message,
	}

	return &Spinner{
		model: model,
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.program = tea.NewProgram(s.model)
	go s.program.Run()
}

// UpdateMessage changes the spinner message
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.model != nil {
		s.model.message = message
	}
}

// Stop stops the spinner and clears the line
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.model != nil {
		s.model.done = true
	}
	if s.program != nil {
		s.program.Quit()
	}
}

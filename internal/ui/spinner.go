package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// messageUpdate is sent to update the spinner message
type messageUpdate string

// Spinner wraps a Bubbletea spinner for simple non-interactive use
type Spinner struct {
	program    *tea.Program
	msgChan    chan string
	done       chan struct{}
	mu         sync.Mutex
	isRunning  bool
	lastMsg    string
}

// spinnerModel is the internal Bubbletea model
type spinnerModel struct {
	spinner spinner.Model
	message string
	msgChan chan string
	quit    bool
}

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.waitForMessage())
}

func (m spinnerModel) waitForMessage() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.msgChan
		if !ok {
			return tea.Quit()
		}
		return messageUpdate(msg)
	}
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quit {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case messageUpdate:
		m.message = string(msg)
		return m, m.waitForMessage()
	case tea.KeyMsg:
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m spinnerModel) View() string {
	if m.quit || m.message == "" {
		return ""
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		msgChan: make(chan string, 10),
		done:    make(chan struct{}),
		lastMsg: message,
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	model := spinnerModel{
		spinner: sp,
		message: s.lastMsg,
		msgChan: s.msgChan,
	}

	s.program = tea.NewProgram(model, tea.WithoutSignalHandler())
	s.isRunning = true

	go func() {
		_, _ = s.program.Run()
		close(s.done)
	}()
}

// UpdateMessage changes the spinner message
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	if !s.isRunning {
		s.lastMsg = message
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	// Non-blocking send - intentionally drops messages if channel is full
	// to avoid blocking the main operation for UI updates
	select {
	case s.msgChan <- message:
	default:
		// Channel full, skip update (acceptable for UI)
	}
}

// Stop stops the spinner and clears the line
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = false
	s.mu.Unlock()

	// Close channel to signal quit
	close(s.msgChan)

	// Quit the program
	if s.program != nil {
		s.program.Quit()
	}

	// Wait for program to finish with timeout
	select {
	case <-s.done:
	case <-time.After(500 * time.Millisecond):
	}

	// Clear the spinner line
	fmt.Print("\r\033[K")
}

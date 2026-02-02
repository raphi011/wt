// Package progress provides progress indication components.
//
// This package contains components for showing progress during
// long-running operations, such as spinners and progress bars.
package progress

import (
	"fmt"
	"os"
	"sync"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// messageUpdate is sent to update the spinner message
type messageUpdate string

// Spinner wraps a Bubbletea spinner for simple non-interactive use
type Spinner struct {
	program   *tea.Program
	msgChan   chan string
	done      chan struct{}
	mu        sync.Mutex
	isRunning bool
	lastMsg   string
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
	case tea.KeyPressMsg:
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m spinnerModel) View() tea.View {
	if m.quit || m.message == "" {
		return tea.NewView("")
	}
	return tea.NewView(fmt.Sprintf("%s %s", m.spinner.View(), m.message))
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

	// Write to stderr so stdout remains clean for piping (e.g., cd $(wt cd ...))
	s.program = tea.NewProgram(model, tea.WithoutSignalHandler(), tea.WithOutput(os.Stderr))
	s.isRunning = true

	go func() {
		_, _ = s.program.Run()
		close(s.done)
	}()
}

// UpdateMessage changes the spinner message
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		s.lastMsg = message
		return
	}

	// Non-blocking send - intentionally drops messages if channel is full
	// to avoid blocking the main operation for UI updates
	// Safe because channel close happens under same mutex
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
	// Close channel inside mutex to prevent race with UpdateMessage
	close(s.msgChan)
	s.mu.Unlock()

	// Quit the program
	if s.program != nil {
		s.program.Quit()
	}

	// Wait for program to finish with timeout
	select {
	case <-s.done:
	case <-time.After(500 * time.Millisecond):
	}

	// Clear to stderr (UI output shouldn't pollute stdout for piping)
	fmt.Fprint(os.Stderr, "\r\033[K")
}

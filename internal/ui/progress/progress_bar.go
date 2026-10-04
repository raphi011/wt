package progress

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"github.com/mattn/go-isatty"
	"github.com/raphi011/wt/internal/ui/styles"
)

// progressUpdate is sent to update the progress bar
type progressUpdate struct {
	current int
	message string
}

// ProgressBar wraps a Bubbletea progress bar for simple non-interactive use.
// Use this for determinate operations where you know the total count.
type ProgressBar struct {
	program     *tea.Program
	ctx         context.Context
	output      io.Writer
	interactive bool
	stopped     bool
	updateCh    chan progressUpdate
	done        chan struct{}
	mu          sync.Mutex
	isRunning   bool
	total       int
	current     int
	message     string
}

// progressBarModel is the internal Bubbletea model
type progressBarModel struct {
	progress progress.Model
	ctx      context.Context
	total    int
	current  int
	message  string
	updateCh chan progressUpdate
	quit     bool
}

func (m progressBarModel) Init() tea.Cmd {
	return m.waitForUpdate()
}

func (m progressBarModel) waitForUpdate() tea.Cmd {
	return func() tea.Msg {
		select {
		case <-m.ctx.Done():
			return tea.Quit()
		case update, ok := <-m.updateCh:
			if !ok {
				return tea.Quit()
			}
			return update
		}
	}
}

func (m progressBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quit {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case progressUpdate:
		m.current = msg.current
		m.message = msg.message
		return m, m.waitForUpdate()
	case tea.KeyPressMsg:
		return m, nil
	default:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.Update(msg)
		return m, cmd
	}
}

func (m progressBarModel) View() tea.View {
	if m.quit || m.message == "" {
		return tea.NewView("")
	}

	// Calculate percentage
	percent := 0.0
	if m.total > 0 {
		percent = float64(m.current) / float64(m.total)
	}

	// Format: [████████░░░░░░░░] 45% Fetching PR status...
	bar := m.progress.ViewAs(percent)
	pct := int(percent * 100)

	return tea.NewView(fmt.Sprintf("%s %3d%% %s", bar, pct, m.message))
}

// Option configures the progress display.
type Option func(*ProgressBar)

// WithContext stops the display and its pending commands on cancellation.
func WithContext(ctx context.Context) Option { return func(p *ProgressBar) { p.ctx = ctx } }

// WithOutput selects the diagnostic destination (stderr by default).
func WithOutput(output io.Writer) Option { return func(p *ProgressBar) { p.output = output } }

// NewProgressBar creates a progress bar with plain diagnostics for non-TTY output.
func NewProgressBar(total int, message string, options ...Option) *ProgressBar {
	p := &ProgressBar{
		ctx:      context.Background(),
		output:   os.Stderr,
		updateCh: make(chan progressUpdate, 10),
		done:     make(chan struct{}),
		total:    total,
		message:  message,
	}
	for _, option := range options {
		option(p)
	}
	if p.ctx == nil {
		p.ctx = context.Background()
	}
	if file, ok := p.output.(*os.File); ok {
		p.interactive = isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
	}
	return p
}

// Start begins the progress bar display.
func (p *ProgressBar) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning || p.stopped || p.ctx.Err() != nil {
		return
	}

	if !p.interactive {
		p.isRunning = true
		p.writePlain(p.current, p.message)
		return
	}

	// Create progress bar with theme colors
	prog := progress.New(
		progress.WithWidth(40),
		progress.WithoutPercentage(),
		progress.WithColors(styles.Primary, styles.Accent),
	)

	model := progressBarModel{
		progress: prog,
		ctx:      p.ctx,
		total:    p.total,
		current:  p.current,
		message:  p.message,
		updateCh: p.updateCh,
	}

	// Write to stderr so stdout remains clean for piping (e.g., cd $(wt cd ...))
	p.program = tea.NewProgram(model, tea.WithoutSignalHandler(), tea.WithOutput(p.output), tea.WithInput(nil), tea.WithContext(p.ctx))
	p.isRunning = true

	go func() {
		_, _ = p.program.Run()
		close(p.done)
	}()
}

// SetProgress updates the current progress and message.
func (p *ProgressBar) SetProgress(current int, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped || p.ctx.Err() != nil {
		return
	}
	if !p.isRunning {
		p.current = current
		p.message = message
		return
	}

	if !p.interactive {
		if current != p.current || message != p.message {
			p.writePlain(current, message)
		}
		p.current, p.message = current, message
		return
	}

	// Non-blocking send - intentionally drops updates if channel is full
	// Safe because channel close happens under same mutex
	select {
	case p.updateCh <- progressUpdate{current: current, message: message}:
	default:
		// Channel full, skip update (acceptable for UI)
	}
}

// Stop stops the progress bar and clears the line.
func (p *ProgressBar) Stop() {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return
	}
	p.isRunning = false
	p.stopped = true
	if !p.interactive {
		p.mu.Unlock()
		return
	}
	// Close channel inside mutex to prevent race with SetProgress
	close(p.updateCh)
	p.mu.Unlock()

	// Quit the program
	if p.program != nil {
		p.program.Quit()
	}

	// Wait for program to finish with timeout
	select {
	case <-p.done:
	case <-time.After(500 * time.Millisecond):
	}

	// Clear to stderr (UI output shouldn't pollute stdout for piping)
	fmt.Fprint(p.output, "\r\033[K")
}

// Total returns the total count for the progress bar.
func (p *ProgressBar) Total() int {
	return p.total
}

func (p *ProgressBar) writePlain(current int, message string) {
	if message == "" {
		return
	}
	fmt.Fprintf(p.output, "%s (%d/%d)\n", message, current, p.total)
}

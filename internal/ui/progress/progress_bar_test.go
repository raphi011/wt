package progress

import (
	"testing"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
)

func TestProgressBar_New(t *testing.T) {
	pb := NewProgressBar(100, "Test message")
	if pb.Total() != 100 {
		t.Errorf("expected total 100, got %d", pb.Total())
	}
}

func TestProgressBar_SetProgressBeforeStart(t *testing.T) {
	pb := NewProgressBar(10, "Test")
	// Should not panic when setting progress before Start()
	pb.SetProgress(5, "Updated")
}

func TestProgressBar_StopBeforeStart(t *testing.T) {
	pb := NewProgressBar(10, "Test")
	// Stop without Start should not panic
	pb.Stop()
}

func newTestModel(total int, message string) progressBarModel {
	return progressBarModel{
		progress: progress.New(progress.WithWidth(40), progress.WithoutPercentage()),
		total:    total,
		message:  message,
		updateCh: make(chan progressUpdate, 10),
	}
}

func TestProgressBarModel_View_Empty(t *testing.T) {
	t.Parallel()

	m := newTestModel(100, "")
	// Should not panic; empty message returns empty view
	_ = m.View()
}

func TestProgressBarModel_View_Quit(t *testing.T) {
	t.Parallel()

	m := newTestModel(100, "Working...")
	m.quit = true
	// Should not panic; quit state returns empty view
	_ = m.View()
}

func TestProgressBarModel_View_WithProgress(t *testing.T) {
	t.Parallel()

	m := newTestModel(100, "Fetching...")
	m.current = 50
	// Should not panic when rendering with valid progress
	_ = m.View()
}

func TestProgressBarModel_View_ZeroTotal(t *testing.T) {
	t.Parallel()

	m := newTestModel(0, "Loading...")
	// Should not panic with zero total (0% case)
	_ = m.View()
}

func TestProgressBarModel_Update_ProgressUpdate(t *testing.T) {
	t.Parallel()

	m := newTestModel(100, "Starting...")

	updated, _ := m.Update(progressUpdate{current: 75, message: "Almost done"})
	um := updated.(progressBarModel)

	if um.current != 75 {
		t.Errorf("current = %d, want 75", um.current)
	}
	if um.message != "Almost done" {
		t.Errorf("message = %q, want %q", um.message, "Almost done")
	}
}

func TestProgressBarModel_Update_KeyPress(t *testing.T) {
	t.Parallel()

	m := newTestModel(100, "Working...")
	m.current = 50

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})

	// KeyPress should return tea.Quit
	if cmd == nil {
		t.Fatal("Update(KeyPressMsg) returned nil cmd, want tea.Quit")
	}
}

func TestProgressBarModel_Update_QuitState(t *testing.T) {
	t.Parallel()

	m := newTestModel(100, "Working...")
	m.quit = true

	_, cmd := m.Update(progressUpdate{current: 50, message: "ignored"})

	// When quit=true, should return tea.Quit regardless of message type
	if cmd == nil {
		t.Fatal("Update when quit=true returned nil cmd, want tea.Quit")
	}
}

func TestProgressBar_Total(t *testing.T) {
	t.Parallel()

	tests := []struct {
		total int
	}{
		{0},
		{1},
		{100},
		{999},
	}

	for _, tt := range tests {
		pb := NewProgressBar(tt.total, "msg")
		if pb.Total() != tt.total {
			t.Errorf("Total() = %d, want %d", pb.Total(), tt.total)
		}
	}
}

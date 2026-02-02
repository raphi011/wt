package progress

import "testing"

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

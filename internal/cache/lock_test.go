package cache

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewFileLock(t *testing.T) {
	t.Parallel()

	lock := NewFileLock("/tmp/test.lock")
	if lock == nil {
		t.Fatal("expected non-nil lock")
	}
	if lock.path != "/tmp/test.lock" {
		t.Errorf("expected path = '/tmp/test.lock', got %q", lock.path)
	}
	if lock.file != nil {
		t.Error("expected file to be nil initially")
	}
}

func TestFileLock_LockUnlock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")
	lock := NewFileLock(lockPath)

	// Lock should succeed
	if err := lock.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	// Lock file should exist
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file should exist after locking")
	}

	// File handle should be set
	if lock.file == nil {
		t.Error("expected file handle to be set after locking")
	}

	// Unlock should succeed
	if err := lock.Unlock(); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	// File handle should be nil after unlock
	if lock.file != nil {
		t.Error("expected file handle to be nil after unlocking")
	}
}

func TestFileLock_UnlockWithoutLock(t *testing.T) {
	t.Parallel()

	lock := NewFileLock("/tmp/never-locked.lock")

	// Unlock without lock should be a no-op
	if err := lock.Unlock(); err != nil {
		t.Errorf("Unlock() without Lock() should not error, got %v", err)
	}
}

func TestFileLock_DoubleUnlock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")
	lock := NewFileLock(lockPath)

	if err := lock.Lock(); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	if err := lock.Unlock(); err != nil {
		t.Fatalf("first Unlock() error = %v", err)
	}

	// Second unlock should be safe (no-op)
	if err := lock.Unlock(); err != nil {
		t.Errorf("second Unlock() should not error, got %v", err)
	}
}

func TestFileLock_Concurrent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	var mu sync.Mutex
	var order []int
	var wg sync.WaitGroup

	// First goroutine acquires lock
	wg.Add(1)
	go func() {
		defer wg.Done()
		lock := NewFileLock(lockPath)
		if err := lock.Lock(); err != nil {
			t.Errorf("goroutine 1 Lock() error = %v", err)
			return
		}
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()

		// Hold lock briefly
		time.Sleep(50 * time.Millisecond)

		if err := lock.Unlock(); err != nil {
			t.Errorf("goroutine 1 Unlock() error = %v", err)
		}
	}()

	// Give first goroutine time to acquire lock
	time.Sleep(10 * time.Millisecond)

	// Second goroutine tries to acquire same lock (should block)
	wg.Add(1)
	go func() {
		defer wg.Done()
		lock := NewFileLock(lockPath)
		if err := lock.Lock(); err != nil {
			t.Errorf("goroutine 2 Lock() error = %v", err)
			return
		}
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()

		if err := lock.Unlock(); err != nil {
			t.Errorf("goroutine 2 Unlock() error = %v", err)
		}
	}()

	wg.Wait()

	// Verify order - first goroutine should always complete first
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 {
		t.Fatalf("expected 2 entries in order, got %d", len(order))
	}
	if order[0] != 1 || order[1] != 2 {
		t.Errorf("expected order [1, 2], got %v", order)
	}
}

func TestFileLock_MultipleSequential(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")
	lock := NewFileLock(lockPath)

	// Multiple lock/unlock cycles
	for i := 0; i < 3; i++ {
		if err := lock.Lock(); err != nil {
			t.Fatalf("iteration %d: Lock() error = %v", i, err)
		}
		if err := lock.Unlock(); err != nil {
			t.Fatalf("iteration %d: Unlock() error = %v", i, err)
		}
	}
}

func TestFileLock_DifferentLockObjects(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// First lock object
	lock1 := NewFileLock(lockPath)
	if err := lock1.Lock(); err != nil {
		t.Fatalf("lock1 Lock() error = %v", err)
	}

	// Second lock object on same path should block
	done := make(chan bool)
	go func() {
		lock2 := NewFileLock(lockPath)
		if err := lock2.Lock(); err != nil {
			t.Errorf("lock2 Lock() error = %v", err)
			return
		}
		lock2.Unlock()
		done <- true
	}()

	// Give time for second lock to block
	select {
	case <-done:
		t.Error("lock2 should have blocked while lock1 is held")
	case <-time.After(30 * time.Millisecond):
		// Expected - lock2 is blocking
	}

	// Release first lock
	if err := lock1.Unlock(); err != nil {
		t.Fatalf("lock1 Unlock() error = %v", err)
	}

	// Second lock should now complete
	select {
	case <-done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("lock2 should have acquired lock after lock1 released")
	}
}

func TestFileLock_InvalidPath(t *testing.T) {
	t.Parallel()

	// Try to create lock in non-existent directory
	lock := NewFileLock("/non-existent-dir/test.lock")
	err := lock.Lock()
	if err == nil {
		lock.Unlock()
		t.Error("expected error for lock in non-existent directory")
	}
}

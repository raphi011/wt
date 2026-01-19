package cache

import (
	"os"
	"syscall"
)

// FileLock provides exclusive file-based locking using flock.
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a new file lock for the given path.
// The lock file will be created if it doesn't exist.
func NewFileLock(path string) *FileLock {
	return &FileLock{path: path}
}

// Lock acquires an exclusive lock on the file.
// Blocks until the lock is acquired.
func (l *FileLock) Lock() error {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	l.file = f

	// Acquire exclusive lock (blocking)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		l.file = nil
		return err
	}

	return nil
}

// Unlock releases the lock and closes the file.
func (l *FileLock) Unlock() error {
	if l.file == nil {
		return nil
	}

	// Release lock
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		l.file.Close()
		l.file = nil
		return err
	}

	err := l.file.Close()
	l.file = nil
	return err
}

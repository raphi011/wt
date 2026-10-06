package fs

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// lock takes an exclusive advisory lock on the file at path, creating it if
// needed, and blocks until the lock is acquired. The kernel releases the lock
// when the holder exits, so a crashed process never leaves it locked.
// The lock file itself is never removed.
func lock(path string) (unlock func() error, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	for {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}

	// Closing the file releases the lock
	return f.Close, nil
}

// UpdateJSON runs a read-modify-write cycle on the JSON file at path while
// holding an exclusive lock, so concurrent updates never overwrite each other.
// fn receives the current contents (the zero value if the file doesn't exist)
// and the result is saved atomically unless fn returns an error.
//
// The lock is taken on path + ".lock" rather than on the file itself, because
// SaveJSON replaces the file by rename. Readers don't need the lock.
//
// fn must only modify the value in memory: the lock is held while it runs.
func UpdateJSON[T any](path string, fn func(*T) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	unlock, err := lock(path + ".lock")
	if err != nil {
		return err
	}

	var data T
	if err := LoadJSON(path, &data); err != nil && !os.IsNotExist(err) {
		return errors.Join(err, unlock())
	}

	if err := fn(&data); err != nil {
		return errors.Join(err, unlock())
	}

	return errors.Join(SaveJSON(path, &data), unlock())
}

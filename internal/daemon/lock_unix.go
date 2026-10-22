//go:build unix

package daemon

import (
	"errors"
	"os"
	"syscall"
)

// tryLockFile takes an exclusive non-blocking flock(2) on f. When the lock
// is already held elsewhere it returns held=true along with the error.
func tryLockFile(f *os.File) (held bool, err error) {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.Is(err, syscall.EWOULDBLOCK), err
	}
	return false, nil
}

// unlockFile releases a lock taken by tryLockFile.
func unlockFile(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

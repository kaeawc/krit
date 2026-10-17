//go:build windows

package fslock

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func acquire(path string, deadline time.Time) (func(), bool) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false
	}
	handle := windows.Handle(f.Fd())
	for {
		ol := new(windows.Overlapped)
		err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
		if err == nil {
			return func() {
				_ = windows.UnlockFileEx(handle, 0, 1, 0, new(windows.Overlapped))
				_ = f.Close()
			}, true
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) || time.Now().After(deadline) {
			_ = f.Close()
			return nil, false
		}
		time.Sleep(retryInterval)
	}
}

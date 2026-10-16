//go:build !unix && !windows

package fslock

import "time"

// No advisory lock primitive is wired up for this platform, so no caller can
// ever hold the lock; publishers stay private.
func acquire(string, time.Time) (func(), bool) { return nil, false }

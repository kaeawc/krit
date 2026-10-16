// Package fslock provides a cross-process exclusive advisory lock on a file.
//
// The kernel releases the lock when the holding process dies, so there is no
// stale-lock classification or recovery. The lock file is created if missing
// and never deleted: removing it would let a second process lock a different
// inode while the first still holds the old one.
//
// Every Acquire opens its own file descriptor. flock(2) locks belong to the
// open file description, so goroutines of one process exclude each other only
// because each call has its own descriptor.
package fslock

import "time"

// retryInterval is how often a blocked Acquire polls for the lock.
const retryInterval = 5 * time.Millisecond

// Acquire takes the exclusive lock on path, polling for up to wait. It
// returns a release function and true on success. It returns false if the
// lock stayed held by someone else for the whole wait, or if the lock file
// cannot be opened or locked; callers then proceed without the lock's
// protection (for a publisher: stay private).
func Acquire(path string, wait time.Duration) (release func(), ok bool) {
	return acquire(path, time.Now().Add(wait))
}

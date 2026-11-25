//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package serve

import "syscall"

// kqueueWatchBudget returns the descriptors the fsnotify (kqueue)
// backend may hold: half the soft RLIMIT_NOFILE, leaving the rest for
// reading sources, sockets and helper processes. Go raises the soft
// limit at startup (to 10240 on macOS), so this is read after that.
func kqueueWatchBudget() int64 {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil || lim.Cur == 0 {
		return 512
	}
	return int64(lim.Cur / 2)
}

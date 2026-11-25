//go:build !(darwin || dragonfly || freebsd || netbsd || openbsd)

package serve

// kqueueWatchBudget is 0 (no cap) where fsnotify does not use kqueue:
// inotify and ReadDirectoryChangesW don't hold a descriptor per file.
func kqueueWatchBudget() int64 { return 0 }

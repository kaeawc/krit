//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package hashutil

import (
	"os"
	"syscall"
)

// deviceOf returns the ID of the filesystem holding the file info describes.
func deviceOf(info os.FileInfo) (uint64, bool) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(stat.Dev), true //nolint:gosec,unconvert // Dev is int32 on some platforms
	}
	return 0, false
}

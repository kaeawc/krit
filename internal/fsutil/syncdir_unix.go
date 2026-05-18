//go:build !windows

package fsutil

import "os"

// syncDir fsyncs the directory at dir so a preceding rename's dirent is durable.
// Best-effort: errors are swallowed because the rename itself already succeeded
// and some filesystems (network, special) reject directory fsync.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

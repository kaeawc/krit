//go:build unix

package daemon

import (
	"io/fs"
	"os"
	"syscall"
)

// ownedByCurrentUser reports whether fi is owned by the current user.
func ownedByCurrentUser(fi fs.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}

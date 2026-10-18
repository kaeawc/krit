//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"syscall"
)

// ensurePrivateDir creates dir with mode 0700 or validates an existing one.
// An existing dir must be a real directory (not a symlink) owned by the
// current user; a looser mode is tightened to 0700. Anything else is
// refused, so another local user cannot pre-create a predictable path and
// intercept the socket.
func ensurePrivateDir(dir string) error {
	if err := os.Mkdir(dir, 0o700); err != nil && !os.IsExist(err) {
		if os.IsNotExist(err) {
			if perr := os.MkdirAll(dir, 0o700); perr != nil {
				return fmt.Errorf("daemon: create socket dir %s: %w", dir, perr)
			}
		} else {
			return fmt.Errorf("daemon: create socket dir %s: %w", dir, err)
		}
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("daemon: stat socket dir %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("daemon: socket dir %s is not a directory (symlink or file); refusing", dir)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("daemon: socket dir %s is not owned by the current user; refusing", dir)
	}
	if fi.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("daemon: socket dir %s has mode %o and could not be tightened: %w", dir, fi.Mode().Perm(), err)
		}
	}
	return nil
}

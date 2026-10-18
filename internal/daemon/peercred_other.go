//go:build !linux && !darwin

package daemon

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

// verifyPeer has no peer-credential syscall here, so it falls back to
// requiring the socket file to be ours with mode 0600.
func verifyPeer(_ net.Conn, socketPath string) error {
	fi, err := os.Lstat(socketPath)
	if err != nil {
		return err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() || fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("daemon: socket %s is not a private socket owned by the current user; refusing", socketPath)
	}
	return nil
}

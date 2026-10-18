package daemon

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// verifyPeer requires the process on the other end of conn to run as the
// current user (LOCAL_PEERCRED).
func verifyPeer(conn net.Conn, socketPath string) error {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return nil
	}
	sc, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	var uid uint32
	var cerr error
	if err := sc.Control(func(fd uintptr) {
		x, e := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if e != nil {
			cerr = e
			return
		}
		uid = x.Uid
	}); err != nil {
		return err
	}
	if cerr != nil {
		return fmt.Errorf("daemon: peer credentials: %w", cerr)
	}
	if int(uid) != os.Getuid() {
		return &PeerUIDError{Path: socketPath, UID: int(uid)}
	}
	return nil
}

package daemon

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

// verifyPeer requires the process on the other end of conn to run as the
// current user (SO_PEERCRED).
func verifyPeer(conn net.Conn, _ string) error {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return nil
	}
	sc, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	var cred *syscall.Ucred
	var cerr error
	if err := sc.Control(func(fd uintptr) {
		cred, cerr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return err
	}
	if cerr != nil {
		return fmt.Errorf("daemon: peer credentials: %w", cerr)
	}
	if int(cred.Uid) != os.Getuid() {
		return fmt.Errorf("daemon: socket peer runs as uid %d, not %d; refusing", cred.Uid, os.Getuid())
	}
	return nil
}

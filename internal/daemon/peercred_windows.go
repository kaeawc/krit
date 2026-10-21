//go:build windows

package daemon

import (
	"errors"
	"net"
)

// errWindowsUnsupported is returned wherever daemon mode needs a Unix-only
// facility (peer credentials on a unix socket).
var errWindowsUnsupported = errors.New("daemon mode is not supported on Windows")

// verifyPeer cannot authenticate the peer of a unix socket on Windows, so it
// refuses rather than accept an unverified connection.
func verifyPeer(net.Conn, string) error {
	return errWindowsUnsupported
}

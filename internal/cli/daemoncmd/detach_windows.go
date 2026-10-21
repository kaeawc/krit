//go:build windows

package daemoncmd

import (
	"errors"
	"os/exec"
)

// errDaemonUnsupported is returned when daemon mode is requested on Windows.
var errDaemonUnsupported = errors.New("daemon mode is not supported on Windows")

// detachProcess refuses to spawn: the daemon relies on Unix sockets with
// peer-credential checks and session detachment that Windows lacks.
func detachProcess(*exec.Cmd) error {
	return errDaemonUnsupported
}

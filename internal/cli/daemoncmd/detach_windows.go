//go:build windows

package daemoncmd

import (
	"os/exec"

	"github.com/kaeawc/krit/internal/daemon"
)

// errDaemonUnsupported is returned when daemon mode is requested on Windows.
var errDaemonUnsupported = daemon.ErrUnsupported

// detachProcess refuses to spawn: the daemon relies on Unix sockets with
// peer-credential checks and session detachment that Windows lacks.
func detachProcess(*exec.Cmd) error {
	return errDaemonUnsupported
}

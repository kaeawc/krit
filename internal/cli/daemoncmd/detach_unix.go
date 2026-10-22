//go:build unix

package daemoncmd

import (
	"os/exec"
	"syscall"
)

// detachProcess puts the child in its own session so a SIGHUP on the
// parent terminal does not take it down.
func detachProcess(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return nil
}

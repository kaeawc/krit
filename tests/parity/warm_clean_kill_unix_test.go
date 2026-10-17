//go:build !windows

package parity_test

import "syscall"

// wcKillProcessGroup kills the process group led by pid, so a helper JVM's
// children die with it. It is best effort: a pid that leads no group is a no-op.
func wcKillProcessGroup(pid int) { _ = syscall.Kill(-pid, syscall.SIGKILL) }

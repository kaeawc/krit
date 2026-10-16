//go:build windows

package parity_test

// wcKillProcessGroup is a no-op where process groups are not addressable by pid.
func wcKillProcessGroup(pid int) {}

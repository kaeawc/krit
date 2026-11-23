//go:build unix

package main

import (
	"os"
	"runtime"
	"syscall"
)

// maxRSSMB returns the peak resident memory of an exited process in
// megabytes. Linux reports Maxrss in KiB, macOS in bytes.
func maxRSSMB(state *os.ProcessState) float64 {
	ru, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return float64(ru.Maxrss) / (1 << 20)
	}
	return float64(ru.Maxrss) / (1 << 10)
}

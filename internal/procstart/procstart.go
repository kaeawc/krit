// Package procstart reads a process's start time, the token that tells one
// incarnation of a PID from the next.
//
// A daemon registry records the token when it publishes a daemon. Before it
// signals the recorded PID it compares the token with the live one: a PID
// reused by an unrelated process after a crash or reboot has a different
// start time, so the registry drops the record instead of killing a stranger.
package procstart

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Of returns a token naming this incarnation of pid: the kernel start time.
// It returns "" when the start time cannot be read (unsupported platform, ps
// missing, process gone); callers must then treat the process as unproven and
// never signal it.
func Of(pid int) string {
	if pid <= 0 {
		return ""
	}
	switch runtime.GOOS {
	case "linux":
		return procStatStartTime(pid)
	case "windows":
		return ""
	default:
		// Other Unixes (darwin, the BSDs) expose no /proc; ps reports the
		// start time with one-second resolution.
		cmd := exec.CommandContext(context.Background(), "ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.Join(strings.Fields(string(out)), " ")
	}
}

// procStatStartTime reads field 22 (starttime, in clock ticks since boot) of
// /proc/<pid>/stat. The fields after the command follow its last ')', since
// the command itself may contain spaces and parentheses.
func procStatStartTime(pid int) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return ""
	}
	return startTimeFromStat(data)
}

func startTimeFromStat(data []byte) string {
	i := bytes.LastIndexByte(data, ')')
	if i < 0 {
		return ""
	}
	// After ")" the fields start at 3 (state), so starttime is the 20th.
	fields := strings.Fields(string(data[i+1:]))
	if len(fields) < 20 {
		return ""
	}
	return fields[19]
}

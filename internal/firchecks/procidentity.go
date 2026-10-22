package firchecks

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// firProcessStart returns a token naming this incarnation of pid: the kernel
// start time. A PID reused by an unrelated process after a crash or reboot
// carries a different token, so a recorded token proves the process is the
// one that was registered. It returns "" when the start time cannot be read
// (unsupported platform, ps missing, process gone); callers must then treat
// the process as unproven and never signal it.
//
// It is a variable so tests can simulate platforms without a readable start
// time.
var firProcessStart = readFirProcessStart

func readFirProcessStart(pid int) string {
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

// firPIDProven reports whether pid is the process the registry recorded,
// by comparing the start time stored at publication with the live one. A
// record without a start time (written before it was recorded, or on a
// platform that cannot read it) is unproven.
func firPIDProven(meta firAOTMeta, pid int) bool {
	if meta.PID != pid || meta.StartTime == "" {
		return false
	}
	return firProcessStart(pid) == meta.StartTime
}

// readFirAOTMeta loads the record's metadata; ok is false when it is missing
// or unreadable.
func readFirAOTMeta(sourcesHash string) (firAOTMeta, bool) {
	var meta firAOTMeta
	data, err := os.ReadFile(firAOTPath(sourcesHash))
	if err != nil || json.Unmarshal(data, &meta) != nil {
		return firAOTMeta{}, false
	}
	return meta, true
}

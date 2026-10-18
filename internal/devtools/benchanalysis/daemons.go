//go:build unix

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// clockTicksPerSec is USER_HZ, the unit of /proc/<pid>/stat utime and stime.
// It is 100 on every Linux platform krit supports.
const clockTicksPerSec = 100

// DaemonUsage is the CPU and memory attributed to long-lived krit daemons
// (krit serve, the krit-fir and krit-types oracle JVMs, the checker daemon)
// during one run. They are not children of the krit process, so the rusage
// columns never include them.
type DaemonUsage struct {
	CPUMs     int64    `json:"cpuMs"`     // utime+stime consumed between the samples
	PeakRSSKB int64    `json:"peakRssKb"` // sum of VmHWM across daemons (lifetime high-water marks)
	RSSKB     int64    `json:"rssKb"`     // sum of VmRSS after the run
	PIDs      []int    `json:"pids,omitempty"`
	Kinds     []string `json:"kinds,omitempty"`
}

// procSample is one daemon's counters at a point in time.
type procSample struct {
	CPUTicks int64
	HWMKB    int64
	RSSKB    int64
	Kind     string
}

// daemonPIDs lists the daemon processes krit records for a run: PID files in
// the helper registry (oracle and checker JVMs) and the krit daemon's
// <work>/.krit/daemon.pid.
func daemonPIDs(registry, work string) []int {
	var files []string
	if registry != "" {
		matches, _ := filepath.Glob(filepath.Join(registry, "*.pid"))
		files = append(files, matches...)
	}
	if work != "" {
		files = append(files, filepath.Join(work, ".krit", "daemon.pid"))
	}
	seen := map[int]bool{}
	var pids []int
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		fields := strings.Fields(string(data))
		if len(fields) == 0 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 || seen[pid] {
			continue
		}
		seen[pid] = true
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	return pids
}

// sampleDaemons reads /proc-style counters for pids under procRoot. PIDs that
// are gone, or a procRoot that doesn't exist (non-Linux), yield no entry.
func sampleDaemons(procRoot string, pids []int) map[int]procSample {
	out := map[int]procSample{}
	for _, pid := range pids {
		dir := filepath.Join(procRoot, strconv.Itoa(pid))
		stat, err := os.ReadFile(filepath.Join(dir, "stat"))
		if err != nil {
			continue
		}
		ticks, ok := parseStatTicks(string(stat))
		if !ok {
			continue
		}
		s := procSample{CPUTicks: ticks}
		if status, err := os.ReadFile(filepath.Join(dir, "status")); err == nil {
			s.HWMKB = statusKB(string(status), "VmHWM:")
			s.RSSKB = statusKB(string(status), "VmRSS:")
		}
		if cmd, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
			s.Kind = daemonKind(string(cmd))
		}
		out[pid] = s
	}
	return out
}

// parseStatTicks returns utime+stime from a /proc/<pid>/stat line. The comm
// field is parenthesized and may contain spaces, so fields are counted from
// the last ')'.
func parseStatTicks(stat string) (int64, bool) {
	end := strings.LastIndexByte(stat, ')')
	if end < 0 {
		return 0, false
	}
	f := strings.Fields(stat[end+1:])
	// f[0] is state (field 3); utime and stime are fields 14 and 15.
	if len(f) < 13 {
		return 0, false
	}
	u, err1 := strconv.ParseInt(f[11], 10, 64)
	s, err2 := strconv.ParseInt(f[12], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return u + s, true
}

func statusKB(status, key string) int64 {
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, key) {
			f := strings.Fields(strings.TrimPrefix(line, key))
			if len(f) > 0 {
				n, _ := strconv.ParseInt(f[0], 10, 64)
				return n
			}
		}
	}
	return 0
}

func daemonKind(cmdline string) string {
	args := strings.ReplaceAll(cmdline, "\x00", " ")
	switch {
	case strings.Contains(args, "krit-fir"):
		return "krit-fir"
	case strings.Contains(args, "krit-types"):
		return "krit-types"
	case strings.Contains(args, " serve") || strings.Contains(args, "daemon"):
		return "krit-serve"
	case strings.Contains(args, "java"):
		return "jvm-helper"
	}
	return "other"
}

// daemonUsage attributes the change between two samples to the daemons: CPU
// is the tick delta (a daemon first seen in after counts in full, since it
// was started by this run), memory is the after-sample high-water mark.
func daemonUsage(before, after map[int]procSample) DaemonUsage {
	var u DaemonUsage
	kinds := map[string]bool{}
	for pid, a := range after {
		ticks := a.CPUTicks - before[pid].CPUTicks
		if ticks < 0 {
			ticks = a.CPUTicks
		}
		u.CPUMs += ticks * 1000 / clockTicksPerSec
		u.PeakRSSKB += max(a.HWMKB, a.RSSKB)
		u.RSSKB += a.RSSKB
		u.PIDs = append(u.PIDs, pid)
		if a.Kind != "" {
			kinds[a.Kind] = true
		}
	}
	sort.Ints(u.PIDs)
	for k := range kinds {
		u.Kinds = append(u.Kinds, k)
	}
	sort.Strings(u.Kinds)
	return u
}

// maxSocketPath is the longest daemon socket path a run can rely on. The
// kernel's sun_path holds 108 bytes; krit falls back to a short temp path past
// that, but a bind failure on older builds costs a full spawn timeout.
const maxSocketPath = 100

// socketPathWarning explains when <work>/.krit/daemon.sock is long enough to
// break or divert daemon-delegated runs, and returns "" otherwise.
func socketPathWarning(work string) string {
	sock := filepath.Join(work, ".krit", "daemon.sock")
	if len(sock) <= maxSocketPath {
		return ""
	}
	return "daemon socket path is " + strconv.Itoa(len(sock)) + " bytes (over " + strconv.Itoa(maxSocketPath) +
		"): " + sock + "; daemon-delegated rows may fall back to in-process and are rejected, use a shorter -out path"
}

// findServeDaemons returns PIDs under procRoot whose command line is
// `krit serve --root <work>`.
func findServeDaemons(procRoot, work string) []int {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(procRoot, e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if isServeRoot(args, work) {
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	return pids
}

func isServeRoot(args []string, work string) bool {
	serve := false
	for i, a := range args {
		switch {
		case a == "serve":
			serve = true
		case a == "--root" && i+1 < len(args) && serve:
			return args[i+1] == work
		case strings.HasPrefix(a, "--root=") && serve:
			return strings.TrimPrefix(a, "--root=") == work
		}
	}
	return false
}

// killProcs sends SIGTERM, waits up to grace for each PID to exit, then
// SIGKILLs the rest.
func killProcs(pids []int, grace time.Duration) {
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	deadline := time.Now().Add(grace)
	for _, pid := range pids {
		for time.Now().Before(deadline) && syscall.Kill(pid, 0) == nil {
			time.Sleep(50 * time.Millisecond)
		}
		if syscall.Kill(pid, 0) == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

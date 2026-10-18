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
	Start    string // starttime (stat field 22); with the PID it identifies the process
	HWMKB    int64
	RSSKB    int64
	Kind     string
}

// daemonPIDs lists the daemon processes krit records for a run: PID files in
// the helper registry (oracle and checker JVMs) and the krit daemon's
// <work>/.krit/daemon.pid, plus any `krit serve --root <work>` found under
// procRoot, since an auto-spawned daemon writes no pidfile.
func daemonPIDs(procRoot, registry, work, bin string) []int {
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
	for _, p := range findServeDaemons(procRoot, work, bin) {
		seen[p.PID] = true
		pids = append(pids, p.PID)
	}
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
		s := procSample{CPUTicks: ticks, Start: statField(string(stat), 22)}
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

// statField returns 1-based field n of a /proc/<pid>/stat line (n >= 3).
func statField(stat string, n int) string {
	end := strings.LastIndexByte(stat, ')')
	if end < 0 {
		return ""
	}
	f := strings.Fields(stat[end+1:])
	if i := n - 3; i >= 0 && i < len(f) {
		return f[i]
	}
	return ""
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
	args := strings.Split(strings.TrimRight(cmdline, "\x00"), "\x00")
	for i, a := range args {
		if a == "-jar" && i+1 < len(args) {
			jar := filepath.Base(args[i+1])
			switch {
			case strings.HasPrefix(jar, "krit-fir"):
				return "krit-fir"
			case strings.HasPrefix(jar, "krit-types"):
				return "krit-types"
			}
			return "jvm-helper"
		}
	}
	if len(args) > 1 && args[1] == "serve" {
		return "krit-serve"
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
		prev, seen := before[pid]
		ticks := a.CPUTicks - prev.CPUTicks
		if !seen || prev.Start != a.Start || ticks < 0 { // new process, or a reused PID
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

// procRef identifies one process: PID plus starttime guards against reuse.
type procRef struct {
	PID   int
	Start string
	Cmd   string
}

// findServeDaemons returns the processes under procRoot that are this
// harness's `krit serve --root <work>` daemons: owned by the current user,
// running the krit binary bin (by exe link, or by name where exe is
// unreadable), with args[1] == "serve", and not zombies. work is compared
// both as given and with symlinks resolved.
func findServeDaemons(procRoot, work, bin string) []procRef {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	roots := map[string]bool{work: true}
	if resolved, err := filepath.EvalSymlinks(work); err == nil {
		roots[resolved] = true
	}
	var out []procRef
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		if ref, ok := serveProc(procRoot, pid, roots, bin); ok {
			out = append(out, ref)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

func serveProc(procRoot string, pid int, roots map[string]bool, bin string) (procRef, bool) {
	dir := filepath.Join(procRoot, strconv.Itoa(pid))
	info, err := os.Stat(dir)
	if err != nil {
		return procRef{}, false
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return procRef{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, "cmdline"))
	if err != nil {
		return procRef{}, false
	}
	cmd := string(data)
	args := strings.Split(strings.TrimRight(cmd, "\x00"), "\x00")
	if len(args) < 2 || args[1] != "serve" || !isServeRoot(args, roots) || !sameBinary(dir, args[0], bin) {
		return procRef{}, false
	}
	ref := procRef{PID: pid, Cmd: cmd}
	if stat, err := os.ReadFile(filepath.Join(dir, "stat")); err == nil {
		if statField(string(stat), 3) == "Z" {
			return procRef{}, false
		}
		ref.Start = statField(string(stat), 22)
	}
	return ref, true
}

// sameBinary reports whether the process runs bin: by its exe link when it is
// readable, else by the basename of its first argument.
func sameBinary(procDir, arg0, bin string) bool {
	if bin == "" {
		return true
	}
	if exe, err := os.Readlink(filepath.Join(procDir, "exe")); err == nil {
		if resolved, err := filepath.EvalSymlinks(bin); err == nil && exe == resolved {
			return true
		}
		return filepath.Base(exe) == filepath.Base(bin)
	}
	return filepath.Base(arg0) == filepath.Base(bin)
}

func isServeRoot(args []string, roots map[string]bool) bool {
	for i, a := range args {
		switch {
		case a == "--root" && i+1 < len(args):
			return roots[args[i+1]]
		case strings.HasPrefix(a, "--root="):
			return roots[strings.TrimPrefix(a, "--root=")]
		}
	}
	return false
}

// alive reports whether ref is still the same running, non-zombie process.
func alive(procRoot string, ref procRef) bool {
	stat, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(ref.PID), "stat"))
	if err != nil {
		return false
	}
	if statField(string(stat), 3) == "Z" {
		return false
	}
	return ref.Start == "" || statField(string(stat), 22) == ref.Start
}

// killProcs sends SIGTERM, waits up to grace for each process to exit, then
// SIGKILLs those still alive and still the same process (same starttime).
func killProcs(procRoot string, refs []procRef, grace time.Duration) {
	for _, ref := range refs {
		if alive(procRoot, ref) {
			_ = syscall.Kill(ref.PID, syscall.SIGTERM)
		}
	}
	deadline := time.Now().Add(grace)
	for _, ref := range refs {
		for time.Now().Before(deadline) && alive(procRoot, ref) {
			time.Sleep(50 * time.Millisecond)
		}
		if alive(procRoot, ref) {
			_ = syscall.Kill(ref.PID, syscall.SIGKILL)
		}
	}
}

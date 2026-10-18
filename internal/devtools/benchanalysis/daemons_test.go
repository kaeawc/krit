//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func writeProc(t *testing.T, root string, pid int, utime, stime, hwm, rss int, cmd string) {
	t.Helper()
	dir := filepath.Join(root, strconv.Itoa(pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// comm contains a space and ')' to exercise the parser.
	stat := strconv.Itoa(pid) + " (java (x)) S 1 1 1 0 -1 4194304 10 0 0 0 " +
		strconv.Itoa(utime) + " " + strconv.Itoa(stime) + " 0 0 20 0 5 0 100 1000 1000\n"
	status := "Name:\tjava\nVmHWM:\t" + strconv.Itoa(hwm) + " kB\nVmRSS:\t" + strconv.Itoa(rss) + " kB\n"
	for name, body := range map[string]string{"stat": stat, "status": status, "cmdline": cmd} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDaemonSamplerFakeProc(t *testing.T) {
	proc := t.TempDir()
	writeProc(t, proc, 100, 200, 50, 1000, 800, "java\x00-jar\x00krit-fir.jar\x00")
	before := sampleDaemons(proc, []int{100, 999})
	if len(before) != 1 || before[100].CPUTicks != 250 || before[100].Kind != "krit-fir" {
		t.Fatalf("before = %+v", before)
	}
	writeProc(t, proc, 100, 300, 70, 2048, 1500, "java\x00-jar\x00krit-fir.jar\x00")
	writeProc(t, proc, 101, 10, 0, 500, 400, "krit\x00serve\x00")
	after := sampleDaemons(proc, []int{100, 101})
	u := daemonUsage(before, after)
	// pid 100: 370-250 = 120 ticks = 1200ms; pid 101: 10 ticks = 100ms.
	if u.CPUMs != 1300 {
		t.Errorf("CPUMs = %d, want 1300", u.CPUMs)
	}
	if u.PeakRSSKB != 2048+500 || u.RSSKB != 1900 {
		t.Errorf("rss = %d/%d", u.PeakRSSKB, u.RSSKB)
	}
	if len(u.PIDs) != 2 || len(u.Kinds) != 2 {
		t.Errorf("usage = %+v", u)
	}
}

func TestDaemonSamplerDegradesWithoutProc(t *testing.T) {
	got := sampleDaemons(filepath.Join(t.TempDir(), "nope"), []int{1, 2})
	if len(got) != 0 {
		t.Fatalf("sample = %v", got)
	}
	if u := daemonUsage(nil, got); u.CPUMs != 0 || u.PeakRSSKB != 0 {
		t.Fatalf("usage = %+v", u)
	}
}

func TestDaemonPIDsFromRegistryAndKritDaemon(t *testing.T) {
	reg, work := t.TempDir(), t.TempDir()
	_ = os.MkdirAll(filepath.Join(work, ".krit"), 0o755)
	_ = os.WriteFile(filepath.Join(reg, "abc.pid"), []byte("4242\n"), 0o644)
	_ = os.WriteFile(filepath.Join(reg, "abc.port"), []byte("1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(reg, "bad.pid"), []byte("zzz"), 0o644)
	_ = os.WriteFile(filepath.Join(work, ".krit", "daemon.pid"), []byte("77 extra"), 0o644)
	pids := daemonPIDs(filepath.Join(t.TempDir(), "noproc"), reg, work, "")
	if len(pids) != 2 || pids[0] != 77 || pids[1] != 4242 {
		t.Fatalf("pids = %v", pids)
	}
}

func TestSocketPathWarning(t *testing.T) {
	if w := socketPathWarning("/tmp/kb/corpus"); w != "" {
		t.Errorf("short path warned: %s", w)
	}
	long := "/" + strings.Repeat("a", 120)
	if w := socketPathWarning(long); !strings.Contains(w, "daemon.sock") {
		t.Errorf("long path: %q", w)
	}
}

func TestFindServeDaemonsMatchesRootExactly(t *testing.T) {
	proc := t.TempDir()
	for pid, cmd := range map[string]string{
		"11": "/bin/krit\x00serve\x00--root\x00/w/a\x00",
		"12": "/bin/krit\x00serve\x00--root\x00/w/b\x00",
		"13": "/bin/krit\x00-f\x00json\x00--root\x00/w/a\x00",
		"14": "/bin/krit\x00serve\x00--root=/w/a\x00",
		"15": "/bin/krit\x00serve\x00--root\x00/w/ab\x00",
		"16": "/bin/krit\x00serve\x00--root\x00/w/a/\x00",
		"17": "/bin/other\x00serve\x00--root\x00/w/a\x00",
	} {
		_ = os.MkdirAll(filepath.Join(proc, pid), 0o755)
		_ = os.WriteFile(filepath.Join(proc, pid, "cmdline"), []byte(cmd), 0o644)
	}
	var got []int
	for _, p := range findServeDaemons(proc, "/w/a", "/x/krit") {
		got = append(got, p.PID)
	}
	if len(got) != 2 || got[0] != 11 || got[1] != 14 {
		t.Fatalf("got %v", got)
	}
}

func TestFindServeDaemonsSkipsForeignUIDAndZombies(t *testing.T) {
	proc := t.TempDir()
	mk := func(pid, state string) {
		dir := filepath.Join(proc, pid)
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "cmdline"), []byte("/x/krit\x00serve\x00--root\x00/w/a\x00"), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "stat"), []byte(pid+" (krit) "+state+" 1 1 1 0 -1 0 0 0 0 0 1 1 0 0 20 0 5 0 777 1 1\n"), 0o644)
	}
	mk("21", "S")
	mk("22", "Z")
	mk("23", "S")
	if err := os.Chown(filepath.Join(proc, "23"), os.Getuid()+12345, -1); err != nil {
		t.Skip("cannot chown to a foreign uid:", err)
	}
	got := findServeDaemons(proc, "/w/a", "/x/krit")
	if len(got) != 1 || got[0].PID != 21 || got[0].Start != "777" {
		t.Fatalf("got %+v", got)
	}
}

func TestSamplerDeltaKeyedOnStartTime(t *testing.T) {
	before := map[int]procSample{7: {CPUTicks: 1000, Start: "1"}}
	after := map[int]procSample{7: {CPUTicks: 50, Start: "2"}} // PID reused by a new process
	if u := daemonUsage(before, after); u.CPUMs != 500 {
		t.Fatalf("CPUMs = %d, want 500", u.CPUMs)
	}
}

func TestDaemonKindClassifiesByJarArgument(t *testing.T) {
	cases := map[string]string{
		"java\x00-jar\x00/j/krit-fir.jar\x00--daemon\x00":     "krit-fir",
		"java\x00-jar\x00/j/krit-types.jar\x00":               "krit-types",
		"java\x00-jar\x00/j/other.jar\x00--note=krit-fir\x00": "jvm-helper",
		"/bin/krit\x00serve\x00--root\x00/w/krit-fir\x00":     "krit-serve",
		"/bin/sh\x00-c\x00echo krit-fir\x00":                  "other",
	}
	for cmd, want := range cases {
		if got := daemonKind(cmd); got != want {
			t.Errorf("daemonKind(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestKillProcsStopsProcess(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Skip(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("no /proc")
	}
	start := ""
	if stat, err := os.ReadFile("/proc/" + strconv.Itoa(cmd.Process.Pid) + "/stat"); err == nil {
		start = statField(string(stat), 22)
	}
	killProcs("/proc", []procRef{{PID: cmd.Process.Pid, Start: start}}, 2*time.Second)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("process still running")
	}
}

func TestDaemonPIDsIncludesAutoSpawnedServeWithoutPidfile(t *testing.T) {
	proc, reg, work := t.TempDir(), t.TempDir(), t.TempDir()
	for pid, cmd := range map[string]string{
		"500": "/x/krit\x00serve\x00--root\x00" + work + "\x00",
		"501": "/x/krit\x00serve\x00--root\x00/elsewhere\x00",
	} {
		_ = os.MkdirAll(filepath.Join(proc, pid), 0o755)
		_ = os.WriteFile(filepath.Join(proc, pid, "cmdline"), []byte(cmd), 0o644)
	}
	// The same PID also appears in a pidfile: it must be listed once.
	_ = os.MkdirAll(filepath.Join(work, ".krit"), 0o755)
	_ = os.WriteFile(filepath.Join(work, ".krit", "daemon.pid"), []byte("500\n"), 0o644)
	_ = os.WriteFile(filepath.Join(reg, "x.pid"), []byte("4242\n"), 0o644)
	got := daemonPIDs(proc, reg, work, "/x/krit")
	if len(got) != 2 || got[0] != 500 || got[1] != 4242 {
		t.Fatalf("pids = %v", got)
	}
}

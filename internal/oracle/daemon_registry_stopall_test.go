package oracle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// exitedPID returns the PID of a process that has already exited.
func exitedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func TestParseDaemonPIDFileNameInvertsDaemonPIDFileName(t *testing.T) {
	for _, slot := range []int{0, 1, 7} {
		hash, got, ok := parseDaemonPIDFileName(daemonPIDFileName("0123abcd", slot))
		if !ok || hash != "0123abcd" || got != slot {
			t.Fatalf("slot %d: parsed (%q, %d, %v)", slot, hash, got, ok)
		}
	}
	for _, name := range []string{".pid", "x.y.pid", "x.0.pid", "x.-1.pid", "x.port"} {
		if _, _, ok := parseDaemonPIDFileName(name); ok {
			t.Fatalf("%q parsed as a daemon PID file", name)
		}
	}
}

// krit daemon stop --all (#974): stale oracle records are cleared, and the
// krit-fir records that share the directory are left to firchecks.
func TestStopRegisteredDaemonsClearsStaleRecords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", dir)
	pid := strconv.Itoa(exitedPID(t))
	for _, name := range []string{"aaaa.pid", "bbbb.2.pid", "cccc.krit-fir.pid"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(pid+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stopped, left := StopRegisteredDaemons(false)
	if stopped != 2 || left != 0 {
		t.Fatalf("stopped=%d left=%d, want 2 and 0", stopped, left)
	}
	for _, name := range []string{"aaaa.pid", "bbbb.2.pid"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still present (err=%v)", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "cccc.krit-fir.pid")); err != nil {
		t.Fatalf("krit-fir record must be left alone: %v", err)
	}
}

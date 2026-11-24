package firchecks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// krit daemon stop --all (#974): a krit-fir record naming a process that
// has exited is cleared without signalling anything; oracle records in the
// same directory are not touched.
func TestStopRegisteredDaemonsClearsStaleRecords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KRIT_DAEMON_REGISTRY_DIR", dir)
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	for _, name := range []string{"role@0123abcd.krit-fir.pid", "role@0123abcd.krit-fir.port", "aaaa.pid"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(pid+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stopped, left := StopRegisteredDaemons(false)
	if stopped != 1 || left != 0 {
		t.Fatalf("stopped=%d left=%d, want 1 and 0", stopped, left)
	}
	for _, name := range []string{"role@0123abcd.krit-fir.pid", "role@0123abcd.krit-fir.port"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still present (err=%v)", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "aaaa.pid")); err != nil {
		t.Fatalf("oracle record must be left alone: %v", err)
	}
}

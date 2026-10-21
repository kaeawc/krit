package daemoncmd

import (
	"os"
	"os/exec"
	"testing"
)

func TestRunStopRefusesUnrelatedProcess(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start victim: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	if err := writePIDFile(pidFilePath(dir), cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if code := runStop([]string{"--repo", dir, "--timeout", "100ms"}); code != 1 {
		t.Fatalf("stop on foreign pid: want 1, got %d", code)
	}
	if !processAlive(cmd.Process.Pid) {
		t.Fatal("unrelated process was signalled")
	}
	if _, err := os.Stat(pidFilePath(dir)); err != nil {
		t.Fatalf("pid file should be left for the user to remove: %v", err)
	}
}

func TestArgvIsDaemonFor(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	cases := []struct {
		name string
		argv []string
		want bool
	}{
		{"daemon", []string{"/x/krit-daemon", "--repo", root}, true},
		{"serve", []string{"/x/krit", "serve", "--root", root, "--socket", "s"}, true},
		{"serve equals", []string{"krit", "serve", "--root=" + root}, true},
		{"other root", []string{"krit-daemon", "--repo", other}, false},
		{"relative root", []string{"krit-daemon", "--repo", "."}, false},
		{"wrong binary", []string{"sleep", "--repo", root}, false},
		{"serve wrong flag", []string{"krit", "serve", "--repo", root}, false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := argvIsDaemonFor(c.argv, root); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

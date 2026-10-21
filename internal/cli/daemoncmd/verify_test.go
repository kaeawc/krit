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
		{"single dash", []string{"krit-daemon", "-repo", root}, true},
		{"single dash equals", []string{"krit-daemon", "-repo=" + root}, true},
		{"value flags before", []string{"krit-daemon", "--socket", "s", "--repo", root, "-v"}, true},
		{"repeated last wins other", []string{"krit-daemon", "--repo", root, "--repo", other}, false},
		{"repeated last wins this", []string{"krit-daemon", "--repo", other, "--repo", root}, true},
		{"serve repeated last other", []string{"krit", "serve", "--root", root, "--root=" + other}, false},
		{"after double dash", []string{"krit-daemon", "--", "--repo", root}, false},
		{"after positional", []string{"krit-daemon", "extra", "--repo", root}, false},
		{"positional after root", []string{"krit-daemon", "--repo", root, "extra"}, false},
		{"serve after positional", []string{"krit", "serve", "x", "--root", root}, false},
		{"root as socket value", []string{"krit-daemon", "--socket", "--repo", "--repo", other}, false},
		{"unknown flag", []string{"krit-daemon", "--repo", root, "--bogus"}, false},
		{"serve unknown flag", []string{"krit", "serve", "--root", root, "--bogus", "x"}, false},
		{"serve stop", []string{"krit", "serve", "--root", root, "--stop"}, false},
		{"serve default root", []string{"krit", "serve"}, false},
	}
	for _, c := range cases {
		if got := argvIsDaemonFor(c.argv, root); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

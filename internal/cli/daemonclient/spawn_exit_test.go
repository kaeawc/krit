package daemonclient

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A spawned daemon that exits before binding its socket must fail
// EnsureRunning promptly, not after the full WaitTimeout.
func TestEnsureRunning_FailsFastWhenDaemonExits(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "krit-spawn-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	bin := filepath.Join(dir, "fake-krit")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	c, err := EnsureRunning(dir, SpawnOptions{Binary: bin, WaitTimeout: 20 * time.Second})
	if err == nil {
		t.Fatalf("expected error, got client %v", c)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("EnsureRunning waited %s after the daemon exited; want fail-fast (err: %v)", elapsed, err)
	}
}

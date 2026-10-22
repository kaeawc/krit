package serve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunUnsupportedExitsBeforeLockOrListen(t *testing.T) {
	orig := daemonSupported
	daemonSupported = func() bool { return false }
	t.Cleanup(func() { daemonSupported = orig })
	root := t.TempDir()
	sock := filepath.Join(root, "d.sock")

	if code := Run([]string{"--root", root, "--socket", sock}); code != 1 {
		t.Fatalf("Run = %d, want 1", code)
	}
	if _, err := os.Stat(sock); err == nil {
		t.Fatal("socket was created on an unsupported platform")
	}
	if _, err := os.Stat(filepath.Join(root, ".krit")); err == nil {
		t.Fatal(".krit was created before the unsupported check")
	}
}

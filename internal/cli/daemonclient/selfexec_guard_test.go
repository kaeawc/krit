package daemonclient

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/selfexec"
)

// TestEnsureRunningRefusesToSpawnTestBinary guards against the daemon
// autostart re-launching `<pkg>.test serve --root ...`. A test binary
// ignores `serve`, reruns its suite, and reaches the spawn again.
func TestEnsureRunningRefusesToSpawnTestBinary(t *testing.T) {
	root := t.TempDir()
	_, err := EnsureRunning(root, SpawnOptions{
		Env:         append(os.Environ(), spawnedTestChildEnv+"=1"),
		WaitTimeout: 200 * time.Millisecond,
	})
	if !errors.Is(err, selfexec.ErrTestBinary) {
		t.Fatalf("EnsureRunning with default binary = %v, want ErrTestBinary", err)
	}
}

func TestEnsureRunningRefusesExplicitTestBinary(t *testing.T) {
	root := t.TempDir()
	_, err := EnsureRunning(root, SpawnOptions{
		Binary:      filepath.Join(t.TempDir(), "scan.test"),
		WaitTimeout: 200 * time.Millisecond,
	})
	if !errors.Is(err, selfexec.ErrTestBinary) {
		t.Fatalf("EnsureRunning with explicit test binary = %v, want ErrTestBinary", err)
	}
}

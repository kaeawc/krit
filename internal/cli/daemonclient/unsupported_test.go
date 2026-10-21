package daemonclient

import (
	"errors"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

func TestEnsureUnsupportedReturnsImmediately(t *testing.T) {
	orig := daemonSupported
	daemonSupported = func() bool { return false }
	t.Cleanup(func() { daemonSupported = orig })
	root := t.TempDir()

	c, err := EnsureRunning(root, SpawnOptions{Binary: "/nonexistent/krit"})
	if c != nil || !errors.Is(err, daemon.ErrUnsupported) {
		t.Fatalf("EnsureRunning = (%v, %v), want (nil, ErrUnsupported)", c, err)
	}
	c, ok, err := EnsureCompatible(root, SpawnOptions{Binary: "/nonexistent/krit", AutoRestart: true})
	if c != nil || ok || err != nil {
		t.Fatalf("EnsureCompatible = (%v, %v, %v), want (nil, false, nil)", c, ok, err)
	}
}

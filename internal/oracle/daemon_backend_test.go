package oracle

import "testing"

func TestDaemonBackendIsIndependentOfJarName(t *testing.T) {
	cfg := resolveDaemonOptions([]DaemonOption{WithBackend(BackendFIR)})
	d := &Daemon{sourcesHash: "custom-abc-123@id", backend: cfg.backend}
	if got := d.Backend(); got != BackendFIR {
		t.Fatalf("Backend = %q, want fir for a custom-named jar", got)
	}
	if got := (&Daemon{sourcesHash: "krit-fir-abc"}).Backend(); got != "" {
		t.Fatalf("backend guessed from the jar name: %q", got)
	}
}

func TestDaemonBackendNilSafe(t *testing.T) {
	var d *Daemon
	if got := d.Backend(); got != "" {
		t.Fatalf("nil daemon backend = %q", got)
	}
}

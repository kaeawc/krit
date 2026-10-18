package oracle

import "testing"

func TestDaemonBackendFromRegistryKey(t *testing.T) {
	cases := map[string]Backend{
		"krit-fir-abc-123@id":   BackendFIR,
		"krit-types-abc-123@id": BackendKAA,
		"":                      "",
		"other-jar":             "",
	}
	for key, want := range cases {
		if got := (&Daemon{sourcesHash: key}).Backend(); got != want {
			t.Errorf("Backend(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestDaemonBackendNilSafe(t *testing.T) {
	var d *Daemon
	if got := d.Backend(); got != "" {
		t.Fatalf("nil daemon backend = %q", got)
	}
}

package scan

import "testing"

func TestDaemonServedPerfMarkerText(t *testing.T) {
	// The benchmark harness matches this text; keep them in sync.
	if DaemonServedPerfMarker != "perf: served by krit daemon" {
		t.Fatalf("marker changed: %q", DaemonServedPerfMarker)
	}
}
